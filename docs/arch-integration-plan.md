# arch-manager + arch-dev + lec 三模块整合开发计划

> 文档版本：v1.0
> 编写日期：2026-06-22
> 任务状态：待启动

---

## 总览

**目标**：将 cmd/arch-manager（32 文件）、cmd/arch-dev（13 文件）、cmd/lec（2 文件 + 模板）整合为一个统一的架构治理平台。

**当前问题**：
- 三模块功能重叠：模板引擎在 arch-dev 和 lec 中各自实现
- 架构分析逻辑分散在 arch-manager 和 arch-dev
- 入口不统一：用户需要记住三个不同的命令
- 共享代码无法复用：核心解析/分析逻辑在多份拷贝中

**整合后目标**：
- 一个统一的 CLI 工具（arch-cli）
- 一个统一的 HTTP 服务器（arch-server）
- 一个共享核心库（go-core/arch/）
- 所有模板统一使用 embed.FS 引擎（lec 的方案）
- 文档和验证完全符合 CLAUDE.md 的 8 层架构 + 四原语约束

---

## 任务单元拆分（共 11 个任务单元）

---

## 任务单元 1：提取共享类型定义

### 目标
将 arch-manager 和 arch-dev 中重复的类型定义（models.go, arch_project.go 等）统一提取到 go-core/arch/types.go。

### 背景
当前 arch-manager 的 models.go（239 行）和 arch-dev 的 arch_project.go（71 行）包含了大量相同/相似的结构体定义，导致维护成本高、类型不一致。

### 输入输出
- **输入**：
  - `cmd/arch-manager/models.go`（FileInfo, LayerStat, ArchData, Violation 等）
  - `cmd/arch-dev/arch_project.go`（ProjectConfig, ProjectMeta 等）
- **输出**：
  - `go-core/arch/types.go`（统一的共享类型定义，≤ 300 行）
  - 所有使用处的 import 路径更新

### 架构设计
- **层级归属**：L1（四原语定义层）— 类型定义是纯数据结构，属于原语的基础设施
- **原语类型**：纯数据结构（支撑 Atom/Port/Adapter/Composer 的类型）
- **依赖关系**：
  ```
  go-core/arch/types.go
    ↳ 仅依赖 go-core/errors.go (L0)
    ↳ 不依赖任何 L2+ 层级
  ```

### 实现步骤
1. 从 models.go 中提取核心类型：FileInfo, LayerStat, ArchData, Violation, PrimitivesInfo 等
2. 从 arch_project.go 中提取：ProjectConfig, TemplateData 等
3. 统一字段命名风格（保持首字母大写导出）
4. 添加必要的注释（中文，简明扼要）
5. 删除原文件中已迁移的类型（后续任务统一清理）

### 验收标准（严格 AC）
1. [ ] **功能正确**：`go build ./go-core/...` 编译通过，无类型错误
2. [ ] **层级合规**：`go-core/arch/types.go` 正确归属 L1 层，不依赖 L2+
3. [ ] **依赖方向合规**：仅 import 同层或更低层包（L0, L1）
4. [ ] **四原语模式**：纯数据结构，无业务逻辑 func
5. [ ] **文件行数合规**：`go-core/arch/types.go` ≤ 300 行
6. [ ] **可观测性**：类型中包含必要的可观测字段（如 GeneratedAt, Count 等）
7. [ ] **测试覆盖**：编写 `go-core/arch/types_test.go`，覆盖主要结构的零值/默认值测试
8. [ ] **编译通过**：`go build ./...` 整个项目编译通过
9. [ ] **无 fmt.Println**：纯类型定义，无 print
10. [ ] **文档完整性**：本节开发文档完整，AC 可判定

### 风险与注意事项
- 迁移后原有引用处需更新 import 路径，这可能影响多个文件
- 需保持类型名称不变（向后兼容），只调整包路径
- 类型方法（String, MarshalJSON 等）需一起迁移

---

## 任务单元 2：提取 AST 解析 Atom

### 目标
将 arch-manager 的 parser_ast.go 和 parser_file.go 提取为可复用的 L1 Atom：`go-core/arch/parser.go`。

### 背景
arch-manager 中 parser_ast.go（185 行）和 parser_file.go（149 行）实现了对 Go 源码文件的 AST 解析和元数据提取。这一功能在 arch-dev 的架构检查中也需要用到，但目前没有共享机制。

### 输入输出
- **输入**：
  - `cmd/arch-manager/parser_ast.go`（AST 遍历和符号提取）
  - `cmd/arch-manager/parser_file.go`（文件级解析）
- **输出**：
  - `go-core/arch/parser.go`（纯函数 Atom：输入文件路径 → FileInfo，≤ 300 行）

### 架构设计
- **层级归属**：L1 — 纯函数计算，无副作用（只读文件）
- **原语类型**：Atom
- **函数签名**：
  ```go
  func ParseFile(path string) (*core.FileInfo, error)         // 单文件解析
  func ParseDirectory(root string) ([]core.FileInfo, error)   // 目录解析
  ```
- **依赖关系**：
  ```
  go-core/arch/parser.go
    ↳ go-core/arch/types.go (L1, FileInfo 等类型)
    ↳ go-core/errors.go (L0, 错误处理)
    ↳ standard library: go/ast, go/parser, go/token (标准库，L0-L3 允许)
  ```

