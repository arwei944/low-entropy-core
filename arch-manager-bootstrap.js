/**
 * arch-manager-bootstrap.js
 * 页面启动数据加载编排
 *
 * 依赖: arch-manager-core.js 中的全局变量
 *   archData, healthScore, violations, primitives,
 *   guardian, healthHistory, devEvents, sseArch
 *   以及: api(), updateTopBar(), initPanoramaCharts()
 */

// ============================================================
// Loading overlay
// ============================================================
function showLoading(msg) {
  const el = document.getElementById('loadingOverlay');
  if (!el) return;
  const text = el.querySelector('.loading-text');
  if (text) text.textContent = msg || '加载中...';
  el.style.display = 'flex';
}
function hideLoading() {
  const el = document.getElementById('loadingOverlay');
  if (!el) return;
  el.style.display = 'none';
}

// ============================================================
// Bootstrap — 并行加载核心数据
// ============================================================
async function loadAllData() {
  console.log('[arch] Loading architecture data...');
  showLoading('初始化架构数据...');

  try {
    const [arch, health, viol, prim, runDrift, hist] = await Promise.allSettled([
      api('/api/arch'),
      api('/api/health-score'),
      api('/api/violations'),
      api('/api/primitives'),
      api('/api/guardian/drift'),
      api('/api/health-score/history'),
    ]);
    if (arch.status === 'fulfilled') archData = arch.value;
    if (health.status === 'fulfilled') healthScore = health.value;
    if (viol.status === 'fulfilled') violations = viol.value;
    if (prim.status === 'fulfilled') primitives = prim.value;
    if (runDrift.status === 'fulfilled') guardian.drift = runDrift.value.drift || 0;
    if (hist.status === 'fulfilled') healthHistory = hist.value;
  } catch (e) { /* ignore */ }

  if (typeof initPanoramaCharts === 'function') initPanoramaCharts();
  updateTopBar();
  setupSSE();

  hideLoading();
  console.log('[arch] Data loaded.');
}

// ============================================================
// SSE — 架构事件监听
// ============================================================
function setupSSE() {
  try {
    const es1 = new EventSource('/api/sse');
    es1.onmessage = (e) => {
      try {
        const ev = JSON.parse(e.data);
        devEvents.push(ev);
        if (devEvents.length > 200) devEvents.shift();
      } catch (e) {}
    };
    sseArch = es1;
  } catch (e) { console.warn('SSE failed', e); }
}

// ============================================================
// Auto-start
// ============================================================
if (typeof window !== 'undefined') {
  window.addEventListener('DOMContentLoaded', () => {
    loadAllData();
  });
}
