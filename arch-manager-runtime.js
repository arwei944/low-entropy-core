/**
 * arch-manager-runtime.js
 * Extracted from: arch-manager.html (lines 728-882)
 *
 * Contains:
 *   - renderHeartbeatView   (心跳 ECG-style TPS)
 *   - renderHeartbeat      (心跳实时刷新)
 *   - setInterval(renderHeartbeat, 2000)
 *   - renderEntropyHeatmapView  (熵值热力图)
 *   - renderEntropyHeatmap       (熵值热力图实时更新)
 *
 * Dependencies (global, from core.js):
 *   runtime, tpsHistory, errorHistory, latencyHistory, MAX_POINTS,
 *   archData, guardian, charts, echarts, esc
 */

// ============================================================
// 动态运行 — 心跳 (ECG-style TPS)
// ============================================================
function renderHeartbeatView(container) {
  container.innerHTML = '<div class="view-title">心跳</div><div class="view-desc">ECG风格的实时TPS监控</div>';
  const grid = document.createElement('div');
  grid.className = 'grid-2';
  grid.innerHTML = `
    <div class="card"><div class="card-title">实时 TPS</div><div class="chart-container" id="tpsChart"></div></div>
    <div class="card"><div class="card-title">错误率</div><div class="chart-container" id="errChart"></div></div>
  `;
  container.appendChild(grid);
  const latCard = document.createElement('div');
  latCard.className = 'card';
  latCard.innerHTML = '<div class="card-title">延迟分布</div><div class="chart-container" id="latChart"></div>';
  container.appendChild(latCard);

  // Check if we have any real data
  const hasData = runtime.tps > 0 || runtime.errors > 0 || runtime.latency > 0;

  if (!hasData) {
    document.getElementById('tpsChart').innerHTML = '<div class="empty-state" style="padding:40px"><p>等待运行时数据...</p></div>';
    document.getElementById('errChart').innerHTML = '<div class="empty-state" style="padding:40px"><p>等待运行时数据...</p></div>';
    document.getElementById('latChart').innerHTML = '<div class="empty-state" style="padding:40px"><p>等待运行时数据...</p></div>';
    return;
  }

  // Seed history with real values if empty
  if (tpsHistory.length === 0) {
    for (let i = 0; i < MAX_POINTS; i++) {
      tpsHistory.push(runtime.tps || 0);
      errorHistory.push(runtime.errors || 0);
      latencyHistory.push(runtime.latency || 0);
    }
  }

  const common = {
    backgroundColor: 'transparent',
    grid: { left: 45, right: 15, top: 10, bottom: 25 },
    xAxis: { type: 'category', data: tpsHistory.map((_, i) => i), axisLabel: { show: false }, axisLine: { show: false }, axisTick: { show: false } },
    yAxis: { type: 'value', axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } }
  };

  charts.tps = echarts.init(document.getElementById('tpsChart'));
  charts.tps.setOption({
    ...common,
    series: [{
      type: 'line', data: tpsHistory,
      smooth: false, symbol: 'none',
      lineStyle: { color: '#0a84ff', width: 2 },
      areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(10,132,255,0.2)' }, { offset: 1, color: 'rgba(10,132,255,0)' }] } }
    }]
  });

  charts.err = echarts.init(document.getElementById('errChart'));
  charts.err.setOption({
    ...common,
    series: [{
      type: 'line', data: errorHistory,
      smooth: false, symbol: 'none',
      lineStyle: { color: '#ff453a', width: 2 },
      areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(255,69,58,0.2)' }, { offset: 1, color: 'rgba(255,69,58,0)' }] } }
    }]
  });

  charts.lat = echarts.init(document.getElementById('latChart'));
  charts.lat.setOption({
    backgroundColor: 'transparent',
    grid: { left: 45, right: 15, top: 10, bottom: 25 },
    xAxis: { type: 'category', data: latencyHistory.map((_, i) => i), axisLabel: { show: false }, axisLine: { show: false }, axisTick: { show: false } },
    yAxis: { type: 'value', axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
    series: [{
      type: 'line', data: latencyHistory,
      smooth: true, symbol: 'none',
      lineStyle: { color: '#30d158', width: 2 },
      areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(48,209,88,0.2)' }, { offset: 1, color: 'rgba(48,209,88,0)' }] } }
    }]
  });
}

