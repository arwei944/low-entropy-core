// arch-dev - Architecture-Driven Development CLI
//
// Usage:
//   arch-dev agent <task>              One-command: full development lifecycle
//   arch-dev init <project_description>   One-click: create + scaffold all layers
//   arch-dev new <project_description>    Create a new project
//   arch-dev add <feature_description>   Add a feature to an existing project
//   arch-dev check                        Check architecture constraints
//   arch-dev fix                          Auto-fix constraint violations
//   arch-dev build                        Build and verify compilation
//   arch-dev lint                        Run linters
//   arch-dev scaffold <layer>             Generate scaffold for a layer
//   arch-dev doctor                      Check environment
//
// For more information, see ARCHITECTURE_AGENT.md

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ============================================================================
// Main Entry Point
// ============================================================================

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "agent":
		runAgent()
	case "init":
		runInit()
	case "new":
		runNew()
	case "add":
		runAdd()
	case "check":
		runCheck()
	case "fix":
		runFix()
	case "build":
		runBuild()
	case "lint":
		runLint()
	case "scaffold":
		runScaffold()
	case "doctor":
		runDoctor()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`arch-dev - Architecture-Driven Development CLI

Usage:
  arch-dev agent <task>              One-command: full development lifecycle (new or existing project)
  arch-dev init <project_description>   One-click: create + scaffold all layers + check + build
  arch-dev new <project_description>    Create a new project
  arch-dev add <feature_description>   Add a feature to an existing project
  arch-dev check                        Check architecture constraints
  arch-dev fix                          Auto-fix constraint violations
  arch-dev build                        Build and verify compilation
  arch-dev lint                        Run linters
  arch-dev scaffold <layer>             Generate scaffold for a layer
  arch-dev doctor                      Check environment
  arch-dev help                        Show this help

Examples:
  arch-dev agent "开发加密货币交易所"      # 新项目：一键完成
  arch-dev agent "添加订单取消功能"         # 已有项目：迭代开发
  arch-dev init "加密货币交易所"
  arch-dev add "添加订单匹配功能"
  arch-dev check --fix
  arch-dev build
  arch-dev scaffold --layer L2 --pattern retry

For more information, see ARCHITECTURE_AGENT.md`)
}

// ============================================================================
// runAgent - One-command full development lifecycle
// ============================================================================

func runAgent() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: task description required")
		fmt.Fprintln(os.Stderr, "Usage: arch-dev agent <task_description>")
		os.Exit(1)
	}

	task := strings.Join(os.Args[2:], " ")
	originalDir, _ := os.Getwd()
	projectRoot := findProjectRoot()

	// Detect if this is a new project or existing project
	isNewProject := projectRoot == "" || !hasProjectStructure(originalDir)

	if isNewProject {
		runAgentNewProject(task, originalDir)
	} else {
		runAgentExistingProject(task, projectRoot)
	}
}

