// assets/charts.js — Low-Entropy Core v4.0 Architecture Dashboard Charts
(function() {
  var style = getComputedStyle(document.documentElement);
  var accent  = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var accent3 = style.getPropertyValue('--accent3').trim();
  var accent4 = style.getPropertyValue('--accent4').trim();
  var accent5 = style.getPropertyValue('--accent5').trim();
  var accent6 = style.getPropertyValue('--accent6').trim();
  var accent7 = style.getPropertyValue('--accent7').trim();
  var accent8 = style.getPropertyValue('--accent8').trim();
  var ink     = style.getPropertyValue('--ink').trim();
  var muted   = style.getPropertyValue('--muted').trim();
  var rule    = style.getPropertyValue('--rule').trim();
  var bg2     = style.getPropertyValue('--bg2').trim();

  var layerColors = ['#7f8ea3', accent, accent2, accent8, accent4, accent6, accent7, accent3];
  var layers = ['L0 性能基础设施', 'L1 四原语定义', 'L2-L3 韧性', 'L4 Guardian 监督', 'L5 Observation 可观测', 'L6 EventStore 事件溯源', 'L7 应用层'];

  // --- Chart 1: Layers — Files vs Lines ---
  (function() {
    var el = document.getElementById('chart-layers');
    if (!el) return;
    var c = echarts.init(el, null, { renderer: 'svg' });
    c.setOption({
      animation: false,
      tooltip: { trigger: 'axis', appendToBody: true },
      legend: {
        data: ['文件数', '代码行数'],
        textStyle: { color: muted },
        top: 0
      },
      grid: { left: '3%', right: '4%', bottom: '3%', top: '15%', containLabel: true },
      xAxis: {
        type: 'category',
        data: layers,
        axisLabel: { color: muted, fontSize: 11, rotate: 20 },
        axisLine: { lineStyle: { color: rule } },
        axisTick: { show: false }
      },
      yAxis: [
        {
          type: 'value', name: '文件数',
          nameTextStyle: { color: muted },
          axisLabel: { color: muted },
          splitLine: { lineStyle: { color: rule } },
          axisLine: { lineStyle: { color: rule } }
        },
        {
          type: 'value', name: '代码行数',
          nameTextStyle: { color: muted },
          axisLabel: { color: muted },
          splitLine: { show: false },
          axisLine: { lineStyle: { color: rule } }
        }
      ],
      series: [
        {
          name: '文件数', type: 'bar',
          data: [6, 5, 3, 4, 6, 7, 9],
          itemStyle: { color: accent },
          barWidth: '40%'
        },
        {
          name: '代码行数', type: 'line', yAxisIndex: 1,
          data: [1939, 574, 1037, 1876, 1537, 1296, 2597],
          itemStyle: { color: accent2 },
          lineStyle: { width: 2 },
          symbol: 'circle',
          symbolSize: 8
        }
      ]
    });
    window.addEventListener('resize', function() { c.resize(); });
  })();

  // --- Chart 2: Pie — Code distribution by layer ---
  (function() {
    var el = document.getElementById('chart-pie');
    if (!el) return;
    var c = echarts.init(el, null, { renderer: 'svg' });
    var pieData = [
      { value: 1939, name: 'L0 性能基础设施' },
      { value: 574,  name: 'L1 四原语定义' },
      { value: 1037, name: 'L2-L3 韧性' },
      { value: 1876, name: 'L4 Guardian 监督' },
      { value: 1537, name: 'L5 Observation 可观测' },
      { value: 1296, name: 'L6 EventStore 事件溯源' },
      { value: 2597, name: 'L7 应用层' }
    ];
    c.setOption({
      animation: false,
      tooltip: { trigger: 'item', appendToBody: true, formatter: '{b}: {c} 行 ({d}%)' },
      color: layerColors,
      series: [{
        name: '代码行数', type: 'pie',
        radius: ['45%', '75%'],
        center: ['50%', '50%'],
        label: { color: ink, fontSize: 12 },
        labelLine: { lineStyle: { color: rule } },
        data: pieData,
        emphasis: {
          label: { fontSize: 16, fontWeight: 'bold' }
        }
      }]
    });
    window.addEventListener('resize', function() { c.resize(); });
  })();

  // --- Chart 3: Bar — Average lines per file per layer ---
  (function() {
    var el = document.getElementById('chart-avg');
    if (!el) return;
    var c = echarts.init(el, null, { renderer: 'svg' });
    var avgLines = [323, 115, 346, 469, 256, 185, 289];
    c.setOption({
      animation: false,
      tooltip: { trigger: 'axis', appendToBody: true, formatter: '{b}: {c} 行/文件' },
      grid: { left: '3%', right: '4%', bottom: '3%', top: '8%', containLabel: true },
      xAxis: {
        type: 'category',
        data: layers,
        axisLabel: { color: muted, fontSize: 11, rotate: 25 },
        axisLine: { lineStyle: { color: rule } },
        axisTick: { show: false }
      },
      yAxis: {
        type: 'value', name: '行/文件',
        nameTextStyle: { color: muted },
        axisLabel: { color: muted },
        splitLine: { lineStyle: { color: rule } },
        axisLine: { lineStyle: { color: rule } }
      },
      series: [{
        type: 'bar',
        data: avgLines.map(function(v, i) {
          return { value: v, itemStyle: { color: layerColors[i] } };
        }),
        barWidth: '50%',
        label: {
          show: true, position: 'top',
          color: ink, fontSize: 12, fontWeight: 600,
          formatter: '{c}'
        }
      }]
    });
    window.addEventListener('resize', function() { c.resize(); });
  })();
})();