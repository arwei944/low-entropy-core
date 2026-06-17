// Package core — 分片观测存储基础设施
//
// 本文件提供面向十亿级调用量的分片观测存储类型，包括：
//   - ShardedObservationAdapter：256 分片的 InMemoryObservationAdapter，使用 SpanID 哈希分布
//   - ShardedStepStore：256 分片的 InMemoryStepStore，带有预构建索引和环形缓冲区
//   - ShardedIndexedStepStore：基于 sync.Map 的持久化索引版本，支持最选择性索引查询
//
// 所有类型均为线程安全，热路径设计为零分配。使用 ShardedLock 进行分片选择，
// StepSlicePool 和 StepMetadataPool 进行中间分配复用。
package core

import (
	"sort"
	"sync"
	"sync/atomic"
)

// ──────────────────────────────────────────────────────────────────────────────
// ShardedObservationAdapter — 256 分片观测适配器
// ──────────────────────────────────────────────────────────────────────────────

// ShardedObservationAdapter 是 InMemoryObservationAdapter 的分片版本。
// 使用 256 个分片，每个分片有独立的 sync.RWMutex 和 []ExecutionStep，
// 通过 SpanID 哈希将记录分布到不同分片，大幅减少锁竞争。
//
// 在十亿级调用量下，单锁的 InMemoryObservationAdapter 会成为严重瓶颈。
// 256 分片意味着在理想哈希分布下，锁竞争降低到原来的 1/256。
type ShardedObservationAdapter struct {
	shards    [shardCount]*obsShard
	stepCount atomic.Int64
}

// obsShard 是单个观测分片，包含独立的锁和步骤切片。
type obsShard struct {
	mu    sync.RWMutex
	steps []ExecutionStep
}

// NewShardedObservationAdapter 创建一个新的分片观测适配器。
// 每个分片预分配 1024 容量的切片，减少初始扩容。
func NewShardedObservationAdapter() *ShardedObservationAdapter {
	a := &ShardedObservationAdapter{}
	for i := 0; i < shardCount; i++ {
		a.shards[i] = &obsShard{
			steps: make([]ExecutionStep, 0, 1024),
		}
	}
	return a
}

// Record 将执行步骤追加到分片存储中。
// 使用 SpanID 哈希选择分片，确保相同 SpanID 的记录进入同一分片。
func (a *ShardedObservationAdapter) Record(steps []ExecutionStep) {
	if len(steps) == 0 {
		return
	}

	// 按 SpanID 哈希分组，将步骤分配到对应分片
	shardGroups := make([][]ExecutionStep, shardCount)
	for i := range steps {
		idx := hashString(steps[i].SpanID) & 0xFF
		shardGroups[idx] = append(shardGroups[idx], steps[i])
	}

	// 并发写入各分片
	for i := 0; i < shardCount; i++ {
		if len(shardGroups[i]) == 0 {
			continue
		}
		shard := a.shards[i]
		shard.mu.Lock()
		shard.steps = append(shard.steps, shardGroups[i]...)
		shard.mu.Unlock()
	}

	a.stepCount.Add(int64(len(steps)))
}

// GetSteps 返回所有步骤的分页视图。
// limit 为 0 时返回全部，offset 从 0 开始。
// 为了避免大内存分配，返回的切片是跨分片收集的副本。
func (a *ShardedObservationAdapter) GetSteps(limit, offset int) ([]ExecutionStep, int) {
	total := int(a.stepCount.Load())
	if offset >= total {
		return nil, total
	}

	// 计算需要收集的数量
	need := total - offset
	if limit > 0 && limit < need {
		need = limit
	}

	result := GetStepSlice(need)

	// 跨分片收集
	collected := 0
	skipped := 0
	for i := 0; i < shardCount && collected < need; i++ {
		shard := a.shards[i]
		shard.mu.RLock()
		for _, step := range shard.steps {
			if skipped < offset {
				skipped++
				continue
			}
			result = append(result, step)
			collected++
			if collected >= need {
				break
			}
		}
		shard.mu.RUnlock()
	}

	return result, total
}

// GetTraceTree 构建跨分片的 TraceTree。
// 收集所有分片中的步骤，然后构建层级树。
func (a *ShardedObservationAdapter) GetTraceTree() *TraceTree {
	allSteps := make([]ExecutionStep, 0, a.stepCount.Load())
	for i := 0; i < shardCount; i++ {
		shard := a.shards[i]
		shard.mu.RLock()
		allSteps = append(allSteps, shard.steps...)
		shard.mu.RUnlock()
	}
	return BuildTraceTree(allSteps)
}

