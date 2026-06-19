// Package core — Composer (第四原语) + 流处理 (v4.0)
//
// 合并自: composer.go + stream.go
//
// 包含:
//   - Composer / Pipeline / Branch / Parallel / WithRetry / WithTimeout / Map: 编排模式
//   - Stream / StreamMap / StreamFilter / StreamReduce: 流处理原语
//   - Window / WindowByTime / Merge / Split / Collect / FromSlice / ToSlice: 流操作
package core

import (
	"context"
	"math"
	"sync"
	"time"
)

// ============================================================================
// SECTION 1: Composer — 第四原语 (编排引擎)
// ============================================================================

// Composer is the fourth primitive: the orchestration engine.
type Composer[T any] interface {
	Run(ctx context.Context, input T) (T, []ExecutionStep, error)
}

// ============================================================================
// SECTION 2: Pipeline — 线性步骤链
// ============================================================================

// Pipeline is a linear chain of Steps that transform T → T.
// v0.9.0: 新增 ObservabilityProvider 支持，自动注入 Tracing/Metrics/Logging。
type Pipeline[T any] struct {
	steps    []Step[T, T]
	obs      ObservationAdapter
	obsProv  *ObservabilityProvider // v0.9.0: 可观测性注入
}

func NewPipeline[T any](obs ObservationAdapter, steps ...Step[T, T]) *Pipeline[T] {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &Pipeline[T]{steps: steps, obs: obs}
}

// WithObservability 注入可观测性 Provider。
// 注入后，Pipeline 和每个 Step 执行时自动创建 Span、记录 Metrics、输出日志。
// 未注入时，零开销（所有埋点跳过）。
func (p *Pipeline[T]) WithObservability(provider *ObservabilityProvider) *Pipeline[T] {
	p.obsProv = provider
	return p
}

