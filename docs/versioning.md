# 低熵核心 (Low-Entropy Core) 版本号管理规范

## 版本号格式

采用 **语义化版本 (Semantic Versioning 2.0.0)** ：

```
MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]
```

| 字段 | 含义 | 递增条件 |
|------|------|----------|
| **MAJOR** | 主版本号 | 不兼容的 API 变更（接口签名变化、类型移除、公开行为变更） |
| **MINOR** | 次版本号 | 向后兼容的新功能（新增 Compose 模式、新增原语类型、新增 Tier 层级） |
| **PATCH** | 修订号 | 向后兼容的 Bug 修复（修复竞态条件、内存泄漏、边界条件错误） |
| **PRERELEASE** | 预发布标识 | `-alpha.N`、`-beta.N`、`-rc.N` |

## 0.x 阶段约定

当前框架处于 0.x 阶段（当前版本：**0.8.0**），采用以下约定：

- 0.x 版本中，MINOR 版本号的递增等同于 MAJOR 级别的变更——即 0.x 到 0.(x+1) 之间**不保证**向后兼容
- 0.x 版本中，PATCH 版本号用于**向后兼容的修复**，与 1.0+ 的 PATCH 语义一致
- 进入 1.0 后，严格遵循标准 SemVer 规则

示例：

```
0.1.0  → 0.2.0   可能不兼容（新增 Tier 系统）
0.2.0  → 0.2.1   向后兼容（修复 Bug）
0.5.0  → 0.5.1   向后兼容（修复测试超时）
```

## 1.0 里程碑

达到以下条件时发布 1.0.0：

| 条件 | 说明 |
|------|------|
| 四原语稳定 | Atom/Port/Adapter/Composer 接口不再变更 |
| Handoff 协议稳定 | DevSnapshot/HandoffContract 不再变更 |
| 渐进复杂度模型完备 | 8 个 Tier 的 Build Tag 体系完整且已验证 |
| 测试覆盖率达标 | 核心模块（L0-L2）测试覆盖率 ≥ 90% |
| 生产验证 | 至少 1 个真实项目在生产环境中使用超过 3 个月 |

## 版本发布流程

每个版本发布必须经过以下步骤：

1. **功能冻结** — 确定本次版本包含的功能范围
2. **全层级构建** — `go build -tags "lecore_tier7" ./...` 通过
3. **全层级测试** — `go test -tags "lecore_tier7" ./...` 通过
4. **架构审查** — 检查新增代码是否违反核心约束
5. **Changelog** — 更新 CHANGELOG.md，记录所有变更
6. **Git Tag** — 打标签 `git tag -a v0.X.Y -m "Release v0.X.Y"`
7. **示例验证** — 所有 examples/ 下的示例项目编译运行通过

## Git Tag 命名

```
v0.5.0          # 正式版本
v0.5.1-alpha.1  # 预发布版本
```

Tag 必须使用 `v` 前缀，与 Go module 版本号规范一致。

## Go Module 版本

Go module 路径中的版本号遵循 Go 官方规范：

- 0.x 和 1.x：`module github.com/low-entropy-core` 无需版本后缀
- 2.x+：`module github.com/low-entropy-core/v2`

## Changelog 格式