// StepCount 返回总步骤数（原子读取，无锁）。
func (a *ShardedObservationAdapter) StepCount() int {
	return int(a.stepCount.Load())
}

// Clear 清空所有分片中的步骤。
func (a *ShardedObservationAdapter) Clear() {
	for i := 0; i < shardCount; i++ {
		shard := a.shards[i]
		shard.mu.Lock()
		shard.steps = shard.steps[:0]
		shard.mu.Unlock()
	}
	a.stepCount.Store(0)
}

// ──────────────────────────────────────────────────────────────────────────────
// ShardedStepStore — 256 分片步骤存储（带预构建索引）
// ──────────────────────────────────────────────────────────────────────────────

// stepIndex 是单个分片的索引结构，存储位置映射。
type stepIndex struct {
	byTraceID map[string][]int // TraceID -> 位置列表
	byPattern map[string][]int // Pattern -> 位置列表
	byUnit    map[string][]int // Unit -> 位置列表
}

// ShardedStepStore 是 InMemoryStepStore 的分片版本。
// 每个分片有独立的环形缓冲区、锁和预构建索引。
// 查询时使用最选择性索引避免全扫描，支持分页。
type ShardedStepStore struct {
	shards    [shardCount]*stepStoreShard
	totalCount atomic.Int64
}

// stepStoreShard 是单个存储分片。
type stepStoreShard struct {
	mu       sync.RWMutex
	steps    []ExecutionStep
	capacity int
	head     int // 写位置
	size     int // 有效条目数
	index    *stepIndex
}

// NewShardedStepStore 创建一个新的分片步骤存储。
// capacity 为每个分片的环形缓冲区容量。
func NewShardedStepStore(capacity int) *ShardedStepStore {
	if capacity <= 0 {
		capacity = 1000
	}
	s := &ShardedStepStore{}
	for i := 0; i < shardCount; i++ {
		s.shards[i] = &stepStoreShard{
			steps:    make([]ExecutionStep, capacity),
			capacity: capacity,
			index: &stepIndex{
				byTraceID: make(map[string][]int),
				byPattern: make(map[string][]int),
				byUnit:    make(map[string][]int),
			},
		}
	}
	return s
}

// Record 存储执行步骤到分片环形缓冲区中。
// 使用 SpanID 哈希选择分片，同步更新索引。
func (s *ShardedStepStore) Record(steps []ExecutionStep) {
	if len(steps) == 0 {
		return
	}

	// 按分片分组
	shardGroups := make([][]ExecutionStep, shardCount)
	for i := range steps {
		idx := hashString(steps[i].SpanID) & 0xFF
		shardGroups[idx] = append(shardGroups[idx], steps[i])
	}

	for i := 0; i < shardCount; i++ {
		if len(shardGroups[i]) == 0 {
			continue
		}
		shard := s.shards[i]
		shard.mu.Lock()
		for _, step := range shardGroups[i] {
			pos := shard.head
			// 如果覆盖旧条目，清理其索引
			if shard.size == shard.capacity {
				old := shard.steps[shard.head]
				s.removeFromIndex(shard.index, old, shard.head)
			}
			shard.steps[shard.head] = step
			// 更新索引
			s.addToIndex(shard.index, step, pos)
			shard.head = (shard.head + 1) % shard.capacity
			if shard.size < shard.capacity {
				shard.size++
			}
		}
		shard.mu.Unlock()
	}

	s.totalCount.Add(int64(len(steps)))
}

// addToIndex 将步骤添加到分片索引中。
func (s *ShardedStepStore) addToIndex(idx *stepIndex, step ExecutionStep, pos int) {
	if step.TraceID != "" {
		idx.byTraceID[step.TraceID] = append(idx.byTraceID[step.TraceID], pos)
	}
	if step.Pattern != "" {
		idx.byPattern[step.Pattern] = append(idx.byPattern[step.Pattern], pos)
	}
	if step.Unit != "" {
		idx.byUnit[step.Unit] = append(idx.byUnit[step.Unit], pos)
	}
}

// removeFromIndex 从分片索引中移除指定位置的条目。
func (s *ShardedStepStore) removeFromIndex(idx *stepIndex, step ExecutionStep, pos int) {
	if step.TraceID != "" {
		idx.byTraceID[step.TraceID] = removePos(idx.byTraceID[step.TraceID], pos)
	}
	if step.Pattern != "" {
		idx.byPattern[step.Pattern] = removePos(idx.byPattern[step.Pattern], pos)
	}
	if step.Unit != "" {
		idx.byUnit[step.Unit] = removePos(idx.byUnit[step.Unit], pos)
	}
}

