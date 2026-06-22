/**
 * arch-manager-panorama-charts.js
 * 架构全景视图 — ECharts 图表渲染函数
 *
 * 从 arch-manager-panorama.js 拆分
 *
 * 包含函数:
 *   - renderTopology        拓扑图 (Force-directed graph)
 *   - renderHealth          健康仪表 (雷达图 + 仪表盘 + 趋势曲线)
 *
 * 依赖全局变量 (来自 core.js):
 *   archData, healthScore, healthHistory, charts, esc
 */

// ============================================================
// 架构全景 — 拓扑图 (Force-directed graph)
// ============================================================
function renderTopology(container) {
  container.innerHTML = '<div class="view-title">拓扑图</div><div class="view-desc">文件/模块间的依赖关系力导向图</div>';

  if (!archData || !archData.files || archData.files.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'empty-state';
    empty.innerHTML = '<p style="color:var(--dim)">' + (archData === null ? '加载中...' : '暂无拓扑数据') + '</p>';
    container.appendChild(empty);
    return;
  }

  const card = document.createElement('div');
  card.className = 'card';
  card.innerHTML = '<div class="chart-container-lg" id="topologyChart"></div>';
  container.appendChild(card);

  const nodes = [], links = [];
  const layerSet = new Set();
  const nameToPath = {};
  if (archData && archData.files) {
    archData.files.forEach((f) => {
      const shortName = (f.path || f.name).replace(/\\/g, '/').split('/').pop() || f.name;
      nameToPath[shortName] = nameToPath[shortName] || f.path || f.name;
      const layer = archData.layers?.find(l => l.layer === f.layer);
      const color = layer?.color || '#888';
      layerSet.add(f.layer);
      nodes.push({
        id: f.path || f.name,
        name: shortName.replace('.go', ''),
        symbolSize: Math.max(14, Math.min(36, 10 + (f.lines || 100) / 20)),
        value: f.lines || 100,
        category: f.layer || 'L0',
        itemStyle: { color: color }
      });
    });
    archData.files.forEach((f) => {
      (f.depends_on || []).forEach(dep => {
        const targetName = dep + (dep.endsWith('.go') ? '' : '.go');
        const targetPath = nameToPath[targetName] || targetName;
        links.push({ source: f.path || f.name, target: targetPath, value: 1 });
      });
    });
  }
  if (nodes.length === 0) {
    document.getElementById('topologyChart').innerHTML = '<div style="display:flex;align-items:center;justify-content:center;height:100%;color:var(--dim)">暂无拓扑数据</div>';
    return;
  }

  const categories = [...layerSet].sort().map(l => {
    const layer = archData.layers?.find(ll => ll.layer === l);
    return { name: l, itemStyle: { color: layer?.color || '#888' } };
  });

  const chartDom = document.getElementById('topologyChart');
  if (!chartDom) return;
  charts.topology = echarts.init(chartDom);
  charts.topology.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      backgroundColor: '#1c1c1e',
      borderColor: '#2c2c2e',
      textStyle: { color: '#f5f5f7' },
      formatter: function(p) {
        if (p.dataType === 'edge') return p.data.source + ' → ' + p.data.target;
        return p.name + '<br/>行数: ' + p.value + '<br/>层级: ' + p.data.category;
      }
    },
    legend: { show: false },
    series: [{
      type: 'graph',
      layout: 'force',
      roam: true,
      draggable: true,
      categories: categories,
      label: { show: true, fontSize: 10, color: '#98989d', fontFamily: 'system-ui' },
      edgeSymbol: ['none', 'arrow'],
      edgeSymbolSize: [0, 6],
      data: nodes,
      links: links,
      force: { repulsion: 350, edgeLength: 100, gravity: 0.08, layoutAnimation: true },
      lineStyle: { color: '#2c2c2e', width: 1, curveness: 0.2, opacity: 0.5 },
      emphasis: {
        focus: 'adjacency',
        lineStyle: { width: 2, color: '#0a84ff', opacity: 1 }
      }
    }]
  });
  charts.topology.on('click', function(p) {
    if (p.data && p.data.id) {
      const f = (archData?.files || []).find(x => (x.path || x.name) === p.data.id);
      if (f) showFileDetail(f);
    }
  });
}

