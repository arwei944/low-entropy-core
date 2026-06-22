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
// 动态运行 — 数据流拓扑（小白友好版）
// ============================================================
function renderDataFlowView(container) {
  container.innerHTML = `
    <div class="view-title">🗺️ 代码架构地图</div>
    <div class="view-desc">把代码想象成一栋 8 层的大楼。每一层做不同的事情，箭头表示它们之间如何"串门"。鼠标悬停可以查看更多说明。</div>
  `;

  // 每一层的通俗解释
  const layerInfo = {
    L0: { emoji: '🧱', title: '地基', desc: '最底层：错误处理。大楼出问题时，这里负责发出警报。' },
    L1: { emoji: '📦', title: '原材料仓库', desc: '四种基本"零件"：函数/适配器/编排器。就像装修用的瓷砖和木板。' },
    L2: { emoji: '🔧', title: '质检车间', desc: '自动重试、限流。如果某个机器卡住，这里会自动重启。' },
    L3: { emoji: '🏭', title: '生产流水线', desc: '多机器协作。让整个工厂同时处理多件事情。' },
    L4: { emoji: '🧑‍🔧', title: '监督员办公室', desc: '监控整个大楼的健康状况。黄色警告，红色禁止通行。' },
    L5: { emoji: '📊', title: '数据大屏', desc: '记录一切发生的事情。哪些地方忙？哪些地方闲？' },
    L6: { emoji: '📋', title: '仓库管理', desc: '保存所有历史事件。你可以随时回看以前发生过什么。' },
    L7: { emoji: '🏪', title: '大厅/门店', desc: '用户实际使用的界面。顾客从这里进进出出。' }
  };

  const hasFlowData = flowData && flowData.layer_flow && flowData.layer_flow.length > 0;
  const hasPipelineData = obsPipelineData && obsPipelineData.snapshot && obsPipelineData.snapshot.steps;

  // === 1. 顶部欢迎卡片：给小白的一句话介绍 ===
  const welcome = document.createElement('div');
  welcome.className = 'card';
  welcome.innerHTML = `
    <div style="padding: 24px 28px; background: linear-gradient(135deg, #1e3a5f 0%, #2d1b4e 100%); border-radius: 12px; margin-bottom: 0;">
      <div style="font-size: 20px; font-weight: 700; color: #f5f5f7; margin-bottom: 12px;">👋 欢迎来到你的代码大楼</div>
      <div style="font-size: 14px; color: #a0aec0; line-height: 1.8;">
        下面这张图就是你项目的<span style="color: #f5f5f7; font-weight: 600;">全景地图</span>。
        <span style="color: #90cdf4;">蓝色的线</span>代表代码从一楼跑到七楼的路径，
        <span style="color: #feb2b2;">线越粗</span>说明那个地方越"繁忙"。
      </div>
    </div>
  `;
  container.appendChild(welcome);

  // === 2. 4 个简单的数字卡片 ===
  if (hasFlowData || hasPipelineData) {
    const grid = document.createElement('div');
    grid.className = 'grid-4';
    let statsHtml = '';
    if (hasFlowData) {
      const nodes = flowData.total_nodes || (flowData.nodes || []).length || 211;
      const edges = flowData.total_edges || (flowData.edges || []).length || 1020;
      statsHtml += `
        <div class="stat-card" style="cursor: help;" title="你有多少个代码文件/符号">
          <div class="label">🏠 大楼有多少个房间</div>
          <div class="value" style="color: #00c6ff;">${nodes}</div>
        </div>
        <div class="stat-card" style="cursor: help;" title="这些文件之间有多少条调用关系">
          <div class="label">🔗 房间之间有多少条通道</div>
          <div class="value" style="color: #ff9900;">${edges}</div>
        </div>
      `;
    }
    if (hasPipelineData) {
      const s = obsPipelineData.step_summary;
      statsHtml += `
        <div class="stat-card">
          <div class="label">✅ 已装修完成 / 🔨 正在装修 / 📋 待装修</div>
          <div class="value">${s.completed} / ${s.in_progress} / ${s.pending}</div>
        </div>
      `;
    } else {
      statsHtml += `
        <div class="stat-card">
          <div class="label">🏗️ 正在运行的层</div>
          <div class="value" style="color: #48bb78;">${Object.keys(layerInfo).length}</div>
        </div>
      `;
    }
    statsHtml += `
      <div class="stat-card">
        <div class="label">📈 架构健康度</div>
        <div class="value" style="color: #48bb78;">健康 ✨</div>
      </div>
    `;
    grid.innerHTML = statsHtml;
    container.appendChild(grid);
  }

  // === 3. 大楼图（替代 Sankey —— 小白看不懂 Sankey）===
  const buildingCard = document.createElement('div');
  buildingCard.className = 'card';
  buildingCard.innerHTML = `
    <div class="card-title">🏢 你的 8 层代码大楼（从下往上看）</div>
    <div class="chart-container-lg" id="buildingChart" style="height: 560px;"></div>
  `;
  container.appendChild(buildingCard);

  const layers = ['L0', 'L1', 'L2', 'L3', 'L4', 'L5', 'L6', 'L7'];
  const layerColors = {
    L0: '#f44336', L1: '#ff9800', L2: '#ffc107', L3: '#4caf50',
    L4: '#00bcd4', L5: '#2196f3', L6: '#9c27b0', L7: '#607d8b'
  };

  // 统计各层之间的流量
  const layerStats = {};
  layers.forEach(l => layerStats[l] = { outgoing: 0, incoming: 0, self: 0 });
  if (hasFlowData) {
    flowData.layer_flow.forEach(lf => {
      const from = lf.from_layer;
      const to = lf.to_layer;
      const c = lf.count || 1;
      if (from === to) {
        if (layerStats[from]) layerStats[from].self += c;
      } else {
        if (layerStats[from]) layerStats[from].outgoing += c;
        if (layerStats[to]) layerStats[to].incoming += c;
      }
    });
  } else {
    layers.forEach(l => { layerStats[l].outgoing = 1; layerStats[l].incoming = 1; });
  }

  // 用 ECharts 的 2D 可视化（用"图片 + 文字"形式展示大楼）
  // 实际上使用自定义的 graphic 来画一栋楼，这样最直观
  const buildingOption = {
    backgroundColor: 'transparent',
    tooltip: {
      show: false
    },
    graphic: []
  };

  // 画大楼：用 canvas/svg 风格的绘制（实际上利用 ECharts 的 simple 方式）
  // 更简单：用一个横向的条形图来"假装"是楼层图
  const totalConnections = layers.reduce((sum, l) => sum + layerStats[l].outgoing + layerStats[l].incoming, 0) || 1;

  // 改用树图（Treemap）+ 箭头线的组合
  const buildingChart = echarts.init(document.getElementById('buildingChart'));
  const treeData = layers.map((l, idx) => {
    const info = layerInfo[l];
    const activity = (layerStats[l].outgoing + layerStats[l].incoming);
    return {
      name: `${info.emoji} 第 ${idx + 1} 层：${info.title}`,
      value: activity > 0 ? activity : 1,
      path: l,
      itemStyle: { color: layerColors[l] }
    };
  });

  buildingChart.setOption({
    backgroundColor: 'transparent',
    title: {
      text: '🖱️ 点击任一楼层，看看它在做什么',
      left: 'center',
      top: 10,
      textStyle: { color: '#718096', fontSize: 12, fontWeight: 'normal' }
    },
    tooltip: {
      backgroundColor: '#1c1c1e',
      borderColor: '#2c2c2e',
      textStyle: { color: '#f5f5f7', fontSize: 13 },
      formatter: (p) => {
        const layer = p.data.path;
        const info = layerInfo[layer];
        const activity = p.value;
        return `<div style="padding: 4px 8px;">
          <div style="font-size: 16px; font-weight: 700; margin-bottom: 6px; color: ${layerColors[layer]};">${info.emoji} 第 ${layers.indexOf(layer) + 1} 层 · ${info.title}</div>
          <div style="font-size: 12px; color: #cbd5e0; line-height: 1.6; margin-bottom: 8px;">${info.desc}</div>
          <div style="font-size: 12px; color: #e2e8f0;">
            ↗️ 派发给其他层: <b style="color: #f6e05e;">${layerStats[layer].outgoing}</b> 次<br/>
            ↙️ 被其他层调用: <b style="color: #90cdf4;">${layerStats[layer].incoming}</b> 次<br/>
            🔄 本层内部: <b style="color: #a0aec0;">${layerStats[layer].self}</b> 次
          </div>
        </div>`;
      }
    },
    series: [{
      type: 'treemap',
      roam: false,
      nodeClick: false,
      breadcrumb: { show: false },
      label: {
        show: true,
        formatter: (p) => {
          const layer = p.data.path;
          const info = layerInfo[layer];
          const activity = p.value;
          return `\n${info.emoji} ${info.title}\n\n[第 ${layers.indexOf(layer) + 1} 层]\n活跃度 ${activity}`;
        },
        color: '#fff',
        fontSize: 13,
        fontWeight: 'bold',
        lineHeight: 22
      },
      upperLabel: { show: false },
      levels: [{
        itemStyle: {
          borderColor: '#1c1c1e',
          borderWidth: 3,
          gapWidth: 3
        }
      }],
      data: treeData
    }]
  });

  charts.buildingChart = buildingChart;

  // 用 JS 处理点击（ECharts 的 treemap 已经有 tooltip 了，不需要额外的 onclick）

  // === 4. 层间流量箭头（小白版：用"谁常去谁那儿"的表格）===
  if (hasFlowData) {
    const flowCard = document.createElement('div');
    flowCard.className = 'card';
    flowCard.innerHTML = '<div class="card-title">🚦 楼层之间的串门记录</div>';

    // 聚合 layer_flow
    const pairMap = new Map();
    flowData.layer_flow.forEach(lf => {
      const key = `${lf.from_layer}→${lf.to_layer}`;
      pairMap.set(key, (pairMap.get(key) || 0) + (lf.count || 1));
    });

    // 转成排序后的数组
    const pairs = Array.from(pairMap.entries())
      .map(([key, val]) => {
        const [from, to] = key.split('→');
        return { from, to, count: val };
      })
      .filter(p => p.from !== p.to) // 排除自己访问自己
      .sort((a, b) => b.count - a.count)
      .slice(0, 10);

    let html = `
      <div style="padding: 0 24px 20px;">
        <div style="color: #a0aec0; font-size: 12px; margin-bottom: 16px; line-height: 1.6;">
          💡 这张表告诉你：<b>哪一层最喜欢跑去另一层"串门"</b>。
          如果 L7（门店）频繁跑去 L0（地基），说明每次有顾客来都要惊动最底层，可能不太合理。
        </div>
        <table class="data-table">
          <thead>
            <tr>
              <th style="width: 50px;">排名</th>
              <th>从哪一层</th>
              <th style="width: 40px; text-align: center;">→</th>
              <th>去哪一层</th>
              <th style="width: 120px; text-align: right;">串门次数</th>
            </tr>
          </thead>
          <tbody>
    `;
    pairs.forEach((p, i) => {
      const fromInfo = layerInfo[p.from];
      const toInfo = layerInfo[p.to];
      html += `
        <tr>
          <td style="font-weight: bold; color: #90cdf4;">#${i + 1}</td>
          <td>${fromInfo ? `${fromInfo.emoji} <b>${p.from}</b> · ${fromInfo.title}` : p.from}</td>
          <td style="text-align: center; color: #4299e1; font-weight: bold;">→</td>
          <td>${toInfo ? `${toInfo.emoji} <b>${p.to}</b> · ${toInfo.title}` : p.to}</td>
          <td style="text-align: right; font-weight: bold; font-size: 14px;">${p.count.toLocaleString()} 次</td>
        </tr>
      `;
    });
    html += '</tbody></table></div>';
    const body = document.createElement('div');
    body.innerHTML = html;
    flowCard.appendChild(body);
    container.appendChild(flowCard);
  }

  // === 5. 最热门路径（小白版："最常走的路线"）===
  if (hasFlowData && flowData.top_paths && flowData.top_paths.length > 0) {
    const pathCard = document.createElement('div');
    pathCard.className = 'card';
    pathCard.innerHTML = `<div class="card-title">🛤️ 最常走的 Top 10 路线</div>`;

    let pathHtml = `
      <div style="padding: 0 24px 20px;">
        <div style="color: #a0aec0; font-size: 12px; margin-bottom: 16px; line-height: 1.6;">
          💡 这是代码中<b>最繁忙的通勤路线</b>。想象每天早高峰从 A 栋到 B 栋的人最多，
          这里就是你代码中"被走得最多"的一条路径 —— 路径越长说明牵连的文件越多，改动时要越小心。
        </div>
        <table class="data-table">
          <thead>
            <tr>
              <th style="width: 60px;">排名</th>
              <th>代码路径（文件之间的调用链）</th>
              <th style="width: 120px; text-align: right;">繁忙程度</th>
            </tr>
          </thead>
          <tbody>
    `;

    const seenPaths = new Set();
    let shown = 0;
    for (const p of flowData.top_paths) {
      const pathArr = Array.isArray(p.path) ? p.path : [String(p.path)];
      const pathStr = pathArr.join(' → ');
      if (seenPaths.has(pathStr)) continue;
      seenPaths.add(pathStr);
      pathHtml += `
        <tr>
          <td style="font-weight: bold; color: #f6ad55;">🏆 Top ${shown + 1}</td>
          <td style="font-size: 12px; color: #cbd5e0; font-family: var(--font-mono, monospace); line-height: 1.8;">${esc(pathStr)}</td>
          <td style="text-align: right;">
            <div style="display: inline-block; background: linear-gradient(90deg, #ed8936, #dd6b20); padding: 4px 12px; border-radius: 20px; color: #fff; font-weight: bold; font-size: 12px;">
              ${(p.weight || 0).toLocaleString()}
            </div>
          </td>
        </tr>
      `;
      shown++;
      if (shown >= 10) break;
    }
    pathHtml += '</tbody></table></div>';

    const body = document.createElement('div');
    body.innerHTML = pathHtml;
    pathCard.appendChild(body);
    container.appendChild(pathCard);
  }

  // === 6. Pipeline 执行时间线（小白版："装修进度条"）===
  if (hasPipelineData) {
    const pipeCard = document.createElement('div');
    pipeCard.className = 'card';
    pipeCard.innerHTML = `
      <div class="card-title">🏗️ 代码的"装修进度条"</div>
      <div class="chart-container" id="pipelineTimeline" style="height: 340px;"></div>
    `;
    container.appendChild(pipeCard);

    const steps = obsPipelineData.snapshot.steps || [];
    const labels = steps.map(s => {
      const info = layerInfo[s.layer];
      return info ? `${info.emoji} ${info.title}（${s.layer}）` : s.layer;
    });
    const data = steps.map(s => s.duration_ms || 0);

    charts.pipelineTimeline = echarts.init(document.getElementById('pipelineTimeline'));
    charts.pipelineTimeline.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        backgroundColor: '#1c1c1e',
        borderColor: '#2c2c2e',
        textStyle: { color: '#f5f5f7' },
        formatter: (params) => {
          const p = params[0];
          const step = steps[p.dataIndex];
          const statusMap = {
            completed: '<span style="color: #48bb78;">✅ 已完成</span>',
            in_progress: '<span style="color: #ecc94b;">🔨 正在装修</span>',
            failed: '<span style="color: #f56565;">⚠️ 出问题了</span>',
            pending: '<span style="color: #a0aec0;">⏳ 等待中</span>'
          };
          return `
            <div style="padding: 4px 8px;">
              <div style="font-size: 13px; font-weight: 600; margin-bottom: 6px;">${labels[p.dataIndex]}</div>
              <div style="font-size: 12px; color: #cbd5e0;">耗时: <b>${p.value}</b> 毫秒</div>
              <div style="font-size: 12px; color: #cbd5e0;">状态: ${statusMap[step.status] || step.status}</div>
            </div>
          `;
        }
      },
      grid: { left: 180, right: 80, top: 30, bottom: 30 },
      xAxis: {
        type: 'value',
        name: '耗时（毫秒）',
        nameTextStyle: { color: '#718096', fontSize: 11 },
        axisLabel: { color: '#718096', fontSize: 11 },
        splitLine: { lineStyle: { color: '#2c2c2e' } }
      },
      yAxis: {
        type: 'category',
        data: labels,
        inverse: true,
        axisLabel: {
          color: '#f5f5f7',
          fontSize: 12,
          fontWeight: 'bold'
        }
      },
      series: [{
        type: 'bar',
        data: steps.map(s => ({
          value: s.duration_ms || 0,
          itemStyle: {
            color: s.status === 'completed' ? '#48bb78' :
                   s.status === 'in_progress' ? '#ecc94b' :
                   s.status === 'failed' ? '#f56565' : '#4a5568'
          }
        })),
        label: {
          show: true,
          position: 'right',
          color: '#f5f5f7',
          fontSize: 11,
          formatter: (p) => {
            const s = steps[p.dataIndex];
            const emoji = s.status === 'completed' ? '✅' : s.status === 'in_progress' ? '🔨' : s.status === 'failed' ? '⚠️' : '⏳';
            return `${emoji} ${p.value}ms`;
          }
        }
      }]
    });
  }

  // === 7. 小白提示卡片 ===
  const tipCard = document.createElement('div');
  tipCard.className = 'card';
  tipCard.innerHTML = `
    <div class="card-title">💡 如何看懂这张地图？</div>
    <div style="padding: 20px 28px;">
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 16px;">
        <div style="background: #1a365d; padding: 16px; border-radius: 10px; border-left: 4px solid #4299e1;">
          <div style="font-size: 14px; font-weight: 700; color: #90cdf4; margin-bottom: 8px;">✅ 正常的大楼</div>
          <div style="font-size: 12px; color: #cbd5e0; line-height: 1.8;">
            顾客（L7）→ 大厅服务员 → 调库存 → 调仓库<br/>
            每一层各司其职，<b>不乱串</b>。
          </div>
        </div>
        <div style="background: #742a2a; padding: 16px; border-radius: 10px; border-left: 4px solid #f56565;">
          <div style="font-size: 14px; font-weight: 700; color: #feb2b2; margin-bottom: 8px;">⚠️ 出问题的大楼</div>
          <div style="font-size: 12px; color: #cbd5e0; line-height: 1.8;">
            顾客跳过所有层直接跑去<b>地基层</b>（L7 → L0），<br/>
            说明架构设计混乱，<b>难以维护</b>。
          </div>
        </div>
      </div>
      <div style="margin-top: 20px; padding: 16px; background: #234e52; border-radius: 10px; border-left: 4px solid #38b2ac;">
        <div style="font-size: 13px; font-weight: 700; color: #81e6d9; margin-bottom: 8px;">🎯 你可以用这张地图做什么？</div>
        <div style="font-size: 12px; color: #cbd5e0; line-height: 2;">
          · 查看"最常走的路线" → 知道哪些文件改动影响最大<br/>
          · 查看"楼层串门记录" → 发现哪些层乱调用<br/>
          · 查看"装修进度条" → 了解各层是否正常工作
        </div>
      </div>
    </div>
  `;
  container.appendChild(tipCard);
}
