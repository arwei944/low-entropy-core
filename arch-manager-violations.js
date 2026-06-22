/**
 * arch-manager-violations.js
 * 架构全景视图 — 违规看板渲染模块
 *
 * 包含函数:
 *   - getViolationItems()   违规数据归一化（兼容数组/ViolationResponse 两种格式）
 *   - renderViolationsView()  违规看板主渲染
 *
 * 依赖全局变量 (来自 core.js):
 *   violations, charts, echarts, esc
 */

// ============================================================
// 数据归一化：同时支持 ViolationResponse 和原始数组
// ============================================================
function getViolationItems() {
  if (Array.isArray(violations)) {
    const result = {
      items: violations,
      total: violations.length,
      bySeverity: { error: 0, warning: 0, info: 0 },
      byRule: {}
    };
    violations.forEach(v => {
      const sev = (v.severity || 'info').toLowerCase();
      if (result.bySeverity[sev] !== undefined) result.bySeverity[sev]++;
      const rule = v.rule_id || v.rule || 'UNKNOWN';
      result.byRule[rule] = (result.byRule[rule] || 0) + 1;
    });
    return result;
  }
  if (violations && typeof violations === 'object') {
    return {
      items: violations.items || violations.violations || violations.list || [],
      total: violations.total !== undefined ? violations.total : (violations.items || violations.violations || []).length,
      bySeverity: violations.by_severity || violations.bySeverity || {},
      byRule: violations.by_rule || violations.byRule || {}
    };
  }
  return { items: [], total: 0, bySeverity: {}, byRule: {} };
}

// ============================================================
// 辅助：严重程度映射
// ============================================================
const SEVERITY_META = {
  error:   { label: '严重违规', icon: '🛑', color: '#ff453a', bg: 'rgba(255,69,58,0.12)' },
  warning: { label: '警告',     icon: '⚠️', color: '#ff9f0a', bg: 'rgba(255,159,10,0.12)' },
  info:    { label: '提示',     icon: 'ℹ️', color: '#0a84ff', bg: 'rgba(10,132,255,0.12)' }
};

function normalizeSeverity(sev) {
  const s = (sev || 'info').toLowerCase();
  return SEVERITY_META[s] ? s : 'info';
}

// ============================================================
// 辅助：渲染单个违规条目
// ============================================================
function renderViolationItem(v) {
  const sev = normalizeSeverity(v.severity);
  const meta = SEVERITY_META[sev];
  const ruleId = v.rule_id || v.rule || 'RULE';
  const message = v.message || '(无描述)';
  const filePath = v.file || v.file_path || v.path || '';
  const line = v.line || v.line_no || 0;
  const consequence = v.consequence || v.detail || '';
  const suggestion = v.suggestion || v.fix || v.suggest || '';
  const snippet = v.code_snippet || v.snippet || '';
  const codeLang = v.language || (filePath ? filePath.split('.').pop() : '') || '';

  const locText = filePath
    ? filePath + (line > 0 ? ':' + line : '')
    : (line > 0 ? '第 ' + line + ' 行' : '');

  let html = '<div class="violation-card" data-severity="' + sev + '" data-rule="' + esc(ruleId) + '">';
  html += '<span class="sev ' + sev + '"></span>';
  html += '<div class="body">';
  html += '<div class="viol-header">';
  html += '<span class="viol-rule-tag" style="background:' + meta.bg + ';color:' + meta.color + '">' + esc(ruleId) + '</span>';
  html += '<span class="viol-sev-tag" style="background:' + meta.bg + ';color:' + meta.color + '">' + meta.icon + ' ' + meta.label + '</span>';
  html += '</div>';
  html += '<div class="msg">' + esc(message) + '</div>';
  if (locText) {
    html += '<div class="detail mono" title="' + esc(locText) + '">📍 ' + esc(locText) + '</div>';
  }
  if (consequence) {
    html += '<div class="viol-block viol-consequence">';
    html += '<div class="viol-block-title">⚠️ 后果</div>';
    html += '<div class="viol-block-body">' + esc(consequence) + '</div>';
    html += '</div>';
  }
  if (suggestion) {
    html += '<div class="viol-block viol-suggestion">';
    html += '<div class="viol-block-title">💡 修复建议</div>';
    html += '<div class="viol-block-body">' + esc(suggestion) + '</div>';
    html += '</div>';
  }
  if (snippet) {
    html += '<div class="viol-block viol-code">';
    html += '<div class="viol-block-title">📄 代码片段' + (codeLang ? ' (' + esc(codeLang) + ')' : '') + '</div>';
    html += '<pre class="viol-code-snippet"><code>' + esc(snippet) + '</code></pre>';
    html += '</div>';
  }
  html += '</div>';
  html += '</div>';
  return html;
}

