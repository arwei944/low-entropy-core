// 架构管理器仪表盘 - 主入口
// 负责：数据加载、面板切换、核心图表渲染

let archData = null;
let healthScore = null;
let primitivesData = null;
let violationsData = null;
let flowData = null;
let originData = null;
let pipelineData = null;
let guardianData = null;
let migrationData = null;
let entropyData = null;
let observeData = null;

// 层级配置
const LAYERS = {
    L0: { name: "错误处理", color: "#f44336" },
    L1: { name: "四原语", color: "#ff9800" },
    L2: { name: "单机韧性", color: "#ffc107" },
    L3: { name: "分布式韧性", color: "#4caf50" },
    L4: { name: "Guardian", color: "#00bcd4" },
    L5: { name: "观测", color: "#2196f3" },
    L6: { name: "事件溯源", color: "#9c27b0" },
    L7: { name: "应用层", color: "#607d8b" }
};

// 原语图标映射
const PRIMITIVE_ICONS = {
    "Atom": "⚛",
    "Port": "🔌",
    "Adapter": "🔄",
    "Composer": "🎼",
    "": "📦"
};

// ========== 工具函数 ==========
function escapeHtml(str) {
    if (str == null) return "";
    return String(str)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#39;");
}

function formatNumber(n) {
    if (n == null) return "0";
    return n.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

function api(path) {
    return fetch(path, { cache: "no-store" })
        .then(r => {
            if (!r.ok) throw new Error("HTTP " + r.status);
            return r.json();
        });
}

function $(sel) { return document.querySelector(sel); }
function $$(sel) { return document.querySelectorAll(sel); }

function showLoading(msg) {
    const el = document.getElementById("loading");
    if (!el) return;
    el.querySelector("p").textContent = msg || "加载中...";
    el.style.display = "flex";
}

function hideLoading() {
    const el = document.getElementById("loading");
    if (el) el.style.display = "none";
}

function renderStatus(key, content) {
    const el = document.getElementById(key);
    if (!el) return;
    el.innerHTML = content;
}

// ========== 数据加载 ==========
async function loadAllDashboardData() {
    showLoading("正在分析架构...");

    const results = await Promise.allSettled([
        api("/api/arch"),
        api("/api/health-score"),
        api("/api/violations"),
        api("/api/primitives"),
        api("/api/flow"),
        api("/api/origin?limit=150"),
        api("/api/observation/pipeline"),
        api("/api/entropy"),
        api("/api/observe"),
        fetch("/api/migrate/status").then(r => r.ok ? r.json() : null),
        fetch("/api/guardian/drift").then(r => r.ok ? r.json() : null),
    ]);

    archData = results[0].status === "fulfilled" ? results[0].value : null;
    healthScore = results[1].status === "fulfilled" ? results[1].value : null;
    violationsData = results[2].status === "fulfilled" ? results[2].value : null;
    primitivesData = results[3].status === "fulfilled" ? results[3].value : null;
    flowData = results[4].status === "fulfilled" ? results[4].value : null;
    originData = results[5].status === "fulfilled" ? results[5].value : null;
    pipelineData = results[6].status === "fulfilled" ? results[6].value : null;
    entropyData = results[7].status === "fulfilled" ? results[7].value : null;
    observeData = results[8].status === "fulfilled" ? results[8].value : null;
    migrationData = results[9].status === "fulfilled" ? results[9].value : null;
    guardianData = results[10].status === "fulfilled" ? results[10].value : null;

    hideLoading();
    renderDashboard();
}

// ========== 渲染主仪表盘 ==========
function renderDashboard() {
    renderTopBar();
    renderOverview();
    renderPrimitivesPanel();
    renderHealthPanel();
    renderViolationsPanel();
    renderOriginPanel();
    renderFlowPanel();
    renderPipelinePanel();
    renderGuardianPanel();
    renderMigrationPanel();
    renderEntropyPanel();
}

// ========== 顶部栏 ==========
function renderTopBar() {
    if (!archData) return;
    const files = archData.total_files || archData.files?.length || 0;
    const lines = archData.total_lines || 0;
    const syms = archData.total_symbols || 0;
    const prim = primitivesData?.total || 0;
    const viol = violationsData?.total || 0;
    const score = healthScore?.overall || 0;
    const grade = healthScore?.grade || "?";

    const summary = document.getElementById("summary-stats");
    if (!summary) return;
    summary.innerHTML = `
        <div class="stat-card">
            <div class="stat-label">文件</div>
            <div class="stat-value">${formatNumber(files)}</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">代码行</div>
            <div class="stat-value">${formatNumber(lines)}</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">符号</div>
            <div class="stat-value">${formatNumber(syms)}</div>
        </div>
        <div class="stat-card prim">
            <div class="stat-label">原语</div>
            <div class="stat-value">${formatNumber(prim)}</div>
        </div>
        <div class="stat-card ${viol > 0 ? "viol" : ""}">
            <div class="stat-label">违规</div>
            <div class="stat-value">${formatNumber(viol)}</div>
        </div>
        <div class="stat-card grade grade-${String(grade).toLowerCase()}">
            <div class="stat-label">评分</div>
            <div class="stat-value">${score} <span class="grade-badge">${grade}</span></div>
        </div>
    `;
}

// ========== 架构总览（8层全景） ==========
function renderOverview() {
    if (!archData) return;
    const layerStats = {};
    for (const l of Object.keys(LAYERS)) layerStats[l] = { files: 0, lines: 0, syms: 0, prims: 0 };

    if (archData.layers && archData.layers.length > 0) {
        for (const l of archData.layers) {
            if (layerStats[l.layer]) {
                layerStats[l.layer].files = l.files || 0;
                layerStats[l.layer].lines = l.lines || 0;
                layerStats[l.layer].syms = l.symbols || 0;
            }
        }
    } else {
        for (const f of archData.files || []) {
            if (layerStats[f.layer]) {
                layerStats[f.layer].files++;
                layerStats[f.layer].lines += f.lines || 0;
                layerStats[f.layer].syms += (f.symbols ? f.symbols.length : 0);
            }
        }
    }

    // 原语按层统计
    if (primitivesData && primitivesData.items) {
        for (const p of primitivesData.items) {
            if (layerStats[p.layer]) layerStats[p.layer].prims++;
        }
    }

    const maxLines = Math.max(...Object.values(layerStats).map(l => l.lines), 1);

    const html = Object.entries(layerStats).map(([k, v]) => {
        const layer = LAYERS[k] || { name: k, color: "#888" };
        const pct = (v.lines / maxLines * 100).toFixed(1);
        return `
        <div class="layer-row" data-layer="${k}">
            <div class="layer-label">
                <span class="layer-code" style="background:${layer.color}">${k}</span>
                <span class="layer-name">${layer.name}</span>
            </div>
            <div class="layer-bar">
                <div class="bar" style="width:${pct}%;background:${layer.color}">
                    <span class="bar-label">${formatNumber(v.files)}文件 / ${formatNumber(v.lines)}行 / ${formatNumber(v.syms)}符号</span>
                </div>
            </div>
            <div class="layer-meta">
                <span class="prim-count">⚛ ${v.prims}</span>
            </div>
        </div>`;
    }).join("");

    const container = document.getElementById("overview");
    if (container) container.innerHTML = `
        <h2 class="panel-title">🏛 架构全景 · 8层结构</h2>
        <div class="layers-container">${html}</div>
    `;
}

// ========== 原语面板 ==========
function renderPrimitivesPanel() {
    if (!primitivesData) return;
    const items = primitivesData.items || [];
    const byType = primitivesData.by_type || {};
    const byLayer = primitivesData.by_layer || {};

    // 按类型分组
    const groups = {};
    for (const item of items) {
        const type = item.type || "Unknown";
        if (!groups[type]) groups[type] = [];
        groups[type].push(item);
    }

    let html = `
        <h2 class="panel-title">⚛ 四原语检测 · ${primitivesData.total} 个</h2>
        <div class="prim-summary">
            ${Object.entries(byType).map(([k, v]) => `
                <div class="prim-chip">
                    <span class="prim-icon">${PRIMITIVE_ICONS[k] || "📦"}</span>
                    <span class="prim-name">${k}</span>
                    <span class="prim-count">${v}</span>
                </div>
            `).join("")}
        </div>
    `;

    // 原语列表
    for (const type of ["Atom", "Port", "Adapter", "Composer"]) {
        if (!groups[type] || groups[type].length === 0) continue;
        const list = groups[type].slice(0, 30);
        html += `
            <div class="prim-group">
                <h3>${PRIMITIVE_ICONS[type] || "📦"} ${type} <span class="muted">(${groups[type].length})</span></h3>
                <div class="prim-list">
                    ${list.map(p => `
                        <div class="prim-item" data-layer="${p.layer || "L7"}">
                            <div class="prim-name">${escapeHtml(p.name)}</div>
                            <div class="prim-file"><span class="muted">在</span> ${escapeHtml(p.file || "")}</div>
                            ${p.layer ? `<span class="layer-badge" style="background:${(LAYERS[p.layer] || {}).color || "#888"}">${p.layer}</span>` : ""}
                        </div>
                    `).join("")}
                </div>
            </div>
        `;
    }

    // 按层分布
    const layerHtml = Object.entries(byLayer).map(([k, v]) => {
        const layer = LAYERS[k] || { name: k, color: "#888" };
        return `
            <div class="layer-chip">
                <span class="layer-code" style="background:${layer.color}">${k}</span>
                <span>${v} 个</span>
            </div>
        `;
    }).join("");
    html += `<div class="prim-layer-group"><h3>按层分布</h3><div class="layer-chips">${layerHtml}</div></div>`;

    const container = document.getElementById("primitives");
    if (container) container.innerHTML = html;
}

// ========== 健康评分面板 ==========
function renderHealthPanel() {
    if (!healthScore) return;
    const factors = healthScore.factor_details || [];
    const stats = healthScore.project_stats || {};
    const grade = healthScore.grade || "?";
    const desc = healthScore.grade_description || "";
    const overall = healthScore.overall || 0;

    let html = `
        <h2 class="panel-title">📊 健康评分 · ${overall} (${grade})</h2>
        <div class="health-header grade-${String(grade).toLowerCase()}">
            <div class="health-score-main">
                <div class="score-circle">
                    <div class="score-value">${overall}</div>
                    <div class="score-grade">${grade}</div>
                </div>
                <div class="score-desc">${escapeHtml(desc)}</div>
            </div>
            <div class="health-stats">
                ${Object.entries(stats).map(([k, v]) => `
                    <div class="stat-item">
                        <span class="stat-name">${escapeHtml(String(k))}</span>
                        <span class="stat-val">${formatNumber(v)}</span>
                    </div>
                `).join("")}
            </div>
        </div>
        <div class="health-factors">
            ${factors.map(f => {
                const isLow = f.score < 70;
                return `
                <div class="factor-item ${isLow ? "factor-warning" : ""}">
                    <div class="factor-header">
                        <span class="factor-name">${escapeHtml(f.name || f.key || "")}</span>
                        <span class="factor-score">${(f.score || 0).toFixed(1)}</span>
                    </div>
                    <div class="factor-bar">
                        <div class="bar-inner" style="width:${f.score || 0}%"></div>
                    </div>
                    <div class="factor-info">
                        ${f.explanation ? `<div><strong>说明:</strong> ${escapeHtml(f.explanation)}</div>` : ""}
                        ${f.raw_value ? `<div><strong>值:</strong> ${escapeHtml(String(f.raw_value))}</div>` : ""}
                        ${f.suggestion ? `<div><strong>建议:</strong> ${escapeHtml(String(f.suggestion))}</div>` : ""}
                    </div>
                </div>`;
            }).join("")}
        </div>
    `;

    const container = document.getElementById("health");
    if (container) container.innerHTML = html;
}

// ========== 违规面板 ==========
function renderViolationsPanel() {
    if (!violationsData) return;
    const items = violationsData.items || [];
    const total = violationsData.total || items.length;
    const errors = items.filter(v => v.severity === "error").length;
    const warnings = items.filter(v => v.severity === "warning").length;
    const infos = items.filter(v => v.severity === "info" || !v.severity).length;

    // 按规则分组
    const byRule = violationsData.by_rule || {};
    const ruleItems = [];
    for (const [ruleId, count] of Object.entries(byRule)) {
        ruleItems.push({ id: ruleId, count });
    }
    ruleItems.sort((a, b) => b.count - a.count);

    let html = `
        <h2 class="panel-title">⚠ 架构违规 · ${total} 项</h2>
        <div class="viol-summary">
            <div class="viol-severity severe">
                <div class="sev-icon">🛑</div>
                <div class="sev-count">${errors}</div>
                <div class="sev-label">错误</div>
            </div>
            <div class="viol-severity warning">
                <div class="sev-icon">⚠️</div>
                <div class="sev-count">${warnings}</div>
                <div class="sev-label">警告</div>
            </div>
            <div class="viol-severity info">
                <div class="sev-icon">ℹ️</div>
                <div class="sev-count">${infos}</div>
                <div class="sev-label">提示</div>
            </div>
        </div>

        <div class="viol-rule-stats">
            <h3>按规则统计（前10项）</h3>
            ${ruleItems.slice(0, 10).map(r => `
                <div class="rule-row">
                    <span class="rule-id">${escapeHtml(r.id)}</span>
                    <span class="rule-count">${r.count}</span>
                </div>
            `).join("")}
        </div>

        <div class="viol-list">
            <h3>错误和警告（前 50 条）</h3>
            ${items.filter(v => v.severity === "error" || v.severity === "warning").slice(0, 50).map(v => `
                <div class="viol-item viol-${v.severity || "info"}">
                    <div class="viol-head">
                        <span class="viol-sev-badge ${v.severity}">${v.severity === "error" ? "ERROR" : v.severity === "warning" ? "WARN" : "INFO"}</span>
                        <span class="viol-rule">${escapeHtml(v.rule_id || "")}</span>
                        <span class="viol-file">${escapeHtml(v.file || "")}</span>
                    </div>
                    <div class="viol-msg">${escapeHtml(v.message || "")}</div>
                    ${v.detail ? `<div class="viol-detail"><strong>详情:</strong> ${escapeHtml(v.detail)}</div>` : ""}
                    ${v.suggestion ? `<div class="viol-suggestion"><strong>建议:</strong> ${escapeHtml(v.suggestion)}</div>` : ""}
                </div>
            `).join("")}
        </div>
    `;

    const container = document.getElementById("violations");
    if (container) container.innerHTML = html;
}

// ========== 溯源面板 ==========
function renderOriginPanel() {
    if (!originData) return;
    const items = originData.symbols || [];
    const byLayer = originData.by_layer || {};
    const byKind = originData.by_kind || {};

    let html = `
        <h2 class="panel-title">🔍 符号溯源 · ${originData.total || items.length} 个</h2>
        <div class="origin-summary">
            <div class="origin-chips">
                <strong>按层:</strong>
                ${Object.entries(byLayer).map(([k, v]) => {
                    const layer = LAYERS[k] || { name: k, color: "#888" };
                    return `<span class="layer-chip"><span class="layer-code" style="background:${layer.color}">${k}</span>${v}</span>`;
                }).join("")}
            </div>
            <div class="origin-chips">
                <strong>按类型:</strong>
                ${Object.entries(byKind).slice(0, 8).map(([k, v]) => `
                    <span class="kind-chip">${escapeHtml(k)} (${v})</span>
                `).join("")}
            </div>
        </div>
        <div class="origin-list">
            ${items.slice(0, 100).map(s => `
                <div class="origin-item" data-layer="${s.layer || ""}">
                    <div class="origin-head">
                        <span class="origin-name ${s.is_exported ? "exported" : ""}">${escapeHtml(s.name)}</span>
                        ${s.primitive ? `<span class="prim-badge ${s.primitive}">${PRIMITIVE_ICONS[s.primitive] || "📦"} ${s.primitive}</span>` : ""}
                        ${s.layer ? `<span class="layer-badge" style="background:${(LAYERS[s.layer] || {}).color || "#888"}">${s.layer}</span>` : ""}
                    </div>
                    <div class="origin-info">
                        <span class="muted">类型:</span> ${escapeHtml(s.kind || "")}
                        <span class="muted">文件:</span> ${escapeHtml(s.file || "")}
                    </div>
                    ${s.doc ? `<div class="origin-doc">${escapeHtml(s.doc)}</div>` : ""}
                </div>
            `).join("")}
        </div>
    `;

    const container = document.getElementById("origin");
    if (container) container.innerHTML = html;
}

// ========== 数据流拓扑 ==========
function renderFlowPanel() {
    if (!flowData) return;
    const nodes = flowData.nodes || [];
    const edges = flowData.edges || [];
    const layerFlow = flowData.layer_flow || [];

    let html = `
        <h2 class="panel-title">🌊 数据流拓扑 · ${nodes.length} 节点 / ${edges.length} 边</h2>
        <div class="flow-layer-stats">
            <h3>层间流量</h3>
            ${layerFlow.slice(0, 15).map(l => `
                <div class="flow-layer-item">
                    <span class="layer-badge" style="background:${(LAYERS[l.from_layer] || {}).color || "#888"}">${l.from_layer}</span>
                    <span class="flow-arrow">→</span>
                    <span class="layer-badge" style="background:${(LAYERS[l.to_layer] || {}).color || "#888"}">${l.to_layer}</span>
                    <span class="flow-count">${l.count} 条</span>
                </div>
            `).join("")}
        </div>
        <div class="flow-nodes-preview">
            <h3>节点预览（前30）</h3>
            <div class="node-grid">
                ${nodes.slice(0, 30).map(n => `
                    <div class="node-card" data-layer="${n.layer || ""}">
                        <span class="layer-badge" style="background:${(LAYERS[n.layer] || {}).color || "#888"};font-size:10px">${n.layer || ""}</span>
                        <div class="node-name">${escapeHtml(n.name || "")}</div>
                        <div class="node-meta muted">${n.line_count}行 ${n.symbol_count}符号</div>
                    </div>
                `).join("")}
            </div>
        </div>
        <div class="flow-paths">
            <h3>关键数据流路径</h3>
            ${(flowData.top_paths || []).slice(0, 10).map(p => `
                <div class="flow-path-item">
                    <div class="flow-path-name">路径权重: ${p.weight || 0}</div>
                    <div class="flow-path-chain">${(p.path || []).map(step => `<span class="path-step">${escapeHtml(step)}</span>`).join(" → ")}</div>
                </div>
            `).join("")}
        </div>
    `;

    const container = document.getElementById("flow");
    if (container) container.innerHTML = html;
}

// ========== 观测管道 ==========
function renderPipelinePanel() {
    if (!pipelineData) return;
    const snapshot = pipelineData.snapshot || {};
    const steps = snapshot.steps || [];
    const stats = pipelineData.aggregate_stats || {};
    const stepSummary = pipelineData.step_summary || {};

    let html = `
        <h2 class="panel-title">🔬 观测管道 · 执行追踪</h2>
        <div class="pipeline-stats">
            <div class="pipeline-stat done"><strong>${stepSummary.completed || 0}</strong><span>已完成</span></div>
            <div class="pipeline-stat doing"><strong>${stepSummary.in_progress || 0}</strong><span>进行中</span></div>
            <div class="pipeline-stat pend"><strong>${stepSummary.pending || 0}</strong><span>待执行</span></div>
            <div class="pipeline-stat fail"><strong>${stepSummary.failed || 0}</strong><span>失败</span></div>
        </div>
        <div class="pipeline-steps">
            ${steps.map(s => `
                <div class="pipeline-step step-${s.status || "pending"}">
                    <div class="step-status-dot"></div>
                    <div class="step-content">
                        <div class="step-header">
                            <span class="step-name">${escapeHtml(s.name || "")}</span>
                            <span class="step-layer" style="background:${(LAYERS[s.layer] || {}).color || "#888"}">${s.layer || ""}</span>
                            <span class="step-dur">${s.duration_ms || 0}ms</span>
                        </div>
                        <div class="step-source muted">${escapeHtml(s.source || "")}</div>
                    </div>
                </div>
            `).join("")}
        </div>
        <div class="pipeline-agg">
            <h3>聚合统计</h3>
            ${Object.entries(stats).map(([k, v]) => `
                <div class="agg-item"><span class="agg-key">${escapeHtml(k)}</span><span class="agg-val">${formatNumber(v)}</span></div>
            `).join("")}
        </div>
    `;

    const container = document.getElementById("pipeline");
    if (container) container.innerHTML = html;
}

// ========== Guardian ==========
function renderGuardianPanel() {
    let html = `<h2 class="panel-title">🛡 Guardian 监督 · 熵监控</h2>`;
    if (entropyData && entropyData.metrics) {
        html += `<div class="guardian-entries">`;
        for (const m of entropyData.metrics) {
            const layer = LAYERS[m.module || m.layer] || { name: m.module || m.layer || "?", color: "#888" };
            html += `
                <div class="guardian-item risk-${m.risk_level || "low"}">
                    <div class="guardian-head">
                        <span class="layer-badge" style="background:${layer.color}">${m.module || m.layer || ""}</span>
                        <span class="guardian-risk">风险: ${escapeHtml(m.risk_level || "unknown")}</span>
                    </div>
                    <div class="guardian-stats">
                        <div><span class="muted">文件:</span> ${m.file_count || 0}</div>
                        <div><span class="muted">行数:</span> ${m.line_count || 0}</div>
                        <div><span class="muted">复杂度:</span> ${(m.cyclomatic || 0).toFixed(2)}</div>
                        <div><span class="muted">熵:</span> ${(m.drift_score || 0).toFixed(1)}</div>
                    </div>
                </div>`;
        }
        html += `</div>`;
    } else {
        html += `<div class="empty-state">⚠ Guardian 需要 tier4 构建才能提供完整漂移检测。当前显示基础熵数据。</div>`;
    }

    if (observeData && observeData.metrics) {
        html += `<div class="observe-metrics">
            <h3>编译与测试指标</h3>
            ${Object.entries(observeData.metrics).map(([k, v]) => `
                <div class="metric-item">
                    <span class="metric-key">${escapeHtml(k)}</span>
                    <span class="metric-val">${typeof v === "number" ? v.toFixed(2) : escapeHtml(String(v))}</span>
                </div>
            `).join("")}
        </div>`;
    }

    const container = document.getElementById("guardian");
    if (container) container.innerHTML = html;
}

// ========== 迁移引擎 ==========
function renderMigrationPanel() {
    let html = `<h2 class="panel-title">🚀 迁移引擎</h2>`;
    if (migrationData) {
        html += `
            <div class="migration-status">
                <div class="status-item"><strong>版本:</strong> ${escapeHtml(migrationData.version || "unknown")}</div>
                <div class="status-item"><strong>状态:</strong> ${escapeHtml(migrationData.status || "unknown")}</div>
                <div class="status-item"><strong>活跃会话:</strong> ${migrationData.active_sessions || 0}</div>
                <div class="status-item"><strong>总会话:</strong> ${migrationData.total_sessions || 0}</div>
            </div>
            <div class="migration-hint muted">
                POST /api/migrate/analyze 启动迁移分析。
            </div>
        `;
    } else {
        html += `<div class="empty-state">迁移状态不可用。需要 tier4 构建或手动启动。</div>`;
    }
    const container = document.getElementById("migration");
    if (container) container.innerHTML = html;
}

// ========== 熵/观测 ==========
function renderEntropyPanel() {
    let html = `<h2 class="panel-title">📈 架构熵与观测</h2>`;

    if (entropyData && entropyData.metrics) {
        const maxDrift = Math.max(...entropyData.metrics.map(m => m.drift_score || 0), 1);
        html += `<div class="entropy-chart">
            ${entropyData.metrics.map(m => {
                const layer = LAYERS[m.module || m.layer] || { name: m.module || m.layer || "?", color: "#888" };
                const pct = ((m.drift_score || 0) / maxDrift * 100).toFixed(1);
                return `
                <div class="entropy-row">
                    <span class="layer-badge" style="background:${layer.color}">${m.module || m.layer || ""}</span>
                    <div class="entropy-bar">
                        <div class="bar-inner" style="width:${pct}%;background:${layer.color}"></div>
                    </div>
                    <span class="entropy-val">${(m.drift_score || 0).toFixed(1)}</span>
                </div>`;
            }).join("")}
        </div>`;
    } else {
        html += `<div class="empty-state">熵数据不可用</div>`;
    }

    if (observeData && observeData.metrics) {
        html += `<div class="observe-grid">
            ${Object.entries(observeData.metrics).map(([k, v]) => `
                <div class="observe-card">
                    <div class="observe-key">${escapeHtml(k)}</div>
                    <div class="observe-val">${typeof v === "number" ? v.toFixed(2) : escapeHtml(String(v))}</div>
                </div>
            `).join("")}
        </div>`;
    }

    const container = document.getElementById("entropy");
    if (container) container.innerHTML = html;
}

// ========== 面板切换 ==========
function initTabs() {
    const tabs = document.querySelectorAll(".nav-tab");
    tabs.forEach(tab => {
        tab.addEventListener("click", function () {
            const target = this.getAttribute("data-target");
            tabs.forEach(t => t.classList.remove("active"));
            this.classList.add("active");
            document.querySelectorAll(".panel").forEach(p => p.classList.remove("active"));
            const panel = document.getElementById(target);
            if (panel) panel.classList.add("active");
        });
    });
}

// ========== 初始化 ==========
document.addEventListener("DOMContentLoaded", function () {
    initTabs();
    loadAllDashboardData();
    // 定时刷新
    setInterval(loadAllDashboardData, 60000);
});