### 实现步骤
1. 从 parser_ast.go 提取核心 AST 遍历逻辑，封装为纯函数
2. 从 parser_file.go 提取文件遍历和包识别逻辑
3. 统一错误处理为 L0 的错误类型
4. 去除与 HTTP/可视化相关的代码（仅保留纯解析）
5. 确保所有函数为纯函数：不修改全局状态，不产生副作用（除读文件外）
6. 原文件保留（后续任务统一删除），但暂时标记为 deprecated

### 验收标准（严格 AC）
1. [ ] **功能正确**：`go test ./go-core/arch/` 中测试通过，解析结果与原 arch-manager 一致
2. [ ] **层级合规**：L1 层，无 L2+ 依赖
3. [ ] **依赖方向合规**：仅 import L0/L1 + 标准库
4. [ ] **四原语模式**：使用 Atom 接口（纯函数签名），禁止裸写 func
5. [ ] **文件行数合规**：`go-core/arch/parser.go` ≤ 300 行，如需拆分为 parser_file.go + parser_ast.go 亦可
6. [ ] **可观测性**：解析函数中可注入 Observation 钩子（通过参数传入，非强制）
7. [ ] **测试覆盖**：`go-core/arch/parser_test.go`，覆盖：空目录、单文件、多文件、含语法错误文件
8. [ ] **编译通过**：`go build ./go-core/...` 无错误
9. [ ] **无 fmt.Println**：纯逻辑，通过 error 返回值表达错误
10. [ ] **文档完整性**：本节完整，AC 可执行

### 风险与注意事项
- AST 解析涉及 `go/parser`，这是标准库，L1 层允许使用
- 需处理解析失败的情况（文件不存在、语法错误等），使用 L0 的错误类型
- 路径处理需兼容 Windows/Unix（filepath.Clean 等）

---

## 任务单元 3：提取架构分析 Atom

### 目标
将 arch-manager 的 builder.go（架构构建）和 violations.go（违规检测）提取为统一的架构分析 Atom：`go-core/arch/analyzer.go`。

### 背景
arch-manager 的 builder.go（181 行）和 violations.go（171 行）实现了：
- 从 FileInfo 列表构建 ArchData（分层统计、原语识别）
- 检测架构违规（跨层依赖、反向依赖、循环依赖等）
- 计算健康评分

这些逻辑是三模块共享的核心功能，当前仅 arch-manager 拥有。

### 输入输出
- **输入**：
  - `cmd/arch-manager/builder.go`（ArchData 构建逻辑）
  - `cmd/arch-manager/violations.go`（违规检测逻辑）
  - `cmd/arch-manager/primitives.go`（四原语识别逻辑）
- **输出**：
  - `go-core/arch/analyzer.go`（纯函数 Atom：输入 []FileInfo → *ArchData + []Violation，≤ 300 行）

### 架构设计
- **层级归属**：L1 — 纯函数计算，从输入数据推导输出数据
- **原语类型**：Atom
- **函数签名**：
  ```go
  func AnalyzeArchitecture(files []core.FileInfo) (*core.ArchData, error)
  func DetectViolations(data *core.ArchData) ([]core.Violation, error)
  func CalculateHealthScore(violations []core.Violation, fileCount int) float64
  ```
- **依赖关系**：
  ```
  go-core/arch/analyzer.go
    ↳ go-core/arch/types.go (L1, ArchData, Violation 等)
    ↳ go-core/errors.go (L0)
  ```

### 实现步骤
1. 从 builder.go 提取 BuildArchData 逻辑（文件分层、统计聚合）
2. 从 violations.go 提取违规检测规则（反向依赖、跨层跳跃、循环依赖）
3. 从 primitives.go 提取四原语识别模式（正则/关键字匹配）
4. 统一为纯函数风格：输入 → 输出，无全局状态
5. 健康评分算法保持与 arch-manager 当前实现一致
6. 违规定义枚举化（ViolationType: ReverseDependency, CrossLayerJump, CircularDependency 等）

### 验收标准（严格 AC）
1. [ ] **功能正确**：对相同输入，输出 ArchData 和 Violations 与原 arch-manager 实现一致（可通过 diff 验证）
2. [ ] **层级合规**：L1 层，仅依赖 L0/L1
3. [ ] **依赖方向合规**：无 L2+ import
4. [ ] **四原语模式**：使用 Atom 接口
5. [ ] **文件行数合规**：如需拆分为 analyzer_build.go + analyzer_violations.go，每个均 ≤ 300 行
6. [ ] **可观测性**：分析过程可通过 Observation Pipeline 记录（通过可选参数注入）
7. [ ] **测试覆盖**：`analyzer_test.go` 覆盖：空输入、单层项目、8 层项目、含违规项目
8. [ ] **编译通过**：`go build ./go-core/...`
9. [ ] **无 fmt.Println**：无 print
10. [ ] **文档完整性**：本节完整

### 风险与注意事项
- 四原语识别依赖于特定的命名约定（Atom/Port/Adapter/Composer），需确保识别逻辑完整迁移
- 健康评分算法：保持与当前 arch-manager 完全一致，避免行为漂移
- violations.go 中可能存在与 arch-manager GUI 绑定的逻辑（如生成 UI 友好的错误消息），需剥离

---

## 任务单元 4：提取模板引擎 Adapter

### 目标
统一 arch-dev 的 arch_scaffold.go（字符串替换模板）和 lec 的 scaffold.go（embed.FS + text/template）为单一模板引擎 Adapter：`go-core/arch/renderer.go`。

