// Package main — Agent Demo 入口。
//
// 架构说明：
//   - agent_demo_types.go    : Config、Task、CodeSubmission、AgentMetrics 等类型定义
//   - agent_demo_handlers.go : submitCode、displayOutput、validateSubmission 等业务逻辑
package main

import (
	"context"
	"fmt"
	"os"
	"time"
)

func main() {
	// 1. 解析命令行参数
	args := os.Args[1:]
	taskDesc := "default task"
	if len(args) > 0 {
		taskDesc = args[0]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 2. 准备代码提交
	submission := CodeSubmission{
		TaskID:    fmt.Sprintf("task-%d", time.Now().Unix()),
		Code:      fmt.Sprintf("// %s\nfmt.Println(\"Hello, Agent!\")", taskDesc),
		Language:  "go",
		Timestamp: time.Now(),
		Status:    "pending",
	}

	// 3. 验证提交
	if err := validateSubmission(submission.Code); err != nil {
		fmt.Printf("[Error] Validation failed: %v\n", err)
		os.Exit(1)
	}

	// 4. 提交到 Agent 执行
	fmt.Printf("[Agent] Starting execution for task: %s\n", submission.TaskID)
	result, err := submitCode(ctx, submission)
	if err != nil {
		fmt.Printf("[Error] Execution failed: %v\n", err)
		os.Exit(1)
	}

	// 5. 展示输出
	displayOutput(result, submission.TaskID)

	// 6. 更新指标
	globalMetrics.Completed++
	globalMetrics.LastTaskTime = time.Now()
	if globalMetrics.TotalTasks > 0 {
		globalMetrics.SuccessRate = float64(globalMetrics.Completed) / float64(globalMetrics.TotalTasks)
	}

	fmt.Printf("[Agent] Task %s completed. Total: %d, Success rate: %.1f%%\n",
		submission.TaskID, globalMetrics.Completed, globalMetrics.SuccessRate*100)
}