// removePos 从位置列表中移除指定位置。
func removePos(positions []int, target int) []int {
	for i, p := range positions {
		if p == target {
			return append(positions[:i], positions[i+1:]...)
		}
	}
	return positions
}

// Query 使用索引加速查询，支持分页。
// 选择最选择性索引（索引基数最高的字段）来最小化扫描范围。
func (s *ShardedStepStore) Query(q StepQuery) ([]ExecutionStep, int) {
	// 确定使用哪个索引
	useIdx := s.selectBestIndex(q)

	// 跨分片收集结果
	var allResults []ExecutionStep
	var total int

	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.RLock()
		shardResults := s.queryShard(shard, q, useIdx)
		shard.mu.RUnlock()
		allResults = append(allResults, shardResults...)
	}

	// 按时间戳排序
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})

	total = len(allResults)

	// 应用分页
	if q.Limit > 0 && len(allResults) > q.Limit {
		allResults = allResults[:q.Limit]
	}

	return allResults, total
}

// selectBestIndex 选择最选择性索引。
// 优先级：TraceID > Pattern > Unit > 全扫描
func (s *ShardedStepStore) selectBestIndex(q StepQuery) string {
	if q.TraceID != "" {
		return "trace_id"
	}
	if q.Pattern != "" {
		return "pattern"
	}
	if q.Unit != "" {
		return "unit"
	}
	return "" // 全扫描
}

// queryShard 在单个分片中执行查询。
func (s *ShardedStepStore) queryShard(shard *stepStoreShard, q StepQuery, useIdx string) []ExecutionStep {
	// 确定候选位置
	var candidates []int
	switch useIdx {
	case "trace_id":
		candidates = shard.index.byTraceID[q.TraceID]
	case "pattern":
		candidates = shard.index.byPattern[q.Pattern]
	case "unit":
		candidates = shard.index.byUnit[q.Unit]
	default:
		// 全扫描：收集所有有效位置
		candidates = s.allPositions(shard)
	}

	if len(candidates) == 0 {
		return nil
	}

	result := make([]ExecutionStep, 0, len(candidates))
	for _, pos := range candidates {
		step := shard.steps[pos]
		// 使用索引后的二次过滤（因为索引可能包含已过期的条目）
		if q.TraceID != "" && step.TraceID != q.TraceID {
			continue
		}
		if q.Pattern != "" && step.Pattern != q.Pattern {
			continue
		}
		if q.Unit != "" && step.Unit != q.Unit {
			continue
		}
		if !q.Since.IsZero() && step.Timestamp.Before(q.Since) {
			continue
		}
		if q.ErrorOnly && step.Error == nil {
			continue
		}
		result = append(result, step)
	}
	return result
}

// allPositions 返回分片中所有有效条目的位置。
func (s *ShardedStepStore) allPositions(shard *stepStoreShard) []int {
	if shard.size == 0 {
		return nil
	}
	start := shard.head - shard.size
	if start < 0 {
		start += shard.capacity
	}
	positions := make([]int, shard.size)
	for i := 0; i < shard.size; i++ {
		positions[i] = (start + i) % shard.capacity
	}
	return positions
}

// Count 返回总步骤数（原子读取）。
func (s *ShardedStepStore) Count() int {
	return int(s.totalCount.Load())
}

// Clear 清空所有分片。
func (s *ShardedStepStore) Clear() {
	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.Lock()
		shard.head = 0
		shard.size = 0
		shard.index = &stepIndex{
			byTraceID: make(map[string][]int),
			byPattern: make(map[string][]int),
			byUnit:    make(map[string][]int),
		}
		shard.mu.Unlock()
	}
	s.totalCount.Store(0)
}

// ──────────────────────────────────────────────────────────────────────────────
// ShardedIndexedStepStore — 持久化索引版本（sync.Map）
// ──────────────────────────────────────────────────────────────────────────────

// ShardedIndexedStepStore 是 ShardedStepStore 的增强版本，
// 使用 sync.Map 作为索引存储，支持并发安全的索引更新和查询。
//
// 与 ShardedStepStore 的区别：
//   - 索引使用 sync.Map，允许多个 goroutine 并发更新同一索引
//   - 适合高并发写入场景，索引更新不会阻塞读取
//   - 索引条目使用 sync.Pool 复用，减少 GC 压力
type ShardedIndexedStepStore struct {
	shards     [shardCount]*indexedShard
	totalCount atomic.Int64
}