// ============================================================
// 辅助：渲染顶部汇总卡片
// ============================================================
function renderSummaryCards(data) {
  const bySev = data.bySeverity || {};
  const errorCount = bySev.error !== undefined ? bySev.error : data.items.filter(v => normalizeSeverity(v.severity) === 'error').length;
  const warningCount = bySev.warning !== undefined ? bySev.warning : data.items.filter(v => normalizeSeverity(v.severity) === 'warning').length;
  const infoCount = bySev.info !== undefined ? bySev.info : data.items.filter(v => normalizeSeverity(v.severity) === 'info').length;

  let html = '<div class="viol-summary">';
  html += '<div class="viol-summary-card" data-sev="error" onclick="filterViolationsBySeverity(\'error\')" style="border-color:' + SEVERITY_META.error.color + '">';
  html += '<div class="viol-summary-icon">' + SEVERITY_META.error.icon + '</div>';
  html += '<div class="viol-summary-label">严重违规</div>';
  html += '<div class="viol-summary-count" style="color:' + SEVERITY_META.error.color + '">' + errorCount + '</div>';
  html += '</div>';
  html += '<div class="viol-summary-card" data-sev="warning" onclick="filterViolationsBySeverity(\'warning\')" style="border-color:' + SEVERITY_META.warning.color + '">';
  html += '<div class="viol-summary-icon">' + SEVERITY_META.warning.icon + '</div>';
  html += '<div class="viol-summary-label">警告</div>';
  html += '<div class="viol-summary-count" style="color:' + SEVERITY_META.warning.color + '">' + warningCount + '</div>';
  html += '</div>';
  html += '<div class="viol-summary-card" data-sev="info" onclick="filterViolationsBySeverity(\'info\')" style="border-color:' + SEVERITY_META.info.color + '">';
  html += '<div class="viol-summary-icon">' + SEVERITY_META.info.icon + '</div>';
  html += '<div class="viol-summary-label">提示</div>';
  html += '<div class="viol-summary-count" style="color:' + SEVERITY_META.info.color + '">' + infoCount + '</div>';
  html += '</div>';
  html += '<div class="viol-summary-card" data-sev="total" style="border-color:#8e8e93">';
  html += '<div class="viol-summary-icon">📊</div>';
  html += '<div class="viol-summary-label">总计</div>';
  html += '<div class="viol-summary-count" style="color:#f5f5f7">' + data.total + '</div>';
  html += '</div>';
  html += '</div>';
  return html;
}

// ============================================================
// 辅助：规则聚合统计（HTML 条形图）
// ============================================================
function renderRuleStats(data) {
  const byRule = data.byRule || {};
  let ruleCounts = {};
  if (Object.keys(byRule).length === 0) {
    data.items.forEach(v => {
      const rule = v.rule_id || v.rule || 'UNKNOWN';
      ruleCounts[rule] = (ruleCounts[rule] || 0) + 1;
    });
  } else {
    ruleCounts = byRule;
  }
  const entries = Object.entries(ruleCounts).sort((a, b) => b[1] - a[1]);
  const maxCount = entries.length ? entries[0][1] : 0;

  let html = '<div class="card" style="margin-top:12px">';
  html += '<div class="card-title">📈 按规则聚合 (Top ' + Math.min(entries.length, 10) + ')</div>';
  if (entries.length === 0) {
    html += '<div class="empty-state-small">暂无规则统计</div>';
  } else {
    html += '<div class="viol-rule-stats">';
    entries.slice(0, 10).forEach(([rule, count]) => {
      const percent = maxCount > 0 ? (count / maxCount) * 100 : 0;
      html += '<div class="viol-rule-row" data-rule="' + esc(rule) + '">';
      html += '<div class="viol-rule-name">' + esc(rule) + '</div>';
      html += '<div class="viol-rule-bar-wrap"><div class="viol-rule-bar" style="width:' + percent + '%"></div></div>';
      html += '<div class="viol-rule-count">' + count + '</div>';
      html += '</div>';
    });
    html += '</div>';
  }
  html += '</div>';
  return html;
}

// ============================================================
// 辅助：过滤器栏
// ============================================================
function renderFilters() {
  let html = '<div class="viol-filters">';
  html += '<div class="viol-filter-group">';
  html += '<button class="viol-filter-btn active" data-sev="all" onclick="filterViolationsBySeverity(\'all\')">全部</button>';
  html += '<button class="viol-filter-btn" data-sev="error" onclick="filterViolationsBySeverity(\'error\')" style="color:' + SEVERITY_META.error.color + '">🛑 严重</button>';
  html += '<button class="viol-filter-btn" data-sev="warning" onclick="filterViolationsBySeverity(\'warning\')" style="color:' + SEVERITY_META.warning.color + '">⚠️ 警告</button>';
  html += '<button class="viol-filter-btn" data-sev="info" onclick="filterViolationsBySeverity(\'info\')" style="color:' + SEVERITY_META.info.color + '">ℹ️ 提示</button>';
  html += '</div>';
  html += '<div class="viol-filter-group">';
  html += '<input type="text" id="violRuleFilter" class="viol-filter-input" placeholder="按规则名搜索..." oninput="filterViolationsByRule(this.value)" />';
  html += '</div>';
  html += '</div>';
  return html;
}

