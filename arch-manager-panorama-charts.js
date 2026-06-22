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
// 架构全景 — 拓扑图（小白友好版）
// ============================================================
function renderTopology(container) {
  container.innerHTML = `
    <div class="view-title">🏢 你的代码架构全景</div>
    <div class="view-desc">把代码想象成一栋 8 层的大楼。每层有不同的用途，下面用图形带你看清整栋楼长什么样。</div>
  `;

  // === 顶部欢迎卡片 ===
  const welcome = document.createElement('div');
  welcome.innerHTML = `
    <div class="card" style="margin-bottom: 16px;">
      <div style="padding: 28px 32px; background: linear-gradient(135deg, #1a365d 0%, #2d1b4e 100%); border-radius: 12px;">
        <div style="font-size: 22px; font-weight: 700; color: #f5f5f7; margin-bottom: 12px;">👋 这里是你代码建筑的全貌</div>
        <div style="font-size: 14px; color: #cbd5e0; line-height: 2;">
          想象你的代码是一栋 <b style="color: #90cdf4;">8 层的办公楼</b>。<br/>
          最底层处理最基础的事（比如"地基"负责出错时的警报），<br/>
          越往上越接近用户实际看到的界面（比如"大厅"就是你打开网站看到的内容）。<br/>
          <span style="color: #fbd38d;">👇 下面这张图就是这栋楼的俯视图</span>
        </div>
      </div>
    </div>
  `;
  container.appendChild(welcome);

  // === 4 个数字卡片 ===
  const totalFiles = archData?.files?.length || 0;
  const totalLines = (archData?.files || []).reduce((sum, f) => sum + (f.lines || 0), 0);
  const totalSymbols = (archData?.files || []).reduce((sum, f) => sum + ((f.symbols || []).length || (f.symbol_count || 0)), 0);
  const totalLayers = (archData?.layers || []).length || 8;

  const stats = document.createElement('div');
  stats.className = 'grid-4';
  stats.style.marginBottom = '16px';
  stats.innerHTML = `
    <div class="stat-card" title="你有多少个独立的代码文件">
      <div class="label">🏠 房间数（文件）</div>
      <div class="value" style="color: #00c6ff; font-size: 28px;">${totalFiles}</div>
    </div>
    <div class="stat-card" title="所有文件加起来有多少行代码">
      <div class="label">📝 总建筑面积（代码行数）</div>
      <div class="value" style="color: #ff9900; font-size: 28px;">${totalLines.toLocaleString()}</div>
    </div>
    <div class="stat-card" title="代码里有多少个函数/变量/类">
      <div class="label">🔌 功能点数（符号）</div>
      <div class="value" style="color: #30d158; font-size: 28px;">${totalSymbols.toLocaleString()}</div>
    </div>
    <div class="stat-card" title="你的代码一共分了多少层架构">
      <div class="label">🏢 楼层数（层级）</div>
      <div class="value" style="color: #bf5af2; font-size: 28px;">${totalLayers}</div>
    </div>
  `;
  container.appendChild(stats);

  // === 8 层大楼（Treemap）—— 每个层是一个大色块，大小代表代码量 ===
  const buildingCard = document.createElement('div');
  buildingCard.className = 'card';
  buildingCard.innerHTML = `
    <div class="card-title">🔍 大楼俯视图 — 每层大小代表代码量</div>
    <div style="padding: 0 24px 12px; color: #a0aec0; font-size: 12px; line-height: 1.8;">
      💡 <b>色块越大</b> → 这一层放的代码越多<br/>
      💡 <b>鼠标悬停</b> → 看这一层具体是做什么的<br/>
      💡 <b>颜色不同</b> → 代表不同的"工种"
    </div>
    <div class="chart-container-lg" id="topologyChart" style="height: 520px;"></div>
  `;
  container.appendChild(buildingCard);

  const layerMeta = {
    L0: { emoji: '🧱', title: '地基', desc: '错误处理。大楼出问题时，这里负责发警报。告诉你哪里坏了。' },
    L1: { emoji: '📦', title: '原材料仓库', desc: '四种基本"零件"：纯函数、外部接口、协议转换、编排组合。所有上层建筑都用这些零件组装。' },
    L2: { emoji: '🔧', title: '质检车间', desc: '自动重试、限流。如果某个机器卡住，这里会自动重启，防止一栋楼因为一台机器停摆。' },
    L3: { emoji: '🏭', title: '生产流水线', desc: '多机器协作。让整个工厂同时处理多件事情，而不是一件一件排队做。' },
    L4: { emoji: '🧑‍🔧', title: '监督员办公室', desc: '监控整栋楼的健康状况。黄色警告，红色禁止通行。随时看哪一层"压力"过大。' },
    L5: { emoji: '📊', title: '数据大屏', desc: '记录一切发生的事情。哪些地方忙、哪些地方闲、每个环节花了多少时间。' },
    L6: { emoji: '📋', title: '仓库管理', desc: '保存所有历史事件。你可以随时回看以前发生过什么，做到真正的"可追溯"。' },
    L7: { emoji: '🏪', title: '大厅/门店', desc: '用户实际使用的界面。顾客从这里进进出出，这里是大楼对外的窗口。' }
  };
  const layerColors = {
    L0: '#ff6b6b', L1: '#ff9f0a', L2: '#ffd60a', L3: '#30d158',
    L4: '#64d2ff', L5: '#0a84ff', L6: '#bf5af2', L7: '#64748b'
  };

  // 统计每层的代码量
  const layerStats = {};
  const layerFileCounts = {};
  (archData?.layers || []).forEach(l => {
    layerStats[l.layer] = layerStats[l.layer] || 0;
    layerFileCounts[l.layer] = layerFileCounts[l.layer] || 0;
    (archData?.files || []).forEach(f => {
      if (f.layer === l.layer) {
        layerStats[l.layer] += f.lines || 0;
        layerFileCounts[l.layer]++;
      }
    });
  });
  // 兜底：如果没有数据，给每层一些默认值
  Object.keys(layerMeta).forEach(layer => {
    if (!layerStats[layer]) { layerStats[layer] = 1; layerFileCounts[layer] = 0; }
  });

  // 构建 Treemap 数据
  const layerList = Object.keys(layerMeta);
  const nodes = layerList.map(layer => {
    const meta = layerMeta[layer];
    const lines = layerStats[layer] || 0;
    return {
      name: `${meta.emoji} ${meta.title}（${layer}）\n${lines} 行 · ${layerFileCounts[layer] || 0} 个文件`,
      value: lines,
      itemStyle: {
        color: layerColors[layer],
        borderColor: '#0f0f10',
        borderWidth: 3,
        gapWidth: 4
      },
      children: (archData?.files || []).filter(f => f.layer === layer).slice(0, 10).map(f => {
        const shortName = (f.path || f.name).replace(/\\/g, '/').split('/').pop() || f.name;
        return {
          name: shortName.replace('.go', ''),
          value: f.lines || 10,
          itemStyle: { color: layerColors[layer], opacity: 0.7 }
        };
      })
    };
  });

  // 如果某层没有子文件，显示为"空层"
  nodes.forEach(n => {
    if (!n.children || n.children.length === 0) {
      n.children = [{ name: '(暂无文件)', value: 1, itemStyle: { color: '#2c2c2e' } }];
    }
  });

  const chartDom = document.getElementById('topologyChart');
  if (chartDom) {
    charts.topology = echarts.init(chartDom);
    charts.topology.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        backgroundColor: '#1c1c1e',
        borderColor: '#2c2c2e',
        textStyle: { color: '#f5f5f7', fontSize: 13 },
        formatter: function(p) {
          const layerName = p.name.match(/（L\d）/)?.[0]?.replace(/[（）]/g, '');
          const match = p.name.match(/\((L\d)\)/);
          const layer = match ? match[1] : '';
          const meta = layerMeta[layer];
          if (meta) {
            return `
              <div style="padding: 4px 8px; max-width: 340px;">
                <div style="font-size: 15px; font-weight: 700; color: ${layerColors[layer]}; margin-bottom: 8px;">${meta.emoji} ${meta.title}</div>
                <div style="font-size: 12px; color: #cbd5e0; line-height: 1.7; margin-bottom: 8px;">${meta.desc}</div>
                <div style="font-size: 12px; color: #e2e8f0; border-top: 1px solid #2c2c2e; padding-top: 8px;">
                  📝 这一层有 <b>${layerStats[layer]}</b> 行代码<br/>
                  🏠 包含 <b>${layerFileCounts[layer]}</b> 个文件
                </div>
              </div>
            `;
          }
          return p.name + '<br/>' + p.value + ' 行';
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
            const match = p.name.match(/\((L\d)\)/);
            if (match) {
              // 这是层节点
              const lines = p.value;
              const fcount = layerFileCounts[match[1]] || 0;
              return p.name.replace(/\n/g, ' ');
            }
            return p.name;
          },
          color: '#fff',
          fontSize: 12,
          fontWeight: '600'
        },
        upperLabel: { show: true, color: '#fff', fontSize: 11, fontWeight: '700', height: 24 },
        levels: [
          {
            itemStyle: { borderColor: '#0f0f10', borderWidth: 4, gapWidth: 5 },
            upperLabel: { show: true }
          },
          {
            colorSaturation: [0.3, 0.6],
            itemStyle: { borderColorSaturation: 0.7, gapWidth: 2, borderWidth: 1, borderColor: '#1c1c1e' }
          }
        ],
        data: nodes
      }]
    });
    charts.topology.on('click', function(p) {
      if (p.name && archData?.files) {
        const f = archData.files.find(x => {
          const sn = (x.path || x.name).split('/').pop().replace('.go', '');
          return sn === p.name;
        });
        if (f) showFileDetail(f);
      }
    });
  }

  // === 架构说明卡（3列布局：正常 / 需要注意 / 可改进）===
  const insightCard = document.createElement('div');
  insightCard.className = 'card';
  insightCard.style.marginTop = '16px';
  insightCard.innerHTML = `
    <div class="card-title">🎯 3 秒看懂你的架构</div>
    <div style="padding: 20px 28px;">
      <div style="display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 14px;">
        <div style="background: #1a365d; padding: 18px; border-radius: 10px; border-left: 4px solid #30d158;">
          <div style="font-size: 14px; font-weight: 700; color: #90cdf4; margin-bottom: 8px;">✅ 这栋楼的优点</div>
          <div style="font-size: 12px; color: #cbd5e0; line-height: 1.9;">
            · 已按"8 层架构"分层管理<br/>
            · 每层都有明确的职责<br/>
            · 最底层（L0-L2）最稳定<br/>
            · 最上层（L6-L7）最灵活<br/>
            · 每层颜色清晰可辨
          </div>
        </div>
        <div style="background: #744210; padding: 18px; border-radius: 10px; border-left: 4px solid #ff9f0a;">
          <div style="font-size: 14px; font-weight: 700; color: #fbd38d; margin-bottom: 8px;">⚠️ 需要留意</div>
          <div style="font-size: 12px; color: #cbd5e0; line-height: 1.9;">
            · 色块<b style="color: #fff;">超大</b>的层 → 可能"职责过重"<br/>
            · 色块<b style="color: #fff;">极小</b>的层 → 可能没被充分利用<br/>
            · 层之间如果"串门"太多 → 可能架构混乱<br/>
            · 点击左侧"💡 数据流拓扑"看层间互动
          </div>
        </div>
        <div style="background: #2d1b4e; padding: 18px; border-radius: 10px; border-left: 4px solid #bf5af2;">
          <div style="font-size: 14px; font-weight: 700; color: #d6bcfa; margin-bottom: 8px;">🚀 如何使用这张图</div>
          <div style="font-size: 12px; color: #cbd5e0; line-height: 1.9;">
            1️⃣ <b>看顶部 4 个数字</b> → 了解整体规模<br/>
            2️⃣ <b>看 8 个色块大小</b> → 知道每层工作量<br/>
            3️⃣ <b>鼠标悬停每个色块</b> → 看详细说明<br/>
            4️⃣ <b>点击左侧菜单</b> → 深入看每个专项面板
          </div>
        </div>
      </div>

      <div style="margin-top: 18px; padding: 16px 20px; background: #234e52; border-radius: 10px; border-left: 4px solid #38b2ac;">
        <div style="font-size: 13px; font-weight: 700; color: #81e6d9; margin-bottom: 8px;">📌 下一步：看代码如何在层间"串门"</div>
        <div style="font-size: 12px; color: #cbd5e0; line-height: 1.8;">
          这张俯视图让你知道"每层有多大"。点击左侧菜单 <b>「动态运行 → 🌊 数据流拓扑」</b>，可以看到代码如何在不同楼层之间流动。
        </div>
      </div>
    </div>
  `;
  container.appendChild(insightCard);
}

