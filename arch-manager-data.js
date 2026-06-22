// ============================================================
// Data Loading & SSE — extracted from arch-manager.html
// ============================================================

async function fetchAll() {
  try {
    const [arch, health, vio, prim, hhist, gsnap, gthresh, gdrift, ghist, tps, errs, lat, sr, trace, vdiff, clog, osteps, oaggs, opipes, oarch, oerrs, mstatus, msessions, mlogs, achangelog, achangelogstats, flow, origin, obsPipeline] = await Promise.all([
      api('/api/arch').catch(() => null),
      api('/api/health-score').catch(() => null),
      api('/api/violations').catch(() => []),
      api('/api/primitives').catch(() => []),
      api('/api/health-score/history').catch(() => []),
      api('/api/guardian/snapshot').catch(() => null),
      api('/api/guardian/thresholds').catch(() => []),
      api('/api/guardian/drift').catch(() => 0),
      api('/api/guardian/history').catch(() => []),
      api('/api/runtime/tps').catch(() => 0),
      api('/api/runtime/errors').catch(() => 0),
      api('/api/runtime/latency').catch(() => 0),
      api('/api/runtime/sampling-rate').catch(() => 100),
      api('/api/observation/trace-tree').catch(() => null),
      api('/api/version/diff').catch(() => null),
      api('/api/version/changelog').catch(() => []),
      api('/api/observation/steps').catch(() => []),
      api('/api/observation/aggregates').catch(() => []),
      api('/api/observation/pipelines').catch(() => []),
      api('/api/observation/architecture').catch(() => null),
      api('/api/observation/steps/errors').catch(() => []),
      api('/api/migrate/status').catch(() => null),
      api('/api/migrate/sessions').catch(() => []),
      api('/api/migrate/logs?limit=100').catch(() => []),
      api('/api/arch-changelog?limit=100').catch(() => []),
      api('/api/arch-changelog/stats').catch(() => null),
      // === Phase B 新增 API ===
      api('/api/flow').catch(() => null),
      api('/api/origin?limit=50').catch(() => null),
      api('/api/observation/pipeline').catch(() => null)
    ]);
    archData = arch;
    healthScore = health;
    violations = Array.isArray(vio) ? vio : [];
    primitives = Array.isArray(prim) ? prim : [];
    healthHistory = Array.isArray(hhist) ? hhist : [];
    guardian = { snapshot: gsnap, thresholds: gthresh, drift: gdrift, history: ghist };
    runtime = { tps: tps || 0, errors: errs || 0, latency: lat || 0, sampling: sr || 100 };
    traceTree = trace;
    versionDiff = vdiff;
    changelog = Array.isArray(clog) ? clog : [];
    obsSteps = Array.isArray(osteps) ? osteps : [];
    obsAggregates = Array.isArray(oaggs) ? oaggs : [];
    obsPipelines = Array.isArray(opipes) ? opipes : [];
    obsArch = oarch;
    obsErrors = Array.isArray(oerrs) ? oerrs : [];
    migStatus = mstatus;
    migSessions = Array.isArray(msessions) ? msessions : [];
    migLogs = Array.isArray(mlogs) ? mlogs : [];
    archChangelog = Array.isArray(achangelog) ? achangelog : [];
    archChangelogStats = achangelogstats;
    // === Phase B 新增数据 ===
    flowData = flow;
    originData = origin;
    obsPipelineData = obsPipeline;
    updateTopBar();
    updateOverviewDash();
    toast('数据已刷新', 'ok');
  } catch (e) {
    toast('刷新失败: ' + e.message, 'err');
  }
}