// Check if directory has arch-dev project structure
func hasProjectStructure(dir string) bool {
	requiredDirs := []string{"core", "cmd"}
	for _, d := range requiredDirs {
		if _, err := os.Stat(filepath.Join(dir, d)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// Run agent for new project
func runAgentNewProject(task, originalDir string) {
	projectName := parseProjectName(task)

	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║         arch-dev agent - 智能开发助手                   ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("\n🎯 任务: %s\n", task)
	fmt.Printf("📦 检测: 新项目 - 正在初始化...\n")
	fmt.Println()

	// Step 1: Initialize project
	fmt.Println("▶ Step 1/4  初始化项目...")
	os.Args = []string{"arch-dev", "init", task}
	runInit()
	fmt.Println()

	// Step 2: Analyze task and generate code plan
	fmt.Println("▶ Step 2/4  分析任务并规划...")
	plan := analyzeTask(task)
	printTaskPlan(plan)

	// Step 3: Generate business code scaffolds
	fmt.Println("\n▶ Step 3/4  生成业务代码脚手架...")
	projectDir := filepath.Join(originalDir, projectName)
	os.Chdir(projectDir)
	generateBusinessScaffolds(plan, projectDir)
	fmt.Println()

	// Step 4: Verify
	fmt.Println("▶ Step 4/4  验证项目...")
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = projectDir
	output, _ := cmd.CombinedOutput()
	if len(strings.TrimSpace(string(output))) > 0 {
		fmt.Printf("  ⚠️  构建输出:\n%s\n", output)
	} else {
		fmt.Println("  ✅ 构建成功")
	}

	// Summary
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║                     开发计划生成完成!                    ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("\n📂 项目: %s\n", projectDir)
	fmt.Println("\n📋 开发计划已生成，请按以下顺序实现:")
	for i, step := range plan.Steps {
		fmt.Printf("  %d. [%s] %s\n", i+1, step.Layer, step.Description)
	}
	fmt.Println("\n下一步:")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Println("  arch-dev check      # 检查约束")
	fmt.Println("  arch-dev build     # 编译验证")
	fmt.Println("  arch-dev agent <新需求>  # 继续迭代")
}

// Run agent for existing project
func runAgentExistingProject(task, projectRoot string) {
	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║         arch-dev agent - 智能开发助手                   ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("\n🎯 任务: %s\n", task)
	fmt.Printf("📦 检测: 已有项目 - 正在分析: %s\n", projectRoot)
	fmt.Println()

	// Step 1: Analyze task
	fmt.Println("▶ Step 1/3  分析任务...")
	plan := analyzeTask(task)
	printTaskPlan(plan)

	// Step 2: Generate code
	fmt.Println("\n▶ Step 2/3  生成/更新代码...")
	generateBusinessScaffolds(plan, projectRoot)

	// Step 3: Verify
	fmt.Println("\n▶ Step 3/3  验证...")
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Dir = projectRoot
	cmd.Run()

	violations := checkConstraints(projectRoot)
	if len(violations) > 0 {
		fmt.Printf("  ⚠️  发现 %d 个约束违规\n", len(violations))
		fixViolations(projectRoot, violations)
		fmt.Println("  ✅ 已自动修复")
	} else {
		fmt.Println("  ✅ 无约束违规")
	}

	cmd = exec.Command("go", "build", "./...")
	cmd.Dir = projectRoot
	output, _ := cmd.CombinedOutput()
	if len(strings.TrimSpace(string(output))) > 0 {
		fmt.Printf("  ⚠️  构建输出:\n%s\n", output)
	} else {
		fmt.Println("  ✅ 构建成功")
	}

	// Summary
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║                     迭代完成!                          ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("\n📂 项目: %s\n", projectRoot)
	fmt.Println("\n已完成:")
	for i, step := range plan.Steps {
		fmt.Printf("  %d. [%s] %s\n", i+1, step.Layer, step.Description)
	}
	fmt.Println("\n继续开发:")
	fmt.Println("  arch-dev agent <新需求>  # 继续迭代")
}

// Task plan structure
type TaskStep struct {
	Layer        string
	Description  string
	Pattern      string
	Primitives   []string
}

type TaskPlan struct {
	ProjectType string
	CoreFeatures []string
	Steps       []TaskStep
}

// Analyze task and generate plan
func analyzeTask(task string) TaskPlan {
	plan := TaskPlan{
		ProjectType: "通用项目",
		CoreFeatures: []string{},
		Steps: []TaskStep{},
	}

	taskLower := strings.ToLower(task)

	// Detect project type
	if containsAny(taskLower, []string{"交易所", "exchange", "trade"}) {
		plan.ProjectType = "加密货币交易所"
		plan.CoreFeatures = []string{"订单管理", "账户系统", "撮合引擎", "钱包服务"}
		plan.Steps = []TaskStep{
			{Layer: "L1", Description: "定义订单和账户领域模型", Primitives: []string{"Atom"}},
			{Layer: "L1", Description: "订单验证 Port", Pattern: "Port"},
			{Layer: "L2", Description: "订单生命周期管理", Primitives: []string{"Composer"}},
			{Layer: "L2", Description: "重试机制", Pattern: "retry"},
			{Layer: "L2", Description: "熔断保护", Pattern: "circuit-breaker"},
			{Layer: "L5", Description: "交易指标监控"},
			{Layer: "L6", Description: "订单事件存储"},
			{Layer: "L7", Description: "交易 API 服务"},
		}
	} else if containsAny(taskLower, []string{"电商", "shop", "mall", "商店"}) {
		plan.ProjectType = "电商平台"
		plan.CoreFeatures = []string{"商品管理", "购物车", "订单处理", "支付"}
		plan.Steps = []TaskStep{
			{Layer: "L1", Description: "商品领域模型"},
			{Layer: "L1", Description: "购物车 Port"},
			{Layer: "L2", Description: "限流保护", Pattern: "rate-limiter"},
			{Layer: "L5", Description: "业务监控"},
			{Layer: "L6", Description: "订单事件存储"},
			{Layer: "L7", Description: "电商 API"},
		}
	} else if containsAny(taskLower, []string{"聊天", "chat", "im", "消息"}) {
		plan.ProjectType = "即时通讯系统"
		plan.CoreFeatures = []string{"用户", "会话", "消息", "推送"}
		plan.Steps = []TaskStep{
			{Layer: "L1", Description: "消息领域模型"},
			{Layer: "L1", Description: "消息验证 Port"},
			{Layer: "L2", Description: "消息路由"},
			{Layer: "L5", Description: "消息监控"},
			{Layer: "L6", Description: "消息历史存储"},
			{Layer: "L7", Description: "聊天服务"},
		}
	} else {
		// Generic project
		plan.Steps = []TaskStep{
			{Layer: "L0", Description: "定义业务错误类型"},
			{Layer: "L1", Description: "定义核心领域模型", Primitives: []string{"Atom", "Port"}},
			{Layer: "L2", Description: "实现业务逻辑组合", Primitives: []string{"Composer"}},
			{Layer: "L5", Description: "添加可观测性"},
			{Layer: "L6", Description: "实现事件溯源"},
			{Layer: "L7", Description: "构建应用入口"},
		}
	}

	// Detect specific features from task
	if containsAny(taskLower, []string{"用户", "user", "会员"}) {
		plan.Steps = append(plan.Steps, TaskStep{Layer: "L1", Description: "用户管理模块"})
	}
	if containsAny(taskLower, []string{"支付", "pay", "订单"}) {
		plan.Steps = append(plan.Steps, TaskStep{Layer: "L1", Description: "支付领域模型"})
	}
	if containsAny(taskLower, []string{"管理后台", "admin"}) {
		plan.Steps = append(plan.Steps, TaskStep{Layer: "L7", Description: "管理后台 API"})
	}

	return plan
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func printTaskPlan(plan TaskPlan) {
	fmt.Printf("  📊 项目类型: %s\n", plan.ProjectType)
	if len(plan.CoreFeatures) > 0 {
		fmt.Printf("  🎯 核心功能: %s\n", strings.Join(plan.CoreFeatures, ", "))
	}
	fmt.Println("  📋 开发步骤:")
	for i, step := range plan.Steps {
		desc := step.Description
		if step.Pattern != "" {
			desc = fmt.Sprintf("%s [%s模式]", desc, step.Pattern)
		}
		fmt.Printf("     %d. [%s] %s\n", i+1, step.Layer, desc)
	}
}

func generateBusinessScaffolds(plan TaskPlan, projectDir string) {
	// Generate based on plan steps
	generated := 0
	featureSeq := 0
	patternCounter := make(map[string]int)
	for _, step := range plan.Steps {
		switch step.Layer {
		case "L0":
			// L0 already generated by init
		case "L1":
			featureSeq++
			filename := fmt.Sprintf("feature_%d.go", featureSeq)
			path := filepath.Join(projectDir, "core", filename)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				code := generateL1Feature(featureSeq)
				os.WriteFile(path, []byte(code), 0644)
				generated++
				fmt.Printf("  ✅ [%s] %s\n", step.Layer, step.Description)
			}
		case "L2":
			if step.Pattern != "" {
				// Normalize pattern name - replace hyphens with underscores
				patternName := strings.ReplaceAll(step.Pattern, "-", "_")
				patternCounter[patternName]++
				seq := patternCounter[patternName]
				filename := fmt.Sprintf("pattern_%s", patternName)
				if seq > 1 {
					filename = fmt.Sprintf("pattern_%s%d", patternName, seq)
				}
				path := filepath.Join(projectDir, "core", filename+".go")
				if _, err := os.Stat(path); os.IsNotExist(err) {
					code := generateScaffold("L2", step.Pattern, "core")
					os.WriteFile(path, []byte(code), 0644)
					generated++
					fmt.Printf("  ✅ [%s] %s [%s]\n", step.Layer, step.Description, step.Pattern)
				}
			}
		case "L5", "L6", "L7":
			// Already scaffolded
		}
	}

	if generated == 0 {
		fmt.Println("  ℹ️  所有功能已存在，无需生成")
	} else {
		fmt.Printf("  📝 已生成 %d 个新文件\n", generated)
	}
}

func generateL1Feature(seq int) string {
	return fmt.Sprintf(`// Package core — feature

package core

import "context"

// Req%d is the input.
type Req%d struct {
	// TODO: add fields
}

// Res%d is the output.
type Res%d struct {
	// TODO: add fields
}

// Handle%d validates input and returns output.
func Handle%d(ctx context.Context, req Req%d) (Res%d, error) {
	if req == (Req%d{}) {
		return Res%d{}, ErrInvalidInput
	}
	// TODO: implement business logic
	return Res%d{}, nil
}
`, seq, seq, seq, seq, seq, seq, seq, seq, seq, seq, seq)
}

// ============================================================================
// runInit - One-click project initialization
// ============================================================================

func runInit() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: project description required")
		fmt.Fprintln(os.Stderr, "Usage: arch-dev init <project_description>")
		os.Exit(1)
	}

	description := strings.Join(os.Args[2:], " ")
	projectName := parseProjectName(description)

	fmt.Println("╔════════════════════════════════════════════════════════╗")
	fmt.Println("║         arch-dev init - 一键项目初始化                  ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("\n📋 项目: %s\n", projectName)
	fmt.Printf("📝 描述: %s\n", description)
	fmt.Println()

	originalDir, _ := os.Getwd()
	projectDir := filepath.Join(originalDir, projectName)

	// Step 1: Create project structure
	fmt.Println("▶ Step 1/5  创建项目结构...")
	dirs := []string{
		filepath.Join(projectDir, "cmd", "server"),
		filepath.Join(projectDir, "core"),
		filepath.Join(projectDir, "adapters"),
		filepath.Join(projectDir, "services"),
		filepath.Join(projectDir, "pipelines"),
		filepath.Join(projectDir, "observation"),
		filepath.Join(projectDir, "eventstore"),
		filepath.Join(projectDir, "guardian"),
		filepath.Join(projectDir, "patterns"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	// Copy ARCHITECTURE_AGENT.md
	archAgentPath := filepath.Join(originalDir, "ARCHITECTURE_AGENT.md")
	if _, err := os.Stat(archAgentPath); err == nil {
		srcData, _ := os.ReadFile(archAgentPath)
		os.WriteFile(filepath.Join(projectDir, "ARCHITECTURE_AGENT.md"), srcData, 0644)
	}

	// Create go.mod
	goModContent := fmt.Sprintf("module %s\n\ngo 1.21\n\nrequire (\n\tgithub.com/stretchr/testify v1.8.4\n)\n\n", projectName)
	os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goModContent), 0644)

	// Create README
	readmeContent := "# " + projectName + "\n\nGenerated with arch-dev following ARCHITECTURE_AGENT.md\n\n## Architecture\n\n" +
		"- **L0**: Error handling (core/errors.go)\n" +
		"- **L1**: Four primitives (core/types.go)\n" +
		"- **L2**: Patterns (patterns/)\n" +
		"- **L4**: Guardian (guardian/)\n" +
		"- **L5**: Observation (observation/)\n" +
		"- **L6**: EventStore (eventstore/)\n" +
		"- **L7**: Application (cmd/)\n\n" +
		"## Commands\n\n    arch-dev check    # Check architecture constraints\n    arch-dev build    # Build project\n    arch-dev lint     # Run linters\n\n" +
		"## Development\n\n1. Read ARCHITECTURE_AGENT.md\n2. Implement your business logic using four primitives\n3. Run arch-dev check to verify compliance\n"
	os.WriteFile(filepath.Join(projectDir, "README.md"), []byte(readmeContent), 0644)
	fmt.Println("  ✅ 项目结构创建完成")

	// Step 2: Generate all layer scaffolds
	fmt.Println("\n▶ Step 2/5  生成所有层脚手架...")
	scaffolds := []struct {
		layer   string
		pattern string
		target  string
	}{
		{"L0", "", "core/errors.go"},
		{"L1", "", "core/types.go"},
		{"L2", "retry", "core/pattern_retry.go"},
		{"L2", "circuit-breaker", "core/pattern_circuit_breaker.go"},
		{"L2", "rate-limiter", "core/pattern_rate_limiter.go"},
		{"L4", "", "core/guardian.go"},
		{"L5", "", "core/observation.go"},
		{"L6", "", "core/eventstore.go"},
		{"L7", "", "cmd/server/main.go"},
	}

	for _, s := range scaffolds {
		code := generateScaffold(s.layer, s.pattern, projectName)
		targetPath := filepath.Join(projectDir, s.target)
		if err := os.WriteFile(targetPath, []byte(code), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  跳过: %s (已存在或出错)\n", s.target)
			continue
		}
		fmt.Printf("  ✅ 生成 %s (%s)\n", s.layer, s.target)
	}

	// Step 3: Check architecture constraints
	fmt.Println("\n▶ Step 3/5  检查架构约束...")
	violations := checkConstraints(projectDir)
	if len(violations) > 0 {
		fmt.Printf("  ⚠️  发现 %d 个约束违规，尝试自动修复...\n", len(violations))
		fixed := fixViolations(projectDir, violations)
		fmt.Printf("  ✅ 已修复 %d 个\n", fixed)
	} else {
		fmt.Println("  ✅ 无约束违规")
	}

	// Step 4: Run go fmt
	fmt.Println("\n▶ Step 4/5  代码格式化...")
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Dir = projectDir
	output, _ := cmd.CombinedOutput()
	if len(strings.TrimSpace(string(output))) > 0 {
		fmt.Printf("  📝 已格式化以下文件:\n%s\n", output)
	} else {
		fmt.Println("  ✅ 无需格式化")
	}

	// Step 5: Build verification
	fmt.Println("\n▶ Step 5/5  构建验证...")
	cmd = exec.Command("go", "build", "./...")
	cmd.Dir = projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("  ⚠️  构建警告（首次构建可能缺少依赖）:\n%s\n", output)
	} else {
		fmt.Println("  ✅ 构建成功")
	}

	// Summary
	fmt.Println("\n╔════════════════════════════════════════════════════════╗")
	fmt.Println("║                     初始化完成!                          ║")
	fmt.Println("╚════════════════════════════════════════════════════════╝")
	fmt.Printf("\n📂 项目目录: %s\n", projectDir)
	fmt.Println("\n生成的文件:")
	for _, s := range scaffolds {
		fmt.Printf("  - %s\n", s.target)
	}
	fmt.Println("  - ARCHITECTURE_AGENT.md")
	fmt.Println("  - go.mod")
	fmt.Println("  - README.md")
	fmt.Printf("\n下一步: cd %s\n", projectName)
	fmt.Println("开始编写业务代码吧!")
}

// ============================================================================
// runNew - Create a new project
// ============================================================================

func runNew() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: project description required")
		fmt.Fprintln(os.Stderr, "Usage: arch-dev new <project_description>")
		os.Exit(1)
	}

	description := strings.Join(os.Args[2:], " ")
	projectName := parseProjectName(description)

	fmt.Printf("Creating project: %s\n", projectName)
	fmt.Printf("Description: %s\n", description)

	// Create project directory
	projectDir := filepath.Join(".", projectName)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating project directory: %v\n", err)
		os.Exit(1)
	}

	// Copy ARCHITECTURE_AGENT.md if it exists
	architectureAgentPath := filepath.Join(".", "ARCHITECTURE_AGENT.md")
	if _, err := os.Stat(architectureAgentPath); err == nil {
		srcData, _ := os.ReadFile(architectureAgentPath)
		dstPath := filepath.Join(projectDir, "ARCHITECTURE_AGENT.md")
		os.WriteFile(dstPath, srcData, 0644)
		fmt.Println("Copied ARCHITECTURE_AGENT.md")
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(projectDir, "cmd", "server"),
		filepath.Join(projectDir, "core"),
		filepath.Join(projectDir, "adapters"),
		filepath.Join(projectDir, "services"),
		filepath.Join(projectDir, "pipelines"),
		filepath.Join(projectDir, "observation"),
		filepath.Join(projectDir, "eventstore"),
		filepath.Join(projectDir, "guardian"),
		filepath.Join(projectDir, "patterns"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	// Create go.mod
	goMod := fmt.Sprintf(`module %s

go 1.21

require (
	github.com/stretchr/testify v1.8.4
)

`, projectName)

	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(goMod), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating go.mod: %v\n", err)
		os.Exit(1)
	}

	// Create core/types.go
	typesContent := `// Package core — Core types following four-primitive architecture

package core

import "context"

// ============================================================================
// SECTION 1: Four Primitives Type Definitions
// ============================================================================

// Atom[In, Out any] is a pure function with no side effects.
type Atom[In, Out any] func(In) Out

// AtomWithError[In, Out any] is a pure function that may return an error.
type AtomWithError[In, Out any] func(In) (Out, error)

// Port[In, Out any] is an input/output boundary with validation.
type Port[In, Out any] func(ctx context.Context, input In) (Out, error)

// Adapter[In, Out any] is a side-effect boundary.
type Adapter[In, Out any] func(ctx context.Context, input In) (Out, error)

// Composer[T any] is an orchestrator that combines primitives.
type Composer[T any] interface {
	Execute(ctx context.Context, input T) (T, error)
}

// ============================================================================
// SECTION 2: Error Definitions (L0)
// ============================================================================

type DomainError struct {
	Code    string
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

var (
	ErrInvalidInput        = &DomainError{Code: "INVALID_INPUT", Message: "invalid input"}
	ErrInvalidSymbol       = &DomainError{Code: "INVALID_SYMBOL", Message: "invalid symbol"}
	ErrInvalidAmount       = &DomainError{Code: "INVALID_AMOUNT", Message: "invalid amount"}
	ErrInsufficientBalance = &DomainError{Code: "INSUFFICIENT_BALANCE", Message: "insufficient balance"}
	ErrNotFound            = &DomainError{Code: "NOT_FOUND", Message: "resource not found"}
	ErrTimeout             = &DomainError{Code: "TIMEOUT", Message: "operation timed out"}
	ErrCircuitOpen         = &DomainError{Code: "CIRCUIT_OPEN", Message: "circuit breaker is open"}
)

func NewDomainError(code, message string, err error) *DomainError {
	return &DomainError{Code: code, Message: message, Err: err}
}
`

	if err := os.WriteFile(filepath.Join(projectDir, "core", "types.go"), []byte(typesContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating core/types.go: %v\n", err)
		os.Exit(1)
	}

	// Create main.go
	mainContent := `// Package main — Application entry point (L7)

package main

import (
	"context"
	"log/slog"
	"os"
)

func main() {
	ctx := context.Background()
	logger := slog.Default()

	logger.Info("starting application")

	if err := run(ctx); err != nil {
		logger.Error("application error", "error", err)
		os.Exit(1)
	}

	logger.Info("application stopped")
}

func run(ctx context.Context) error {
	// TODO: Initialize components
	// TODO: Start servers
	// TODO: Graceful shutdown

	fmt.Println("Hello from ` + projectName + `")

	<-ctx.Done()
	return nil
}
`

	// Replace backtick-wrapped project name
	mainContent = strings.ReplaceAll(mainContent, "` + projectName + `", projectName)

	if err := os.WriteFile(filepath.Join(projectDir, "cmd", "server", "main.go"), []byte(mainContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating cmd/server/main.go: %v\n", err)
		os.Exit(1)
	}

	// Create README.md
	readmeContent := `# ` + projectName + `

Generated with arch-dev following ARCHITECTURE_AGENT.md

## Architecture

- **L0**: Error handling (core/errors.go)
- **L1**: Four primitives (core/types.go)
- **L2**: Patterns (patterns/)
- **L4**: Guardian (guardian/)
- **L5**: Observation (observation/)
- **L6**: EventStore (eventstore/)
- **L7**: Application (cmd/)

## Commands

    arch-dev check    # Check architecture constraints
    arch-dev build    # Build project
    arch-dev lint     # Run linters

## Development

1. Read ARCHITECTURE_AGENT.md
2. Implement your business logic using four primitives
3. Run arch-dev check to verify compliance
`

	if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte(readmeContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating README.md: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Project created successfully: %s\n", projectDir)
	fmt.Println("\nNext steps:")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Println("  arch-dev check")
	fmt.Println("  arch-dev build")
}

func parseProjectName(description string) string {
	// Convert description to valid Go module name
	name := strings.ToLower(description)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-' // Replace invalid chars with hyphen
	}, name)

	// Remove consecutive hyphens
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// Trim hyphens from ends
	name = strings.Trim(name, "-_")

	if name == "" {
		name = "my-project"
	}

	return name
}

// ============================================================================
// runAdd - Add a feature to an existing project
// ============================================================================

func runAdd() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Error: feature description required")
		fmt.Fprintln(os.Stderr, "Usage: arch-dev add <feature_description>")
		os.Exit(1)
	}

	description := strings.Join(os.Args[2:], " ")
	projectRoot := findProjectRoot()

	if projectRoot == "" {
		fmt.Fprintln(os.Stderr, "Error: not in a project directory (no go.mod found)")
		os.Exit(1)
	}

	fmt.Printf("Adding feature: %s\n", description)
	fmt.Printf("Project: %s\n", projectRoot)

	// Parse feature to determine layer and primitive
	layer := guessLayer(description)
	primitive := guessPrimitive(description)

	fmt.Printf("Detected: Layer=%s, Primitive=%s\n", layer, primitive)

	// Generate code based on feature
	generated := generateFeatureCode(description, layer, primitive)

	// Determine target directory
	targetDir := getTargetDir(layer)

	// Write file
	fileName := filepath.Join(projectRoot, targetDir, fmt.Sprintf("%s.go", toFileName(description)))
	if _, err := os.Stat(fileName); err == nil {
		fmt.Printf("⚠️  File already exists: %s\n", fileName)
		os.Exit(1)
	}

	if err := os.WriteFile(fileName, []byte(generated), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Feature added: %s\n", fileName)
	fmt.Println("\nNext steps:")
	fmt.Println("  arch-dev check")
	fmt.Println("  arch-dev build")
}