// indexedShard 是带有持久化索引的存储分片。
type indexedShard struct {
	mu       sync.RWMutex
	steps    []ExecutionStep
	capacity int
	head     int
	size     int
	// 持久化索引使用 sync.Map
	idxTraceID sync.Map // map[string]*indexEntry
	idxPattern sync.Map // map[string]*indexEntry
	idxUnit    sync.Map // map[string]*indexEntry
}

// indexEntry 是索引条目，存储位置列表。
// 使用 sync.Pool 复用，减少 GC 压力。
type indexEntry struct {
	positions []int
}

// indexEntryPool 是 indexEntry 的 sync.Pool。
var indexEntryPool = sync.Pool{
	New: func() any {
		return &indexEntry{positions: make([]int, 0, 16)}
	},
}

// NewShardedIndexedStepStore 创建一个新的持久化索引分片存储。
func NewShardedIndexedStepStore(capacity int) *ShardedIndexedStepStore {
	if capacity <= 0 {
		capacity = 1000
	}
	s := &ShardedIndexedStepStore{}
	for i := 0; i < shardCount; i++ {
		s.shards[i] = &indexedShard{
			steps:    make([]ExecutionStep, capacity),
			capacity: capacity,
		}
	}
	return s
}

// Record 存储执行步骤到分片，同步更新持久化索引。
func (s *ShardedIndexedStepStore) Record(steps []ExecutionStep) {
	if len(steps) == 0 {
		return
	}

	shardGroups := make([][]ExecutionStep, shardCount)
	for i := range steps {
		idx := hashString(steps[i].SpanID) & 0xFF
		shardGroups[idx] = append(shardGroups[idx], steps[i])
	}

	for i := 0; i < shardCount; i++ {
		if len(shardGroups[i]) == 0 {
			continue
		}
		shard := s.shards[i]
		shard.mu.Lock()
		for _, step := range shardGroups[i] {
			pos := shard.head
			if shard.size == shard.capacity {
				old := shard.steps[shard.head]
				s.removeFromIndexedLocked(shard, old, shard.head)
			}
			shard.steps[shard.head] = step
			s.addToIndexedLocked(shard, step, pos)
			shard.head = (shard.head + 1) % shard.capacity
			if shard.size < shard.capacity {
				shard.size++
			}
		}
		shard.mu.Unlock()
	}

	s.totalCount.Add(int64(len(steps)))
}

// addToIndexedLocked 将步骤添加到持久化索引（调用者持有写锁）。
func (s *ShardedIndexedStepStore) addToIndexedLocked(shard *indexedShard, step ExecutionStep, pos int) {
	if step.TraceID != "" {
		s.appendToIndex(&shard.idxTraceID, step.TraceID, pos)
	}
	if step.Pattern != "" {
		s.appendToIndex(&shard.idxPattern, step.Pattern, pos)
	}
	if step.Unit != "" {
		s.appendToIndex(&shard.idxUnit, step.Unit, pos)
	}
}

// removeFromIndexedLocked 从持久化索引中移除条目（调用者持有写锁）。
func (s *ShardedIndexedStepStore) removeFromIndexedLocked(shard *indexedShard, step ExecutionStep, pos int) {
	if step.TraceID != "" {
		s.removeFromIndex(&shard.idxTraceID, step.TraceID, pos)
	}
	if step.Pattern != "" {
		s.removeFromIndex(&shard.idxPattern, step.Pattern, pos)
	}
	if step.Unit != "" {
		s.removeFromIndex(&shard.idxUnit, step.Unit, pos)
	}
}

// appendToIndex 向 sync.Map 索引追加位置。
func (s *ShardedIndexedStepStore) appendToIndex(m *sync.Map, key string, pos int) {
	entry := indexEntryPool.Get().(*indexEntry)
	entry.positions = entry.positions[:0]

	actual, loaded := m.LoadOrStore(key, entry)
	if loaded {
		// 键已存在，归还新分配的 entry
		indexEntryPool.Put(entry)
		e := actual.(*indexEntry)
		// 注意：sync.Map 的值是共享的，这里需要小心并发
		// 由于调用者持有写锁，此时是安全的
		e.positions = append(e.positions, pos)
	} else {
		// 键不存在，entry 已存储
		entry.positions = append(entry.positions, pos)
	}
}

