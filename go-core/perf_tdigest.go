//go:build lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — TDigest 近似分位数算法 (v4.0)
package core

import (
	"math"
	"sort"
)

// tDigestCompression 控制精度与内存的折中。值越大精度越高（默认 100）。
const tDigestCompression = 100.0

// tDigestMaxCentroids 质心数量上限 = 2 * compression。
const tDigestMaxCentroids = 2 * tDigestCompression

// TDigest 是 t-digest 分位数近似算法的实现。
// 线程不安全——需要外部同步（配合 ShardedLock 使用）。
type TDigest struct {
	centroids   []tdCentroid
	count       int64   // 总插入数
	sum         float64 // 总和（用于均值）
	min         float64
	max         float64
	compression float64
}

// tdCentroid 是 t-digest 的质心，表示一组数据点的均值。
type tdCentroid struct {
	mean  float64
	weight float64
}

// NewTDigest 创建一个新的 TDigest 实例。
// compression 控制精度（默认 100，范围 10-1000）。
func NewTDigest(compression float64) *TDigest {
	if compression <= 0 {
		compression = tDigestCompression
	}
	if compression > 1000 {
		compression = 1000
	}
	return &TDigest{
		centroids:   make([]tdCentroid, 0, 2*int(compression)),
		compression: compression,
		min:         math.MaxFloat64,
		max:         -math.MaxFloat64,
	}
}

// NewTDigestDefault 使用默认压缩因子创建 TDigest。
func NewTDigestDefault() *TDigest {
	return NewTDigest(tDigestCompression)
}

// Add 插入一个值到 t-digest 中。
// 时间复杂度 O(n) 其中 n 为质心数量（约 100-200）。
func (td *TDigest) Add(value float64) {
	td.count++
	td.sum += value
	if value < td.min {
		td.min = value
	}
	if value > td.max {
		td.max = value
	}

	// 查找最近的质心
	idx := sort.Search(len(td.centroids), func(i int) bool {
		return td.centroids[i].mean >= value
	})

	// 尝试合并到相邻质心
	if idx < len(td.centroids) && td.centroids[idx].weight < td.maxWeight(td.count) {
		td.centroids[idx].mean = td.weightedMerge(
			td.centroids[idx].mean, td.centroids[idx].weight,
			value, 1,
		)
		td.centroids[idx].weight++
		return
	}
	if idx > 0 && td.centroids[idx-1].weight < td.maxWeight(td.count) {
		td.centroids[idx-1].mean = td.weightedMerge(
			td.centroids[idx-1].mean, td.centroids[idx-1].weight,
			value, 1,
		)
		td.centroids[idx-1].weight++
		return
	}

	// 创建新质心
	c := tdCentroid{mean: value, weight: 1}
	if idx == len(td.centroids) {
		td.centroids = append(td.centroids, c)
	} else {
		td.centroids = append(td.centroids, tdCentroid{})
		copy(td.centroids[idx+1:], td.centroids[idx:])
		td.centroids[idx] = c
	}

	// 如果质心数量超过上限，执行压缩
	if len(td.centroids) > 2*int(td.compression) {
		td.compress()
	}
}

// maxWeight 计算给定总数下单个质心的最大权重。
// 标准 t-digest 公式：maxWeight = 4 * n / compression
// 确保质心均匀分布在整个数据范围内，避免中位数附近的精度损失。
func (td *TDigest) maxWeight(n int64) float64 {
	return 4.0 * float64(n) / td.compression
}

// weightedMerge 计算两个值的加权平均。
func (td *TDigest) weightedMerge(mean1, weight1, mean2, weight2 float64) float64 {
	return (mean1*weight1 + mean2*weight2) / (weight1 + weight2)
}

// compress 压缩质心，合并相邻质心以减少数量。
func (td *TDigest) compress() {
	if len(td.centroids) <= 2*int(td.compression) {
		return
	}

	// 按权重排序，合并权重最小的相邻质心
	// 简化实现：基于最近邻合并
	newCentroids := make([]tdCentroid, 0, len(td.centroids))
	sort.Slice(td.centroids, func(i, j int) bool {
		return td.centroids[i].mean < td.centroids[j].mean
	})

	i := 0
	for i < len(td.centroids) {
		if i+1 < len(td.centroids) {
			// 合并相邻质心
			merged := tdCentroid{
				mean: td.weightedMerge(
					td.centroids[i].mean, td.centroids[i].weight,
					td.centroids[i+1].mean, td.centroids[i+1].weight,
				),
				weight: td.centroids[i].weight + td.centroids[i+1].weight,
			}
			newCentroids = append(newCentroids, merged)
			i += 2
		} else {
			newCentroids = append(newCentroids, td.centroids[i])
			i++
		}
	}

	td.centroids = newCentroids
}

// Quantile 计算指定分位数的近似值。
// q 的范围为 [0.0, 1.0]，其中 0.5 为中位数。
// 时间复杂度 O(1)（质心数量固定）。
func (td *TDigest) Quantile(q float64) float64 {
	if td.count == 0 {
		return 0
	}
	if q <= 0 {
		return td.min
	}
	if q >= 1 {
		return td.max
	}
	if len(td.centroids) == 0 {
		return 0
	}

	target := q * float64(td.count)
	cumulative := 0.0

	for i, c := range td.centroids {
		prevCumulative := cumulative
		cumulative += c.weight

		if cumulative >= target {
			if i == 0 || cumulative-prevCumulative < 1e-10 {
				return c.mean
			}
			// 线性插值
			frac := (target - prevCumulative) / (cumulative - prevCumulative)
			if i == 0 {
				return c.mean
			}
			return td.centroids[i-1].mean + frac*(c.mean-td.centroids[i-1].mean)
		}
	}

	return td.centroids[len(td.centroids)-1].mean
}

// Count 返回插入的总值数。
func (td *TDigest) Count() int64 {
	return td.count
}

// Mean 返回所有插入值的均值。
func (td *TDigest) Mean() float64 {
	if td.count == 0 {
		return 0
	}
	return td.sum / float64(td.count)
}

// Min 返回最小值。
func (td *TDigest) Min() float64 {
	if td.count == 0 {
		return 0
	}
	return td.min
}

// Max 返回最大值。
func (td *TDigest) Max() float64 {
	if td.count == 0 {
		return 0
	}
	return td.max
}

// Merge 合并另一个 TDigest 到当前实例中。
// 用于分片聚合的最终合并阶段。
func (td *TDigest) Merge(other *TDigest) {
	if other.count == 0 {
		return
	}

	td.count += other.count
	td.sum += other.sum
	if other.min < td.min {
		td.min = other.min
	}
	if other.max > td.max {
		td.max = other.max
	}

	// 合并质心
	td.centroids = append(td.centroids, other.centroids...)
	sort.Slice(td.centroids, func(i, j int) bool {
		return td.centroids[i].mean < td.centroids[j].mean
	})
	td.compress()
}

// Clone 深度复制 TDigest。
func (td *TDigest) Clone() *TDigest {
	clone := &TDigest{
		centroids:   make([]tdCentroid, len(td.centroids)),
		count:       td.count,
		sum:         td.sum,
		min:         td.min,
		max:         td.max,
		compression: td.compression,
	}
	copy(clone.centroids, td.centroids)
	return clone
}