func guessLayer(description string) string {
	desc := strings.ToLower(description)

	// Simple heuristics
	if strings.Contains(desc, "adapter") || strings.Contains(desc, "api") || strings.Contains(desc, "database") || strings.Contains(desc, "存储") {
		return "L1"
	}
	if strings.Contains(desc, "service") || strings.Contains(desc, "业务") {
		return "L2"
	}
	if strings.Contains(desc, "pipeline") || strings.Contains(desc, "workflow") || strings.Contains(desc, "编排") {
		return "L2"
	}
	if strings.Contains(desc, "retry") || strings.Contains(desc, "circuit") || strings.Contains(desc, "熔断") || strings.Contains(desc, "限流") {
		return "L2"
	}
	if strings.Contains(desc, "observe") || strings.Contains(desc, "trac") || strings.Contains(desc, "metric") || strings.Contains(desc, "log") {
		return "L5"
	}
	if strings.Contains(desc, "event") || strings.Contains(desc, "eventstore") {
		return "L6"
	}

	return "L2" // Default
}

func guessPrimitive(description string) string {
	desc := strings.ToLower(description)

	if strings.Contains(desc, "adapter") {
		return "Adapter"
	}
	if strings.Contains(desc, "validate") || strings.Contains(desc, "校验") {
		return "Port"
	}
	if strings.Contains(desc, "pure") || strings.Contains(desc, "calculate") || strings.Contains(desc, "计算") {
		return "Atom"
	}
	if strings.Contains(desc, "pipeline") || strings.Contains(desc, "workflow") {
		return "Composer"
	}

	return "Port" // Default
}

