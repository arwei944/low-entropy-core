package main

import (
	"fmt"
	"os"
	"time"
)

func cmdInit(statePath, mdPath string) error {
	if _, err := os.Stat(statePath); err == nil {
		fmt.Printf("⚠️  进度文件已存在: %s\n", statePath)
		return cmdRender(statePath, mdPath)
	}
	p := &Project{
		Name:        "架构管理器重构",
		Description: "架构管理器 + CLI 工具生产级可用化改造",
		CreatedAt:   time.Now().Format(timeLayout),
		UpdatedAt:   time.Now().Format(timeLayout),
		Tasks:       defaultTasks(),
		Events: []Event{{
			Time: time.Now().Format(timeLayout), TaskID: "*",
			Action: "init", Detail: "进度追踪工具初始化",
		}},
	}
	if err := writeProject(statePath, p); err != nil {
		return err
	}
	fmt.Printf("✅ 进度文件已创建: %s\n", statePath)
	return cmdRender(statePath, mdPath)
}

func cmdStart(statePath, mdPath, id string) error {
	p, err := readProject(statePath)
	if err != nil {
		return err
	}
	t, idx := findTask(p, id)
	if t == nil {
		return fmt.Errorf("未找到任务: %s", id)
	}
	p.Tasks[idx].Status = StatusInProgress
	p.Tasks[idx].Progress = 10
	if t.StartedAt == "" {
		p.Tasks[idx].StartedAt = time.Now().Format(timeLayout)
	}
	addEvent(p, id, "start", fmt.Sprintf("开始任务: %s", t.Title))
	if err := writeProject(statePath, p); err != nil {
		return err
	}
	fmt.Printf("🚀 %s 开始: %s\n", id, t.Title)
	return cmdRender(statePath, mdPath)
}

func cmdComplete(statePath, mdPath, id string) error {
	p, err := readProject(statePath)
	if err != nil {
		return err
	}
	t, idx := findTask(p, id)
	if t == nil {
		return fmt.Errorf("未找到任务: %s", id)
	}
	p.Tasks[idx].Status = StatusCompleted
	p.Tasks[idx].Progress = 100
	p.Tasks[idx].CompletedAt = time.Now().Format(timeLayout)
	addEvent(p, id, "complete", fmt.Sprintf("完成任务: %s", t.Title))
	if err := writeProject(statePath, p); err != nil {
		return err
	}
	fmt.Printf("✅ %s 完成: %s\n", id, t.Title)
	return cmdRender(statePath, mdPath)
}

func cmdNote(statePath, mdPath, id, note string) error {
	p, err := readProject(statePath)
	if err != nil {
		return err
	}
	t, idx := findTask(p, id)
	if t == nil {
		return fmt.Errorf("未找到任务: %s", id)
	}
	p.Tasks[idx].Notes = append(p.Tasks[idx].Notes,
		fmt.Sprintf("[%s] %s", time.Now().Format(timeLayout), note))
	addEvent(p, id, "note", note)
	if err := writeProject(statePath, p); err != nil {
		return err
	}
	fmt.Printf("📝 %s 添加备注: %s\n", id, note)
	return cmdRender(statePath, mdPath)
}

func cmdSetProgress(statePath, mdPath, id string, pct int) error {
	if pct < 0 || pct > 100 {
		return fmt.Errorf("进度百分比必须在 0-100 之间")
	}
	p, err := readProject(statePath)
	if err != nil {
		return err
	}
	t, idx := findTask(p, id)
	if t == nil {
		return fmt.Errorf("未找到任务: %s", id)
	}
	if pct == 100 {
		p.Tasks[idx].Status = StatusCompleted
		p.Tasks[idx].CompletedAt = time.Now().Format(timeLayout)
	} else if pct > 0 {
		p.Tasks[idx].Status = StatusInProgress
		if t.StartedAt == "" {
			p.Tasks[idx].StartedAt = time.Now().Format(timeLayout)
		}
	}
	p.Tasks[idx].Progress = pct
	addEvent(p, id, "progress", fmt.Sprintf("进度更新到 %d%%", pct))
	if err := writeProject(statePath, p); err != nil {
		return err
	}
	fmt.Printf("📊 %s 进度: %d%%\n", id, pct)
	return cmdRender(statePath, mdPath)
}

func cmdStatus(statePath string) error {
	p, err := readProject(statePath)
	if err != nil {
		return err
	}
	total := len(p.Tasks)
	done, inprog, pending := 0, 0, 0
	totalPct := 0
	for _, t := range p.Tasks {
		totalPct += t.Progress
		switch t.Status {
		case StatusCompleted:
			done++
		case StatusInProgress:
			inprog++
		default:
			pending++
		}
	}
	overall := totalPct / total
	fmt.Println("================ 项目进度 ================")
	fmt.Printf("  项目: %s\n", p.Name)
	fmt.Printf("  总任务: %d  |  ✅ %d  |  🚀 %d  |  ⏳ %d\n", total, done, inprog, pending)
	fmt.Printf("  整体进度: [%s] %d%%\n", renderBar(overall, 30), overall)
	fmt.Printf("  创建: %s  |  更新: %s\n", p.CreatedAt, p.UpdatedAt)
	fmt.Println()
	fmt.Println("---------- 任务列表 ----------")
	for _, t := range p.Tasks {
		mark := "⏳"
		switch t.Status {
		case StatusCompleted:
			mark = "✅"
		case StatusInProgress:
			mark = "🚀"
		case StatusSkipped:
			mark = "⚪"
		}
		fmt.Printf("  %s %-8s %3d%%  %s\n", mark, t.ID, t.Progress, t.Title)
	}
	fmt.Println()
	if len(p.Events) > 0 {
		fmt.Println("---------- 最近事件 ----------")
		last := len(p.Events)
		if last > 5 {
			last = 5
		}
		for _, e := range p.Events[len(p.Events)-last:] {
			fmt.Printf("  [%s] %-8s %-10s %s\n", e.Time, e.TaskID, e.Action, e.Detail)
		}
	}
	return nil
}
