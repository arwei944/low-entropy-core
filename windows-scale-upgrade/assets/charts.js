// assets/charts.js
// Low-Entropy Core Windows-Scale Upgrade Analysis — Charts
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
  var purple = style.getPropertyValue('--purple').trim();

  // ============================================================
  // Chart 1: Capability Gap Radar
  // ============================================================
  var radarChart = echarts.init(document.getElementById('chart-radar'), null, { renderer: 'svg' });
  radarChart.setOption({
    animation: false,
    backgroundColor: 'transparent',
    tooltip: { appendToBody: true },
    legend: {
      data: ['当前能力', 'Windows 级目标'],
      bottom: 0,
      textStyle: { color: muted, fontSize: 12 }
    },
    radar: {
      center: ['50%', '52%'],
      radius: '68%',
      indicator: [
        { name: '多语言支持', max: 10 },
        { name: '分布式编排', max: 10 },
        { name: 'Guardian 监督', max: 10 },
        { name: '事件溯源', max: 10 },
        { name: '企业观测', max: 10 },
        { name: '多团队协作', max: 10 },
        { name: '遗留集成', max: 10 },
        { name: '安全合规', max: 10 },
        { name: '构建 CI/CD', max: 10 },
        { name: '百亿级性能', max: 10 },
        { name: '企业测试', max: 10 }
      ],
      axisName: { color: muted, fontSize: 11, borderRadius: 3, padding: [3, 5] },
      splitArea: { areaStyle: { color: [bg2, bg2] } },
      splitLine: { lineStyle: { color: rule } },
      axisLine: { lineStyle: { color: rule } }
    },
    series: [
      {
        type: 'radar',
        name: '当前能力',
        data: [{ value: [2, 3, 3, 5, 5, 0, 0, 3, 2, 6, 2], name: '当前能力' }],
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { color: accent, width: 2 },
        areaStyle: { color: accent + '33' },
        itemStyle: { color: accent }
      },
      {
        type: 'radar',
        name: 'Windows 级目标',
        data: [{ value: [10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10], name: 'Windows 级目标' }],
        symbol: 'circle',
        symbolSize: 6,
        lineStyle: { color: accent2, width: 2, type: 'dashed' },
        areaStyle: { color: 'transparent' },
        itemStyle: { color: accent2 }
      }
    ]
  });
  window.addEventListener('resize', function() { radarChart.resize(); });

  // ============================================================
  // Chart 2: Effort vs Impact Bubble Chart
  // ============================================================
  var effortChart = echarts.init(document.getElementById('chart-effort'), null, { renderer: 'svg' });
  effortChart.setOption({
    animation: false,
    backgroundColor: 'transparent',
    tooltip: {
      appendToBody: true,
      formatter: function(p) {
        return '<strong>' + p.data[3] + '</strong><br/>'
          + '投入: ' + p.data[0] + ' 人月<br/>'
          + '影响力: ' + p.data[1] + '/10<br/>'
          + 'Phase: ' + p.data[4];
      }
    },
    grid: { left: 60, right: 30, top: 30, bottom: 50 },
    xAxis: {
      name: '投入 (人月)',
      nameLocation: 'center',
      nameGap: 35,
      nameTextStyle: { color: muted, fontSize: 12 },
      type: 'value',
      min: 0,
      max: 14,
      axisLine: { lineStyle: { color: rule } },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: rule } },
      axisLabel: { color: dim, fontSize: 11 }
    },
    yAxis: {
      name: '影响力',
      nameLocation: 'center',
      nameGap: 40,
      nameTextStyle: { color: muted, fontSize: 12 },
      type: 'value',
      min: 0,
      max: 10,
      axisLine: { lineStyle: { color: rule } },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: rule } },
      axisLabel: { color: dim, fontSize: 11 }
    },
    series: [
      {
        type: 'scatter',
        symbolSize: function(data) { return data[2] * 4; },
        data: [
          [9, 9.5, 8, '多语言 IDL', 'P1'],
          [11, 9, 7, '分布式编排', 'P1'],
          [7, 8.5, 6, '事件溯源升级', 'P1'],
          [5, 8, 5, '企业观测', 'P1'],
          [9, 9, 7, '企业 Guardian', 'P2'],
          [7, 8, 6, '多团队协作', 'P2'],
          [7, 7, 5, '构建系统', 'P2'],
          [5, 6.5, 4, '安全合规', 'P2'],
          [7, 7.5, 6, '遗留集成', 'P3'],
          [7, 7, 5, '百亿级性能', 'P3'],
          [5, 6, 4, '企业测试', 'P3']
        ],
        itemStyle: {
          color: function(p) {
            var phase = p.data[4];
            if (phase === 'P1') return accent;
            if (phase === 'P2') return accent2;
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
        emphasis: {
          scale: 1.5,
          label: { fontSize: 13, fontWeight: 'bold' }
        }
      }
    ]
  });
  window.addEventListener('resize', function() { effortChart.resize(); });
})();