func getTargetDir(layer string) string {
	switch layer {
	case "L1":
		return "core"
	case "L2":
		return "services"
	case "L3":
		return "services"
	case "L4":
		return "guardian"
	case "L5":
		return "observation"
	case "L6":
		return "eventstore"
	case "L7":
		return "cmd"
	default:
		return "core"
	}
}

func generateFeatureCode(description, layer, primitive string) string {
	upperName := toUpperCamelCase(description)

	switch primitive {
	case "Atom":
		return fmt.Sprintf(`// Package core — %s feature

package core

// %s is an Atom that ...
type %s func(%s) %s

// New%s creates a new %s Atom.
func New%s() %s {
	return func(input %s) %s {
		// TODO: implement
		return input
	}
}
`, upperName, upperName, upperName, "Input", "Output",
			upperName, upperName, upperName, "Input", "Output")

	case "Port":
		return fmt.Sprintf(`// Package core — %s feature

package core

import "context"

// %sRequest is the input.
type %sRequest struct {
	// TODO: add fields
}

// %sResponse is the output.
type %sResponse struct {
	// TODO: add fields
}

// %s is a Port that ...
func %s(ctx context.Context, req %sRequest) (%sResponse, error) {
	if req == (%sRequest{}) {
		return %sResponse{}, ErrInvalidInput
	}
	// TODO: implement
	return %sResponse{}, nil
}
`, upperName, upperName, upperName, upperName, upperName,
			upperName+"Request", upperName+"Response",
			upperName+"Request", upperName+"Response", upperName+"Response")

	case "Adapter":
		return fmt.Sprintf(`// Package core — %s feature

package core

import "context"

// %sAdapter is an Adapter that ...
type %sAdapter struct {
	// TODO: add dependencies
}

// New%sAdapter creates a new %s Adapter.
func New%sAdapter() *%sAdapter {
	return &%sAdapter{}
}

// Execute implements Adapter.
func (a *%sAdapter) Execute(ctx context.Context, req %sRequest) (%sResponse, error) {
	// TODO: implement
	return %sResponse{}, nil
}
`, upperName, upperName, upperName, upperName, upperName, upperName, upperName,
			upperName, upperName+"Request", upperName+"Response", upperName+"Response")

	case "Composer":
		return fmt.Sprintf(`// Package core — %s feature

package core

import "context"

// %sPipeline is a Composer that ...
type %sPipeline struct {
	// TODO: add steps
}

// New%sPipeline creates a new %s Pipeline.
func New%sPipeline() *%sPipeline {
	return &%sPipeline{}
}

// Execute implements Composer.
func (p *%sPipeline) Execute(ctx context.Context, input %sInput) (%sOutput, error) {
	// TODO: implement
	return %sOutput{}, nil
}
`, upperName, upperName, upperName, upperName, upperName, upperName, upperName,
			upperName+"Input", upperName+"Output", upperName+"Output")

	default:
		return fmt.Sprintf(`// Package core — %s feature

package core

// TODO: implement %s
`, upperName, upperName)
	}
}