### 背景
当前两套模板系统：
- arch-dev：使用 `strings.ReplaceAll` 的简单字符串替换（arch_scaffold.go 111 行，arch_tmpl_*.go 3 文件）
- lec：使用标准库 `embed.FS` + `text/template`（scaffold.go 140 行 + templates/ 目录）

**lec 的方案更优雅、可维护**：模板文件独立存放，支持条件/循环等模板语法。统一为 lec 风格。

### 输入输出
- **输入**：
  - `cmd/lec/scaffold.go`（embed.FS 引擎，模板渲染逻辑）
  - `cmd/lec/templates/`（模板文件目录）
  - `cmd/arch-dev/arch_scaffold.go` + `arch_tmpl_*.go`（字符串替换模板，参考功能）
- **输出**：
  - `go-core/arch/renderer.go`（模板引擎 Adapter：输入模板路径 + 数据 → 渲染后的字符串，≤ 300 行）
  - `go-core/arch/templates/`（统一的模板文件目录，通过 embed.FS 嵌入）

### 架构设计
- **层级归属**：L1 — Adapter 协议转换（模板文件 → Go 代码字符串）
- **原语类型**：Adapter
- **接口签名**：
  ```go
  type TemplateRenderer interface {
      Render(ctx context.Context, templatePath string, data interface{}) (string, error)
      RenderToFile(ctx context.Context, templatePath, outputPath string, data interface{}) error
      ListTemplates() []string
  }

  type EmbedRenderer struct {
      fs embed.FS
  }
  ```
- **依赖关系**：
  ```
  go-core/arch/renderer.go
    ↳ standard library: embed, text/template, os, path/filepath (L0 层允许)
    ↳ go-core/errors.go (L0)
  go-core/arch/templates/
    ↳ l0/  (基础错误处理模板)
    ↳ l1/  (四原语模板)
    ↳ l3/  (韧性模板)
    ↳ ...  (根据 arch-dev 的 arch_tmpl_*.go 补充)
  ```

### 实现步骤
1. 将 lec 的 templates/ 目录迁移到 go-core/arch/templates/
2. 将 arch-dev 的 arch_tmpl_l0l1.go, arch_tmpl_l2.go, arch_tmpl_l4l7.go 中的模板字符串转换为模板文件
3. 实现 renderer.go：封装 embed.FS + text/template 的组合
4. 提供统一的模板数据结构（TemplateData），与 arch-dev 保持兼容
5. 实现错误处理：模板不存在、渲染失败等场景

### 验收标准（严格 AC）
1. [ ] **功能正确**：`go test ./go-core/arch/` 中模板测试通过，渲染结果与原 lec/arch-dev 一致
2. [ ] **层级合规**：L1 层，仅依赖 L0 + 标准库
3. [ ] **依赖方向合规**：无 L2+ import
4. [ ] **四原语模式**：实现 Adapter 接口（TemplateRenderer）
5. [ ] **文件行数合规**：renderer.go ≤ 300 行；如需额外文件（如 template_data.go）也需 ≤ 300 行
6. [ ] **可观测性**：渲染过程可通过 Observation Pipeline 记录（可选注入）
7. [ ] **测试覆盖**：`renderer_test.go` 覆盖：模板存在/不存在、数据填充正确、多文件渲染
8. [ ] **编译通过**：`go build ./go-core/...`
9. [ ] **无 fmt.Println**：无 print
10. [ ] **文档完整性**：本节完整

### 风险与注意事项
- 模板文件使用 `.tmpl` 或 `.go.tmpl` 扩展名，避免与实际代码混淆
- arch-dev 的模板是内联字符串，需要转换为独立文件，注意转义字符
- 模板语法从 `{{.Placeholder}}` 风格转换为 `text/template` 风格

---

## 任务单元 5：提取架构校验 Port

### 目标
将 arch-dev 的 arch_check.go（142 行）和 arch-manager 的 violations.go（违规检测相关）统一为架构校验 Port：`go-core/arch/validator.go`。

### 背景
arch-dev 的 arch_check.go 实现了基于 CLI 的架构检查，arch-manager 的 violations.go 实现了基于 HTTP 的违规检测。两者的核心校验逻辑相同，但调用方式不同（CLI vs HTTP）。

### 输入输出
- **输入**：
  - `cmd/arch-dev/arch_check.go`（CLI 校验逻辑）
  - `cmd/arch-manager/violations.go`（违规检测规则，与任务单元 3 共享）
- **输出**：
  - `go-core/arch/validator.go`（校验 Port：输入项目路径 → ValidationResult，≤ 300 行）

### 架构设计
- **层级归属**：L1 — Port 边界接口（读取项目文件系统，产生校验结果）
- **原语类型**：Port
- **接口签名**：
  ```go
  type Validator interface {
      Validate(ctx context.Context, projectPath string) (*ValidationResult, error)
      ValidateFile(ctx context.Context, filePath string) (*FileValidationResult, error)
  }

  type ValidationResult struct {
      FileCount      int
      ViolationCount int
      Violations     []core.Violation
      HealthScore    float64
      Duration       time.Duration
  }
  ```
- **依赖关系**：
  ```
  go-core/arch/validator.go
    ↳ go-core/arch/types.go (L1)
    ↳ go-core/arch/parser.go (L1, 读取解析)
    ↳ go-core/arch/analyzer.go (L1, 分析)
    ↳ go-core/errors.go (L0)
  ```

