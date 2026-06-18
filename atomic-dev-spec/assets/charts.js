// assets/charts.js — Charts for atomic-dev-spec.html
(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();
  var l0 = style.getPropertyValue('--l0').trim();
  var l1 = style.getPropertyValue('--l1').trim();
  var l2 = style.getPropertyValue('--l2').trim();
  var l3 = style.getPropertyValue('--l3').trim();
  var l4 = style.getPropertyValue('--l4').trim();
  var l5 = style.getPropertyValue('--l5').trim();
  var l6 = style.getPropertyValue('--l6').trim();
  var l7 = style.getPropertyValue('--l7').trim();

  var layerColors = [l0, l1, l2, l3, l4, l5, l6, l7];
  var layerNames = [
    'L0 核心基础', 'L1 四原语定义', 'L2 单机韧性', 'L3 分布式韧性',
    'L4 Guardian', 'L5 Observation', 'L6 EventStore', 'L7 应用层'
  ];

  // --- Chart 1: Symbol Kind Distribution ---
  var chart1 = echarts.init(document.getElementById('chart-symbol-kinds'), null, { renderer: 'svg' });
  chart1.setOption({
    animation: false,
    tooltip: {
      trigger: 'item',
      appendToBody: true,
      backgroundColor: bg2,
      borderColor: rule,
      textStyle: { color: ink, fontSize: 12 }
    },
    legend: {
      orient: 'vertical',
      right: 10,
      top: 'center',
      textStyle: { color: muted, fontSize: 11 }
    },
    series: [{
      type: 'pie',
      radius: ['50%', '80%'],
      center: ['40%', '50%'],
      label: { show: false },
      emphasis: { label: { show: true, fontSize: 16, fontWeight: 'bold' } },
      data: [
        { value: 184, name: 'type', itemStyle: { color: '#58a6ff' } },
        { value: 176, name: 'func', itemStyle: { color: '#d2a8ff' } },
        { value: 310, name: 'method', itemStyle: { color: '#a371f7' } },
        { value: 16, name: 'interface', itemStyle: { color: '#3fb950' } },
        { value: 53, name: 'const', itemStyle: { color: '#f0883e' } },
        { value: 18, name: 'var', itemStyle: { color: '#e06c75' } },
        { value: 6, name: 'func-type', itemStyle: { color: '#56d364' } },
        { value: 19, name: 'type-alias', itemStyle: { color: '#a5d6ff' } }
      ]
    }]
  });

  // --- Chart 2: Layer Distribution ---
  var chart2 = echarts.init(document.getElementById('chart-layer-dist'), null, { renderer: 'svg' });
  chart2.setOption({
    animation: false,
    tooltip: {
      trigger: 'axis',
      appendToBody: true,
      backgroundColor: bg2,
      borderColor: rule,
      textStyle: { color: ink, fontSize: 12 }
    },
    legend: {
      data: ['文件数', '行数(x100)', '符号数'],
      textStyle: { color: muted, fontSize: 11 },
      top: 0
    },
    grid: { left: 50, right: 20, top: 30, bottom: 30 },
    xAxis: {
      type: 'category',
      data: layerNames,
      axisLabel: { color: muted, fontSize: 10, rotate: 30 },
      axisLine: { lineStyle: { color: rule } }
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: muted, fontSize: 10 },
      splitLine: { lineStyle: { color: rule } }
    },
    series: [
      {
        name: '文件数', type: 'bar',
        data: [6, 5, 2, 1, 4, 6, 7, 9].map(function(v, i) {
          return { value: v, itemStyle: { color: layerColors[i] } };
        }),
        barWidth: '25%'
      },
      {
        name: '行数(x100)', type: 'bar',
        data: [21.88, 6.76, 6.36, 5.52, 21.35, 17.94, 14.80, 30.14].map(function(v, i) {
          return { value: v, itemStyle: { color: layerColors[i], opacity: 0.5 } };
        }),
        barWidth: '25%'
      },
      {
        name: '符号数', type: 'line',
        data: [139, 49, 39, 39, 127, 91, 95, 227],
        lineStyle: { color: accent, width: 2 },
        itemStyle: { color: accent },
        symbol: 'circle', symbolSize: 6
      }
    ]
  });

  window.addEventListener('resize', function() {
    chart1.resize();
    chart2.resize();
  });
})();