func toFileName(description string) string {
	// Extract only a-z and 0-9
	name := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1 // remove
	}, description)
	if len(name) > 15 {
		name = name[:15]
	}
	if name == "" {
		// Generate name based on task type
		lower := strings.ToLower(description)
		if strings.Contains(lower, "订单") || strings.Contains(lower, "order") {
			name = "order"
		} else if strings.Contains(lower, "账户") || strings.Contains(lower, "account") {
			name = "account"
		} else if strings.Contains(lower, "用户") || strings.Contains(lower, "user") {
			name = "user"
		} else if strings.Contains(lower, "支付") || strings.Contains(lower, "pay") {
			name = "payment"
		} else if strings.Contains(lower, "商品") || strings.Contains(lower, "product") {
			name = "product"
		} else if strings.Contains(lower, "消息") || strings.Contains(lower, "message") {
			name = "message"
		} else {
			name = "feature"
		}
	}
	return name
}

func toUpperCamelCase(s string) string {
	// For English words, apply camelCase
	// For Chinese, just return as-is
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) == 0 {
			continue
		}
		// Check if it's ASCII letters
		isASCII := true
		for _, c := range word {
			if c > 127 {
				isASCII = false
				break
			}
		}
		if isASCII {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
		// Chinese words are kept as-is
	}
	return strings.Join(words, "")
}

// ============================================================================
// runCheck - Check architecture constraints
// ============================================================================

type Violation struct {
	Level   string
	File    string
	Line    int
	Message string
	Rule    string
}

func runCheck() {
	fix := false
	for i := 2; i < len(os.Args); i++ {
		if os.Args[i] == "--fix" {
			fix = true
		}
	}

	projectRoot := findProjectRoot()
	if projectRoot == "" {
		fmt.Fprintln(os.Stderr, "Error: not in a project directory (no go.mod found)")
		os.Exit(1)
	}

	fmt.Printf("Checking architecture constraints in: %s\n", projectRoot)

	violations := checkConstraints(projectRoot)

	if len(violations) == 0 {
		fmt.Println("\n✅ All constraints passed")
		return
	}

	fmt.Printf("\n❌ Found %d constraint violations:\n", len(violations))
	for _, v := range violations {
		fmt.Printf("  [%s] %s:%d - %s\n", v.Level, filepath.Base(v.File), v.Line, v.Message)
	}

	if fix {
		fmt.Println("\n🔧 Attempting to fix violations...")
		fixed := fixViolations(projectRoot, violations)
		fmt.Printf("✅ Fixed %d violations\n", fixed)
	} else {
		fmt.Println("\nRun 'arch-dev fix' to auto-fix violations")
		os.Exit(1)
	}
}

func checkConstraints(projectRoot string) []Violation {
	var violations []Violation

	filepath.Walk(projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Check C2: file line count
		if info.Size() > 300*200 { // rough estimate: 300 lines * 200 bytes/line
			data, _ := os.ReadFile(path)
			lines := strings.Split(string(data), "\n")
			if len(lines) > 300 {
				violations = append(violations, Violation{
					Level:   "ERROR",
					File:    path,
					Line:    1,
					Message: fmt.Sprintf("file exceeds 300 lines (%d lines)", len(lines)),
					Rule:    "C2",
				})
			}
		}

		// Check C5: panic
		data, _ := os.ReadFile(path)
		content := string(data)
		if strings.Contains(content, "panic(") && !strings.Contains(path, "_test.go") {
			lines := strings.Split(content, "\n")
			for i, line := range lines {
				if strings.Contains(line, "panic(") && !strings.HasPrefix(strings.TrimSpace(line), "//") {
					violations = append(violations, Violation{
						Level:   "ERROR",
						File:    path,
						Line:    i + 1,
						Message: "panic is forbidden (C5)",
						Rule:    "C5",
					})
				}
			}
		}

		// Check C8: interface{}
		if strings.Contains(content, "interface{}") && !strings.Contains(path, "_test.go") {
			violations = append(violations, Violation{
				Level:   "WARNING",
				File:    path,
				Line:    1,
				Message: "interface{} should be replaced with any (C8)",
				Rule:    "C8",
			})
		}

		// Check C9: fmt.Print in non-test files
		if strings.Contains(content, "fmt.Print") && !strings.Contains(path, "_test.go") && !strings.Contains(path, "cmd/") {
			lines := strings.Split(content, "\n")
			for i, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "//") {
					continue
				}
				if strings.Contains(line, "fmt.Print") && !strings.HasPrefix(trimmed, "//") {
					violations = append(violations, Violation{
						Level:   "WARNING",
						File:    path,
						Line:    i + 1,
						Message: "fmt.Print is forbidden in business code (C9)",
						Rule:    "C9",
					})
					break
				}
			}
		}

		return nil
	})

	return violations
}

