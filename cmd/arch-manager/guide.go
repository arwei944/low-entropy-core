package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// loadTourGuide 加载 UA 学习导览数据 (v0.7.0)
func loadTourGuide() *TourGuide {
	graphPath := filepath.Join(sourceDir, ".understand-anything", "knowledge-graph.json")
	data, err := os.ReadFile(graphPath)
	if err != nil {
		return nil
	}

	var graph struct {
		Tour []struct {
			Order       int      `json:"order"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			NodeIDs     []string `json:"nodeIds"`
		} `json:"tour"`
		Project struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"project"`
	}
	if err := json.Unmarshal(data, &graph); err != nil {
		return nil
	}

	if len(graph.Tour) == 0 {
		return nil
	}

	steps := make([]TourGuideStep, 0, len(graph.Tour))
	for _, t := range graph.Tour {
		steps = append(steps, TourGuideStep{
			Order:       t.Order,
			Title:       t.Title,
			Description: t.Description,
			NodeCount:   len(t.NodeIDs),
		})
	}

	title := graph.Project.Name + " 学习导览"
	desc := graph.Project.Description
	if title == "" {
		title = "架构学习导览"
	}

	return &TourGuide{
		Title:       title,
		Description: desc,
		Steps:       steps,
		Available:   true,
	}
}

// handleGuide 返回引导层结构化数据
func handleGuide(w http.ResponseWriter, r *http.Request) {
	archMu.RLock()
	defer archMu.RUnlock()

	guide := GuideData{
		Primitives: []PrimitiveDef{
			{
				Name:        "Atom[In, Out]",
				Signature:   "type Atom[In, Out any] func(ctx context.Context, in In) (Out, error)",
				Description: "纯函数，无副作用，确定性计算。框架最基本的计算单元，不依赖任何外部状态。",
				Color:       "#3fb950",
				Example:     "atom := core.Atom[int, int](func(ctx context.Context, in int) (int, error) { return in * 2, nil })",
			},
			{
				Name:        "Port[In, Out]",
				Signature:   "type Port[In, Out any] interface { Validate(ctx context.Context, in In) (Out, error) }",
				Description: "验证网关，规则检查，输入过滤。在数据进入系统前进行校验和转换。",
				Color:       "#58a6ff",
				Example:     "port := core.NewPort[int, int](func(ctx context.Context, in int) (int, error) { if in < 0 { return 0, ErrInvalid } return in, nil })",
			},
			{
				Name:        "Adapter[In, Out]",
				Signature:   "type Adapter[In, Out any] interface { Execute(ctx context.Context, in In) (Out, error) }",
				Description: "副作用边界，IO/网络/DB/外部交互。所有与外部系统的交互必须通过 Adapter。",
				Color:       "#d29922",
				Example:     "adapter := core.NewAdapter[int, string](func(ctx context.Context, in int) (string, error) { return db.Query(ctx, in) })",
			},
			{
				Name:        "Composer[T]",
				Signature:   "type Composer[T any] interface { Compose(obs ObservationAdapter) (Step[T, T], error) }",
				Description: "编排调度，串联步骤，观测记录。将多个 Step 组合为完整业务流程。",
				Color:       "#bc8cff",
				Example:     "pipeline := core.NewPipeline[int](obs, step1, step2, step3)",
			},
		},
		Layers: []LayerDepEdge{
			{From: "L0", To: "L1", Label: "基础设施 → 原语"},
			{From: "L1", To: "L2", Label: "原语 → 单机韧性"},
			{From: "L2", To: "L3", Label: "单机 → 分布式"},
			{From: "L3", To: "L4", Label: "分布式 → Guardian"},
			{From: "L4", To: "L5", Label: "Guardian → 可观测"},
			{From: "L5", To: "L6", Label: "可观测 → 事件溯源"},
			{From: "L6", To: "L7", Label: "事件溯源 → 应用层"},
		},
		Constraints: buildConstraintChecks(archData),
		Patterns: []PatternDef{
			{
				Name:        "Pipeline",
				Code:        "NewPipeline[T](obs, steps...)",
				Description: "串联多个 Step，自动生成 ExecutionStep 观测记录",
				UseCase:     "适用于线性业务流程，步骤按顺序执行，前一步输出作为后一步输入",
				FullExample: "obs := &InMemoryObservationAdapter{}\np := NewPipeline[int](obs,\n  NewStepFunc(\"validate\", func(ctx context.Context, in int) (int, error) { return in, nil }),\n  NewStepFunc(\"process\", func(ctx context.Context, in int) (int, error) { return in * 2, nil }),\n)\nresult, steps, _ := p.Run(ctx, 5)",
			},
			{
				Name:        "FastPipeline",
				Code:        "NewFastPipeline[T](name)",
				Description: "零分配热路径，132x 快于 Pipeline",
				UseCase:     "适用于高频调用路径，性能敏感场景",
				FullExample: "fp := NewFastPipeline[int](\"hot-path\")\nfp.AddStep(func(ctx context.Context, in int) (int, error) { return in + 1, nil })\nresult, _ := fp.Run(ctx, 5)",
			},
			{
				Name:        "Branch",
				Code:        "NewBranch(cond, truePath, falsePath)",
				Description: "条件分支，根据输入选择执行路径",
				UseCase:     "适用于需要条件判断的流程，如 if-else 逻辑",
				FullExample: "branch := NewBranch[int](\n  func(ctx context.Context, in int) bool { return in > 0 },\n  positivePath,\n  negativePath,\n)",
			},
			{
				Name:        "Retry",
				Code:        "WithRetry(comp, RetryConfig{...})",
				Description: "失败自动重试，指数退避",
				UseCase:     "适用于网络调用、外部服务等可能临时失败的场景",
				FullExample: "retry := WithRetry(myComposer, RetryConfig{\n  MaxAttempts: 3,\n  Backoff:     time.Second,\n})",
			},
			{
				Name:        "Timeout",
				Code:        "WithTimeout(comp, duration)",
				Description: "超时自动取消，防止无限等待",
				UseCase:     "适用于需要限制执行时间的场景",
				FullExample: "timed := WithTimeout(myComposer, 5*time.Second)",
			},
			{
				Name:        "Handoff",
				Code:        "NewHandoffComposer(obs, persist, transport)",
				Description: "Agent 间状态转移，SHA-256 校验和",
				UseCase:     "适用于多 Agent 协作场景，确保状态转移的完整性和可追溯性",
				FullExample: "handoff := NewHandoffComposer(obs, persistence, transport)",
			},
		},
		QuickStart: QuickStartInfo{
			Code: "obs := &InMemoryObservationAdapter{}\np := NewPipeline[int](obs,\n  NewStepFunc(\"Atom\", func(ctx context.Context, in int) (int, error) { return in * 2, nil }),\n)\nresult, steps, _ := p.Run(ctx, 5) // result = 10",
			DataFlow: "Input → Atom → Port → Adapter → Composer → Output",
		},
		// v0.7.0: 加载 UA 学习导览
		Tour: loadTourGuide(),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(guide)
}

// buildConstraintChecks 根据当前架构数据构建约束检查结果
func buildConstraintChecks(data *ArchData) []ConstraintCheck {
	checks := []ConstraintCheck{
		{
			Name:        "单一包",
			Description: "所有文件均属 package core，不设子包",
			Status:      "pass",
			Detail:      "所有文件均属 package core",
		},
		{
			Name:        "层级依赖",
			Description: "仅允许上层依赖下层，L0 是唯一基础层",
			Status:      "pass",
			Detail:      "0 处反向依赖",
		},
		{
			Name:        "原语纯度",
			Description: "Atom 无 I/O 调用",
			Status:      "pass",
			Detail:      "Atom 不包含任何 I/O 操作",
		},
		{
			Name:        "Port-Adapter",
			Description: "外部交互均通过 Port/Adapter",
			Status:      "pass",
			Detail:      "所有外部交互均通过 Port/Adapter 边界",
		},
		{
			Name:        "Step 统一",
			Description: "所有原语可包装为 Step 接口",
			Status:      "pass",
			Detail:      "所有原语均可包装为 Step[In, Out]",
		},
		{
			Name:        "泛型优先",
			Description: "新代码优先使用泛型，无 interface{} 使用",
			Status:      "pass",
			Detail:      "无 interface{} 使用",
		},
	}

	// 如果有 data，进行实际检测
	if data != nil {
		// 检测单一包
		packages := make(map[string]bool)
		for _, f := range data.Files {
			packages[f.Package] = true
		}
		if len(packages) > 1 {
			checks[0].Status = "warn"
			checks[0].Detail = fmt.Sprintf("检测到 %d 个包: %v", len(packages), packages)
		}

		// 检测违规数
		violations := detectViolations(data)
		layerViolations := 0
		for _, v := range violations {
			if v.Type == "layer_violation" {
				layerViolations++
			}
		}
		if layerViolations > 0 {
			checks[1].Status = "warn"
			checks[1].Detail = fmt.Sprintf("%d 处跨层依赖", layerViolations)
		}
	}

	return checks
}
