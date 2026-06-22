package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// renderBar 渲染 ASCII 进度条
func renderBar(pct, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := (pct * width) / 100
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bar
}

// renderMarkdownBar 渲染 Markdown 友好进度条
func renderMarkdownBar(pct int) string {
	return fmt.Sprintf("`%s` %d%%", renderBar(pct, 20), pct)
}

// cmdRender 渲染 Markdown 文档
func cmdRender(statePath, mdPath string) error {
	p, err := readProject(statePath)
	if err != nil {
		return err
	}

	var sb strings.Builder

	// ---- 顶部标题
	sb.WriteString(fmt.Sprintf("# %s · 进度同步\n\n", p.Name))
	sb.WriteString(fmt.Sprintf("> **自动生成于**: `%s`\n\n", time.Now().Format(timeLayout)))
	sb.WriteString(fmt.Sprintf("> **描述**: %s\n\n", p.Description))

	// ---- 汇总面板
	total := len(p.Tasks)
	done, inprog, pending, skipped := 0, 0, 0, 0
	totalPct := 0
	for _, t := range p.Tasks {
		totalPct += t.Progress
		switch t.Status {
		case StatusCompleted:
			done++
		case StatusInProgress:
			inprog++
		default:
			if t.Status == StatusSkipped {
				skipped++
			} else {
				pending++
			}
		}
	}
	overall := 0
	if total > 0 {
		overall = totalPct / total
	}

	sb.WriteString("## 📊 总体进度\n\n")
	sb.WriteString("| 指标 | 数值 |\n|---|---|\n")
	sb.WriteString(fmt.Sprintf("| **总任务数** | %d |\n", total))
	sb.WriteString(fmt.Sprintf("| ✅ 已完成 | %d |\n", done))
	sb.WriteString(fmt.Sprintf("| 🚀 进行中 | %d |\n", inprog))
	sb.WriteString(fmt.Sprintf("| ⏳ 待开始 | %d |\n", pending))
	if skipped > 0 {
		sb.WriteString(fmt.Sprintf("| ⚪ 已跳过 | %d |\n", skipped))
	}
	sb.WriteString(fmt.Sprintf("| **整体完成度** | **%d%%** |\n", overall))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("**进度条**: %s\n\n", renderMarkdownBar(overall)))

	// ---- 按阶段汇总
	sb.WriteString("## 🧩 按阶段进度\n\n")
	phaseMap := make(map[string][]Task)
	phaseOrder := []string{"Phase 1", "Phase 2", "Phase 3", "Phase 4", "Phase 5", "Phase 6"}
	for _, t := range p.Tasks {
		phaseMap[t.Phase] = append(phaseMap[t.Phase], t)
	}
	for _, phase := range phaseOrder {
		tasks, ok := phaseMap[phase]
		if !ok || len(tasks) == 0 {
			continue
		}
		sumPct := 0
		d, ip := 0, 0
		for _, t := range tasks {
			sumPct += t.Progress
			if t.Status == StatusCompleted {
				d++
			} else if t.Status == StatusInProgress {
				ip++
			}
		}
		pct := sumPct / len(tasks)
		sb.WriteString(fmt.Sprintf("### %s  \n", phase))
		sb.WriteString(fmt.Sprintf("- 任务: %d (✅%d 🚀%d)  平均进度: %s\n\n", len(tasks), d, ip, renderMarkdownBar(pct)))
	}

	// ---- 任务列表
	sb.WriteString("## 📋 任务明细\n\n")
	for _, phase := range phaseOrder {
		tasks, ok := phaseMap[phase]
		if !ok || len(tasks) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n", phase))
		sb.WriteString("| # | 任务 | 标题 | 优先级 | 状态 | 进度 | 开始时间 | 完成时间 |\n")
		sb.WriteString("|---|------|------|--------|------|------|---------|----------|\n")
		for _, t := range tasks {
			statusMark := "⏳ 待开始"
			switch t.Status {
			case StatusCompleted:
				statusMark = "✅ 完成"
			case StatusInProgress:
				statusMark = "🚀 进行中"
			case StatusSkipped:
				statusMark = "⚪ 跳过"
			}
			started := "-"
			if t.StartedAt != "" {
				started = t.StartedAt
			}
			completed := "-"
			if t.CompletedAt != "" {
				completed = t.CompletedAt
			}
			sb.WriteString(fmt.Sprintf("| %s | **%s** | %s | `%s` | %s | %s | `%s` | `%s` |\n",
				strings.TrimPrefix(t.ID, "TU-"),
				t.ID,
				t.Title,
				t.Priority,
				statusMark,
				renderMarkdownBar(t.Progress),
				started,
				completed,
			))
		}
		sb.WriteString("\n")
	}

	// ---- 当前进行中的任务
	inProgressTasks := []Task{}
	for _, t := range p.Tasks {
		if t.Status == StatusInProgress {
			inProgressTasks = append(inProgressTasks, t)
		}
	}
	if len(inProgressTasks) > 0 {
		sb.WriteString("## 🚀 进行中任务\n\n")
		for _, t := range inProgressTasks {
			sb.WriteString(fmt.Sprintf("### %s · %s\n\n", t.ID, t.Title))
			sb.WriteString(fmt.Sprintf("- **优先级**: %s\n", t.Priority))
			sb.WriteString(fmt.Sprintf("- **阶段**: %s\n", t.Phase))
			sb.WriteString(fmt.Sprintf("- **进度**: %s\n", renderMarkdownBar(t.Progress)))
			sb.WriteString(fmt.Sprintf("- **开始时间**: `%s`\n", t.StartedAt))
			if t.Description != "" {
				sb.WriteString(fmt.Sprintf("- **描述**: %s\n", t.Description))
			}
			if len(t.Notes) > 0 {
				sb.WriteString("- **备注**:\n")
				for _, n := range t.Notes {
					sb.WriteString(fmt.Sprintf("  - %s\n", n))
				}
			}
			sb.WriteString("\n")
		}
	}

	// ---- 已完成任务汇总
	completedTasks := []Task{}
	for _, t := range p.Tasks {
		if t.Status == StatusCompleted {
			completedTasks = append(completedTasks, t)
		}
	}
	if len(completedTasks) > 0 {
		sb.WriteString("## ✅ 已完成任务汇总\n\n")
		sb.WriteString("| 任务 | 标题 | 进度 | 开始 | 完成 |\n")
		sb.WriteString("|------|------|------|------|------|\n")
		for _, t := range completedTasks {
			sb.WriteString(fmt.Sprintf("| **%s** | %s | %s | `%s` | `%s` |\n",
				t.ID, t.Title, renderMarkdownBar(t.Progress), t.StartedAt, t.CompletedAt))
		}
		sb.WriteString("\n")
	}

	// ---- 事件时间线
	if len(p.Events) > 0 {
		sb.WriteString("## 🕐 事件时间线\n\n")
		count := len(p.Events)
		if count > 30 {
			count = 30
		}
		for i := len(p.Events) - 1; i >= len(p.Events)-count; i-- {
			e := p.Events[i]
			actionEmoji := "•"
			switch e.Action {
			case "init":
				actionEmoji = "🎉"
			case "start":
				actionEmoji = "🚀"
			case "complete":
				actionEmoji = "✅"
			case "note":
				actionEmoji = "📝"
			case "progress":
				actionEmoji = "📊"
			case "skip":
				actionEmoji = "⚪"
			}
			sb.WriteString(fmt.Sprintf("- `%s` %s **%s** — %s\n", e.Time, actionEmoji, e.TaskID, e.Detail))
		}
		sb.WriteString("\n")
	}

	// ---- 备注列表
	hasNotes := false
	for _, t := range p.Tasks {
		if len(t.Notes) > 0 {
			hasNotes = true
			break
		}
	}
	if hasNotes {
		sb.WriteString("## 📝 任务备注\n\n")
		for _, t := range p.Tasks {
			if len(t.Notes) == 0 {
				continue
			}
			sb.WriteString(fmt.Sprintf("### %s · %s\n\n", t.ID, t.Title))
			for _, n := range t.Notes {
				sb.WriteString(fmt.Sprintf("- %s\n", n))
			}
			sb.WriteString("\n")
		}
	}

	// ---- 底部说明
	sb.WriteString("---\n\n")
	sb.WriteString("### 🔧 使用方法\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# 构建工具\ngo build -o progress-tool.exe ./cmd/progress\n\n")
	sb.WriteString("# 标记任务开始\n./progress-tool.exe start TU-1\n\n")
	sb.WriteString("# 标记任务完成\n./progress-tool.exe complete TU-1\n\n")
	sb.WriteString("# 添加备注\n./progress-tool.exe note TU-1 \"已完成 AST 分析\"\n\n")
	sb.WriteString("# 设置进度百分比\n./progress-tool.exe set TU-1 50\n\n")
	sb.WriteString("# 查看状态\n./progress-tool.exe status\n\n")
	sb.WriteString("# 手动重新渲染 Markdown\n./progress-tool.exe render\n")
	sb.WriteString("```\n\n")
	sb.WriteString("> 每次调用 `start` / `complete` / `note` / `set` 命令后，都会自动更新 `PROGRESS.md`\n\n")

	if err := os.WriteFile(mdPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("写入 Markdown 文件失败: %w", err)
	}
	fmt.Printf("📄 Markdown 已生成: %s\n", mdPath)
	return nil
}

// cmdWatch 监听文件变化并自动渲染
func cmdWatch(statePath, mdPath string) error {
	fmt.Printf("👁️  正在监听 %s 的变化... (Ctrl+C 退出)\n", statePath)
	var lastMod time.Time
	for {
		info, err := os.Stat(statePath)
		if err == nil && info.ModTime() != lastMod {
			lastMod = info.ModTime()
			if err := cmdRender(statePath, mdPath); err != nil {
				fmt.Fprintf(os.Stderr, "渲染失败: %v\n", err)
			}
		}
		time.Sleep(2 * time.Second)
	}
}
