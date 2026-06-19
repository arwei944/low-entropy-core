# ADR-0001: ADR-001: 选择 Provider 模式实现可观测性

- Status: Accepted
- Date: 2026-06-19

## Context

v0.8.0 架构评估发现框架缺乏可观测性集成。需要选择一种方式让框架支持 OpenTelemetry/Prometheus/slog 而不引入外部依赖。

## Decision

采用 Provider 接口模式——框架定义 Tracer/Meter/Logger 接口，提供 NoOp 默认实现（零开销），用户按需注入真实 SDK 实现。

## Consequences

优点：零外部依赖，渐进式采用，测试友好。缺点：需要用户自行实现 Provider 适配器。