// ============================================================
// 架构全景 — 健康仪表
// ============================================================
function renderHealth(container) {
  container.innerHTML = '<div class="view-title">健康仪表</div><div class="view-desc">五维雷达图 + 评分仪表盘 + 趋势曲线</div>';
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

  const factors = healthScore?.factors || { layer_balance: 70, file_granularity: 65, symbol_density: 80, dependency_depth: 60, interface_ratio: 75 };
  const names = { layer_balance: '层级平衡', file_granularity: '文件粒度', symbol_density: '符号密度', dependency_depth: '依赖深度', interface_ratio: '接口率' };
  const indicator = Object.keys(factors).map(k => ({ name: names[k] || k, max: 100 }));
  const values = Object.values(factors);

  charts.radar = echarts.init(document.getElementById('radarChart'));
  charts.radar.setOption({
    backgroundColor: 'transparent',
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
  const score = healthScore?.overall || 72;
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

  // Trend from healthHistory API data
  let trendData, trendLabels;
  if (healthHistory && healthHistory.length > 0) {
    trendData = healthHistory.map(h => {
      const s = h.score || h;
      return s.overall ?? s.score ?? s.value ?? 0;
    });
    trendLabels = healthHistory.map((_, i) => 'T-' + (healthHistory.length - i));
  } else {
    // Show empty state for trend
    trendData = [];
    trendLabels = [];
    const trendEl = document.getElementById('trendChart');
    if (trendEl) {
      trendEl.innerHTML = '<div class="empty-state" style="padding:40px"><p>暂无历史数据</p></div>';
      return;
    }
  }

  if (trendData.length === 0) {
    const trendEl = document.getElementById('trendChart');
    if (trendEl) {
      trendEl.innerHTML = '<div class="empty-state" style="padding:40px"><p>暂无历史数据</p></div>';
    }
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
