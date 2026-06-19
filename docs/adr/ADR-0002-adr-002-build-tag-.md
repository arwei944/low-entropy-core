# ADR-0002: ADR-002: Build Tag 隔离外部数据库依赖

- Status: Accepted
- Date: 2026-06-19

## Context

框架需要支持 PostgreSQL 和 Redis 但不引入外部依赖。直接 import pgx/go-redis 会强制所有使用者安装这些依赖。

## Decision

使用 Go build tag 隔离：lecore_pgx 启用 PostgreSQL 支持，lecore_redis 启用 Redis 支持。未启用时提供 fallback 文件返回明确错误提示。

## Consequences

优点：核心框架零外部依赖，按需引入。缺点：需要维护两套文件（启用/回退），增加构建复杂度。

