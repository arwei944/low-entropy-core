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
  var orange = style.getPropertyValue('--orange').trim();

  // --- Chart: Dead Code by Tier ---
  var chartDead = echarts.init(document.getElementById('chart-deadcode'), null, { renderer: 'svg' });
  chartDead.setOption({
    tooltip: { trigger: 'axis', appendToBody: true },
    legend: { bottom: 0, textStyle: { color: muted } },
    grid: { left: '3%', right: '4%', bottom: '12%', top: '5%', containLabel: true },
    xAxis: {
      type: 'category',
      data: ['L0', 'L1', 'L2', 'L3', 'L4', 'L5'],
      axisLabel: { color: muted },
      axisLine: { lineStyle: { color: rule } }
    },
    yAxis: {
      type: 'value',
      name: '行',
      axisLabel: { color: muted },
      axisLine: { lineStyle: { color: rule } },
      splitLine: { lineStyle: { color: rule } }
    },
    series: [
      {
        name: '死代码',
        type: 'bar',
        data: [3, 0, 216, 120, 1255, 579],
        itemStyle: { color: accent },
        barWidth: '40%',
        animation: false
      },
      {
        name: '活代码',
        type: 'bar',
        data: [2077, 349, 1320, 3160, 3865, 810],
        itemStyle: { color: green },
        barWidth: '40%',
        animation: false
      }
    ]
  });
  window.addEventListener('resize', function() { chartDead.resize(); });

  // --- Chart: Kernel File Sizes ---
  var chartKernel = echarts.init(document.getElementById('chart-kernel'), null, { renderer: 'svg' });
  chartKernel.setOption({
    tooltip: { trigger: 'axis', appendToBody: true,
      formatter: function(params) {
        return params[0].name + '<br/>' + params[0].value + ' 行';
      }
    },
    grid: { left: '3%', right: '4%', bottom: '5%', top: '5%', containLabel: true },
    xAxis: {
      type: 'category',
      data: ['atom.go', 'port.go', 'step.go', 'tier_check.go', 'types.go', 'complexity_profile.go', 'auto_detect.go', 'errors.go', 'observation.go', 'composer.go', 'perf_core.go'],
      axisLabel: { color: muted, rotate: 35, fontSize: 10 },
      axisLine: { lineStyle: { color: rule } }
    },
    yAxis: {
      type: 'value',
      name: '行',
      axisLabel: { color: muted },
      axisLine: { lineStyle: { color: rule } },
      splitLine: { lineStyle: { color: rule } }
    },
    series: [{
      type: 'bar',
      data: [
        { value: 6, itemStyle: { color: orange } },
        { value: 35, itemStyle: { color: green } },
        { value: 43, itemStyle: { color: green } },
        { value: 55, itemStyle: { color: green } },
        { value: 72, itemStyle: { color: green } },
        { value: 118, itemStyle: { color: green } },
        { value: 145, itemStyle: { color: green } },
        { value: 154, itemStyle: { color: green } },
        { value: 330, itemStyle: { color: green } },
        { value: 545, itemStyle: { color: green } },
        { value: 580, itemStyle: { color: orange } }
      ],
      barWidth: '60%',
      animation: false,
      label: { show: true, position: 'top', color: muted, fontSize: 10 }
    }]
  });
  window.addEventListener('resize', function() { chartKernel.resize(); });

  // --- Chart: Slimdown Stages ---
  var chartSlim = echarts.init(document.getElementById('chart-slimdown'), null, { renderer: 'svg' });
  chartSlim.setOption({
    tooltip: { trigger: 'axis', appendToBody: true },
    legend: { bottom: 0, textStyle: { color: muted } },
    grid: { left: '3%', right: '4%', bottom: '12%', top: '5%', containLabel: true },
    xAxis: {
      type: 'category',
      data: ['当前', 'P0 后', 'P0+P1 后', 'P0+P1+P2 后'],
      axisLabel: { color: muted },
      axisLine: { lineStyle: { color: rule } }
    },
    yAxis: {
      type: 'value',
      name: '代码行数',
      min: 10000,
      axisLabel: { color: muted },
      axisLine: { lineStyle: { color: rule } },
      splitLine: { lineStyle: { color: rule } }
    },
    series: [
      {
        name: '非测试代码',
        type: 'line',
        data: [14500, 13900, 12400, 11800],
        itemStyle: { color: accent },
        lineStyle: { color: accent, width: 3 },
        symbol: 'circle',
        symbolSize: 10,
        animation: false,
        markLine: {
          silent: true,
          symbol: 'none',
          lineStyle: { color: accent2, type: 'dashed', width: 1 },
          data: [{ yAxis: 11800, label: { formatter: '目标: 11,800', color: accent2, fontSize: 11 } }]
        }
      },
      {
        name: 'L0 内核',
        type: 'line',
        data: [2080, 2070, 1970, 1970],
        itemStyle: { color: accent2 },
        lineStyle: { color: accent2, width: 2 },
        symbol: 'diamond',
        symbolSize: 8,
        animation: false
      }
    ]
  });
  window.addEventListener('resize', function() { chartSlim.resize(); });
})();