### 实现步骤
1. 从 arch_check.go 提取校验流程：解析 → 分析 → 汇总 → 输出结果
2. 复用任务单元 3 的 analyzer.go 中的 AnalyzeArchitecture + DetectViolations
3. 实现结果汇总：统计违规、计算健康评分、记录耗时
4. 提供文件级和项目级两个入口方法
5. 错误处理：项目路径不存在、解析失败等场景

### 验收标准（严格 AC）
1. [ ] **功能正确**：对当前项目运行校验，结果与 arch-manager/violations.go 一致（违规数 = 0）
2. [ ] **层级合规**：L1 层
3. [ ] **依赖方向合规**：仅依赖 L0/L1
4. [ ] **四原语模式**：实现 Validator Port 接口
5. [ ] **文件行数合规**：validator.go ≤ 300 行
6. [ ] **可观测性**：校验过程可注入 Observation Pipeline
7. [ ] **测试覆盖**：`validator_test.go` 覆盖：合规项目、含违规项目、空目录、不存在路径
8. [ ] **编译通过**：`go build ./go-core/...`
9. [ ] **无 fmt.Println**：无 print
10. [ ] **文档完整性**：本节完整

### 风险与注意事项
- 校验逻辑会调用 parser 和 analyzer，需确保任务单元 2、3 先完成
- 校验结果格式需同时支持 CLI 输出（arch-dev）和 HTTP JSON 输出（arch-manager）

---

## 任务单元 6：提取代码生成 Composer

### 目标
将 arch-dev 的代码生成功能（arch_init.go, arch_new.go, arch_add.go, arch_generate*.go）整合为统一的代码生成 Composer：`go-core/arch/generator.go`。

### 背景
arch-dev 实现了多种代码生成命令：
- init：初始化新项目
- new：创建新文件/模块
- add：添加新功能
- generate：从模板生成代码

这些命令共享相似的流程：解析参数 → 加载模板 → 渲染 → 写入文件。可统一为 Composer 编排。

### 输入输出
- **输入**：
  - `cmd/arch-dev/arch_init.go`（项目初始化）
  - `cmd/arch-dev/arch_new.go`（新建模块）
  - `cmd/arch-dev/arch_add.go`（添加功能）
  - `cmd/arch-dev/arch_scaffold.go`（脚手架）
  - `cmd/arch-dev/arch_commands.go`（命令路由）
- **输出**：
  - `go-core/arch/generator.go`（代码生成 Composer：输入 GenConfig → 生成文件/目录结构，≤ 300 行）

### 架构设计
- **层级归属**：L1 — Composer（编排 parser/analyzer/renderer/validator 的流程）
- **原语类型**：Composer
- **接口签名**：
  ```go
  type Generator interface {
      // 初始化新项目
      InitProject(ctx context.Context, config ProjectConfig) error
      // 创建新模块
      NewModule(ctx context.Context, config ModuleConfig) error
      // 添加新功能
      AddFeature(ctx context.Context, config FeatureConfig) error
      // 从模板生成文件
      GenerateFromTemplate(ctx context.Context, templatePath, outputPath string, data interface{}) error
  }

  type ArchitectureGenerator struct {
      renderer *EmbedRenderer  // 依赖任务单元 4 的模板引擎
  }
  ```
- **依赖关系**：
  ```
  go-core/arch/generator.go
    ↳ go-core/arch/renderer.go (L1, 模板渲染)
    ↳ go-core/arch/types.go (L1)
    ↳ go-core/errors.go (L0)
    ↳ standard library: os, path/filepath (L0)
  ```

### 实现步骤
1. 从 arch_init.go 提取项目初始化流程（创建目录、go.mod、CLAUDE.md 等）
2. 从 arch_new.go 提取模块创建流程（按层级生成 .go 文件）
3. 从 arch_add.go 提取功能添加流程（识别现有结构，插入新代码）
4. 统一使用 renderer.go（任务单元 4）渲染模板
5. 实现文件写入：确保目录存在、避免覆盖已有文件（提供 force 选项）
6. 生成的代码需符合 CLAUDE.md 规范（四原语命名、≤ 300 行等）

### 验收标准（严格 AC）
1. [ ] **功能正确**：`go test ./go-core/arch/` 中生成测试通过；生成的项目能 `go build` 通过
2. [ ] **层级合规**：L1 Composer，仅依赖 L0/L1
3. [ ] **依赖方向合规**：无 L2+ import
4. [ ] **四原语模式**：实现 Generator Composer 接口
5. [ ] **文件行数合规**：generator.go ≤ 300 行，如需拆分为多个文件（如 gen_init.go, gen_new.go）亦可
6. [ ] **可观测性**：生成过程记录到 Observation Pipeline（可选）
7. [ ] **测试覆盖**：`generator_test.go` 覆盖：init 项目、new 模块、add 功能
8. [ ] **编译通过**：`go build ./go-core/...`
9. [ ] **无 fmt.Println**：无 print
10. [ ] **文档完整性**：本节完整

### 风险与注意事项
- 需确保任务单元 4（模板引擎）先完成
- 生成的文件必须遵守 CLAUDE.md 约束（这是该 Generator 的核心价值）
- 文件写入需小心：避免误覆盖用户已有代码
- arch_add.go 的"识别现有结构并插入新代码"比较复杂，初期可简化为"生成新文件"

---

## 任务单元 7：构建 ArchitecturePipeline Composer

### 目标
将以上 L1 原语（parser → analyzer → validator → renderer → generator）编排为一个统一的 ArchitecturePipeline Composer。