// ============================================================
// 过滤器状态（全局，供事件处理函数使用）
// ============================================================
let _violFilterState = { severity: 'all', ruleKeyword: '' };

function filterViolationsBySeverity(sev) {
  _violFilterState.severity = sev;
  document.querySelectorAll('.viol-filter-btn').forEach(btn => {
    btn.classList.toggle('active', btn.dataset.sev === sev);
  });
  _applyViolationFilters();
}

function filterViolationsByRule(keyword) {
  _violFilterState.ruleKeyword = keyword || '';
  _applyViolationFilters();
}

function _applyViolationFilters() {
  const container = document.getElementById('violationsList');
  if (!container) return;
  const cards = container.querySelectorAll('.violation-card');
  let visibleCount = 0;
  cards.forEach(card => {
    const sev = card.dataset.severity;
    const rule = (card.dataset.rule || '').toLowerCase();
    const matchSev = _violFilterState.severity === 'all' || sev === _violFilterState.severity;
    const matchRule = !_violFilterState.ruleKeyword || rule.indexOf(_violFilterState.ruleKeyword.toLowerCase()) >= 0;
    const show = matchSev && matchRule;
    card.style.display = show ? '' : 'none';
    if (show) visibleCount++;
  });

  ['error', 'warning', 'info'].forEach(sev => {
    const group = document.getElementById('violGroup_' + sev);
    if (!group) return;
    const itemsInGroup = group.querySelectorAll('.violation-card');
    const visibleInGroup = Array.from(itemsInGroup).filter(c => c.style.display !== 'none').length;
    if (visibleInGroup === 0) {
      group.style.display = 'none';
    } else {
      group.style.display = '';
    }
  });

  const empty = document.getElementById('violFilterEmpty');
  if (empty) {
    empty.style.display = visibleCount === 0 ? '' : 'none';
  }
}

// ============================================================
// 主渲染函数：违规看板
// ============================================================
function renderViolationsView(container) {
  const data = getViolationItems();

  container.innerHTML = '';
  const wrap = document.createElement('div');
  wrap.className = 'viol-wrap';
  container.appendChild(wrap);

  let html = '<div class="view-title">违规看板</div>';
  html += '<div class="view-desc">按严重程度分组的架构违规，含代码上下文与修复建议</div>';

  if (!violations) {
    html += renderEmptyState('loading');
    wrap.innerHTML = html;
    return;
  }

  if (data.total === 0 && data.items.length === 0) {
    html += renderEmptyState('empty');
    wrap.innerHTML = html;
    return;
  }

  html += renderSummaryCards(data);
  html += renderFilters();
  html += renderRuleStats(data);

  ['error', 'warning', 'info'].forEach(sev => {
    const items = data.items.filter(v => normalizeSeverity(v.severity) === sev);
    if (items.length === 0) return;
    const meta = SEVERITY_META[sev];
    html += '<div class="card" id="violGroup_' + sev + '" data-sev="' + sev + '" style="margin-top:12px">';
    html += '<div class="card-title" style="color:' + meta.color + '">' + meta.icon + ' ' + meta.label + ' (' + items.length + ')</div>';
    html += '<div class="viol-group-list">';
    items.forEach(v => {
      html += renderViolationItem(v);
    });
    html += '</div>';
    html += '</div>';
  });

  html += '<div class="empty-state" id="violFilterEmpty" style="display:none;margin-top:20px">';
  html += '<p>没有符合当前筛选条件的违规</p>';
  html += '</div>';

  wrap.innerHTML = html;
}

// ============================================================
// 辅助：空状态
// ============================================================
function renderEmptyState(mode) {
  if (mode === 'loading') {
    return '<div class="card" style="text-align:center;padding:60px;color:var(--dim)"><svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="#0a84ff" stroke-width="1.5"><path d="M21 12a9 9 0 1 1-6.22-8.56" stroke-linecap="round"/><circle cx="12" cy="12" r="3"/></svg><p style="margin-top:12px;font-size:13px">违规数据加载中...</p></div>';
  }
  return '<div class="card" style="text-align:center;padding:60px;color:var(--dim)"><svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="#30d158" stroke-width="1.5"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg><p style="margin-top:12px;font-size:13px">暂无违规，架构状态良好</p></div>';
}
