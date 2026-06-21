// arch-cli: 统一架构治理 CLI (L7)
//
// 命令列表:
//   arch analyze --dir ./project    解析并分析项目架构
//   arch check --dir ./project      检查架构违规
//   arch validate --dir ./project   完整校验（同 check）
//   arch init <name> --tier ...     初始化新项目
//   arch new --project ...          创建新模块
//   arch add --feature ...          添加新功能
//   arch serve --port 8090         启动 Web UI
//   arch version                    版本信息
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"low-entropy-core/go-core/arch"
)

const version = "1.0.0"

var (
	archData    *ArchData
	archMu      sync.RWMutex
	sourceDir   string
	enableWatch bool
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}

	cmd := strings.ToLower(args[0])
	subArgs := args[1:]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	switch cmd {
	case "analyze":
		cmdAnalyze(ctx, subArgs)
	case "check", "validate":
		cmdCheck(ctx, subArgs)
	case "guardian":
		cmdGuardian(ctx, subArgs)
	case "entropy":
		cmdEntropy(ctx, subArgs)
	case "agent":
		cmdAgent(subArgs)
	case "migrate":
		cmdMigrate(ctx, subArgs)
	case "init":
		cmdInit(subArgs)
	case "new":
		cmdNew(subArgs)
	case "add":
		cmdAdd(subArgs)
	case "serve":
		cmdServe(subArgs)
	case "version", "--version", "-v":
		fmt.Println("arch-cli", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Println("未知命令:", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("arch-cli - 架构治理工具")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  arch analyze --dir <path>     分析项目架构")
	fmt.Println("  arch check --dir <path>       检查架构违规")
	fmt.Println("  arch guardian --dir <path>    Guardian 决策分析")
	fmt.Println("  arch entropy --dir <path>     计算代码熵值")
	fmt.Println("  arch agent list               列出 Agent")
	fmt.Println("  arch migrate --dir <path>     迁移分析")
	fmt.Println("  arch init <name> [--tier ...] 初始化新项目")
	fmt.Println("  arch new --project <path>     创建新模块")
	fmt.Println("  arch add --feature <name>     添加新功能")
	fmt.Println("  arch serve [--port 8090]      启动 Web UI")
	fmt.Println("  arch version                  版本信息")
	fmt.Println("  arch help                     帮助")
}

// ──────────────────────────────────────────────
// analyze 命令
// ──────────────────────────────────────────────
func cmdAnalyze(ctx context.Context, args []string) {
	dir := parseFlag(args, "--dir", ".")
	jsonOut := hasFlag(args, "--json")

	p, err := arch.NewPipeline()
	if err != nil {
		fatal("创建 pipeline 失败:", err)
	}

	data, err := p.Analyze(ctx, dir)
	if err != nil {
		fatal("分析失败:", err)
	}

	if jsonOut {
		b, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Println("=== 架构分析报告 ===")
	fmt.Println("项目目录:", dir)
	fmt.Println("文件总数:", data.TotalFiles)
	fmt.Println("代码行数:", data.TotalLines)
	fmt.Println("")
	fmt.Println("=== 层级分布 ===")
	for _, l := range data.Layers {
		fmt.Printf("  %s: %d 文件, %d 行\n", l.Layer, l.Files, l.Lines)
	}
}

// ──────────────────────────────────────────────
// check/validate 命令
// ──────────────────────────────────────────────
func cmdCheck(ctx context.Context, args []string) {
	dir := parseFlag(args, "--dir", ".")
	jsonOut := hasFlag(args, "--json")

	p, err := arch.NewPipeline()
	if err != nil {
		fatal("创建 pipeline 失败:", err)
	}

	result, err := p.Validate(ctx, dir)
	if err != nil {
		fatal("校验失败:", err)
	}

	if jsonOut {
		b, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Println("=== 架构校验报告 ===")
	fmt.Println("项目目录:", dir)
	fmt.Println("文件数:", result.FileCount)
	fmt.Println("违规数:", result.ViolationCount)
	fmt.Println("健康评分:", fmt.Sprintf("%.2f", result.HealthScore),
		"(等级:", result.HealthGrade, ")")
	fmt.Println("耗时:", result.Duration)

	if len(result.Violations) > 0 {
		fmt.Println("\n=== 违规详情 ===")
		for i, v := range result.Violations {
			fmt.Printf("\n[%d] %s [%s]\n", i+1, v.Type, v.Severity)
			fmt.Printf("  文件: %s\n", v.File)
			fmt.Printf("  描述: %s\n", v.Message)
			fmt.Printf("  建议: %s\n", v.Suggestion)
		}
	}
}

// ──────────────────────────────────────────────
// init / new / add 命令
// ──────────────────────────────────────────────
func cmdInit(args []string) {
	if len(args) < 1 {
		fatal("请指定项目名: arch init <name>")
	}
	name := args[0]
	tier := parseFlag(args, "--tier", "microservice")
	root := parseFlag(args, "--dir", "./"+name)

	gen, err := arch.NewGenerator()
	if err != nil {
		fatal("创建生成器失败:", err)
	}
	result, err := gen.InitProject(arch.GenConfig{
		Name:    name,
		Package: name,
		Tier:    tier,
		Root:    root,
	})
	if err != nil {
		fatal("初始化项目失败:", err)
	}

	fmt.Println("=== 项目已初始化 ===")
	fmt.Println("目录:", result.Root)
	fmt.Println("生成文件:")
	for _, f := range result.Files {
		fmt.Println("  ", f)
	}
}

func cmdNew(args []string) {
	root := parseFlag(args, "--project", ".")
	name := parseFlag(args, "--name", "mymodule")
	tier := parseFlag(args, "--tier", "microservice")

	gen, err := arch.NewGenerator()
	if err != nil {
		fatal("创建生成器失败:", err)
	}
	result, err := gen.NewModule(arch.GenConfig{
		Name:    name,
		Package: name,
		Tier:    tier,
		Root:    root,
	})
	if err != nil {
		fatal("创建模块失败:", err)
	}

	fmt.Println("=== 模块已创建 ===")
	for _, f := range result.Files {
		fmt.Println("  ", f)
	}
}

func cmdAdd(args []string) {
	feature := parseFlag(args, "--feature", "")
	if feature == "" {
		fatal("请用 --feature 指定功能名")
	}
	root := parseFlag(args, "--dir", ".")
	pkg := parseFlag(args, "--package", "mymodule")

	gen, err := arch.NewGenerator()
	if err != nil {
		fatal("创建生成器失败:", err)
	}
	result, err := gen.AddFeature(arch.GenConfig{
		Name:    feature,
		Package: pkg,
		Root:    root,
	}, feature)
	if err != nil {
		fatal("添加功能失败:", err)
	}

	fmt.Println("=== 功能已添加 ===")
	for _, f := range result.Files {
		fmt.Println("  ", f)
	}
}

// ──────────────────────────────────────────────
// serve 命令（启动完整 Web 服务器）
// ──────────────────────────────────────────────
func cmdServe(args []string) {
	port := parseFlag(args, "--port", "8090")
	dir := parseFlag(args, "--dir", ".")

	sourceDir = dir
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fatal("无法解析目录:", err)
	}
	sourceDir = absDir

	log.Printf("arch-cli v%s - 架构治理服务器", version)
	log.Printf("源代码目录: %s", sourceDir)
	log.Printf("监听端口: %s", port)

	// 初始构建
	log.Println("正在解析源代码...")
	data, err := buildArchData(sourceDir)
	if err != nil {
		log.Printf("WARN: 构建架构数据失败: %v", err)
	} else {
		archData = data
		log.Printf("解析完成: %d 文件, %d 行, %d 符号", data.TotalFiles, data.TotalLines, data.TotalSymbols)
	}

	// 启动 AgentPool 事件广播
	agentPool.init()

	// 设置路由
	mux := http.NewServeMux()
	registerRoutes(mux)

	addr := ":" + port
	log.Printf("服务器已启动: http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

// ──────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────
func parseFlag(args []string, flag, def string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return def
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func fatal(v ...interface{}) {
	fmt.Println("错误:", fmt.Sprint(v...))
	os.Exit(1)
}
