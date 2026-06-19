(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var green = style.getPropertyValue('--green').trim();
  var orange = style.getPropertyValue('--orange').trim();

  var chartCompare = echarts.init(document.getElementById('chart-compare'), null, { renderer: 'svg' });
  chartCompare.setOption({
    tooltip: { trigger: 'axis', appendToBody: true },
    legend: { bottom: 0, textStyle: { color: muted } },
    grid: { left: '3%', right: '4%', bottom: '12%', top: '5%', containLabel: true },
    xAxis: {
      type: 'category',
      data: ['原方案声称', '修正后实际', '实际保留'],
      axisLabel: { color: muted },
      axisLine: { lineStyle: { color: rule } }
    },
    yAxis: {
      type: 'value',
      name: '可删除行数',
      axisLabel: { color: muted },
      axisLine: { lineStyle: { color: rule } },
      splitLine: { lineStyle: { color: rule } }
    },
    series: [
      {
        name: '安全删除',
        type: 'bar',
        data: [630, 630, 0],
        itemStyle: { color: green },
        barWidth: '30%',
        animation: false,
        label: { show: true, position: 'top', color: green, fontSize: 11, formatter: '{c}' }
      },
      {
        name: '精简保留',
        type: 'bar',
        data: [0, 470, 0],
        itemStyle: { color: accent2 },
        barWidth: '30%',
        animation: false,
        label: { show: true, position: 'top', color: accent2, fontSize: 11, formatter: '{c}' }
      },
      {
        name: '误判项（无法删除）',
        type: 'bar',
        data: [1570, 0, 1570],
        itemStyle: { color: accent },
        barWidth: '30%',
        animation: false,
        label: { show: true, position: 'top', color: accent, fontSize: 11, formatter: '{c}' }
      }
    ]
  });
  window.addEventListener('resize', function() { chartCompare.resize(); });
})();