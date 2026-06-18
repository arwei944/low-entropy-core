# 文档处理 Agent 示例

展示 Agent Workbench 完整生命周期的端到端演示。

## 流程

```
用户请求 → Port 验证 → Atom 提取文本 → Atom 分析关键词 → Atom 生成摘要 → Adapter 存储文件
                        ↓
              Agent 提交代码 → StaticGuard 审核 → DecisionEngine 决策 → 返回结果
```

## 使用的 4 原语

| 原语 | 实现 | 说明 |
|------|------|------|
| **Atom** | `extractText`, `analyzeContent`, `generateSummary` | 纯函数：文本提取、关键词分析、摘要生成 |
| **Port** | `docPort` | 输入验证契约：检查内容非空 |
| **Adapter** | `storeAdapter` | 文件系统存储：将结果写入 TXT |
| **Composer** | `Pipeline` | 编排：串联 Port → Atom×3 → Adapter |

## 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/process?name=doc&content=...` | GET | 处理文档（完整 Pipeline） |
| `/api/agent/submit` | POST | Agent 提交代码（经过 Guardian 审核） |
| `/api/observation/steps` | GET | 查看执行步骤追踪 |
| `/health` | GET | 健康检查 |

## 运行

```bash
cd examples/doc-processor
go build -o doc-processor.exe .
./doc-processor.exe
```

## 测试

```bash
# 健康检查
curl http://localhost:8084/health

# 处理文档
curl "http://localhost:8084/api/process?name=test&content=The+quick+brown+fox+jumps+over+the+lazy+dog"

# Agent 提交代码（会被 Guardian 审核）
curl -X POST http://localhost:8084/api/agent/submit

# 查看执行追踪
curl http://localhost:8084/api/observation/steps
```

## Agent Workbench 生命周期

1. **Agent 提交代码** → `POST /api/agent/submit`
2. **StaticGuard 审核** → 检查 Manifest 声明与实际代码是否一致
3. **DecisionEngine 决策** → Allow/Block/Warn
4. **返回结果** → `SubmissionResult`（含 Violation 列表和修复建议）
5. **Agent 迭代** → 根据 Violation 修复代码 → 重新提交