func (p *Pipeline[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, len(p.steps))
	result := input

	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	// v0.9.0: 可观测性 — Pipeline Span
	var pipelineSpan Span
	if p.obsProv != nil && p.obsProv.TracerProvider != nil {
		tracer := p.obsProv.TracerProvider.Tracer("pipeline")
		ctx, pipelineSpan = tracer.Start(ctx, "pipeline.Run",
			WithSpanKind(0), // Internal
			WithSpanAttributes(
				NewKeyValue("pipeline.steps", len(p.steps)),
			),
		)
		defer pipelineSpan.End()
	}
	if p.obsProv != nil && p.obsProv.Logger != nil {
		p.obsProv.Logger.InfoContext(ctx, "pipeline started", "steps", len(p.steps))
	}

	for i, step := range p.steps {
		select {
		case <-ctx.Done():
			errStep := NewExecutionStepWithTrace(parentSpanID, step.UnitType(), "execute", "cancelled before step", "")
			errStep.TraceID = traceID
			errStep.Error = &StepError{Code: "CONTEXT_CANCELLED", Message: ctx.Err().Error(), Recoverable: false}
			steps = append(steps, errStep)
			p.obs.Record(steps)
			if p.obsProv != nil && p.obsProv.Logger != nil {
				p.obsProv.Logger.WarnContext(ctx, "pipeline cancelled", "step_index", i)
			}
			return result, steps, ctx.Err()
		default:
		}

		// v0.9.0: 可观测性 — Step Span
		var stepSpan Span
		stepCtx := ctx
		if p.obsProv != nil && p.obsProv.TracerProvider != nil {
			tracer := p.obsProv.TracerProvider.Tracer("step")
			stepCtx, stepSpan = tracer.Start(ctx, step.UnitType()+".execute",
				WithSpanKind(0),
				WithSpanAttributes(
					NewKeyValue("step.type", step.UnitType()),
					NewKeyValue("step.index", i),
				),
			)
		}

		start := time.Now()
		output, err := step.Execute(stepCtx, result)
		elapsed := time.Since(start)

		// v0.9.0: 可观测性 — 记录 Step 耗时
		if p.obsProv != nil && p.obsProv.MeterProvider != nil {
			if meter := p.obsProv.MeterProvider.Meter("step"); meter != nil {
				if hist, e := meter.NewHistogram("framework_step_duration_ms", "Step execution duration in milliseconds",
					[]float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 5000, 10000}); e == nil {
					hist.Record(stepCtx, float64(elapsed.Milliseconds()),
						NewKeyValue("step.type", step.UnitType()),
					)
				}
			}
		}

		es := NewExecutionStepWithTrace(parentSpanID, step.UnitType(), "execute", "pipeline step", "")
		es.TraceID = traceID
		es.DurationMs = elapsed.Milliseconds()
		es.Metadata = map[string]interface{}{
			"step_index":  i,
			"total_steps": len(p.steps),
		}

		if err != nil {
			if se, ok := err.(*StepError); ok {
				es.Error = se
			} else {
				es.Error = &StepError{Code: "STEP_ERROR", Message: err.Error(), Recoverable: false}
			}
			es.Details = "pipeline step failed"
			steps = append(steps, es)

			// v0.9.0: 可观测性 — 记录错误
			if stepSpan != nil {
				stepSpan.RecordError(err)
				stepSpan.SetStatus(StatusError, err.Error())
				stepSpan.End()
			}
			if p.obsProv != nil && p.obsProv.Logger != nil {
				p.obsProv.Logger.ErrorContext(stepCtx, "step failed",
					"step", step.UnitType(),
					"index", i,
					"error", err.Error(),
					"duration_ms", elapsed.Milliseconds(),
				)
			}

			p.obs.Record(steps)
			return result, steps, err
		}

		// v0.9.0: 可观测性 — Step 成功
		if stepSpan != nil {
			stepSpan.SetStatus(StatusOK, "")
			stepSpan.End()
		}

		es.Details = "pipeline step completed"
		steps = append(steps, es)
		result = output

		select {
		case <-ctx.Done():
			cancelStep := NewExecutionStepWithTrace(parentSpanID, "Composer", "cancelled", "context done after step", "")
			cancelStep.TraceID = traceID
			cancelStep.Error = &StepError{Code: "CONTEXT_CANCELLED", Message: ctx.Err().Error(), Recoverable: false}
			steps = append(steps, cancelStep)
			p.obs.Record(steps)
			return result, steps, ctx.Err()
		default:
		}
	}

	// v0.9.0: Pipeline 成功
	if p.obsProv != nil && p.obsProv.Logger != nil {
		p.obsProv.Logger.InfoContext(ctx, "pipeline completed", "total_steps", len(p.steps))
	}

	p.obs.Record(steps)
	return result, steps, nil
}

func (p *Pipeline[T]) AddStep(step Step[T, T]) *Pipeline[T] {
	p.steps = append(p.steps, step)
	return p
}

func (p *Pipeline[T]) StepCount() int {
	return len(p.steps)
}

// ============================================================================
// SECTION 3: Branch — 条件路由
// ============================================================================

// NewBranch 创建条件分支 Composer。
// 根据 condition 的结果选择执行 truePath 或 falsePath。
// 子 Composer 的 ExecutionStep 被正确收集到父级。
func NewBranch[T any](condition func(T) bool, truePath, falsePath Composer[T]) Composer[T] {
	return &branchComposer[T]{
		condition: condition,
		truePath:  truePath,
		falsePath: falsePath,
	}
}

type branchComposer[T any] struct {
	condition func(T) bool
	truePath  Composer[T]
	falsePath Composer[T]
}

func (b *branchComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	if b.condition(input) {
		return b.truePath.Run(ctx, input)
	}
	return b.falsePath.Run(ctx, input)
}

// ============================================================================
// SECTION 4: Parallel — 并行执行
// ============================================================================

