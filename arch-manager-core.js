// ============================================================
// State
// ============================================================
let currentView = 'topology';
let archData = null, healthScore = null, violations = [];
let primitives = [], runtime = { tps: 0, errors: 0, latency: 0 };
let guardian = { snapshot: null, thresholds: [], drift: 0, history: [] };
let traceTree = null, versionDiff = null, changelog = [];
let healthHistory = [];
let obsSteps = [], obsAggregates = [], obsPipelines = [], obsArch = null, obsErrors = [];
let flowData = null, originData = null, obsPipelineData = null;
let obsTraceDetail = null;
let obsStepsQuery = { pattern: '', unit: '', error_only: false, limit: 50 };
let obsAggQuery = { window: '', unit: '', pattern: '', limit: 20 };
let sseArch = null, sseGuardian = null, sseDev = null;
let devEvents = [];
let migStatus = null, migPatternMap = null, migGateChain = null, migLogs = [], migSessions = [];
let sseMigrate = null;
let archChangelog = [], archChangelogStats = null;
let sseChangelog = null;
let charts = {};
let tpsHistory = [], errorHistory = [], latencyHistory = [];
const MAX_POINTS = 60;
let collapsed = { panorama: false, runtime: false, control: false, traceability: false, observability: false, migration: false };

// ============================================================
// API
// ============================================================
const API = '';
async function api(url) {
  const r = await fetch(API + url);
  if (!r.ok) throw new Error('HTTP ' + r.status);
  return r.json();
}

// Observation lazy-load query helpers
async function queryObsSteps() {
  const q = obsStepsQuery;
  let url = '/api/observation/steps/query?limit=' + q.limit;
  if (q.pattern) url += '&pattern=' + encodeURIComponent(q.pattern);
  if (q.unit) url += '&unit=' + encodeURIComponent(q.unit);
  if (q.error_only) url += '&error_only=true';
  try { obsSteps = await api(url); toast('查询完成: ' + obsSteps.length + ' 条', 'ok'); } catch(e) { toast('查询失败: ' + e.message, 'err'); }
}

async function fetchObsErrors() {
  try { obsErrors = await api('/api/observation/steps/errors'); toast('错误步骤: ' + obsErrors.length + ' 条', 'ok'); } catch(e) { toast('查询失败: ' + e.message, 'err'); }
}

async function fetchObsTrace(traceId) {
  try { obsTraceDetail = await api('/api/observation/trace/' + encodeURIComponent(traceId)); toast('Trace 已加载', 'ok'); } catch(e) { toast('Trace 加载失败: ' + e.message, 'err'); }
}

async function queryObsAggregates() {
  const q = obsAggQuery;
  let url = '/api/observation/aggregates/query?limit=' + q.limit;
  if (q.window) url += '&window=' + encodeURIComponent(q.window);
  if (q.unit) url += '&unit=' + encodeURIComponent(q.unit);
  if (q.pattern) url += '&pattern=' + encodeURIComponent(q.pattern);
  try { obsAggregates = await api(url); toast('聚合查询完成: ' + obsAggregates.length + ' 条', 'ok'); } catch(e) { toast('查询失败: ' + e.message, 'err'); }
}

// ============================================================
// Top Bar
// ============================================================
function updateTopBar() {
  const h = document.getElementById('topHealth');
  const e = document.getElementById('topEntropy');
  const t = document.getElementById('topTPS');
  if (healthScore) {
    const s = parseFloat(healthScore.overall) || 0;
    h.textContent = s + ' (' + (healthScore.grade || '--') + ')';
    h.className = 'value ' + (s >= 80 ? 'good' : s >= 60 ? 'warn' : 'bad');
  }
  if (guardian && guardian.drift != null) {
    const d = parseFloat(guardian.drift);
    if (!isNaN(d)) {
      e.textContent = d.toFixed(3);
      e.className = 'value ' + (d < 0.1 ? 'good' : d < 0.3 ? 'warn' : 'bad');
    }
  }
  if (runtime && runtime.tps != null) {
    const tps = parseFloat(runtime.tps);
    if (!isNaN(tps)) t.textContent = tps.toFixed(0);
  }
}