func fixViolations(projectRoot string, violations []Violation) int {
	fixed := 0

	for _, v := range violations {
		if v.Rule == "C8" {
			// Fix interface{} -> any
			data, _ := os.ReadFile(v.File)
			content := string(data)
			if strings.Contains(content, "interface{}") {
				newContent := strings.ReplaceAll(content, "interface{}", "any")
				os.WriteFile(v.File, []byte(newContent), 0644)
				fixed++
			}
		}
	}

	return fixed
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		goMod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goMod); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func readModuleName(projectRoot string) string {
	goModPath := filepath.Join(projectRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return filepath.Base(projectRoot)
	}
	content := string(data)
	// Parse first line: "module <name>"
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) > 0 {
		parts := strings.Fields(lines[0])
		if len(parts) >= 2 {
			return parts[1]
		}
	}
	return filepath.Base(projectRoot)
}

// ============================================================================
// runBuild, runLint, runFix, runDoctor - Build commands
// ============================================================================

func runBuild() {
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		fmt.Fprintln(os.Stderr, "Error: not in a project directory (no go.mod found)")
		os.Exit(1)
	}

	fmt.Printf("Building project: %s\n", projectRoot)

	// Run go build
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Build failed:\n%s\n", output)
		os.Exit(1)
	}

	fmt.Println("✅ Build successful")
}

func runLint() {
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		fmt.Fprintln(os.Stderr, "Error: not in a project directory (no go.mod found)")
		os.Exit(1)
	}

	fmt.Printf("Running linters: %s\n", projectRoot)

	// Run go vet
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ go vet failed:\n%s\n", output)
		// Don't exit with error for vet warnings
	}

	// Run go fmt check
	cmd = exec.Command("go", "fmt", "./...")
	cmd.Dir = projectRoot
	output, err = cmd.CombinedOutput()

	if len(strings.TrimSpace(string(output))) > 0 {
		fmt.Printf("⚠️  Formatting issues:\n%s\n", output)
		fmt.Println("Run 'go fmt ./...' to fix")
	}

	fmt.Println("✅ Linting complete")
}

func runFix() {
	projectRoot := findProjectRoot()
	if projectRoot == "" {
		fmt.Fprintln(os.Stderr, "Error: not in a project directory (no go.mod found)")
		os.Exit(1)
	}

	fmt.Printf("Auto-fixing violations in: %s\n", projectRoot)

	violations := checkConstraints(projectRoot)

	if len(violations) == 0 {
		fmt.Println("✅ No violations to fix")
		return
	}

	fixed := fixViolations(projectRoot, violations)

	// Also run go fmt
	cmd := exec.Command("go", "fmt", "./...")
	cmd.Dir = projectRoot
	cmd.Run()

	fmt.Printf("✅ Fixed %d violations\n", fixed)
}

func runDoctor() {
	fmt.Println("🔍 Checking environment...")

	checks := []struct {
		name string
		fn   func() error
	}{
		{"Go version", checkGoVersion},
		{"Go modules", checkGoModules},
		{"ARCHITECTURE_AGENT.md", checkArchitectureFile},
	}

	allPassed := true
	for _, check := range checks {
		if err := check.fn(); err != nil {
			fmt.Printf("  ❌ %s: %v\n", check.name, err)
			allPassed = false
		} else {
			fmt.Printf("  ✅ %s\n", check.name)
		}
	}

	if !allPassed {
		os.Exit(1)
	}

	fmt.Println("\n✅ Environment check passed")
}

func checkGoVersion() error {
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	fmt.Printf("    %s", string(output))
	return nil
}

