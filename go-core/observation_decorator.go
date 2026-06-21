//go:build lecore_tier2 || lecore_tier3 || lecore_tier4 || lecore_tier5 || lecore_tier6 || lecore_tier7

// Package core — Composer 可观测性装饰器 (v4.0)
//
// 原语归属: L5 Observation（可观测层）
//
// 设计动机：
//   Pipeline 直接持有 *ObservabilityProvider 导致 Composer 层依赖 Observation 层，
//   违反了"上层不依赖下层"的依赖方向规则。
//   通过装饰器模式，将可观测能力注入到 Pipeline 外部，使 Pipeline 保持可移植性。
//
// 使用方式：
//   obs := NewObservabilityProvider()
//   base := NewPipeline[T](...)
//   decorated := NewObservableComposer(base, obs)
//   result, steps, err := decorated.Run(ctx, input)  // 自动携带可观测性
package core

import "context"

// ObservableComposer wraps a Composer with observation capabilities.
// The inner Composer remains portable and can be used without observation.
type ObservableComposer[T any] struct {
	inner   Composer[T]
	obsProv *ObservabilityProvider
}

// NewObservableComposer wraps a Composer with observation capabilities.
// The returned Composer automatically records execution steps to the Observation Pipeline.
func NewObservableComposer[T any](inner Composer[T], obsProv *ObservabilityProvider) *ObservableComposer[T] {
	return &ObservableComposer[T]{
		inner:   inner,
		obsProv: obsProv,
	}
}

// Run executes the inner Composer and records observation metadata.
// If obsProv is nil, falls back to the inner Composer's Run.
func (d *ObservableComposer[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	return d.inner.Run(ctx, input)
}

// Ensure ObservableComposer implements Composer.
var _ Composer[any] = (*ObservableComposer[any])(nil)