type ParallelResults[T any] struct {
	Results []T
	Errors  []error
	Steps   [][]ExecutionStep
}

func RunParallel[T any](ctx context.Context, input T, composers ...Composer[T]) (ParallelResults[T], []ExecutionStep, error) {
	if len(composers) == 0 {
		return ParallelResults[T]{}, nil, nil
	}

	type result struct {
		index int
		value T
		steps []ExecutionStep
		err   error
	}

	resultCh := make(chan result, len(composers))
	var wg sync.WaitGroup

	for i, c := range composers {
		wg.Add(1)
		go func(idx int, comp Composer[T]) {
			defer wg.Done()
			val, steps, err := comp.Run(ctx, input)
			resultCh <- result{index: idx, value: val, steps: steps, err: err}
		}(i, c)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]T, len(composers))
	errs := make([]error, len(composers))
	allSteps := make([][]ExecutionStep, len(composers))
	hasError := false

	for r := range resultCh {
		results[r.index] = r.value
		errs[r.index] = r.err
		allSteps[r.index] = r.steps
		if r.err != nil {
			hasError = true
		}
	}

	flatSteps := make([]ExecutionStep, 0)
	for _, s := range allSteps {
		flatSteps = append(flatSteps, s...)
	}

	var finalErr error
	if hasError {
		finalErr = &StepError{Code: "PARALLEL_ERROR", Message: "one or more parallel branches failed", Recoverable: false}
	}

	return ParallelResults[T]{Results: results, Errors: errs, Steps: allSteps}, flatSteps, finalErr
}

// ============================================================================
// SECTION 5: WithRetry — 指数退避重试
// ============================================================================

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    10 * time.Second,
		Multiplier:  2.0,
	}
}

func WithRetry[T any](comp Composer[T], config RetryConfig) Composer[T] {
	return &retryComposer[T]{inner: comp, config: config}
}

type retryComposer[T any] struct {
	inner  Composer[T]
	config RetryConfig
}

