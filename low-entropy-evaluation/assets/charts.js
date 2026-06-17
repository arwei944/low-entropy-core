// assets/charts.js - Low-Entropy Core Evaluation Charts
(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();
  var green = style.getPropertyValue('--green').trim();
  var amber = style.getPropertyValue('--amber').trim();
  var red = style.getPropertyValue('--red').trim();

  // --- Radar Chart: Architecture Maturity ---
  var radarDom = document.getElementById('chart-radar');
  if (radarDom) {
    var radarChart = echarts.init(radarDom, null, { renderer: 'svg' });
    radarChart.setOption({
      animation: false,
      radar: {
        center: ['50%', '55%'],
        radius: '70%',
        indicator: [
          { name: '类型安全', max: 10 },
          { name: '错误处理', max: 10 },
          { name: '测试覆盖', max: 10 },
          { name: '可观测性', max: 10 },
          { name: '并发安全', max: 10 },
          { name: '分布式支持', max: 10 },
          { name: 'API 设计', max: 10 },
          { name: '文档质量', max: 10 },
          { name: 'CI/CD', max: 10 }
        ],
        axisName: { color: muted, fontSize: 11 },
        splitArea: {
          areaStyle: { color: [bg2, bg2, bg2, bg2, bg2] }
        },
        splitLine: { lineStyle: { color: rule } },
        axisLine: { lineStyle: { color: rule } }
      },
      series: [{
        type: 'radar',
        data: [
          {
            value: [2, 3, 0, 5, 3, 1, 5, 7, 4],
            name: '当前状态',
            areaStyle: { color: accent + '33' },
            lineStyle: { color: accent, width: 2 },
            itemStyle: { color: accent },
            symbol: 'circle',
            symbolSize: 6
          },
          {
            value: [7, 7, 8, 8, 7, 6, 7, 8, 7],
            name: '目标状态 (v2.0)',
            areaStyle: { color: 'transparent' },
            lineStyle: { color: accent2, width: 2, type: 'dashed' },
            itemStyle: { color: accent2 },
            symbol: 'diamond',
            symbolSize: 6
          }
        ]
      }],
      legend: {
        bottom: 10,
        textStyle: { color: muted, fontSize: 12 },
        data: ['当前状态', '目标状态 (v2.0)']
      },
      tooltip: {
        appendToBody: true,
        backgroundColor: bg2,
        borderColor: rule,
        textStyle: { color: ink }
      }
    });
    window.addEventListener('resize', function() { radarChart.resize(); });
  }
})();