// ============================================================
// 架构全景 — 健康仪表
// ============================================================
function renderHealth(container) {
  container.innerHTML = '<div class="view-title">健康仪表</div><div class="view-desc">五维雷达图 + 评分仪表盘 + 趋势曲线 + 改进建议</div>';

  if (healthScore === null || healthScore === undefined) {
    const empty = document.createElement('div');
    empty.className = 'empty-state';
    empty.innerHTML = '<p style="color:var(--dim)">健康评分加载中...</p>';
    container.appendChild(empty);
    return;
  }

  if (typeof renderHealthDetail === 'function') {
    renderHealthDetail(container, healthScore);
  }

  const grid = document.createElement('div');
  grid.className = 'grid-2';
  grid.innerHTML = `
    <div class="card"><div class="card-title">五维雷达</div><div class="chart-container" id="radarChart"></div></div>
    <div class="card"><div class="card-title">评分仪表盘</div><div class="chart-container" id="gaugeChart"></div></div>
  `;
  container.appendChild(grid);
  const trendCard = document.createElement('div');
  trendCard.className = 'card';
  trendCard.innerHTML = '<div class="card-title">健康趋势</div><div class="chart-container" id="trendChart"></div>';
  container.appendChild(trendCard);

  const factors = healthScore.factors || { layer_balance: 70, file_granularity: 65, symbol_density: 80, dependency_depth: 60, interface_ratio: 75 };
  const names = { layer_balance: '层级平衡', file_granularity: '文件粒度', symbol_density: '符号密度', dependency_depth: '依赖深度', interface_ratio: '接口率' };
  const factorKeys = Object.keys(factors);
  const indicator = factorKeys.map(k => ({ name: names[k] || k, max: 100 }));
  const values = factorKeys.map(k => Math.round(factors[k] || 0));

  const tooltipFormatter = (typeof radarTooltipFormatter === 'function')
    ? radarTooltipFormatter(healthScore.factor_details, factors)
    : function(p) { return (p.name || '') + '：' + (p.value || 0); };

  charts.radar = echarts.init(document.getElementById('radarChart'));
  charts.radar.setOption({
    backgroundColor: 'transparent',
    tooltip: {
      backgroundColor: '#1c1c1e',
      borderColor: '#2c2c2e',
      textStyle: { color: '#f5f5f7' },
      formatter: tooltipFormatter
    },
    radar: {
      indicator,
      axisName: { color: '#98989d', fontSize: 11 },
      splitArea: { areaStyle: { color: ['rgba(255,255,255,0.02)', 'rgba(255,255,255,0.04)'] } },
      axisLine: { lineStyle: { color: '#2c2c2e' } },
      splitLine: { lineStyle: { color: '#2c2c2e' } }
    },
    series: [{
      type: 'radar',
      data: [{ value: values, name: '健康度', areaStyle: { color: 'rgba(10,132,255,0.2)' }, lineStyle: { color: '#0a84ff' }, itemStyle: { color: '#0a84ff' } }]
    }]
  });

  charts.gauge = echarts.init(document.getElementById('gaugeChart'));
  const score = Math.round(healthScore.overall || 0);
  charts.gauge.setOption({
    backgroundColor: 'transparent',
    series: [{
      type: 'gauge',
      startAngle: 200, endAngle: -20,
      min: 0, max: 100,
      splitNumber: 10,
      itemStyle: { color: score >= 80 ? '#30d158' : score >= 60 ? '#ff9f0a' : '#ff453a' },
      progress: { show: true, width: 18 },
      pointer: { show: false },
      axisLine: { lineStyle: { width: 18, color: [[1, '#2c2c2e']] } },
      axisTick: { show: false },
      splitLine: { show: false },
      axisLabel: { show: false },
      anchor: { show: false },
      title: { show: false },
      detail: {
        valueAnimation: true,
        fontSize: 48,
        fontWeight: 700,
        fontFamily: 'var(--font)',
        color: '#f5f5f7',
        offsetCenter: [0, '10%'],
        formatter: '{value}'
      },
      data: [{ value: score }]
    }]
  });

  let trendData = [], trendLabels = [];
  if (healthHistory && healthHistory.length > 0) {
    trendData = healthHistory.map(h => {
      const s = h.score || h;
      return Math.round(s.overall ?? s.score ?? s.value ?? 0);
    });
    trendLabels = healthHistory.map((_, i) => 'T-' + (healthHistory.length - i));
  }
  if (trendData.length === 0) {
    const trendEl = document.getElementById('trendChart');
    if (trendEl) trendEl.innerHTML = '<div class="empty-state" style="padding:40px"><p>暂无历史数据</p></div>';
    return;
  }

  charts.trend = echarts.init(document.getElementById('trendChart'));
  charts.trend.setOption({
    backgroundColor: 'transparent',
    grid: { left: 50, right: 20, top: 20, bottom: 30 },
    xAxis: { type: 'category', data: trendLabels, axisLabel: { color: '#6e6e73', fontSize: 10 }, axisLine: { lineStyle: { color: '#2c2c2e' } } },
    yAxis: { type: 'value', min: 0, max: 100, axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
    series: [{
      type: 'line', data: trendData, smooth: true,
      lineStyle: { color: '#0a84ff', width: 2 },
      itemStyle: { color: '#0a84ff' },
      areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: 'rgba(10,132,255,0.2)' }, { offset: 1, color: 'rgba(10,132,255,0.02)' }] } }
    }]
  });
}
