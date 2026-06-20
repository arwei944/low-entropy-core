/**
 * arch-manager-runtime2.js
 * Extracted from: arch-manager.html (lines 884-1059)
 *
 * Contains:
 *   - renderErrorBPView      (错误血压计)
 *   - renderNeuralTraceView  (Trace神经传导图)
 *   - renderDataFlowView     (数据流拓扑 Sankey)
 *
 * Dependencies (global, from core.js):
 *   runtime, errorHistory, archData, guardian, traceTree,
 *   obsPipelines, obsArch, charts, echarts
 */

// ============================================================
// 动态运行 — 错误血压计
// ============================================================
function renderErrorBPView(container) {
  container.innerHTML = '<div class="view-title">错误血压计</div><div class="view-desc">系统错误压力的动态血压计式可视化</div>';
  const grid = document.createElement('div');
  grid.className = 'grid-2';
  grid.innerHTML = `
    <div class="card"><div class="card-title">收缩压 (峰值错误)</div><div class="chart-container" id="sysChart"></div></div>
    <div class="card"><div class="card-title">舒张压 (基线错误)</div><div class="chart-container" id="diaChart"></div></div>
  `;
  container.appendChild(grid);
  const stats = document.createElement('div');
  stats.className = 'grid-4';
  const errVal = runtime.errors || 0;
  const errClass = errVal < 5 ? 'var(--green)' : errVal < 20 ? 'var(--orange)' : 'var(--red)';
  const errStatus = errVal < 5 ? '正常' : errVal < 20 ? '偏高' : '危险';
  stats.innerHTML = `
    <div class="stat-card"><div class="label">当前错误</div><div class="value" id="bpCurrent">${errVal}</div></div>
    <div class="stat-card"><div class="label">峰值 / 分钟</div><div class="value" id="bpPeak">${Math.round(errVal * 1.5)}</div></div>
    <div class="stat-card"><div class="label">基线</div><div class="value" id="bpBase">${Math.round(errVal * 0.3)}</div></div>
    <div class="stat-card"><div class="label">健康状态</div><div class="value" id="bpStatus" style="color:${errClass}">${errStatus}</div></div>
  `;
  container.appendChild(stats);

  // Use real runtime error data for chart, not random
  const sysData = Array.from({length: 20}, (_, i) => {
    if (i < errorHistory.length) return errorHistory[errorHistory.length - 20 + i] || 0;
    return errVal || 0;
  });
  const diaData = sysData.map(v => v * 0.6);

  charts.sys = echarts.init(document.getElementById('sysChart'));
  charts.sys.setOption({
    backgroundColor: 'transparent',
    grid: { left: 40, right: 10, top: 10, bottom: 20 },
    xAxis: { type: 'category', data: sysData.map((_, i) => i), axisLabel: { show: false }, axisLine: { show: false } },
    yAxis: { type: 'value', axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
    series: [{
      type: 'bar', data: sysData,
      itemStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: '#ff453a' }, { offset: 1, color: '#ff9f0a' }] } }
    }]
  });

  charts.dia = echarts.init(document.getElementById('diaChart'));
  charts.dia.setOption({
    backgroundColor: 'transparent',
    grid: { left: 40, right: 10, top: 10, bottom: 20 },
    xAxis: { type: 'category', data: diaData.map((_, i) => i), axisLabel: { show: false }, axisLine: { show: false } },
    yAxis: { type: 'value', axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
    series: [{
      type: 'bar', data: diaData,
      itemStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: '#0a84ff' }, { offset: 1, color: '#30d158' }] } }
    }]
  });
}

