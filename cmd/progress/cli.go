package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

func printUsage() {
	fmt.Println(`用法:
  progress <命令> [参数]

命令:
  init                    初始化进度文件 (首次使用)
  start   <TU-ID>         标记任务开始
  complete<TU-ID>         标记任务完成
  note    <TU-ID> <备注>  添加任务备注
  set     <TU-ID> <0-100> 设置任务进度百分比
  status                  显示当前进度摘要
  render                  重新渲染 PROGRESS.md
  watch                   监听 progress.json 变化并自动渲染

示例:
  progress init
  progress start TU-1
  progress note TU-3 "已完成 API 拉取逻辑"
  progress complete TU-1
  progress status`)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// 路径解析：优先使用当前目录下的文件
	statePath := filepath.Join(".", defaultStateFile)
	mdPath := filepath.Join(".", defaultMarkdown)

	// 支持通过环境变量覆盖路径
	if v := os.Getenv("PROGRESS_STATE"); v != "" {
		statePath = v
	}
	if v := os.Getenv("PROGRESS_MD"); v != "" {
		mdPath = v
	}

	cmd := os.Args[1]
	var err error

	switch cmd {
	case "init":
		err = cmdInit(statePath, mdPath)

	case "start":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定任务 ID, 例如: progress start TU-1")
			os.Exit(1)
		}
		err = cmdStart(statePath, mdPath, os.Args[2])

	case "complete":
		if len(os.Args) < 3 {
			fmt.Println("❌ 请指定任务 ID, 例如: progress complete TU-1")
			os.Exit(1)
		}
		err = cmdComplete(statePath, mdPath, os.Args[2])

	case "note":
		if len(os.Args) < 4 {
			fmt.Println("❌ 请指定任务 ID 和备注, 例如: progress note TU-1 \"备注内容\"")
			os.Exit(1)
		}
		note := os.Args[3]
		// 支持多字符合并
		for i := 4; i < len(os.Args); i++ {
			note += " " + os.Args[i]
		}
		err = cmdNote(statePath, mdPath, os.Args[2], note)

	case "set":
		if len(os.Args) < 4 {
			fmt.Println("❌ 请指定任务 ID 和进度, 例如: progress set TU-1 50")
			os.Exit(1)
		}
		pct, pErr := strconv.Atoi(os.Args[3])
		if pErr != nil {
			fmt.Println("❌ 进度必须是 0-100 的数字")
			os.Exit(1)
		}
		err = cmdSetProgress(statePath, mdPath, os.Args[2], pct)

	case "status":
		err = cmdStatus(statePath)

	case "render":
		err = cmdRender(statePath, mdPath)

	case "watch":
		err = cmdWatch(statePath, mdPath)

	case "-h", "--help", "help":
		printUsage()

	default:
		fmt.Printf("❌ 未知命令: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ 错误: %v\n", err)
		os.Exit(1)
	}
}