function renderHeartbeat() {
  if (!charts.tps) return;
  // Use real values only, no random fallback
  tpsHistory.push(runtime.tps || 0);
  errorHistory.push(runtime.errors || 0);
  latencyHistory.push(runtime.latency || 0);
  if (tpsHistory.length > MAX_POINTS) tpsHistory.shift();
  if (errorHistory.length > MAX_POINTS) errorHistory.shift();
  if (latencyHistory.length > MAX_POINTS) latencyHistory.shift();
  charts.tps.setOption({ series: [{ data: tpsHistory }] });
  if (charts.err) charts.err.setOption({ series: [{ data: errorHistory }] });
  if (charts.lat) charts.lat.setOption({ series: [{ data: latencyHistory }] });
}
setInterval(renderHeartbeat, 2000);

// ============================================================
// 动态运行 — 熵值热力图
// ============================================================
function renderEntropyHeatmapView(container) {
  container.innerHTML = '<div class="view-title">熵值热力图</div><div class="view-desc">各模块的熵值分布热力图</div>';
  const card = document.createElement('div');
  card.className = 'card';
  card.innerHTML = '<div class="chart-container-lg" id="heatmapChart"></div>';
  container.appendChild(card);

  const files = archData?.files || [];
  const complexityScores = archData?.complexity_scores || {};
  const guardianEntropies = guardian?.snapshot?.ModuleEntropies || {};

  // Use real complexity scores (prefer archData.complexity_scores, fallback to guardian snapshot)
  const hasComplexity = Object.keys(complexityScores).length > 0;
  const hasGuardianEntropies = Object.keys(guardianEntropies).length > 0;

  if (!hasComplexity && !hasGuardianEntropies) {
    document.getElementById('heatmapChart').innerHTML = '<div class="empty-state" style="padding:40px"><p>暂无熵值数据</p></div>';
    return;
  }

  const data = files.map((f, i) => {
    const score = hasComplexity ? (complexityScores[f.name] || 0) : (guardianEntropies[f.name] || 0);
    return [i % 8, Math.floor(i / 8), score, f.name];
  });
  const maxX = 8, maxY = Math.ceil(files.length / 8) || 8;

  charts.heatmap = echarts.init(document.getElementById('heatmapChart'));
  charts.heatmap.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7', fontSize: 11 },
      formatter: (p) => p.data ? '<b>' + esc(p.data[3]) + '</b><br/>熵值: ' + p.data[2].toFixed(1) : ''
    },
    grid: { left: 60, right: 20, top: 10, bottom: 30 },
    xAxis: { type: 'category', data: Array.from({length: maxX}, (_, i) => 'C' + (i+1)), axisLabel: { color: '#6e6e73', fontSize: 10 }, axisLine: { lineStyle: { color: '#2c2c2e' } } },
    yAxis: { type: 'category', data: Array.from({length: maxY}, (_, i) => 'R' + (i+1)), axisLabel: { color: '#6e6e73', fontSize: 10 }, axisLine: { lineStyle: { color: '#2c2c2e' } } },
    visualMap: {
      min: 0, max: 100,
      calculable: true,
      orient: 'horizontal',
      left: 'center',
      bottom: 0,
      inRange: { color: ['#30d158', '#ff9f0a', '#ff453a'] },
      textStyle: { color: '#98989d', fontSize: 10 }
    },
    series: [{
      type: 'heatmap',
      data: data,
      label: { show: false },
      emphasis: { itemStyle: { borderColor: '#0a84ff', borderWidth: 2 } }
    }]
  });
}

function renderEntropyHeatmap() {
  // Real-time update handled by re-fetch or SSE; chart auto-updates on next view render
}