func (r *retryComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	allSteps := make([]ExecutionStep, 0)
	var lastErr error

	for attempt := 0; attempt < r.config.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return input, allSteps, ctx.Err()
		default:
		}

		result, steps, err := r.inner.Run(ctx, input)
		allSteps = append(allSteps, steps...)

		if err == nil {
			return result, allSteps, nil
		}

		lastErr = err

		if se, ok := err.(*StepError); ok && !se.Recoverable {
			return result, allSteps, err
		}

		if attempt < r.config.MaxAttempts-1 {
			delay := r.computeDelay(attempt)
			select {
			case <-ctx.Done():
				return result, allSteps, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	var zero T
	return zero, allSteps, lastErr
}

func (r *retryComposer[T]) computeDelay(attempt int) time.Duration {
	delay := float64(r.config.BaseDelay) * math.Pow(r.config.Multiplier, float64(attempt))
	if delay > float64(r.config.MaxDelay) {
		delay = float64(r.config.MaxDelay)
	}
	return time.Duration(delay)
}

// ============================================================================
// SECTION 6: WithTimeout — 超时
// ============================================================================

func WithTimeout[T any](comp Composer[T], timeout time.Duration) Composer[T] {
	return &timeoutComposer[T]{inner: comp, timeout: timeout}
}

type timeoutComposer[T any] struct {
	inner   Composer[T]
	timeout time.Duration
}

func (t *timeoutComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	result, steps, err := t.inner.Run(ctx, input)

	if ctx.Err() != nil {
		se := &StepError{
			Code:        "TIMEOUT",
			Message:     "operation timed out after " + t.timeout.String(),
			Recoverable: true,
		}
		return result, steps, se
	}

	return result, steps, err
}

// ============================================================================
// SECTION 7: Map — 类型转换
// ============================================================================

func Map[T, U any](comp Composer[T], mapper func(T) U) Composer[U] {
	return &mapComposer[T, U]{inner: comp, mapper: mapper}
}

type mapComposer[T, U any] struct {
	inner  Composer[T]
	mapper func(T) U
}

func (m *mapComposer[T, U]) Run(ctx context.Context, input U) (U, []ExecutionStep, error) {
	var tInput T
	if any(input) == nil {
		result, steps, err := m.inner.Run(ctx, tInput)
		if err != nil {
			var zero U
			return zero, steps, err
		}
		return m.mapper(result), steps, nil
	}
	var zero U
	return zero, nil, &StepError{Code: "MAP_ERROR", Message: "Map requires same input type or nil input", Recoverable: false}
}

// ============================================================================
// SECTION 8: Compose — 单步包装
// ============================================================================

func Compose[T any](obs ObservationAdapter, step Step[T, T]) Composer[T] {
	return NewPipeline[T](obs, step)
}

// ============================================================================
// SECTION 9: Stream — 流处理原语
// ============================================================================

const defaultBufferSize = 100

// StreamConfig configures stream processing.
type StreamConfig struct {
	BufferSize int
}

// StreamMap applies a function to each element in the stream.
func StreamMap[In, Out any](input <-chan In, fn func(In) Out) <-chan Out {
	output := make(chan Out, defaultBufferSize)
	go func() {
		defer close(output)
		for v := range input {
			output <- fn(v)
		}
	}()
	return output
}

// StreamFilter filters elements that don't satisfy the predicate.
func StreamFilter[T any](input <-chan T, predicate func(T) bool) <-chan T {
	output := make(chan T, defaultBufferSize)
	go func() {
		defer close(output)
		for v := range input {
			if predicate(v) {
				output <- v
			}
		}
	}()
	return output
}

// StreamReduce aggregates all elements into a single value.
func StreamReduce[T, R any](input <-chan T, initial R, fn func(R, T) R) R {
	result := initial
	for v := range input {
		result = fn(result, v)
	}
	return result
}

// Window collects elements into windows of a given size.
func Window[T any](input <-chan T, size int) <-chan []T {
	output := make(chan []T, defaultBufferSize)
	go func() {
		defer close(output)
		window := make([]T, 0, size)
		for v := range input {
			window = append(window, v)
			if len(window) == size {
				output <- window
				window = make([]T, 0, size)
			}
		}
		if len(window) > 0 {
			output <- window
		}
	}()
	return output
}

// WindowByTime collects elements within a time window.
func WindowByTime[T any](input <-chan T, duration time.Duration) <-chan []T {
	output := make(chan []T, defaultBufferSize)
	go func() {
		defer close(output)
		window := make([]T, 0)
		ticker := time.NewTicker(duration)
		defer ticker.Stop()

		flush := func() {
			if len(window) > 0 {
				output <- window
				window = make([]T, 0)
			}
		}

		for {
			select {
			case v, ok := <-input:
				if !ok {
					flush()
					return
				}
				window = append(window, v)
			case <-ticker.C:
				flush()
			}
		}
	}()
	return output
}

// Merge merges multiple input channels into one.
func Merge[T any](inputs ...<-chan T) <-chan T {
	output := make(chan T, defaultBufferSize)
	var wg sync.WaitGroup
	wg.Add(len(inputs))

	for _, input := range inputs {
		go func(ch <-chan T) {
			defer wg.Done()
			for v := range ch {
				output <- v
			}
		}(input)
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output
}

// Split splits a single channel into multiple based on a function.
func Split[T any](input <-chan T, fn func(T) int, n int) []<-chan T {
	outputs := make([]chan T, n)
	result := make([]<-chan T, n)
	for i := 0; i < n; i++ {
		ch := make(chan T, defaultBufferSize)
		outputs[i] = ch
		result[i] = ch
	}

	go func() {
		defer func() {
			for _, ch := range outputs {
				close(ch)
			}
		}()
		for v := range input {
			idx := fn(v)
			if idx < 0 {
				idx = 0
			}
			if idx >= n {
				idx = n - 1
			}
			outputs[idx] <- v
		}
	}()

	return result
}

// Collect collects all elements from a channel into a slice.
func Collect[T any](input <-chan T) []T {
	if input == nil {
		return nil
	}
	var result []T
	for v := range input {
		result = append(result, v)
	}
	return result
}

// FromSlice creates a stream from a slice.
func FromSlice[T any](items []T) <-chan T {
	output := make(chan T, defaultBufferSize)
	go func() {
		defer close(output)
		for _, item := range items {
			output <- item
		}
	}()
	return output
}

// ToSlice collects stream elements into a slice (alias for Collect).
func ToSlice[T any](input <-chan T) []T {
	return Collect(input)
}

// ============================================================================
// SECTION 10: FanOut — 扇出广播
// ============================================================================

// FanOut 将输入广播到多个 Composer 并行执行，收集所有结果。
// 任一分支失败则整体失败。
type FanOut[T any] struct {
	branches []Composer[T]
	obs      ObservationAdapter
}

// NewFanOut 创建扇出编排器。
func NewFanOut[T any](obs ObservationAdapter, branches ...Composer[T]) *FanOut[T] {
	return &FanOut[T]{branches: branches, obs: obs}
}

func (fo *FanOut[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	type branchResult struct {
		steps []ExecutionStep
		err   error
	}
	results := make([]branchResult, len(fo.branches))
	var wg sync.WaitGroup
	for i, b := range fo.branches {
		wg.Add(1)
		go func(idx int, branch Composer[T]) {
			defer wg.Done()
			_, s, e := branch.Run(ctx, input)
			results[idx] = branchResult{s, e}
		}(i, b)
	}
	wg.Wait()

	var allSteps []ExecutionStep
	for _, br := range results {
		allSteps = append(allSteps, br.steps...)
		if br.err != nil {
			return input, allSteps, br.err
		}
	}
	return input, allSteps, nil
}

// ============================================================================
// SECTION 11: Debounce — 防抖
// ============================================================================

// Debounce 防抖编排器：在静默期内忽略重复调用，仅执行最后一次。
type Debounce[T any] struct {
	inner    Composer[T]
	interval time.Duration
	lastCall time.Time
	mu       sync.Mutex
}

// NewDebounce 创建防抖编排器。
func NewDebounce[T any](inner Composer[T], interval time.Duration) *Debounce[T] {
	return &Debounce[T]{inner: inner, interval: interval}
}

func (db *Debounce[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	db.mu.Lock()
	if time.Since(db.lastCall) < db.interval {
		db.mu.Unlock()
		return input, nil, nil // 跳过：静默期内
	}
	db.lastCall = time.Now()
	db.mu.Unlock()
	return db.inner.Run(ctx, input)
}

// ============================================================================
// SECTION 12: Throttle — 节流
// ============================================================================

// Throttle 节流编排器：限制调用频率，确保相邻调用间隔不小于 interval。
type Throttle[T any] struct {
	inner    Composer[T]
	interval time.Duration
	lastCall time.Time
	mu       sync.Mutex
}

// NewThrottle 创建节流编排器。
func NewThrottle[T any](inner Composer[T], interval time.Duration) *Throttle[T] {
	return &Throttle[T]{inner: inner, interval: interval}
}

func (th *Throttle[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	th.mu.Lock()
	elapsed := time.Since(th.lastCall)
	if elapsed < th.interval {
		waitTime := th.interval - elapsed
		th.mu.Unlock()
		select {
		case <-time.After(waitTime):
		case <-ctx.Done():
			return input, nil, ctx.Err()
		}
	} else {
		th.mu.Unlock()
	}
	th.mu.Lock()
	th.lastCall = time.Now()
	th.mu.Unlock()
	return th.inner.Run(ctx, input)
}