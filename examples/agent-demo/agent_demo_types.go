// Package main — Agent Demo 类型定义。
// 包含 Config、Task、CodeSubmission、AgentMetrics 及全局配置。
package main

import "time"

// Config Agent Demo 配置。
type Config struct {
	MaxConcurrent int
	AgentName     string
	Model         string
	MaxRetries    int
	RetryDelay    time.Duration
	OutputLimit   int
	Verbose       bool
	Metrics       bool
}

// Task AI Agent 任务。
type Task struct {
	ID          string
	Description string
	Priority    int
	Status      string
}

// CodeSubmission 代码提交。
type CodeSubmission struct {
	TaskID    string
	Code      string
	Language  string
	Timestamp time.Time
	Status    string
	Output    string
	Error     error
}

// AgentMetrics Agent 运行指标。
type AgentMetrics struct {
	TotalTasks   int
	Completed    int
	Failed       int
	Pending      int
	AvgLatency   time.Duration
	SuccessRate  float64
	StartTime    time.Time
	LastTaskTime time.Time
}

// ──────────────────────────────────────────────
// 全局配置
// ──────────────────────────────────────────────

var defaultConfig = Config{
	MaxConcurrent: 5,
	AgentName:     "CodeAgent",
	Model:         "claude-opus-4",
	MaxRetries:    3,
	RetryDelay:    2 * time.Second,
	OutputLimit:   5000,
	Verbose:       true,
	Metrics:       true,
}

var globalMetrics = AgentMetrics{
	TotalTasks:   0,
	Completed:    0,
	Failed:       0,
	Pending:      0,
	AvgLatency:   0,
	SuccessRate:  0,
	StartTime:    time.Now(),
	LastTaskTime: time.Now(),
}