// ============================================================
// SSE
// ============================================================
function connectSSE() {
  if (sseArch) sseArch.close();
  if (sseGuardian) sseGuardian.close();

  try {
    sseArch = new EventSource('/api/sse');
    sseArch.onmessage = (ev) => {
      try {
        const d = JSON.parse(ev.data);
        if (d.arch) archData = d.arch;
        if (d.healthScore) healthScore = d.healthScore;
        if (d.violations) violations = d.violations;
        if (d.runtime) runtime = { ...runtime, ...d.runtime };
        updateTopBar();
        updateOverviewDash();
        if (currentView === 'heartbeat') renderHeartbeat();
        if (currentView === 'entropyHeatmap') renderEntropyHeatmap();
        if (currentView === 'errorBP') renderErrorBP();
      } catch (e) {}
    };
    sseArch.onerror = () => { if (sseArch) { sseArch.close(); sseArch = null; } };
  } catch (e) {}

  try {
    sseGuardian = new EventSource('/api/guardian/sse');
    sseGuardian.onmessage = (ev) => {
      try {
        const d = JSON.parse(ev.data);
        if (d.snapshot) guardian.snapshot = d.snapshot;
        if (d.drift !== undefined) guardian.drift = d.drift;
        if (d.thresholds) guardian.thresholds = d.thresholds;
        updateTopBar();
        updateOverviewDash();
        if (currentView === 'entropyHeatmap') renderEntropyHeatmap();
        if (currentView === 'threshold') renderThresholdView();
      } catch (e) {}
    };
    sseGuardian.onerror = () => { if (sseGuardian) { sseGuardian.close(); sseGuardian = null; } }
  } catch (e) {}

  // Dev Events SSE — 实时开发进度
  try {
    sseDev = new EventSource('/api/sse/dev-events');
    sseDev.onmessage = (ev) => {
      try {
        const e = JSON.parse(ev.data);
        e._ts = Date.now();
        devEvents.unshift(e);
        if (devEvents.length > 200) devEvents.length = 200;
        // 实时更新视图
        if (e.type === 'file_changed' || e.type === 'build_done') {
          updateTopBar();
          updateOverviewDash();
        }
        if (currentView === 'devProgress') renderDevProgress();
      } catch (ex) {}
    };
    sseDev.onerror = () => { if (sseDev) { sseDev.close(); sseDev = null; } };
  } catch (e) {}

  // 迁移引擎 SSE
  if (sseMigrate) sseMigrate.close();
  try {
    sseMigrate = new EventSource('/api/sse/migrate');
    sseMigrate.onmessage = (ev) => {
      try {
        const e = JSON.parse(ev.data);
        if (e.type === 'analyze_done' && e.data) migPatternMap = e.data.pattern_map;
        if (e.type === 'validate_done' && e.data) migGateChain = e.data;
        if (e.type === 'log_append' && e.data) { migLogs.unshift(e.data); if (migLogs.length > 500) migLogs.length = 500; }
        if (currentView === 'migStatus' || currentView === 'migLog') renderCurrentView();
      } catch (ex) {}
    };
    sseMigrate.onerror = () => { if (sseMigrate) { sseMigrate.close(); sseMigrate = null; } };
  } catch (e) {}

  // 架构变动日志 SSE
  if (sseChangelog) sseChangelog.close();
  try {
    sseChangelog = new EventSource('/api/sse/arch-changelog');
    sseChangelog.onmessage = (ev) => {
      try {
        const e = JSON.parse(ev.data);
        if (e.type !== 'connected') {
          archChangelog.unshift(e);
          if (archChangelog.length > 500) archChangelog.length = 500;
          const badge = document.getElementById('changelogBadge');
          if (badge) { badge.style.display = 'inline'; badge.textContent = archChangelog.length; }
          if (currentView === 'archChangelog') renderArchChangelog(document.getElementById('mainContent'));
        }
      } catch (ex) {}
    };
    sseChangelog.onerror = () => { if (sseChangelog) { sseChangelog.close(); sseChangelog = null; } };
  } catch (e) {}
}

// ============================================================
// Init
// ============================================================
async function init() {
  try {
    await fetchAll();
    updateOverviewDash();
    updateTopBar();
    connectSSE();
    setTimeout(() => { updateOverviewDash(); updateTopBar(); }, 500);
    renderCurrentView();
    toast('架构仪表盘已就绪', 'ok');
  } catch (e) {
    console.error('[init]', e.message, e.stack);
    // Fallback: ensure view renders
    setTimeout(() => { try { renderCurrentView(); } catch(e2) {} }, 1000);
    toast('初始化出错: ' + e.message, 'err');
  }
}
window.addEventListener('DOMContentLoaded', init);
window.addEventListener('resize', () => {
  Object.values(charts).forEach(c => c && c.resize && c.resize());
});