// ============================================================
// 动态运行 — Trace神经传导
// ============================================================
function renderNeuralTraceView(container) {
  container.innerHTML = '<div class="view-title">Trace神经传导图</div><div class="view-desc">分布式追踪的神经传导式可视化</div>';
  const card = document.createElement('div');
  card.className = 'card';
  card.innerHTML = '<div class="chart-container-lg" id="neuralChart"></div>';
  container.appendChild(card);

  const nodes = [], links = [];
  if (traceTree && traceTree.spans) {
    traceTree.spans.forEach((span, i) => {
      nodes.push({
        id: span.id,
        name: span.name || span.id,
        symbolSize: 10 + (span.duration || 10) / 5,
        value: span.duration || 0,
        itemStyle: { color: span.error ? '#ff453a' : '#0a84ff' },
        x: span.depth * 150,
        y: i * 40
      });
      if (span.parentId) {
        links.push({ source: span.parentId, target: span.id, value: 1 });
      }
    });
  }

  if (nodes.length === 0) {
    document.getElementById('neuralChart').innerHTML = '<div class="empty-state" style="padding:40px"><p>暂无Trace数据</p></div>';
    return;
  }

  charts.neural = echarts.init(document.getElementById('neuralChart'));
  charts.neural.setOption({
    backgroundColor: 'transparent',
    tooltip: { backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' }, formatter: (p) => p.data ? p.data.name + '<br/>耗时: ' + p.data.value + 'ms' : '' },
    series: [{
      type: 'graph',
      layout: 'none',
      roam: true,
      animation: false,
      label: { show: true, fontSize: 11, color: '#98989d', fontFamily: 'var(--mono)' },
      edgeSymbol: ['none', 'arrow'],
      edgeSymbolSize: [0, 8],
      data: nodes,
      links: links,
      lineStyle: { color: '#2c2c2e', width: 2, curveness: 0.1 },
      emphasis: { focus: 'adjacency', lineStyle: { width: 3, color: '#0a84ff' } }
    }]
  });
}

// ============================================================
// 动态运行 — 数据流拓扑
// ============================================================
function renderDataFlowView(container) {
  container.innerHTML = '<div class="view-title">数据流拓扑</div><div class="view-desc">基于 Observation Pipeline 数据的 Sankey 拓扑图</div>';
  const card = document.createElement('div');
  card.className = 'card';
  card.innerHTML = '<div class="chart-container-lg" id="sankeyChart"></div>';
  container.appendChild(card);

  // Build Sankey from real pipeline data and layer dependencies
  const nodes = [], links = [], nodeSet = new Set();
  const addNode = (name) => { if (!nodeSet.has(name)) { nodeSet.add(name); nodes.push({ name }); } };

  if (obsPipelines && obsPipelines.length > 0) {
    obsPipelines.forEach(p => {
      addNode(p.name || p.id || 'unknown');
    });
    // Add flows between layers
    if (archData && archData.files) {
      const layerFlows = {};
      archData.files.forEach(f => {
        (f.depends_on || []).forEach(dep => {
          const key = f.layer + '→' + (archData.files.find(x => x.name === dep || x.name === dep + '.go')?.layer || '?');
          layerFlows[key] = (layerFlows[key] || 0) + 1;
        });
      });
      const layerNames = {};
      (archData.layers || []).forEach(l => { layerNames[l.layer] = l.name; });
      Object.entries(layerFlows).forEach(([key, val]) => {
        const [src, tgt] = key.split('→');
        const srcName = src + ' ' + (layerNames[src] || '');
        const tgtName = tgt + ' ' + (layerNames[tgt] || '');
        addNode(srcName);
        addNode(tgtName);
        links.push({ source: srcName, target: tgtName, value: val });
      });
    }
  } else {
    // Fallback: use layer data
    (archData?.layers || []).forEach(l => { addNode(l.layer + ' ' + l.name); });
    const flows = obsArch?.layer_flows;
    if (flows) {
      Object.entries(flows).forEach(([k, v]) => {
        const [s, t] = k.split('→');
        links.push({ source: s, target: t, value: v });
      });
    }
  }

  charts.sankey = echarts.init(document.getElementById('sankeyChart'));
  charts.sankey.setOption({
    backgroundColor: 'transparent',
    tooltip: { backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' } },
    series: [{
      type: 'sankey',
      layout: 'none',
      emphasis: { focus: 'adjacency' },
      data: nodes.length ? nodes : [{ name: '暂无数据' }],
      links: links.length ? links : [{ source: '暂无数据', target: '暂无数据', value: 0 }],
      lineStyle: { color: 'gradient', curveness: 0.5, opacity: 0.4 },
      itemStyle: { color: '#0a84ff', borderColor: '#0a0a0a' },
      label: { color: '#f5f5f7', fontSize: 11, fontFamily: 'var(--font)' }
    }]
  });
}
