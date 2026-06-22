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
  container.innerHTML = '<div class="view-title">数据流拓扑</div><div class="view-desc">基于 /api/flow 数据的层间数据流 Sankey + 关键路径</div>';

  const hasFlowData = flowData && (flowData.nodes || flowData.layer_flow);
  const hasPipelineData = obsPipelineData && obsPipelineData.snapshot && obsPipelineData.snapshot.steps;

  // === 统计卡片 ===
  if (hasFlowData || hasPipelineData) {
    const grid = document.createElement('div');
    grid.className = 'grid-4';
    let html = '';
    if (hasFlowData) {
      html += `<div class="stat-card"><div class="label">数据流节点</div><div class="value">${flowData.total_nodes || (flowData.nodes || []).length || 0}</div></div>`;
      html += `<div class="stat-card"><div class="label">数据流边</div><div class="value">${flowData.total_edges || (flowData.edges || []).length || 0}</div></div>`;
    }
    if (hasPipelineData) {
      const s = obsPipelineData.step_summary;
      html += `<div class="stat-card"><div class="label">完成/进行/待定</div><div class="value">${s.completed}/${s.in_progress}/${s.pending}</div></div>`;
      html += `<div class="stat-card"><div class="label">Pipeline 耗时</div><div class="value">${(obsPipelineData.aggregate_stats?.pipeline_duration_ms || 0).toLocaleString()} ms</div></div>`;
    }
    grid.innerHTML = html;
    container.appendChild(grid);
  }

  // === 1. 层间数据流 Sankey（使用 /api/flow 的 layer_flow）===
  if (hasFlowData && flowData.layer_flow && flowData.layer_flow.length > 0) {
    const sankeyCard = document.createElement('div');
    sankeyCard.className = 'card';
    sankeyCard.innerHTML = '<div class="card-title">层间数据流 (Sankey)</div><div class="chart-container-lg" id="flowSankeyChart"></div>';
    container.appendChild(sankeyCard);

    const sNodes = [], sLinks = [], sNodeSet = new Set();
    const addNode = (name) => { if (!sNodeSet.has(name)) { sNodeSet.add(name); sNodes.push({ name }); } };

    // 层节点 (L0-L7)
    const layerColors = { L0:'#f44336', L1:'#ff9800', L2:'#ffc107', L3:'#4caf50', L4:'#00bcd4', L5:'#2196f3', L6:'#9c27b0', L7:'#607d8b' };
    Object.keys(layerColors).forEach(l => addNode(l));

    // 层间数据流 edges
    flowData.layer_flow.forEach(lf => {
      sLinks.push({ source: lf.from_layer, target: lf.to_layer, value: lf.count || 1 });
    });

    if (sNodes.length > 0) {
      charts.flowSankey = echarts.init(document.getElementById('flowSankeyChart'));
      charts.flowSankey.setOption({
        backgroundColor: 'transparent',
        tooltip: { backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' } },
        series: [{
          type: 'sankey',
          layout: 'none',
          emphasis: { focus: 'adjacency' },
          data: sNodes.map(n => ({ ...n, itemStyle: { color: layerColors[n.name] || '#888' } })),
          links: sLinks,
          lineStyle: { color: 'gradient', curveness: 0.5, opacity: 0.5 },
          itemStyle: { color: '#0a84ff', borderColor: '#0a0a0a' },
          label: { color: '#f5f5f7', fontSize: 11 }
        }]
      });
    }
  }

  // === 2. Top 关键路径表（使用 /api/flow 的 top_paths）===
  if (hasFlowData && flowData.top_paths && flowData.top_paths.length > 0) {
    const pathCard = document.createElement('div');
    pathCard.className = 'card';
    pathCard.innerHTML = '<div class="card-title">Top 关键路径 (按依赖权重排序)</div>';
    let pathHtml = '<table class="data-table"><thead><tr><th style="width:60px">#</th><th>路径</th><th style="width:120px">权重</th></tr></thead><tbody>';
    const seenPaths = new Set();
    let shown = 0;
    for (const p of flowData.top_paths) {
      const pathStr = Array.isArray(p.path) ? p.path.join(' → ') : String(p.path);
      if (seenPaths.has(pathStr)) continue;
      seenPaths.add(pathStr);
      pathHtml += `<tr><td class="mono">#${shown + 1}</td><td class="mono" style="font-size:12px;color:#a0aec0">${esc(pathStr)}</td><td style="font-weight:bold;color:#4299e1">${(p.weight || 0).toLocaleString()}</td></tr>`;
      shown++;
      if (shown >= 20) break;
    }
    pathHtml += '</tbody></table>';
    const body = document.createElement('div');
    body.style.padding = '0 20px 20px';
    body.innerHTML = pathHtml;
    pathCard.appendChild(body);
    container.appendChild(pathCard);
  }

  // === 3. Pipeline 时间线（使用 obsPipelineData） ===
  if (hasPipelineData) {
    const pipeCard = document.createElement('div');
    pipeCard.className = 'card';
    pipeCard.innerHTML = '<div class="card-title">8层 Pipeline 执行时间线</div><div class="chart-container" id="pipelineTimeline"></div>';
    container.appendChild(pipeCard);

    const steps = obsPipelineData.snapshot.steps || [];
    const labels = steps.map(s => `${s.layer} · ${s.name}`);
    const data = steps.map(s => s.duration_ms || 0);
    const statusColors = steps.map(s => {
      if (s.status === 'completed') return '#48bb78';
      if (s.status === 'in_progress') return '#ecc94b';
      if (s.status === 'failed') return '#f56565';
      return '#4a5568';
    });

    charts.pipelineTimeline = echarts.init(document.getElementById('pipelineTimeline'));
    charts.pipelineTimeline.setOption({
      backgroundColor: 'transparent',
      tooltip: { trigger: 'axis', backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' } },
      grid: { left: 120, right: 30, top: 20, bottom: 40 },
      xAxis: { type: 'value', axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
      yAxis: { type: 'category', data: labels, axisLabel: { color: '#f5f5f7', fontSize: 11 } },
      series: [{
        type: 'bar',
        data: steps.map((s, i) => ({ value: s.duration_ms || 0, itemStyle: { color: statusColors[i] } })),
        label: { show: true, position: 'right', color: '#f5f5f7', fontSize: 10, formatter: (p) => `${p.value}ms [${steps[p.dataIndex].status}]` }
      }]
    });
  }

  // === Fallback：沿用原有的数据结构 ===
  if (!hasFlowData && !hasPipelineData) {
    const card = document.createElement('div');
    card.className = 'card';
    card.innerHTML = '<div class="chart-container-lg" id="sankeyChart"></div>';
    container.appendChild(card);

    const nodes = [], links = [], nodeSet = new Set();
    const addNode = (name) => { if (!nodeSet.has(name)) { nodeSet.add(name); nodes.push({ name }); } };

    if (obsPipelines && obsPipelines.length > 0) {
      obsPipelines.forEach(p => { addNode(p.name || p.id || 'unknown'); });
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
}