// ============================================================
// Overview Dashboard
// ============================================================
function updateOverviewDash() {
  // Health Score
  const ovHealth = document.querySelector('#ovHealth .ov-value');
  const ovHealthEl = ovHealth;
  if (healthScore && healthScore.overall != null) {
    const s = parseFloat(healthScore.overall) || 0;
    ovHealthEl.textContent = s + ' (' + (healthScore.grade || '--') + ')';
    ovHealthEl.style.color = s >= 80 ? 'var(--green)' : s >= 60 ? 'var(--orange)' : 'var(--red)';
  } else {
    ovHealthEl.textContent = '--';
    ovHealthEl.style.color = '';
  }

  // Total Files
  const ovFiles = document.querySelector('#ovFiles .ov-value');
  if (archData && archData.total_files !== undefined) {
    ovFiles.textContent = archData.total_files;
  } else if (archData && archData.files) {
    ovFiles.textContent = archData.files.length;
  } else {
    ovFiles.textContent = '--';
  }

  // Total Lines
  const ovLines = document.querySelector('#ovLines .ov-value');
  if (archData && archData.total_lines !== undefined) {
    ovLines.textContent = archData.total_lines;
  } else {
    ovLines.textContent = '--';
  }

  // Total Symbols
  const ovSymbols = document.querySelector('#ovSymbols .ov-value');
  if (archData && archData.total_symbols !== undefined) {
    ovSymbols.textContent = archData.total_symbols;
  } else {
    ovSymbols.textContent = '--';
  }

  // Violations Count
  const ovViolations = document.querySelector('#ovViolations .ov-value');
  ovViolations.textContent = violations.length;
  if (violations.length > 0) {
    ovViolations.style.color = 'var(--orange)';
  } else {
    ovViolations.style.color = 'var(--green)';
  }

  // Primitives Count
  const ovPrimitives = document.querySelector('#ovPrimitives .ov-value');
  ovPrimitives.textContent = primitives.length;
}

function updateClock() {
  const now = new Date();
  document.getElementById('topTime').textContent = now.toLocaleTimeString('zh-CN', { hour12: false });
}
setInterval(updateClock, 1000);
updateClock();

// ============================================================
// Navigation
// ============================================================
function toggleSection(id) {
  collapsed[id] = !collapsed[id];
  const el = document.getElementById('sec-' + id);
  const chev = document.getElementById('chev-' + id);
  if (el) el.style.maxHeight = collapsed[id] ? '0' : '400px';
  if (el) {
    if (id === 'panorama') el.style.maxHeight = collapsed[id] ? '0' : '500px';
    else el.style.maxHeight = collapsed[id] ? '0' : '400px';
  }
  if (chev) chev.classList.toggle('open', !collapsed[id]);
}

function switchView(view) {
  currentView = view;
  document.querySelectorAll('.sidebar-item').forEach(el => el.classList.remove('active'));
  document.querySelectorAll('.sidebar-item[data-view="' + view + '"]').forEach(el => el.classList.add('active'));
  renderCurrentView();
}

function renderCurrentView() {
  disposeCharts();
  const container = document.getElementById('mainContent');
  container.innerHTML = '';
  switch (currentView) {
    case 'fileTree': renderFileTreeView(container); break;
    case 'topology': renderTopology(container); break;
    case 'health': renderHealth(container); break;
    case 'violations': renderViolationsView(container); break;
    case 'primitives': renderPrimitivesView(container); break;
    case 'layerMatrix': renderLayerMatrix(container); break;
    case 'heartbeat': renderHeartbeatView(container); break;
    case 'entropyHeatmap': renderEntropyHeatmapView(container); break;
    case 'errorBP': renderErrorBPView(container); break;
    case 'neuralTrace': renderNeuralTraceView(container); break;
    case 'dataFlow': renderDataFlowView(container); break;
    case 'sampling': renderSamplingView(container); break;
    case 'threshold': renderThresholdView(container); break;
    case 'whatif': renderWhatIfView(container); break;
    case 'causation': renderCausationView(container); break;
    case 'timetravel': renderTimeTravelView(container); break;
    case 'attribution': renderAttributionView(container); break;
    case 'obsSteps': renderObsSteps(container); break;
    case 'obsAggregates': renderObsAggregates(container); break;
    case 'obsPipelines': renderObsPipelines(container); break;
    case 'obsArch': renderObsArch(container); break;
    case 'devProgress': renderDevProgress(container); break;
    case 'migStatus': renderMigStatus(container); break;
    case 'migPatternMap': renderMigPatternMap(container); break;
    case 'migGateChain': renderMigGateChain(container); break;
    case 'migLog': renderMigLog(container); break;
    case 'migSessions': renderMigSessions(container); break;
    case 'archChangelog': renderArchChangelog(container); break;
    default: renderTopology(container);
  }
}

function disposeCharts() {
  Object.values(charts).forEach(c => { if (c && c.dispose) c.dispose(); });
  charts = {};
}

// ============================================================
// Right Panel Detail
// ============================================================
function showDetail(title, content) {
  const body = document.getElementById('rightPanelBody');
  body.innerHTML = '<div class="detail-field"><div class="dl">' + esc(title) + '</div><div class="dd">' + esc(content) + '</div></div>';
}

// ============================================================
// Helpers
// ============================================================
function esc(s) {
  return String(s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}
function toast(msg, type) {
  const w = document.getElementById('toastWrap');
  if (!w) { console.log('[toast]', type, msg); return; }
  const el = document.createElement('div');
  el.className = 'toast ' + type;
  el.textContent = msg;
  w.appendChild(el);
  setTimeout(() => el.remove(), 3000);
}
