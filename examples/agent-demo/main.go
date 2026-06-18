package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "go-core"
)

func main() {
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("  Phase 2 P5 端到端演示：Agent Workbench 完整生命周期")
	fmt.Println("  AgentCodeSubmission → StaticGuardPort → AgentRunner → Handoff")
	fmt.Println(strings.Repeat("=", 70))

	// ──────────────────────────────────────────────
	// 步骤 1：初始化基础设施
	// ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println("[1/7] 初始化基础设施：AgentPool + TaskQueue + SchedulerComposer")
	fmt.Println(strings.Repeat("-", 50))

	obs := &core.InMemoryObservationAdapter{}
	pool := core.NewAgentPool()
	queue := core.NewTaskQueue()
	transport := core.NewInProcHandoffTransport()
	persistence := core.NewInMemorySnapshotAdapter()
	handoff := core.NewHandoffComposer(obs, persistence, transport)
	scheduler := core.NewSchedulerComposer(pool, queue, handoff, obs)

	// 构建 StaticGuardPort（深度静态审核）
	staticGuard := core.NewStaticGuardPort(
		core.GlobalPrimitiveTypeSet,
		core.NewArchitectureGuard(),
		obs,
	)

	decisionEngine := core.NewDecisionEngine(obs)
	runner := core.NewAgentRunner("", obs)
	workbench := core.NewDefaultAgentWorkbench(obs, staticGuard, decisionEngine, runner)

	_ = scheduler // SchedulerComposer 已就绪，供后续调度使用

	fmt.Println("  AgentPool          : 已创建")
	fmt.Println("  TaskQueue          : 已创建（容量无限制）")
	fmt.Println("  SchedulerComposer  : 已创建（绑定 Pool + Queue + Handoff）")
	fmt.Println("  StaticGuardPort    : 已创建（4 项静态检查）")
	fmt.Println("  DecisionEngine     : 已创建（4 级决策引擎）")
	fmt.Println("  AgentRunner        : 已创建（编译 + 执行）")
	fmt.Println("  DefaultAgentWorkbench : 已创建（串联Submit→Audit→Build→Run）")

	// ──────────────────────────────────────────────
	// 步骤 2：注册 Agent A 和 Agent B
	// ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println("[2/7] 注册 Agent A（计算专家）和 Agent B（展示专家）")
	fmt.Println(strings.Repeat("-", 50))

	err := core.RegisterAgent(pool, "agent-a", []string{"compute"}, "compute")
	if err != nil {
		fmt.Printf("  [错误] 注册 Agent A 失败: %v\n", err)
		return
	}
	err = core.RegisterAgent(pool, "agent-b", []string{"render"}, "render")
	if err != nil {
		fmt.Printf("  [错误] 注册 Agent B 失败: %v\n", err)
		return
	}

	// 发送心跳
	core.AgentHeartbeat(pool, "agent-a")
	core.AgentHeartbeat(pool, "agent-b")

	agentA, _ := pool.Get("agent-a")
	agentB, _ := pool.Get("agent-b")
	fmt.Printf("  Agent A: ID=%s | 能力=%v | 状态=%s | 阶段=%s\n",
		agentA.ID, agentA.Capabilities, agentA.Status, agentA.Phase)
	fmt.Printf("  Agent B: ID=%s | 能力=%v | 状态=%s | 阶段=%s\n",
		agentB.ID, agentB.Capabilities, agentB.Status, agentB.Phase)
	fmt.Printf("  AgentPool 当前 Agent 数: %d\n", pool.Count())

	// ──────────────────────────────────────────────
	// 步骤 3：Agent A 编写合规代码并提交
	// ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println("[3/7] Agent A 编写合规代码（计算 3+5*2）并提交")
	fmt.Println(strings.Repeat("-", 50))

	// Agent A 编写的 Go 源代码：计算表达式 3+5*2
	codeCompliant := `package main

import "fmt"

func main() {
	// 计算 3+5*2，注意运算符优先级：先乘后加
	result := 3 + 5*2
	fmt.Println("计算结果:", result)
}
`
	submissionA := core.AgentCodeSubmission{
		AgentID:    "agent-a",
		TaskID:     "task-calc-001",
		SourceCode: codeCompliant,
		Manifest: []core.PrimitiveManifest{
			{
				PrimitiveType: "Atom",
				Name:          "computeResult",
				Layer:         "L1",
				InputType:     "int",
				OutputType:    "int",
			},
		},
		Attempt: 1,
	}

	fmt.Println("  提交内容:")
	fmt.Printf("    AgentID   : %s\n", submissionA.AgentID)
	fmt.Printf("    TaskID    : %s\n", submissionA.TaskID)
	fmt.Printf("    Attempt   : %d\n", submissionA.Attempt)
	fmt.Printf("    代码行数  : %d\n", len(strings.Split(submissionA.SourceCode, "\n")))
	fmt.Printf("    Manifest  : %d 个原语声明\n", len(submissionA.Manifest))

	ctx := context.Background()
	resultA, err := workbench.SubmitAndRun(ctx, submissionA)
	if err != nil {
		fmt.Printf("  [错误] SubmitAndRun 失败: %v\n", err)
	} else {
		fmt.Println()
		fmt.Printf("  提交结果: Status=%s | Violations=%d\n",
			resultA.Status, len(resultA.Violations))
		if resultA.Error != "" {
			fmt.Printf("  执行错误: %s\n", resultA.Error)
		}
	}

	// ──────────────────────────────────────────────
	// 步骤 4：打印 ExecutionStep 时间线
	// ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println("[4/7] ExecutionStep 时间线（完整审核→编译→执行追踪）")
	fmt.Println(strings.Repeat("-", 50))

	if len(resultA.ExecutionSteps) == 0 {
		fmt.Println("  （无执行步骤 - AgentRunner 可能未配置或为空）")
	} else {
		for i, step := range resultA.ExecutionSteps {
			idx := fmt.Sprintf("[%02d]", i+1)
			duration := fmt.Sprintf("%dms", step.DurationMs)
			fmt.Printf("  %s %-12s | %-14s | %-40s | %6s\n",
				idx, step.Unit, step.Action, step.Details, duration)
			if step.Error != nil {
				fmt.Printf("      错误: [%s] %s\n", step.Error.Code, step.Error.Message)
			}
			if step.Metadata != nil {
				if output, ok := step.Metadata["output"]; ok {
					fmt.Printf("      输出: %v\n", output)
				}
				if status, ok := step.Metadata["status"]; ok {
					fmt.Printf("      状态: %v\n", status)
				}
			}
		}
	}

	// ──────────────────────────────────────────────
	// 步骤 5：通过 HandoffComposer 将 DevSnapshot 传给 Agent B
	// ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println("[5/7] Handoff: Agent A → Agent B（DevSnapshot 传递）")
	fmt.Println(strings.Repeat("-", 50))

	// Agent A 构建 DevSnapshot：包含计算结果、产物、决策
	snapshot := core.NewDevSnapshot(
		"task-calc-001",
		"agent-a",
		"compute",
		"计算完成: 3+5*2 = 13",
	)
	snapshot.Artifacts = append(snapshot.Artifacts, core.Artifact{
		Path:        "result.txt",
		Type:        "computation",
		Description: "表达式 3+5*2 的计算结果为 13",
		Hash:        "abc123def456",
	})
	snapshot.Artifacts = append(snapshot.Artifacts, core.Artifact{
		Path:        "steps.log",
		Type:        "log",
		Description: "执行步骤日志",
		Hash:        "log789ghi012",
	})
	snapshot.Decisions = append(snapshot.Decisions, core.Decision{
		ID:        "dec-001",
		Title:     "运算符优先级确认",
		Rationale: "Go 中 * 优先级高于 +，因此 3+5*2 = 3+(5*2) = 13",
		Alternatives: []string{
			"显式加括号: (3+5)*2 = 16（不符合需求）",
		},
	})
	snapshot.Constraints = append(snapshot.Constraints,
		"结果必须为整数",
		"使用标准运算符优先级",
	)

	fmt.Println("  DevSnapshot 内容:")
	fmt.Printf("    TaskID      : %s\n", snapshot.TaskID)
	fmt.Printf("    AgentID     : %s\n", snapshot.AgentID)
	fmt.Printf("    Phase       : %s\n", snapshot.Phase)
	fmt.Printf("    Checkpoint  : %s\n", snapshot.Checkpoint)
	fmt.Printf("    Artifacts   : %d 个\n", len(snapshot.Artifacts))
	for _, a := range snapshot.Artifacts {
		fmt.Printf("      - %s [%s]: %s\n", a.Path, a.Type, a.Description)
	}
	fmt.Printf("    Decisions   : %d 个\n", len(snapshot.Decisions))
	for _, d := range snapshot.Decisions {
		fmt.Printf("      - %s: %s\n", d.Title, d.Rationale)
	}
	fmt.Printf("    Constraints : %d 条\n", len(snapshot.Constraints))

	// 执行 Handoff
	handoffOutput, handoffSteps, handoffErr := handoff.Execute(ctx, core.HandoffInput{
		SourceAgent:   snapshot,
		TargetAgentID: "agent-b",
		TaskID:        "task-calc-001",
		Phase:         "render",
	})

	if handoffErr != nil {
		fmt.Printf("  [错误] Handoff 执行失败: %v\n", handoffErr)
	} else {
		fmt.Println()
		fmt.Printf("  Handoff 结果: Success=%v | TargetConfirmed=%v\n",
			handoffOutput.Success, handoffOutput.TargetConfirmed)
		fmt.Printf("  Snapshot 校验和: %s\n", handoffOutput.SnapshotChecksum)
		if handoffOutput.Contract != nil {
			fmt.Printf("  Handoff 合约: Source=%s → Target=%s | Phase=%s\n",
				handoffOutput.Contract.SourceID,
				handoffOutput.Contract.TargetID,
				handoffOutput.Contract.Phase)
		}
		fmt.Println()
		fmt.Println("  Handoff ExecutionSteps:")
		for i, step := range handoffSteps {
			fmt.Printf("    [H%02d] %-10s | %-18s | %-40s | %dms\n",
				i+1, step.Unit, step.Action, step.Details, step.DurationMs)
		}
	}

	// ──────────────────────────────────────────────
	// 步骤 6：Agent B 接收 DevSnapshot 并渲染结果
	// ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println("[6/7] Agent B 接收 DevSnapshot 并渲染结果")
	fmt.Println(strings.Repeat("-", 50))

	if handoffOutput.Success {
		receivedSnapshot, receiveSteps, receiveErr := handoff.ReceiveSnapshot(
			ctx, handoffOutput.SnapshotChecksum,
		)

		if receiveErr != nil {
			fmt.Printf("  [错误] 接收快照失败: %v\n", receiveErr)
		} else {
			fmt.Println("  Agent B 成功接收 DevSnapshot:")
			fmt.Printf("    TaskID      : %s\n", receivedSnapshot.TaskID)
			fmt.Printf("    来源 Agent  : %s\n", receivedSnapshot.AgentID)
			fmt.Printf("    阶段        : %s\n", receivedSnapshot.Phase)
			fmt.Printf("    检查点      : %s\n", receivedSnapshot.Checkpoint)
			fmt.Printf("    校验和      : %s\n", receivedSnapshot.Checksum)
			fmt.Printf("    校验和验证  : %v\n", receivedSnapshot.VerifyChecksum())

			fmt.Println()
			fmt.Println("  Agent B 渲染结果:")
			fmt.Println("  ┌──────────────────────────────────────────┐")
			fmt.Printf("  │  Task: %-32s │\n", receivedSnapshot.TaskID)
			fmt.Printf("  │  From: %-32s │\n", receivedSnapshot.AgentID)
			fmt.Printf("  │  Checkpoint: %-28s │\n", receivedSnapshot.Checkpoint)
			fmt.Println("  │                                          │")
			fmt.Println("  │  Artifacts:                              │")
			for _, a := range receivedSnapshot.Artifacts {
				fmt.Printf("  │    [%s] %-30s │\n", a.Type, a.Description)
			}
			fmt.Println("  │                                          │")
			fmt.Println("  │  Decisions:                              │")
			for _, d := range receivedSnapshot.Decisions {
				fmt.Printf("  │    %s: %-30s │\n", d.Title, d.Rationale)
			}
			fmt.Println("  └──────────────────────────────────────────┘")

			fmt.Println()
			fmt.Println("  接收 ExecutionSteps:")
			for i, step := range receiveSteps {
				fmt.Printf("    [R%02d] %-10s | %-18s | %-40s | %dms\n",
					i+1, step.Unit, step.Action, step.Details, step.DurationMs)
			}
		}
	}

	// ──────────────────────────────────────────────
	// 步骤 7：Guardian 介入演示
	// ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println("[7/7] Guardian 介入演示：违规代码 → Block → 修复 → 重新提交")
	fmt.Println(strings.Repeat("-", 50))

	// 7a. 提交违规代码
	fmt.Println()
	fmt.Println("  [7a] 提交违规代码（直接使用 http.Client，未通过 Adapter 包装）")
	fmt.Println("       触发 StaticGuardPort 的 primitive_compliance 检查")

	codeViolating := `package main

import "net/http"

type MyClient struct {
	client *http.Client
}

func main() {
	c := &MyClient{client: &http.Client{}}
	http.Get("http://example.com")
	_ = c
}
`
	violatingSubmission := core.AgentCodeSubmission{
		AgentID:    "agent-a",
		TaskID:     "task-violation-001",
		SourceCode: codeViolating,
		Manifest: []core.PrimitiveManifest{
			{
				PrimitiveType: "Atom",
				Name:          "MyClient",
				Layer:         "L1",
				InputType:     "any",
				OutputType:    "any",
			},
		},
		Attempt: 1,
	}

	fmt.Println("  违规代码片段:")
	for _, line := range strings.Split(strings.TrimSpace(codeViolating), "\n") {
		fmt.Printf("    | %s\n", line)
	}
	fmt.Println()

	violationResult, err := workbench.Submit(ctx, violatingSubmission)
	if err != nil {
		fmt.Printf("  [错误] Submit 失败: %v\n", err)
	}

	fmt.Printf("  审核结果: Status=%s\n", violationResult.Status)
	fmt.Printf("  违规数量: %d\n", len(violationResult.Violations))
	if len(violationResult.Violations) > 0 {
		fmt.Println()
		fmt.Println("  StaticGuardPort 检测到的违规:")
		for i, v := range violationResult.Violations {
			fmt.Printf("  ┌─ 违规 #%d ──────────────────────────────\n", i+1)
			fmt.Printf("  │ 规则      : %s\n", v.Rule)
			fmt.Printf("  │ 严重性    : %s\n", v.Severity)
			fmt.Printf("  │ 位置      : %s\n", v.Location)
			fmt.Printf("  │ 详情      : %s\n", v.Detail)
			fmt.Printf("  │ 修复建议  : %s\n", v.Suggestion)
			fmt.Printf("  └──────────────────────────────────────────\n")
		}
	}

	// 检查是否被 Block
	decision, _ := decisionEngine.Run(ctx, core.GuardianInput{
		StaticReviewResult: core.StaticReviewResult{
			Violations: violationResult.Violations,
		},
	})
	fmt.Printf("  DecisionEngine 决策: Action=%s | Reason=%s\n",
		decision.Action, decision.Reason)

	// 7b. Agent 根据修复建议修改代码
	fmt.Println()
	fmt.Println("  [7b] Agent 根据修复建议修改代码：将 http.Client 包装为 Adapter 原语")
	fmt.Println("       修改点：")
	fmt.Println("       1. 移除 net/http import")
	fmt.Println("       2. 创建 httpAdapter 结构体实现 Adapter 接口")
	fmt.Println("       3. 将 httpAdapter 的 Manifest 类型改为 Adapter，层级改为 L7")

	codeFixed := `package main

import "fmt"

// httpAdapter 将 HTTP 客户端包装为 Adapter 原语
// 实现了 Adapter 接口，隔离外部 I/O 依赖
type httpAdapter struct {
	// 内部持有 http.Client，但通过 Adapter 模式隔离
}

func (a *httpAdapter) Execute() {
	fmt.Println("HTTP call via Adapter primitive - compliant with Low-Entropy Core")
}

func main() {
	adapter := &httpAdapter{}
	adapter.Execute()
}
`
	fixedSubmission := core.AgentCodeSubmission{
		AgentID:    "agent-a",
		TaskID:     "task-violation-001",
		SourceCode: codeFixed,
		Manifest: []core.PrimitiveManifest{
			{
				PrimitiveType: "Adapter",
				Name:          "httpAdapter",
				Layer:         "L7",
				InputType:     "any",
				OutputType:    "any",
			},
		},
		Attempt: 2,
	}

	fmt.Println()
	fmt.Println("  修复后代码片段:")
	for _, line := range strings.Split(strings.TrimSpace(codeFixed), "\n") {
		fmt.Printf("    | %s\n", line)
	}
	fmt.Println()

	fixedResult, err := workbench.SubmitAndRun(ctx, fixedSubmission)
	if err != nil {
		fmt.Printf("  [错误] 重新提交失败: %v\n", err)
	}

	fmt.Printf("  重新提交结果: Status=%s | Violations=%d | Attempt=%d\n",
		fixedResult.Status, len(fixedResult.Violations), fixedSubmission.Attempt)

	if len(fixedResult.Violations) > 0 {
		fmt.Println("  （仍有违规，需要继续修复）")
		for _, v := range fixedResult.Violations {
			fmt.Printf("    规则: %s | 严重性: %s | %s\n", v.Rule, v.Severity, v.Detail)
		}
	} else {
		fmt.Println("  审核通过！代码已通过 StaticGuardPort 全部检查。")
	}

	if len(fixedResult.ExecutionSteps) > 0 {
		fmt.Println()
		fmt.Println("  修复后提交的 ExecutionSteps:")
		for i, step := range fixedResult.ExecutionSteps {
			fmt.Printf("    [F%02d] %-10s | %-18s | %-40s | %dms\n",
				i+1, step.Unit, step.Action, step.Details, step.DurationMs)
			if step.Metadata != nil {
				if output, ok := step.Metadata["output"]; ok {
					fmt.Printf("           输出: %v\n", output)
				}
			}
		}
	}

	// ──────────────────────────────────────────────
	// 总览
	// ──────────────────────────────────────────────
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("  演示完成：Phase 2 P5 Agent Workbench 端到端流程")
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println()
	fmt.Println("  流程总结:")
	fmt.Println("  ┌──────┬──────────────────────────────────────────────────────┐")
	fmt.Println("  │ 步骤 │ 描述                                                 │")
	fmt.Println("  ├──────┼──────────────────────────────────────────────────────┤")
	fmt.Println("  │  1   │ 创建 AgentPool + TaskQueue + SchedulerComposer       │")
	fmt.Println("  │  2   │ 注册 Agent A (compute) 和 Agent B (render)           │")
	fmt.Println("  │  3   │ Agent A 编写合规代码（3+5*2=13）→ SubmitAndRun       │")
	fmt.Println("  │  4   │ 打印完整 ExecutionStep 时间线                        │")
	fmt.Println("  │  5   │ HandoffComposer: Agent A → Agent B（DevSnapshot）    │")
	fmt.Println("  │  6   │ Agent B 接收快照并渲染结果                           │")
	fmt.Println("  │  7a  │ Agent A 提交违规代码 → StaticGuardPort Block         │")
	fmt.Println("  │  7b  │ Agent 修复后重新提交 → 审核通过 → 编译执行           │")
	fmt.Println("  └──────┴──────────────────────────────────────────────────────┘")
	fmt.Println()

	// 观察层统计
	fmt.Printf("  观察层 (InMemoryObservationAdapter) 统计:\n")
	fmt.Printf("    记录步骤总数: %d\n", obs.StepCount())

	tree := obs.GetTraceTree()
	fmt.Printf("    Trace Tree 根节点: %d | 总节点: %d\n", len(tree.Roots), tree.TotalNodes())

	// 输出所有步骤摘要
	fmt.Println()
	fmt.Println("  全部 ExecutionStep 摘要:")
	allSteps := obs.GetSteps()
	for i, step := range allSteps {
		status := "OK"
		if step.Error != nil {
			status = "ERR"
		}
		fmt.Printf("    [%03d] %-15s | %-15s | %-6s | %s\n",
			i+1, step.Unit, step.Action, status, step.Details)
	}

	fmt.Println()
	fmt.Println("  Agent 池状态:")
	fmt.Printf("    Agent A: %s\n", describeAgent(pool, "agent-a"))
	fmt.Printf("    Agent B: %s\n", describeAgent(pool, "agent-b"))

	// 清理：注销 Agent
	core.DeregisterAgent(pool, "agent-a")
	core.DeregisterAgent(pool, "agent-b")
	fmt.Printf("  Agent 已注销，池中剩余: %d\n", pool.Count())

	// 避免 unused import 警告
	_ = time.Now
}

// describeAgent 获取 Agent 信息的可读描述。
func describeAgent(pool *core.AgentPool, agentID string) string {
	info, ok := pool.Get(agentID)
	if !ok {
		return "未注册"
	}
	return fmt.Sprintf("ID=%s | 能力=%v | 状态=%s | 阶段=%s | 最后心跳=%s",
		info.ID, info.Capabilities, info.Status, info.Phase,
		info.LastHeartbeat.Format("15:04:05"))
}