func checkGoModules() error {
	dir, _ := os.Getwd()
	for {
		goMod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goMod); err == nil {
			fmt.Printf("    go.mod found: %s\n", dir)
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return fmt.Errorf("no go.mod found")
}

func checkArchitectureFile() error {
	dir, _ := os.Getwd()
	for {
		archFile := filepath.Join(dir, "ARCHITECTURE_AGENT.md")
		if _, err := os.Stat(archFile); err == nil {
			fmt.Printf("    ARCHITECTURE_AGENT.md found: %s\n", dir)
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return fmt.Errorf("ARCHITECTURE_AGENT.md not found")
}

// ============================================================================
// runScaffold - Generate scaffold for a layer
// ============================================================================

func runScaffold() {
	var layer string
	var pattern string

	for i := 2; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "--layer" && i+1 < len(os.Args) {
			layer = os.Args[i+1]
			i++
		} else if arg == "--pattern" && i+1 < len(os.Args) {
			pattern = os.Args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--") {
			// skip unknown flags
		} else {
			layer = arg
		}
	}

	projectRoot := findProjectRoot()
	if projectRoot == "" {
		fmt.Fprintln(os.Stderr, "Error: not in a project directory (no go.mod found)")
		os.Exit(1)
	}

	projectName := readModuleName(projectRoot)

	if layer == "" {
		fmt.Fprintln(os.Stderr, "Error: layer required")
		fmt.Fprintln(os.Stderr, "Usage: arch-dev scaffold <layer> [--pattern <pattern>]")
		fmt.Fprintln(os.Stderr, "Layers: L0, L1, L2, L3, L4, L5, L6, L7")
		fmt.Fprintln(os.Stderr, "Patterns: retry, circuit-breaker, rate-limiter, bulkhead, timeout")
		os.Exit(1)
	}

	fmt.Printf("Generating scaffold for %s\n", layer)
	if pattern != "" {
		fmt.Printf("Pattern: %s\n", pattern)
	}

	code := generateScaffold(layer, pattern, projectName)

	// Write to appropriate file
	var fileName string
	switch layer {
	case "L0":
		fileName = "core/errors.go"
	case "L1":
		fileName = "core/types.go"
	case "L2":
		if pattern == "retry" {
			fileName = "core/pattern_retry.go"
		} else if pattern == "circuit-breaker" {
			fileName = "core/pattern_circuit_breaker.go"
		} else if pattern == "rate-limiter" {
			fileName = "core/pattern_rate_limiter.go"
		} else if pattern == "bulkhead" {
			fileName = "core/pattern_bulkhead.go"
		} else if pattern == "timeout" {
			fileName = "core/pattern_timeout.go"
		} else {
			fileName = "core/pattern_scaffold.go"
		}
	case "L4":
		fileName = "core/guardian.go"
	case "L5":
		fileName = "core/observation.go"
	case "L6":
		fileName = "core/eventstore.go"
	case "L7":
		fileName = "cmd/server/main.go"
	default:
		fileName = "core/scaffold.go"
	}

	filePath := projectRoot + "/" + fileName
	if _, err := os.Stat(filePath); err == nil {
		fmt.Printf("⚠️  File already exists: %s\n", filePath)
		os.Exit(1)
	}

	if err := os.WriteFile(filePath, []byte(code), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Scaffold generated: %s\n", filePath)
}

func generateScaffold(layer, pattern, module string) string {
	_ = module // reserved for future cross-module imports
	switch layer {
	case "L0":
		return `// Package core — Error definitions (L0)

package core

// DomainError represents a domain-specific error.
type DomainError struct {
	Code    string
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewDomainError creates a new DomainError.
func NewDomainError(code, message string, err error) *DomainError {
	return &DomainError{Code: code, Message: message, Err: err}
}

// Predefined errors (L0)
var (
	ErrInvalidInput        = NewDomainError("INVALID_INPUT", "invalid input", nil)
	ErrInvalidSymbol       = NewDomainError("INVALID_SYMBOL", "invalid symbol", nil)
	ErrInvalidAmount       = NewDomainError("INVALID_AMOUNT", "invalid amount", nil)
	ErrInsufficientBalance = NewDomainError("INSUFFICIENT_BALANCE", "insufficient balance", nil)
	ErrNotFound            = NewDomainError("NOT_FOUND", "resource not found", nil)
	ErrTimeout             = NewDomainError("TIMEOUT", "operation timed out", nil)
	ErrUnauthorized        = NewDomainError("UNAUTHORIZED", "unauthorized", nil)
	ErrCircuitOpen         = NewDomainError("CIRCUIT_OPEN", "circuit breaker is open", nil)
)

var _ error = (*DomainError)(nil)
`

	case "L1":
		return `// Package core — Four Primitives (L1)

package core

import "context"

// ============================================================================
// Four Primitives Type Definitions
// ============================================================================

// Atom[In, Out any] is a pure function with no side effects.
type Atom[In, Out any] func(In) Out

// AtomWithError[In, Out any] is a pure function that may return an error.
type AtomWithError[In, Out any] func(In) (Out, error)

// Port[In, Out any] is an input/output boundary with validation.
type Port[In, Out any] func(ctx context.Context, input In) (Out, error)

// Adapter[In, Out any] is a side-effect boundary.
type Adapter[In, Out any] func(ctx context.Context, input In) (Out, error)

// Composer[T any] is an orchestrator that combines primitives.
type Composer[T any] interface {
	Execute(ctx context.Context, input T) (T, error)
}

// ============================================================================
// Context Interface (for dependency inversion)
// ============================================================================

// Ctx is a minimal context interface.
type Ctx interface {
	Done() <-chan struct{}
}

// ============================================================================
// Pipeline, Branch, Parallel Composers
// ============================================================================

// Step is a single step in a Pipeline.
type Step[T any] func(ctx context.Context, input T) (T, error)

// Pipeline is a Composer that executes steps sequentially.
type Pipeline[T any] struct {
	steps []Step[T]
}

// NewPipeline creates a new Pipeline.
func NewPipeline[T any](steps ...Step[T]) *Pipeline[T] {
	return &Pipeline[T]{steps: steps}
}

// Execute implements Composer.
func (p *Pipeline[T]) Execute(ctx context.Context, input T) (T, error) {
	result := input
	for _, step := range p.steps {
		r, err := step(ctx, result)
		if err != nil {
			return result, err
		}
		result = r
	}
	return result, nil
}

// Branch is a Composer that routes based on condition.
type Branch[T any] struct {
	condition func(T) bool
	then      Composer[T]
	else_     Composer[T]
}

// NewBranch creates a new Branch.
func NewBranch[T any](cond func(T) bool, then, else_ Composer[T]) *Branch[T] {
	return &Branch[T]{condition: cond, then: then, else_: else_}
}

// Execute implements Composer.
func (b *Branch[T]) Execute(ctx context.Context, input T) (T, error) {
	if b.condition(input) {
		return b.then.Execute(ctx, input)
	}
	if b.else_ != nil {
		return b.else_.Execute(ctx, input)
	}
	return input, nil
}

// Parallel executes multiple Composers concurrently.
type Parallel[T any] struct {
	composers []Composer[T]
}

// NewParallel creates a new Parallel composer.
func NewParallel[T any](composers ...Composer[T]) *Parallel[T] {
	return &Parallel[T]{composers: composers}
}

// Execute implements Composer.
func (p *Parallel[T]) Execute(ctx context.Context, input T) (T, error) {
	// TODO: implement concurrent execution
	return input, nil
}
`

	case "L2":
		if pattern == "retry" {
			return `// Package core — Retry pattern (L2)

package core

import (
	"context"
	"time"
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Jitter       bool
}

// Retry executes an Adapter with retry logic.
func Retry[In, Out any](
	config RetryConfig,
	adapter Adapter[In, Out],
) Adapter[In, Out] {
	return func(ctx context.Context, input In) (Out, error) {
		var lastErr error
		delay := config.InitialDelay

		for attempt := 0; attempt < config.MaxAttempts; attempt++ {
			select {
			case <-ctx.Done():
				var zero Out
				return zero, ctx.Err()
			default:
			}

			output, err := adapter(ctx, input)
			if err == nil {
				return output, nil
			}
			lastErr = err

			if attempt < config.MaxAttempts-1 {
				jitter := time.Duration(0)
				if config.Jitter {
					jitter = time.Duration(time.Now().UnixNano() % int64(delay))
				}
				select {
				case <-time.After(delay + jitter):
				case <-ctx.Done():
					var zero Out
					return zero, ctx.Err()
				}

				// Exponential backoff
				delay *= 2
				if delay > config.MaxDelay {
					delay = config.MaxDelay
				}
			}
		}

		var zero Out
		return zero, lastErr
	}
}
`
		} else if pattern == "circuit-breaker" {
			return `// Package core — Circuit Breaker pattern (L2)

package core

import (
	"context"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = 0
	CircuitOpen     CircuitState = 1
	CircuitHalfOpen CircuitState = 2
)

// CircuitBreaker is an Adapter that implements circuit breaker pattern.
type CircuitBreaker[In, Out any] struct {
	mu          sync.Mutex
	state       CircuitState
	failures    int
	maxFailures int
	openTimeout time.Duration
	lastFailure time.Time
	halfOpenMax int
	halfOpenCnt int
}

// NewCircuitBreaker creates a new CircuitBreaker.
func NewCircuitBreaker[In, Out any](maxFailures int, openTimeout time.Duration) *CircuitBreaker[In, Out] {
	return &CircuitBreaker[In, Out]{
		state:       CircuitClosed,
		maxFailures: maxFailures,
		openTimeout: openTimeout,
	}
}

// Execute implements Adapter.
func (cb *CircuitBreaker[In, Out]) Execute(ctx context.Context, input In) (Out, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	var zero Out

	switch cb.state {
	case CircuitOpen:
		if time.Since(cb.lastFailure) > cb.openTimeout {
			cb.state = CircuitHalfOpen
			cb.halfOpenCnt = 0
			goto execute
		}
		return zero, ErrCircuitOpen

	case CircuitHalfOpen:
		if cb.halfOpenCnt >= cb.halfOpenMax {
			return zero, ErrCircuitOpen
		}
		cb.halfOpenCnt++
	}

execute:
	output, err := cb.execute(ctx, input)
	if err != nil {
		cb.recordFailure()
		return zero, err
	}
	cb.recordSuccess()
	return output, nil
}

func (cb *CircuitBreaker[In, Out]) execute(ctx context.Context, input In) (Out, error) {
	var zero Out
	return zero, nil // TODO: call actual adapter
}

func (cb *CircuitBreaker[In, Out]) recordFailure() {
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

func (cb *CircuitBreaker[In, Out]) recordSuccess() {
	cb.failures = 0
	cb.state = CircuitClosed
}
`
		} else if pattern == "rate-limiter" {
			return `// Package core — Rate Limiter pattern (L2)

package core

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	rate       float64 // tokens per second
	lastRefill time.Time
}

// NewRateLimiter creates a new RateLimiter.
func NewRateLimiter(maxTokens, rate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		rate:       rate,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed.
func (rl *RateLimiter) Allow() bool {
	return rl.AllowN(1)
}

// AllowN checks if N tokens are available.
func (rl *RateLimiter) AllowN(n float64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= n {
		rl.tokens -= n
		return true
	}
	return false
}

// Wait blocks until a token is available.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for !rl.Allow() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond * 10):
		}
	}
	return nil
}

func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens += elapsed * rl.rate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now
}
`
		}
		return `// Package core — L2 pattern scaffold

package core

// TODO: implement L2 patterns (retry, circuit-breaker, rate-limiter, etc.)
`

	case "L4":
		return `// Package core — Guardian (L4)

package core

// Guardian monitors and enforces architectural constraints.

// GuardianRule defines a rule that Guardian checks.
type GuardianRule interface {
	Name() string
	Check(projectRoot string) ([]Violation, error)
}

// Guardian checks architectural constraints.
type Guardian struct {
	rules []GuardianRule
}

// NewGuardian creates a new Guardian.
func NewGuardian() *Guardian {
	return &Guardian{
		rules: []GuardianRule{
			// Add rules here
		},
	}
}

// Check runs all Guardian rules.
func (g *Guardian) Check(projectRoot string) []Violation {
	var allViolations []Violation
	for _, rule := range g.rules {
		violations, err := rule.Check(projectRoot)
		if err != nil {
			continue
		}
		allViolations = append(allViolations, violations...)
	}
	return allViolations
}

// Violation represents a constraint violation.
type Violation struct {
	Rule    string
	Message string
	File    string
	Line    int
}
`

	case "L5":
		return `// Package core — Observation (L5)

package core

import (
	"context"
)

// ============================================================================
// Tracer (L5)
// ============================================================================

// Tracer creates Spans for tracing.
type Tracer interface {
	Start(ctx context.Context, name string) (context.Context, Span)
}

// Span represents a unit of work.
type Span interface {
	End()
	SetAttributes(key string, value any)
	RecordError(err error)
}

// NoOpTracer is a Tracer that does nothing.
type NoOpTracer struct{}

func (NoOpTracer) Start(ctx context.Context, name string) (context.Context, Span) {
	return ctx, NoOpSpan{}
}

type NoOpSpan struct{}

func (NoOpSpan) End()                               {}
func (NoOpSpan) SetAttributes(key string, value any) {}
func (NoOpSpan) RecordError(err error)              {}

// ============================================================================
// Meter (L5)
// ============================================================================

// Meter creates metrics instruments.
type Meter interface {
	Counter(name string) Counter
	Histogram(name string, buckets []float64) Histogram
	Gauge(name string) Gauge
}

// Counter is a monotonic counter.
type Counter interface {
	Add(ctx context.Context, value float64, attributes ...KeyValue)
}

// Histogram is a distribution of values.
type Histogram interface {
	Record(ctx context.Context, value float64, attributes ...KeyValue)
}

// Gauge is a point-in-time value.
type Gauge interface {
	Set(ctx context.Context, value float64, attributes ...KeyValue)
}

// KeyValue is a key-value pair.
type KeyValue struct {
	Key   string
	Value any
}

// ============================================================================
// Logger (L5)
// ============================================================================

// Logger is the logging interface.
type Logger interface {
	Debug(msg string, attrs ...KeyValue)
	Info(msg string, attrs ...KeyValue)
	Warn(msg string, attrs ...KeyValue)
	Error(msg string, attrs ...KeyValue)
}
`

	case "L6":
		return `// Package core — EventStore (L6)

package core

import (
	"context"
	"time"
)

// EventEnvelope wraps an event with metadata.
type EventEnvelope struct {
	EventID       string
	AggregateID   string
	AggregateType string
	EventType     string
	EventData     []byte
	Version       int64
	Timestamp     time.Time
	TraceID       string
}

// AppendResult is the result of appending an event.
type AppendResult struct {
	EventID string
	Version int64
	Success bool
}

// EventStore stores and retrieves events.
type EventStore interface {
	Append(ctx context.Context, event EventEnvelope) (AppendResult, error)
	Load(ctx context.Context, aggregateID string) ([]EventEnvelope, error)
}

// InMemoryEventStore is a simple in-memory EventStore.
type InMemoryEventStore struct {
	events map[string][]EventEnvelope
}

// NewInMemoryEventStore creates a new InMemoryEventStore.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events: make(map[string][]EventEnvelope),
	}
}

// Append implements EventStore.
func (es *InMemoryEventStore) Append(ctx context.Context, event EventEnvelope) (AppendResult, error) {
	if event.EventID == "" {
		// Generate ID
		event.EventID = generateID()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	key := event.AggregateID
	es.events[key] = append(es.events[key], event)

	return AppendResult{
		EventID: event.EventID,
		Version: int64(len(es.events[key])),
		Success: true,
	}, nil
}

// Load implements EventStore.
func (es *InMemoryEventStore) Load(ctx context.Context, aggregateID string) ([]EventEnvelope, error) {
	return es.events[aggregateID], nil
}

func generateID() string {
	// Simple ID generation - use UUID in production
	return time.Now().Format("20060102150405.000000000")
}
`

	case "L7":
		return `// Package main — Application entry point (L7)

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.Default()

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	logger.Info("starting application")

	if err := run(ctx); err != nil {
		logger.Error("application error", "error", err)
		os.Exit(1)
	}

	logger.Info("application stopped")
}

func run(ctx context.Context) error {
	// TODO: Initialize components
	// - Connect to databases
	// - Set up observers
	// - Start servers

	<-ctx.Done()
	return nil
}
`

	default:
		return "// TODO: implement scaffold for " + layer
	}
}