### 背景
整合后的核心价值：用户只需调用一个入口，即可完成"解析 → 分析 → 校验 → 生成"的完整流程。这是 arch-manager 和 arch-dev 功能的统一编排。

### 输入输出
- **输入**：项目路径（字符串）
- **输出**：PipelineResult（包含 ArchData + Violations + 生成的文件列表）

### 架构设计
- **层级归属**：L1 — Composer（顶层编排器）
- **原语类型**：Composer
- **接口签名**：
  ```go
  type ArchitecturePipeline interface {
      // 完整流程：解析 → 分析 → 校验 → (可选) 生成
      Run(ctx context.Context, projectPath string) (*PipelineResult, error)
      // 仅分析（不生成）
      Analyze(ctx context.Context, projectPath string) (*core.ArchData, error)
      // 仅校验（不生成）
      Validate(ctx context.Context, projectPath string) (*ValidationResult, error)
  }

  type Pipeline struct {
      parser    ParserAtom
      analyzer  AnalyzerAtom
      validator ValidatorPort
      generator GeneratorComposer
      obs       core.ObservationAdapter  // 可注入 L5
  }
  ```
- **依赖关系**：
  ```
  go-core/arch/pipeline.go
    ↳ go-core/arch/parser.go (L1, Atom)
    ↳ go-core/arch/analyzer.go (L1, Atom)
    ↳ go-core/arch/validator.go (L1, Port)
    ↳ go-core/arch/generator.go (L1, Composer)
    ↳ go-core/observation_aggregator.go (L5, 可观测性)
  ```

### 实现步骤
1. 定义 Pipeline 结构体，包含各原语的引用
2. 实现 Run 方法：顺序调用 parser → analyzer → validator → generator
3. 实现 Analyze 方法：仅 parser + analyzer
4. 实现 Validate 方法：parser + analyzer + validator
5. 每个步骤的错误处理使用 L0 错误类型
6. 通过 ObservationAdapter 记录每个步骤的执行情况（L5 可观测性）

### 验收标准（严格 AC）
1. [ ] **功能正确**：对当前项目运行 Run，输出正确的 ArchData + ValidationResult
2. [ ] **层级合规**：L1 Composer，正确编排同层原语
3. [ ] **依赖方向合规**：仅 import L0/L1；L5 依赖通过接口注入（编译时依赖）
4. [ ] **四原语模式**：完整的 Composer 实现，编排 Atom/Port/Composer
5. [ ] **文件行数合规**：pipeline.go ≤ 300 行
6. [ ] **可观测性**：每个步骤有 Observation 记录（开始/结束/耗时/错误）
7. [ ] **测试覆盖**：`pipeline_test.go` 覆盖：完整流程、部分流程、错误处理
8. [ ] **编译通过**：`go build ./go-core/...`
9. [ ] **无 fmt.Println**：无 print
10. [ ] **文档完整性**：本节完整

### 风险与注意事项
- 需确保任务单元 2-6 先完成
- L5 Observation 通过接口注入（而非直接 import），保持依赖方向正确（L1 → L5 通过接口抽象，实际由 L7 注入）
- Pipeline 的错误处理需明确：哪个步骤失败、失败原因、是否可恢复

---

## 任务单元 8：构建统一 CLI 入口 arch-cli

### 目标
将 arch-dev 的 main.go（CLI 入口）和 arch-manager 的 CLI 相关功能整合为统一的 `cmd/arch-cli`。

### 背景
当前：
- arch-dev main.go 76 行：基于 flag 的简单 CLI，支持 init/new/add/check/analyze 等命令
- arch-manager main.go 78 行：HTTP 服务器启动，无完整 CLI
- lec main.go 150 行：基于 flag 的脚手架 CLI

整合后：一个统一的 `arch` CLI 工具，支持所有命令。

### 输入输出
- **输入**：
  - `cmd/arch-dev/main.go`（CLI 框架）
  - `cmd/arch-dev/arch_commands.go`（命令注册）
  - `cmd/arch-dev/arch_analyze.go`（analyze 命令）
  - `cmd/arch-dev/arch_agent.go`（agent 命令）
  - `cmd/lec/main.go`（参考：lec CLI）
- **输出**：
  - `cmd/arch-cli/main.go`（统一 CLI 入口，≤ 300 行）
  - `cmd/arch-cli/commands.go`（命令路由与参数解析，≤ 300 行）

### 架构设计
- **层级归属**：L7 — 应用层入口
- **原语类型**：应用层，调用 L1 的 ArchitecturePipeline Composer
- **命令列表**：
  ```
  arch analyze --dir ./project      # 分析项目架构
  arch check --dir ./project        # 检查架构违规
  arch validate --dir ./project     # 完整校验（同 check）
  arch init myproject --tier microservice  # 初始化新项目
  arch new --project ./p --tier l2        # 新建层级模块
  arch add --feature "order processing"   # 添加功能
  arch agent --task "..."                 # Agent 任务
  arch serve --port 8090                  # 启动 Web UI
  arch version                           # 版本信息
  ```
- **依赖关系**：
  ```
  cmd/arch-cli/
    ↳ go-core/arch/pipeline.go (L1, ArchitecturePipeline)
    ↳ go-core/arch/generator.go (L1, Generator)
    ↳ go-core/arch/types.go (L1)
    ↳ cmd/arch-server/ (L7, 当 serve 命令调用时)
  ```

