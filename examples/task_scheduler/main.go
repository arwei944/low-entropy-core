package main

import (
	"fmt"
	"./go-core"  // 相对路径，便于从项目根目录直接运行
	"time"
)

// 短期目标示例：中等规模任务调度系统
// 完全使用 4 个单元实现：Atom, Port, Adapter, Composer

// 任务状态
type Task struct {
	ID     string
	Status string // "pending", "running", "completed", "failed"
	Data   map[string]interface{}
}

// Atom: 状态转换
func transitionState(input interface{}) interface{} {
	task := input.(Task)
	switch task.Status {
	case "pending":
		task.Status = "running"
	case "running":
		task.Status = "completed"
	}
	fmt.Printf("[Atom] 任务 %s 状态 -> %s\n", task.ID, task.Status)
	return task
}

// Atom: 分配资源
func allocateResource(input interface{}) interface{} {
	task := input.(Task)
	task.Data["resource"] = "cpu-1"
	fmt.Printf("[Atom] 为 %s 分配资源\n", task.ID)
	return task
}

// Port: 任务验证契约
type TaskPort struct{}

func (p *TaskPort) Call(input interface{}) interface{} {
	task := input.(Task)
	if task.ID == "" {
		return map[string]interface{}{"error": "invalid task"}
	}
	return task
}

// Adapter: 持久化（唯一副作用）
type PersistenceAdapter struct{}

func (a *PersistenceAdapter) Save(input interface{}) interface{} {
	task := input.(Task)
	fmt.Printf("[Adapter] 持久化任务 %s 状态: %s\n", task.ID, task.Status)
	return task
}

// Adapter: 日志
type LogAdapter struct{}

func (l *LogAdapter) Log(msg string) {
	fmt.Printf("[Log] %s\n", msg)
}

func main() {
	fmt.Println("=== 任务调度系统示例（纯 4 单元实现） ===")

	validatePort := &TaskPort{}
	persist := &PersistenceAdapter{}
	log := &LogAdapter{}

	// 使用 Composer 构建调度流程
	scheduler := core.NewPipeline(
		func(i interface{}) interface{} {
			// 通过 Port 验证
			return validatePort.Call(i)
		},
		transitionState,     // Atom
		allocateResource,    // Atom
		func(i interface{}) interface{} {
			// 持久化
			return persist.Save(i)
		},
	)

	// 创建初始任务
	initialTask := Task{
		ID:     "task-042",
		Status: "pending",
		Data:   map[string]interface{}{},
	}

	log.Log("开始调度任务")

	// 执行
	final := scheduler.Run(initialTask).(Task)

	log.Log("调度完成")

	fmt.Printf("\n最终任务状态: %+v\n", final)
	time.Sleep(100 * time.Millisecond)
}