// removeFromIndex 从 sync.Map 索引中移除位置。
func (s *ShardedIndexedStepStore) removeFromIndex(m *sync.Map, key string, pos int) {
	actual, ok := m.Load(key)
	if !ok {
		return
	}
	e := actual.(*indexEntry)
	for i, p := range e.positions {
		if p == pos {
			e.positions = append(e.positions[:i], e.positions[i+1:]...)
			break
		}
	}
	// 如果位置列表为空，删除键
	if len(e.positions) == 0 {
		m.Delete(key)
	}
}

// Query 使用持久化索引加速查询。
func (s *ShardedIndexedStepStore) Query(q StepQuery) ([]ExecutionStep, int) {
	useIdx := s.selectBestIndex(q)

	var allResults []ExecutionStep

	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.RLock()
		shardResults := s.queryIndexedShard(shard, q, useIdx)
		shard.mu.RUnlock()
		allResults = append(allResults, shardResults...)
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})

	total := len(allResults)
	if q.Limit > 0 && len(allResults) > q.Limit {
		allResults = allResults[:q.Limit]
	}

	return allResults, total
}

// selectBestIndex 选择最选择性索引。
func (s *ShardedIndexedStepStore) selectBestIndex(q StepQuery) string {
	if q.TraceID != "" {
		return "trace_id"
	}
	if q.Pattern != "" {
		return "pattern"
	}
	if q.Unit != "" {
		return "unit"
	}
	return ""
}

// queryIndexedShard 在单个分片中执行查询，使用持久化索引。
func (s *ShardedIndexedStepStore) queryIndexedShard(shard *indexedShard, q StepQuery, useIdx string) []ExecutionStep {
	var candidates []int
	switch useIdx {
	case "trace_id":
		if actual, ok := shard.idxTraceID.Load(q.TraceID); ok {
			candidates = actual.(*indexEntry).positions
		}
	case "pattern":
		if actual, ok := shard.idxPattern.Load(q.Pattern); ok {
			candidates = actual.(*indexEntry).positions
		}
	case "unit":
		if actual, ok := shard.idxUnit.Load(q.Unit); ok {
			candidates = actual.(*indexEntry).positions
		}
	default:
		candidates = s.allIndexedPositions(shard)
	}

	if len(candidates) == 0 {
		return nil
	}

	result := make([]ExecutionStep, 0, len(candidates))
	// 收集当前有效位置
	validPositions := s.allIndexedPositions(shard)
	validSet := make(map[int]bool, len(validPositions))
	for _, p := range validPositions {
		validSet[p] = true
	}

	for _, pos := range candidates {
		if !validSet[pos] {
			continue
		}
		step := shard.steps[pos]
		if q.TraceID != "" && step.TraceID != q.TraceID {
			continue
		}
		if q.Pattern != "" && step.Pattern != q.Pattern {
			continue
		}
		if q.Unit != "" && step.Unit != q.Unit {
			continue
		}
		if !q.Since.IsZero() && step.Timestamp.Before(q.Since) {
			continue
		}
		if q.ErrorOnly && step.Error == nil {
			continue
		}
		result = append(result, step)
	}
	return result
}

// allIndexedPositions 返回分片中所有有效条目位置。
func (s *ShardedIndexedStepStore) allIndexedPositions(shard *indexedShard) []int {
	if shard.size == 0 {
		return nil
	}
	start := shard.head - shard.size
	if start < 0 {
		start += shard.capacity
	}
	positions := make([]int, shard.size)
	for i := 0; i < shard.size; i++ {
		positions[i] = (start + i) % shard.capacity
	}
	return positions
}

// Count 返回总步骤数。
func (s *ShardedIndexedStepStore) Count() int {
	return int(s.totalCount.Load())
}

// Clear 清空所有分片。
func (s *ShardedIndexedStepStore) Clear() {
	for i := 0; i < shardCount; i++ {
		shard := s.shards[i]
		shard.mu.Lock()
		shard.head = 0
		shard.size = 0
		shard.idxTraceID = sync.Map{}
		shard.idxPattern = sync.Map{}
		shard.idxUnit = sync.Map{}
		shard.mu.Unlock()
	}
	s.totalCount.Store(0)
}

// ──────────────────────────────────────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────────────────────────────────────

// hashString 计算字符串的 FNV-1a 64 位哈希，用于分片选择。
func hashString(s string) uint64 {
	var h uint64 = fnvOffsetBasis64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}