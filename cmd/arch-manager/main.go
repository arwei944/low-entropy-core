// Architecture Manager v1.0 — Low-Entropy Core 架构管理器
//
// 功能：
//   - 解析所有 Go 源文件，提取原子级符号清单
//   - 构建文件依赖图
//   - 提供 REST API 供前端交互
//   - 实时文件变更检测（轮询模式）
//   - 嵌入式 Web 前端
//
// 用法: go run ./cmd/arch-manager [--port=8090] [--dir=./go-core] [--watch]
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// HTTP API 服务器
// ============================================================================

var (
	archData    *ArchData
	archMu      sync.RWMutex
	sourceDir   string
	enableWatch bool
)

// ============================================================================
// 主入口
// ============================================================================

func main() {
	port := "8090"
	dir := "."

	// 解析命令行参数
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "--port=") {
			port = strings.TrimPrefix(arg, "--port=")
		} else if strings.HasPrefix(arg, "--dir=") {
			dir = strings.TrimPrefix(arg, "--dir=")
		} else if arg == "--watch" {
			enableWatch = true
		}
	}

	sourceDir = dir

	// 确保目录存在
	absDir, err := filepath.Abs(dir)
	if err != nil {
		log.Fatalf("无法解析目录: %v", err)
	}
	sourceDir = absDir

	log.Printf("Architecture Manager v1.0")
	log.Printf("源代码目录: %s", sourceDir)
	log.Printf("监听端口: %s", port)
	log.Printf("文件监视: %v", enableWatch)

	// 初始构建
	log.Println("正在解析源代码...")
	data, err := buildArchData(sourceDir)
	if err != nil {
		log.Fatalf("构建架构数据失败: %v", err)
	}
	archData = data
	log.Printf("解析完成: %d 文件, %d 行, %d 符号", data.TotalFiles, data.TotalLines, data.TotalSymbols)

	// 启动文件监视
	if enableWatch {
		go watchFiles(sourceDir, 3*time.Second)
	}

	// 启动 AgentPool 事件广播
	agentPool.init()

	// 设置路由
	mux := http.NewServeMux()
	registerRoutes(mux)

	addr := ":" + port
	log.Printf("架构管理器已启动: http://localhost%s/arch-manager.html", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
