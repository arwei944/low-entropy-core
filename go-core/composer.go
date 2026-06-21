// Package core — Composer (第四原语) (v5.0 整合版)
//
// 合并自: composer.go + composer_fanout.go + composer_parallel.go + composer_stream.go
//
// 包含:
//   - Composer / Pipeline / Branch / Map / Compose: 基础编排
//   - RunParallel / WithRetry / WithTimeout: 并行/重试/超时
//   - FanOut / Debounce / Throttle: 扇出/防抖/节流
//   - StreamMap / StreamFilter / StreamReduce / Window / WindowByTime / Merge / Split: 流处理
//   - FromSlice / Collect / ToSlice: 辅助工具
package core

import (
	"context"
	"time"
)

// ==================== 核心接口与 Pipeline ====================

// Composer is the fourth primitive: the orchestration engine.
type Composer[T any] interface {
	Run(ctx context.Context, input T) (T, []ExecutionStep, error)
}

// Pipeline is a linear chain of Steps that transform T → T.
type Pipeline[T any] struct {
	steps   []Step[T, T]
	obs     ObservationAdapter
	obsProv *ObservabilityProvider
}

func NewPipeline[T any](obs ObservationAdapter, steps ...Step[T, T]) *Pipeline[T] {
	if obs == nil {
		obs = &NoOpObservationAdapter{}
	}
	return &Pipeline[T]{steps: steps, obs: obs}
}

// WithObservability 注入可观测性 Provider。
func (p *Pipeline[T]) WithObservability(provider *ObservabilityProvider) *Pipeline[T] {
	p.obsProv = provider
	return p
}

func (p *Pipeline[T]) Run(ctx context.Context, input T) (T, []ExecutionStep, error) {
	steps := make([]ExecutionStep, 0, len(p.steps))
	result := input

	traceID := string(NewTraceID())
	parentSpanID := string(NewSpanID())

	var pipelineSpan Span
	if p.obsProv != nil && p.obsProv.TracerProvider != nil {
		tracer := p.obsProv.TracerProvider.Tracer("pipeline")
		ctx, pipelineSpan = tracer.Start(ctx, "pipeline.Run",
			WithSpanKind(0),
			WithSpanAttributes(NewKeyValue("pipeline.steps", len(p.steps))),
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

		nowFunc := time.Now
		if p.obsProv != nil && p.obsProv.NowFunc != nil {
			nowFunc = p.obsProv.NowFunc
		}
		start := nowFunc()
		output, err := step.Execute(stepCtx, result)
		elapsed := nowFunc().Sub(start)

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
		es.Metadata = map[string]any{
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

			if stepSpan != nil {
				stepSpan.RecordError(err)
				stepSpan.SetStatus(StatusError, err.Error())
				stepSpan.End()
			}
			if p.obsProv != nil && p.obsProv.Logger != nil {
				p.obsProv.Logger.ErrorContext(stepCtx, "step failed",
					"step", step.UnitType(), "index", i, "error", err.Error(),
					"duration_ms", elapsed.Milliseconds(),
				)
			}

			p.obs.Record(steps)
			return result, steps, err
		}

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

// ==================== 条件分支 ====================

// NewBranch 创建条件分支 Composer。
func NewBranch[T any](condition func(T) bool, truePath, falsePath Composer[T]) Composer[T] {
	return &branchComposer[T]{condition: condition, truePath: truePath, falsePath: falsePath}
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

// ==================== Map / Compose ====================

func Map[T, U any](comp Composer[T], mapper func(T) U) Composer[U] {
	return &mapComposer[T, U]{inner: comp, mapper: mapper}
}

type mapComposer[T, U any] struct {
	inner  Composer[T]
	mapper func(T) U
}

func (m *mapComposer[T, U]) Run(ctx context.Context, _ U) (U, []ExecutionStep, error) {
	var tInput T
	result, steps, err := m.inner.Run(ctx, tInput)
	if err != nil {
		var zero U
		return zero, steps, err
	}
	return m.mapper(result), steps, nil
}

func Compose[T any](obs ObservationAdapter, step Step[T, T]) Composer[T] {
	return NewPipeline[T](obs, step)
}

// 注: 并行执行 (ParallelResults, RunParallel) 定义于 composer_parallel.go