// assets/charts.js
// Progressive Complexity Model — Charts
(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var dim = style.getPropertyValue('--dim').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();
  var green = style.getPropertyValue('--green').trim();
  var orange = style.getPropertyValue('--orange').trim();
  var red = style.getPropertyValue('--red').trim();

  // ============================================================
  // Chart 1: Tier Capability Stacked Bar
  // ============================================================
  var tierChart = echarts.init(document.getElementById('chart-tiers'), null, { renderer: 'svg' });

  var tierColors = ['#6b7280', '#6366f1', '#3b82f6', '#0ea5e9', '#10b981', '#f59e0b', '#f97316', '#ef4444'];

  var categories = ['L0\n原型', 'L1\n微服务', 'L2\n中型', 'L3\n大型', 'T4\n平台', 'T5\n企业', 'T6\n系统', 'T7\nWin级'];

  // Stacked capability layers
  var coreVal    = [1, 1, 1, 1, 1, 1, 1, 1];  // Core kernel always on
  var obsVal     = [0, 1, 1, 1, 1, 1, 1, 1];  // Observation
  var eventVal   = [0, 0, 1, 1, 1, 1, 1, 1];  // EventStore
  var guardVal   = [0, 0, 0, 1, 1, 1, 1, 1];  // Guardian basic
  var fullGV     = [0, 0, 0, 0, 1, 1, 1, 1];  // Guardian full
  var teamVal    = [0, 0, 0, 0, 0, 1, 1, 1];  // Multi-team
  var multiVal   = [0, 0, 0, 0, 0, 0, 1, 1];  // Multi-lang + distributed
  var extremeVal = [0, 0, 0, 0, 0, 0, 0, 1];  // NUMA + legacy

  tierChart.setOption({
    animation: false,
    backgroundColor: 'transparent',
    tooltip: {
      appendToBody: true,
      trigger: 'axis',
      axisPointer: { type: 'shadow' }
    },
    legend: {
      data: ['核心内核', '基础观测', '事件溯源', 'Guardian 基础', 'Guardian 完整', '多团队', '多语言+分布式', '百亿级+遗留'],
      bottom: 0,
      textStyle: { color: muted, fontSize: 11 }
    },
    grid: { left: 50, right: 20, top: 20, bottom: 60 },
    xAxis: {
      type: 'category',
      data: categories,
      axisLabel: { color: muted, fontSize: 11 },
      axisLine: { lineStyle: { color: rule } },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      name: '能力层级',
      nameTextStyle: { color: muted, fontSize: 11 },
      min: 0, max: 8,
      axisLabel: { color: dim, fontSize: 11 },
      splitLine: { lineStyle: { color: rule } },
      axisLine: { lineStyle: { color: rule } }
    },
    series: [
      { name: '核心内核', type: 'bar', stack: 'total', data: coreVal, itemStyle: { color: tierColors[0] }, barWidth: '50%' },
      { name: '基础观测', type: 'bar', stack: 'total', data: obsVal, itemStyle: { color: tierColors[1] } },
      { name: '事件溯源', type: 'bar', stack: 'total', data: eventVal, itemStyle: { color: tierColors[2] } },
      { name: 'Guardian 基础', type: 'bar', stack: 'total', data: guardVal, itemStyle: { color: tierColors[3] } },
      { name: 'Guardian 完整', type: 'bar', stack: 'total', data: fullGV, itemStyle: { color: tierColors[4] } },
      { name: '多团队', type: 'bar', stack: 'total', data: teamVal, itemStyle: { color: tierColors[5] } },
      { name: '多语言+分布式', type: 'bar', stack: 'total', data: multiVal, itemStyle: { color: tierColors[6] } },
      { name: '百亿级+遗留', type: 'bar', stack: 'total', data: extremeVal, itemStyle: { color: tierColors[7] } }
    ]
  });
  window.addEventListener('resize', function() { tierChart.resize(); });

  // ============================================================
  // Chart 2: Refactor Steps — Effort vs Impact
  // ============================================================
  var refactorChart = echarts.init(document.getElementById('chart-refactor'), null, { renderer: 'svg' });

  refactorChart.setOption({
    animation: false,
    backgroundColor: 'transparent',
    tooltip: {
      appendToBody: true,
      formatter: function(p) {
        return '<strong>' + p.data[3] + '</strong><br/>'
          + '投入: ' + p.data[0] + ' 天<br/>'
          + '影响力: ' + p.data[1] + '/10';
      }
    },
    grid: { left: 55, right: 80, top: 30, bottom: 40 },
    xAxis: {
      name: '投入 (天)',
      nameLocation: 'center',
      nameGap: 28,
      nameTextStyle: { color: muted, fontSize: 12 },
      type: 'value',
      min: 0, max: 16,
      axisLine: { lineStyle: { color: rule } },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: rule } },
      axisLabel: { color: dim, fontSize: 11 }
    },
    yAxis: {
      name: '影响力',
      nameLocation: 'center',
      nameGap: 35,
      nameTextStyle: { color: muted, fontSize: 12 },
      type: 'value',
      min: 0, max: 10,
      axisLine: { lineStyle: { color: rule } },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: rule } },
      axisLabel: { color: dim, fontSize: 11 }
    },
    series: [{
      type: 'scatter',
      symbolSize: function(data) { return data[2] * 3.5; },
      data: [
        [5, 9.5, 8, '提取核心内核', 'P0'],
        [5, 8, 7, '添加 Build Tags', 'P0'],
        [3, 7, 5, 'ComplexityTier 类型', 'P1'],
        [3, 6, 5, 'AutoDetect 扫描器', 'P1'],
        [10, 7.5, 6, '适配测试', 'P1'],
        [10, 5, 4, '文档与示例', 'P2']
      ],
      itemStyle: {
        color: function(p) {
          var pri = p.data[4];
          if (pri === 'P0') return accent;
          if (pri === 'P1') return accent2;
          return green;
        },
        opacity: 0.85
      },
      label: {
        show: true,
        formatter: function(p) { return p.data[3]; },
        position: 'right',
        fontSize: 11,
        color: muted
      },
      emphasis: { scale: 1.4, label: { fontSize: 13, fontWeight: 'bold' } }
    }]
  });
  window.addEventListener('resize', function() { refactorChart.resize(); });
})();