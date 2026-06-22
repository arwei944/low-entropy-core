package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultStateFile = "progress.json"
	defaultMarkdown  = "PROGRESS.md"
	timeLayout       = "2006-01-02 15:04:05"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	StatusSkipped    TaskStatus = "skipped"
)

type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Phase       string     `json:"phase"`
	Priority    string     `json:"priority"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	Progress    int        `json:"progress"`
	StartedAt   string     `json:"started_at,omitempty"`
	CompletedAt string     `json:"completed_at,omitempty"`
	Notes       []string   `json:"notes,omitempty"`
}

type Project struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	Tasks       []Task `json:"tasks"`
	Events      []Event `json:"events"`
}

type Event struct {
	Time   string `json:"time"`
	TaskID string `json:"task_id"`
	Action string `json:"action"`
	Detail string `json:"detail,omitempty"`
}

func readProject(path string) (*Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取进度文件失败: %w\n请先运行: progress init", err)
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("解析进度文件失败: %w", err)
	}
	return &p, nil
}

func writeProject(path string, p *Project) error {
	p.UpdatedAt = time.Now().Format(timeLayout)
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func addEvent(p *Project, taskID, action, detail string) {
	p.Events = append(p.Events, Event{
		Time:   time.Now().Format(timeLayout),
		TaskID: taskID,
		Action: action,
		Detail: detail,
	})
	if len(p.Events) > 500 {
		p.Events = p.Events[len(p.Events)-500:]
	}
}

func findTask(p *Project, id string) (*Task, int) {
	for i := range p.Tasks {
		if strings.EqualFold(p.Tasks[i].ID, id) {
			return &p.Tasks[i], i
		}
	}
	return nil, -1
}