### 实现步骤
1. 从 arch-dev main.go 提取 CLI 框架（flag 解析、命令分发）
2. 从 arch_commands.go 提取命令注册逻辑
3. 将各命令的实现替换为调用 go-core/arch 的原语
4. 实现 arch serve 命令：启动 arch-server HTTP 服务器（任务单元 9）
5. 输出格式：支持文本（默认）和 JSON（--json）
6. 统一错误码：0=成功, 1=违规, 2=内部错误

### 验收标准（严格 AC）
1. [ ] **功能正确**：`go run ./cmd/arch-cli/ analyze --dir ./go-core` 输出正确的架构数据
2. [ ] **层级合规**：L7 应用层，正确调用 L1 原语
3. [ ] **依赖方向合规**：仅 import L0/L1/L5（通过注入）/自身
4. [ ] **四原语模式**：调用 L1 的 ArchitecturePipeline Composer
5. [ ] **文件行数合规**：main.go ≤ 300 行，commands.go ≤ 300 行
6. [ ] **可观测性**：CLI 运行时可输出 verbose 日志（通过 L5 Observation）
7. [ ] **测试覆盖**：`cmd/arch-cli/cli_test.go` 覆盖主要命令的参数解析和调用流程
8. [ ] **编译通过**：`go build -o arch.exe ./cmd/arch-cli/`
9. [ ] **无 fmt.Println**：使用 log 或输出到 stdout（L7 应用层允许输出到控制台）
10. [ ] **文档完整性**：本节完整

### 风险与注意事项
- CLI 属于 L7 应用层，允许使用 fmt（用户交互输出）
- 实际逻辑必须委托给 L1 的原语，CLI 仅负责参数解析和结果格式化
- serve 命令需等待任务单元 9 完成后实现

---

## 任务单元 9：构建统一 HTTP 服务器 arch-server

### 目标
将 arch-manager 的 HTTP 相关文件（routes.go, main.go, 各 handler 文件）整合为统一的 `cmd/arch-server`。

### 背景
arch-manager 提供完整的 Web UI：
- routes.go（217 行）：HTTP 路由注册
- main.go（78 行）：HTTP 服务器启动
- guardian.go, violations.go, migrate_*.go, version_*.go, agent.go 等：各 API handler
- arch-manager.html 等静态文件：Web 前端

这些功能需要迁移到 arch-server，同时将核心逻辑委托给 go-core/arch。

### 输入输出
- **输入**：
  - `cmd/arch-manager/main.go`（服务器启动）
  - `cmd/arch-manager/routes.go`（路由表）
  - `cmd/arch-manager/guardian.go`（Guardian API）
  - `cmd/arch-manager/violations.go`（违规 API）
  - `cmd/arch-manager/builder.go`（架构数据 API，部分逻辑）
  - `cmd/arch-manager/agent.go`（Agent API）
  - `cmd/arch-manager/entropy_observe.go`（熵监控 API）
  - `cmd/arch-manager/health.go`（健康检查 API）
  - `cmd/arch-manager/*.html` 等静态文件
  - `cmd/arch-manager/migrate_*.go`（迁移 API）
  - `cmd/arch-manager/version_*.go`（版本 API）
  - `cmd/arch-manager/sse.go`（SSE 实时推送）
- **输出**：
  - `cmd/arch-server/main.go`（服务器入口，≤ 300 行）
  - `cmd/arch-server/routes.go`（路由注册，≤ 300 行）
  - `cmd/arch-server/handlers/`（各 API handler，按功能拆分文件）
  - `cmd/arch-server/static/`（Web 前端静态文件）

### 架构设计
- **层级归属**：L7 — 应用层（HTTP 服务器）
- **原语类型**：应用层，调用 L1 的 ArchitecturePipeline，调用 L4 Guardian、L5 Observation
- **API 路由（保留 arch-manager 的主要 API）**：
  ```
  GET  /api/arch                  # 架构数据（委托 go-core/arch/analyzer）
  GET  /api/violations            # 违规列表（委托 go-core/arch/validator）
  GET  /api/primitives            # 原语识别
  GET  /api/health                # 健康检查
  GET  /api/guardian/status       # Guardian 状态
  GET  /api/entropy/stream        # 熵监控 SSE
  POST /api/agent/task            # Agent 任务提交
  GET  /api/version               # 版本信息
  GET  /                          # Web UI 首页
  ```
- **依赖关系**：
  ```
  cmd/arch-server/
    ↳ go-core/arch/pipeline.go (L1, 架构分析)
    ↳ go-core/guardian.go (L4, Guardian 监督)
    ↳ go-core/observation_aggregator.go (L5, 可观测性)
    ↳ go-core/eventstore.go (L6, 事件溯源)
    ↳ standard library: net/http, encoding/json
  ```

### 实现步骤
1. 从 arch-manager main.go 提取 HTTP 服务器启动逻辑
2. 从 routes.go 提取路由注册，统一使用 http.ServeMux
3. 每个 API handler 重写为调用 go-core/arch 的原语（而非 arch-manager 内部实现）
4. 将静态文件（HTML/JS/CSS）迁移到 arch-server/static/
5. 实现 CORS 和基本的配置（端口、静态目录等）
6. SSE（实时推送）保持原 arch-manager 的实现，作为 L7 的实时输出机制

