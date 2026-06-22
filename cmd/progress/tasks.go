package main

// defaultTasks 返回架构重构项目的 24 个默认最小任务单元
func defaultTasks() []Task {
	return []Task{
		// ===== Phase 1: 立即可见效 =====
		{ID: "TU-1", Title: "原语识别后端增强", Phase: "Phase 1", Priority: "P0",
			Description: "让 /api/primitives 返回完整正确的原语数据", Status: StatusPending, Progress: 0},
		{ID: "TU-2", Title: "拓扑图后端依赖数据增强", Phase: "Phase 1", Priority: "P0",
			Description: "构建准确的文件依赖图，让拓扑图有真实数据", Status: StatusPending, Progress: 0},
		{ID: "TU-3", Title: "前端启动数据加载编排", Phase: "Phase 1", Priority: "P0",
			Description: "页面打开即有真实数据，不再空屏", Status: StatusPending, Progress: 0},
		{ID: "TU-4", Title: "健康评分后端增强", Phase: "Phase 1", Priority: "P1",
			Description: "健康评分返回详细解释信息和因子明细", Status: StatusPending, Progress: 0},
		{ID: "TU-5", Title: "健康仪表前端增强", Phase: "Phase 1", Priority: "P1",
			Description: "前端显示详细的五维雷达解释和评分原因", Status: StatusPending, Progress: 0},
		{ID: "TU-6", Title: "违规规则集扩展（3→15+）", Phase: "Phase 1", Priority: "P1",
			Description: "建立完整的架构违规检测规则引擎", Status: StatusPending, Progress: 0},
		{ID: "TU-7", Title: "违规看板前端增强", Phase: "Phase 1", Priority: "P1",
			Description: "违规按严重程度分组，显示后果和建议", Status: StatusPending, Progress: 0},

		// ===== Phase 2: 可视化增强 =====
		{ID: "TU-8", Title: "原语分布前端增强", Phase: "Phase 2", Priority: "P2",
			Description: "原语数据可视化 + 清单 + 示例代码弹窗", Status: StatusPending, Progress: 0},
		{ID: "TU-9", Title: "文件变更监测实现", Phase: "Phase 2", Priority: "P2",
			Description: "自动监测源代码变更并写入变更日志", Status: StatusPending, Progress: 0},
		{ID: "TU-10", Title: "架构变更日志可视化", Phase: "Phase 2", Priority: "P2",
			Description: "前端显示架构变更时间线和统计", Status: StatusPending, Progress: 0},

		// ===== Phase 3: 可观测性 =====
		{ID: "TU-11", Title: "数据流拓扑实现", Phase: "Phase 3", Priority: "P2",
			Description: "实现数据流的可视化分析", Status: StatusPending, Progress: 0},
		{ID: "TU-12", Title: "可观测性 - 执行步骤", Phase: "Phase 3", Priority: "P2",
			Description: "提取函数调用链并可视化", Status: StatusPending, Progress: 0},
		{ID: "TU-13", Title: "可观测性 - Pipeline 状态", Phase: "Phase 3", Priority: "P2",
			Description: "列出活跃 Pipeline + 状态可视化", Status: StatusPending, Progress: 0},
		{ID: "TU-14", Title: "可观测性 - 架构快照", Phase: "Phase 3", Priority: "P2",
			Description: "记录和对比不同时间点的架构状态", Status: StatusPending, Progress: 0},
		{ID: "TU-15", Title: "聚合指标趋势", Phase: "Phase 3", Priority: "P2",
			Description: "展示文件数/行数/符号数/违规数等趋势图", Status: StatusPending, Progress: 0},

		// ===== Phase 4: 追踪与时间 =====
		{ID: "TU-16", Title: "开发进度面板", Phase: "Phase 4", Priority: "P3",
			Description: "展示架构重构进度可视化", Status: StatusPending, Progress: 0},
		{ID: "TU-17", Title: "溯源面板 - 因果链", Phase: "Phase 4", Priority: "P3",
			Description: "文件/函数调用因果关系可视化", Status: StatusPending, Progress: 0},
		{ID: "TU-18", Title: "溯源面板 - 时间旅行", Phase: "Phase 4", Priority: "P3",
			Description: "可查看历史架构快照并做对比", Status: StatusPending, Progress: 0},
		{ID: "TU-20", Title: "SSE 事件连接", Phase: "Phase 4", Priority: "P3",
			Description: "前端订阅服务端实时事件", Status: StatusPending, Progress: 0},

		// ===== Phase 5: 迁移引擎 =====
		{ID: "TU-19", Title: "迁移引擎状态面板", Phase: "Phase 5", Priority: "P3",
			Description: "显示迁移引擎运行状态和配置", Status: StatusPending, Progress: 0},

		// ===== Phase 6: CLI 增强 =====
		{ID: "TU-21", Title: "拓扑图布局增强", Phase: "Phase 6", Priority: "P3",
			Description: "同心圆/分区布局突出层级关系", Status: StatusPending, Progress: 0},
		{ID: "TU-22", Title: "CLI serve 增强", Phase: "Phase 6", Priority: "P3",
			Description: "完善 arch serve 命令参数和行为", Status: StatusPending, Progress: 0},
		{ID: "TU-23", Title: "CLI analyze 命令", Phase: "Phase 6", Priority: "P3",
			Description: "命令行输出完整架构分析报告", Status: StatusPending, Progress: 0},
		{ID: "TU-24", Title: "CLI 其他命令", Phase: "Phase 6", Priority: "P3",
			Description: "完善 arch check / primitives / health / dashboard", Status: StatusPending, Progress: 0},
	}
}