采用 [Keep a Changelog](https://keepachangelog.com/) 格式：

```markdown
## [0.8.0] - 2026-06-19

### Added
- 通用版本管理模块 (`go-core/version_*.go`)：8 个文件，涵盖全生命周期
  - `version_types.go`：Semver、ConventionalCommit、ChangeIntent、ADR、ReleasePlan 等 15+ 核心类型
  - `version_semver.go`：SemVer 2.0.0 解析器 + 自动版本推断引擎
  - `version_commit.go`：Conventional Commits 1.0.0 完整解析器（支持 !、BREAKING CHANGE、多行 body）
  - `version_change.go`：ArchChange 意图文件系统（仿 changesets 模式）
  - `version_changelog.go`：Keep a Changelog 格式生成器
  - `version_git.go`：Git 操作 Sidecar 适配器（log、tag、branch、diff）
  - `version_adr.go`：架构决策记录 (ADR) 系统
  - `version_composer.go`：6 步发布流水线编排器
- 版本管理 API 端点（5 个新增）：
  - `GET /api/version/commit-analyze`：提交分析
  - `GET /api/version/next-version`：下一版本自动推断
  - `GET/POST/DELETE /api/version/arch-change`：ArchChange 意图文件 CRUD
  - `GET/POST /api/version/adr`：ADR 创建与查询
  - `POST /api/version/release`：发布流水线执行
- 版本管理前端增强：5 个子 Tab（概览、Changelog、ADR、ArchChange、发布流水线）
- 版本管理单元测试：29 个测试用例 + 8 个性能基准，全部通过
- 开发文档：`docs/version-management-v080-dev/version-management-v080-dev.html`（19 个任务 × 10 条验收标准）

### Changed
- `cmd/arch-manager/version.go`：重构为使用 go-core 版本管理模块
- `cmd/arch-manager/main.go`：版本号从硬编码 0.5.1 改为动态获取，所有 API 端点增加 CORS 头
- `go.mod`：增加 go-core 子模块依赖（`replace low-entropy-core/go-core => ./go-core`）
- `arch-manager.html`：版本管理 Tab 从单页升级为 5 个子 Tab 面板

### Fixed
- Conventional Commits 解析器 body/footer 边界判定逻辑修复
- 非十六进制 hash 导致的提交解析跳过问题

## [0.7.0] - 2026-06-19

### Added
- Understand-Anything (UA) 集成：类型系统 (`understand_types.go`)、适配器 (`understand_adapter.go`)
- 迁移层 (`understand_migration.go`)：基线创建、差异对比、层级漂移检测
- 监督层 (`understand_supervisor.go`)：6 条核心约束 (C1-C6) 自动验证
- 观测层 (`understand_observer.go`)：结构观测、变更观测、语义搜索（倒排索引）
- 架构管理器 UA API：`/api/ua/graph`、`/api/ua/validate`、`/api/ua/search`
- 架构管理器前端：知识图谱 Tab（散点图、类型分布、节点详情、搜索）
- 引导层 Tour 集成：UA 学习导览数据自动加载到引导层
- 单元测试：52 个测试用例覆盖类型系统、迁移层、监督层、观测层、集成测试、性能基准

### Changed
- 架构管理器增加第 7 个 Tab（知识图谱），快捷键 Ctrl+7
- 引导层 API 增加 `tour` 字段，支持 UA 学习导览

### Fixed
- 无

## [0.6.0] - 2026-06-19

### Added
- 架构管理器 v0.6.0：复杂度 Treemap、版本管理、引导层
- 版本快照模块：创建/存储/列表/对比
- 引导层 API：DNA 卡、决策树、模式库、约束检查器
- 复杂度评分系统：加权 0.4×行数 + 0.3×符号 + 0.3×依赖
- 版本时间线：可视化版本历史
- 版本 diff 对比：文件变更 + 层级漂移

### Changed
- 复杂度热力图 → Treemap 树图（面积=行数，颜色=复杂度评分）
- 复杂度排序表替代原热力图的数据表格
- 仪表盘 Tab 默认页改为引导层

### Fixed
- 无

## [0.5.1] - 2026-06-19

### Added
- TierCheck: 编译期层级漂移检测
- TierDrift: 持续漂移监控与趋势预测
- TierTransition: Feature Flag 渐进迁移
- MigrateAnalyze/MigrateAdopt/MigrateValidate: 迁移工具链
- TierBridge: 兼容性适配器代码生成

### Changed
- Build Tags 添加到 39 个非内核文件
- AppConfig 从 app.go (L5) 移至 complexity_profile.go (L0)

### Fixed
- migration_validate_test.go 子进程调用导致的测试超时
```

## 版本号与 Tier 层级的关系

| 场景 | 版本号变更 |
|------|-----------|
| 新增 Tier 层级（如 L6→L7） | MINOR +1 |
| 修改 Tier 层级划分阈值 | MINOR +1 |
| 修复 Tier 内 Bug | PATCH +1 |
| 移除 Tier 层级 | MAJOR +1（0.x 阶段 MINOR +1） |
| 修改 Build Tag 语义 | MAJOR +1（0.x 阶段 MINOR +1） |