### 验收标准（严格 AC）
1. [ ] **功能正确**：`go run ./cmd/arch-server/` 启动后，访问 `http://localhost:8090/api/arch` 返回正确的 JSON
2. [ ] **层级合规**：L7 应用层，正确分层
3. [ ] **依赖方向合规**：仅 import L0/L1/L4/L5/L6 + 标准库
4. [ ] **四原语模式**：handler 中实际逻辑委托给 L1 的 ArchitecturePipeline Composer
5. [ ] **文件行数合规**：main.go ≤ 300 行，每个 handler 文件 ≤ 300 行
6. [ ] **可观测性**：HTTP 请求通过 L5 Observation 记录
7. [ ] **测试覆盖**：`cmd/arch-server/server_test.go` 覆盖主要 API 的 HTTP 测试
8. [ ] **编译通过**：`go build -o arch-server.exe ./cmd/arch-server/`
9. [ ] **无 fmt.Println**：使用 log 或 Observation Pipeline（L7 允许必要的 HTTP 日志输出）
10. [ ] **文档完整性**：本节完整

### 风险与注意事项
- 这是最复杂的任务单元，涉及 arch-manager 大部分文件的迁移
- 部分 handler（如 guardian.go, entropy_observe.go, migrate_*.go）本身包含业务逻辑，需判断：
  - 通用逻辑 → 迁移到 go-core/arch（或合适的 Lx 层）
  - UI 特定逻辑 → 保留在 arch-server/handlers/
- SSE 实现较复杂，建议直接迁移原实现
- Web 前端静态文件需与路由配合（提供 static file server）

---

## 任务单元 10：Agent 与 Guardian 功能迁移

### 目标
将 arch-manager 中与 Agent 和 Guardian 相关的功能迁移到合适的层级，并在 arch-cli/arch-server 中暴露。

### 背景
arch-manager 包含以下与 Agent 和 Guardian 相关的功能：
- agent.go（243 行）：Agent 提交/管理
- ua.go, ua_search.go, ua_validate.go：用户代理识别
- guardian.go（190 行）+ guardian_stub.go：Guardian 监督
- entropy_observe.go（214 行）：熵监控
- migrate_*.go（5 文件）：架构迁移/回滚

这些是系统的核心治理能力，但当前仅 arch-manager 拥有。

### 输入输出
- **输入**：
  - `cmd/arch-manager/agent.go`（Agent 管理）
  - `cmd/arch-manager/ua.go, ua_search.go, ua_validate.go`（用户代理识别）
  - `cmd/arch-manager/guardian.go`（Guardian API）
  - `cmd/arch-manager/entropy_observe.go`（熵监控）
  - `cmd/arch-manager/migrate_api.go, migrate_analyze.go, migrate_execute.go, migrate_rollback.go, migrate_sse.go`（迁移功能）
- **输出**：
  - Agent 相关类型/逻辑 → 迁移到 `go-core/arch/agent.go`（或 go-core/guardian 相关文件，视内容而定）
  - Guardian API handler → `cmd/arch-server/handlers/guardian.go`
  - Agent API handler → `cmd/arch-server/handlers/agent.go`
  - Entropy API handler → `cmd/arch-server/handlers/entropy.go`
  - 迁移功能 → `cmd/arch-server/handlers/migrate.go`
  - CLI 命令 → `cmd/arch-cli/commands.go` 中添加对应命令

### 架构设计
- **层级归属**：
  - 类型/数据结构 → L1（与 go-core/arch/types.go 同层）
  - 核心业务逻辑 → L4（Guardian 监督层）
  - API handler → L7（arch-server）
  - CLI 命令 → L7（arch-cli）
- **原语类型**：
  - Guardian 决策 → L4 Atom/Port
  - Agent 管理 → L4 Port（外部系统交互边界）
  - 熵监控 → L4 Atom（纯计算：输入架构状态 → 熵值）

### 实现步骤
1. 阅读 agent.go, ua_*.go：识别核心数据结构（Agent, AgentTask 等），迁移到 go-core/arch/agent.go
2. 阅读 guardian.go：识别 Guardian 的决策逻辑，迁移到 go-core 的 L4 Guardian 层（如 guardian_*.go 已有部分实现，检查是否需补充）
3. 阅读 entropy_observe.go：识别熵计算逻辑，迁移到 go-core 的 L4 Guardian 熵监控层
4. 阅读 migrate_*.go：识别迁移执行和回滚逻辑，核心逻辑迁移到 go-core，UI 部分保留在 arch-server
5. 在 arch-server handlers/ 目录下创建各功能的 handler 文件
6. 在 arch-cli 中添加对应的 CLI 命令（如 `arch agent`, `arch guardian check`, `arch entropy`, `arch migrate`）

### 验收标准（严格 AC）
1. [ ] **功能正确**：arch-server 的 /api/guardian/*, /api/agent/*, /api/entropy/* API 返回正确结果
2. [ ] **层级合规**：核心逻辑在 L1/L4，UI handler 在 L7
3. [ ] **依赖方向合规**：无反向依赖（高层调用低层，低层不 import 高层）
4. [ ] **四原语模式**：Guardian 决策使用 Atom/Port，Agent 使用 Port 接口
5. [ ] **文件行数合规**：每个迁移后的文件 ≤ 300 行
6. [ ] **可观测性**：Guardian/Agent/Entropy 的运行通过 L5 Observation 记录
7. [ ] **测试覆盖**：各功能单元测试，确保决策/监控逻辑正确
8. [ ] **编译通过**：`go build ./...` 全项目通过
9. [ ] **无 fmt.Println**：L1/L4 无 print；L7 handler 允许必要的日志输出
10. [ ] **文档完整性**：本节完整

### 风险与注意事项
- Agent 和 Guardian 的逻辑可能复杂，需仔细判断哪些属于"核心业务"（→ go-core），哪些属于"UI/API 适配"（→ arch-server）
- migrate_*.go 共 5 个文件，迁移量较大，可能需要进一步拆分（migrate_api, migrate_analyze, migrate_execute, migrate_rollback, migrate_sse）
- entropy_observe.go 包含 SSE 推送逻辑，这是 L7 的实时输出，核心计算逻辑应剥离到 L4

---

## 任务单元 11：清理旧目录与最终验证

### 目标
删除原有的 arch-manager、arch-dev、lec 三个目录，验证整合后的系统完全可用。

### 背景
任务单元 1-10 完成后，原有的三个目录（arch-manager, arch-dev, lec）中的代码已全部迁移到 go-core/arch/ 和 cmd/arch-cli/, cmd/arch-server/。此时旧目录应可以安全删除。

### 输入输出
- **输入**：
  - `cmd/arch-manager/`（32 文件）
  - `cmd/arch-dev/`（13 文件）
  - `cmd/lec/`（2 文件 + templates）
- **输出**：
  - 删除以上三个目录
  - 更新项目文档（如 README.md，如有），说明新的使用方式

### 架构设计
- **层级归属**：无（删除操作）
- **影响范围**：cmd/ 目录结构变更

### 实现步骤
1. 确认任务单元 1-10 全部完成，所有功能测试通过
2. 运行 `go build ./...` 确保删除旧目录后编译通过
3. 删除 `cmd/arch-manager/` 目录
4. 删除 `cmd/arch-dev/` 目录
5. 删除 `cmd/lec/` 目录
6. 运行完整测试：`go test ./...`
7. 启动 arch-server 验证 Web UI：`go run ./cmd/arch-server/`
8. 验证 arch-cli 命令：`go run ./cmd/arch-cli/ analyze --dir ./go-core`
9. 如项目有 CI/CD 配置，更新相关路径

### 验收标准（严格 AC）
1. [ ] **功能正确**：删除旧目录后，arch-cli 和 arch-server 全部功能正常
2. [ ] **层级合规**：go-core/arch/ 中代码正确归属 L1 层
3. [ ] **依赖方向合规**：无反向依赖
4. [ ] **四原语模式**：所有业务逻辑使用四原语实现
5. [ ] **文件行数合规**：所有文件 ≤ 300 行
6. [ ] **可观测性**：arch-server 的 Observation 正常工作
7. [ ] **测试覆盖**：`go test ./...` 全部通过
8. [ ] **编译通过**：`go build ./...` 全部通过
9. [ ] **无 fmt.Println**：L0-L6 无 print
10. [ ] **文档完整性**：更新了相关文档说明使用方法

### 风险与注意事项
- 删除操作不可逆，执行前必须确保任务单元 1-10 全部通过
- 建议在独立分支上执行删除，验证无误后再合并
- 删除前可做一次备份（或依赖 git 历史）
- arch-manager 的静态文件（HTML/JS/CSS）必须已迁移到 arch-server/static/，否则 Web UI 会缺失

---

## 任务依赖图

```
任务单元 1 (类型定义) ─┐
                        ├→ 任务单元 2 (Parser) ─┐
                        │                        ├→ 任务单元 3 (Analyzer) ┐
                        │                        │                         ├→ 任务单元 5 (Validator)
                        │                        │                         │
                        │                        └→ 任务单元 4 (Renderer) ┼→ 任务单元 6 (Generator)
                        │                                                  │
                        └──────────────────────────────────────────────────┼→ 任务单元 7 (Pipeline)
                                                                           │
                        任务单元 8 (arch-cli)  ←───────────────────────────┘
                                   ↑
                        任务单元 9 (arch-server) ←── 任务单元 10 (Agent/Guardian)
                                   ↑
                        任务单元 11 (清理 + 验证)
```

**并行可执行的任务组**：
- 组 A（最早）：任务单元 1（仅需这一个开始）
- 组 B（任务单元 1 完成后）：任务单元 2 + 任务单元 4（并行）
- 组 C（2+4 完成后）：任务单元 3 + 任务单元 6（并行）+ 任务单元 5
- 组 D（3+4+5+6 完成后）：任务单元 7（Pipeline）
- 组 E（7 完成后）：任务单元 8 + 任务单元 9（并行）
- 组 F（9 进行中）：任务单元 10（Agent/Guardian）
- 组 G（全部完成后）：任务单元 11

---

## 总体验收标准（完成全部任务后验证）

1. [ ] **编译**：`go build ./...` 全项目 0 错误
2. [ ] **测试**：`go test ./...` 全项目 100% 通过
3. [ ] **架构违规**：arch-manager 检查违规数 = 0
4. [ ] **文件行数**：所有 .go 文件 ≤ 300 行
5. [ ] **CLI 功能**：arch-cli 所有命令正常运行
6. [ ] **Web UI**：arch-server Web 页面正常访问
7. [ ] **四原语模式**：go-core/arch/ 中所有代码使用 Atom/Port/Adapter/Composer

---

## 参考资源

- CLAUDE.md（项目约束文档）
- `cmd/arch-manager/`（当前架构管理仪表盘）
- `cmd/arch-dev/`（当前架构开发 CLI）
- `cmd/lec/`（当前低熵代码脚手架）
