(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();

  // --- Chart: Star Comparison ---
  var chart1 = echarts.init(document.getElementById('chart-stars'), null, { renderer: 'svg' });
  var projects = ['semantic-release', 'goreleaser', 'commitlint', 'changesets', 'standard-version', 'conventional-changelog', 'release-please', 'svu'];
  var stars = [22000, 14000, 17000, 9000, 9000, 8000, 5000, 800];
  var colors = [accent, accent2, accent, accent2, accent, accent2, accent, accent2];
  chart1.setOption({
    animation: false,
    tooltip: { trigger: 'axis', appendToBody: true },
    grid: { left: 160, right: 40, top: 20, bottom: 30 },
    xAxis: { type: 'value', name: 'GitHub Stars', nameTextStyle: { color: muted }, axisLabel: { color: muted }, splitLine: { lineStyle: { color: rule } } },
    yAxis: { type: 'category', data: projects.reverse(), axisLabel: { color: ink, fontSize: 12 }, axisLine: { lineStyle: { color: rule } } },
    series: [{
      type: 'bar',
      data: stars.reverse().map(function(v, i) { return { value: v, itemStyle: { color: colors.reverse()[i], borderRadius: [0, 4, 4, 0] } }; }),
      label: { show: true, position: 'right', color: muted, fontSize: 11, formatter: function(p) { return (p.value/1000).toFixed(1) + 'k'; } }
    }]
  });
  window.addEventListener('resize', function() { chart1.resize(); });

  // --- Chart: Language Distribution ---
  var chart2 = echarts.init(document.getElementById('chart-lang'), null, { renderer: 'svg' });
  chart2.setOption({
    animation: false,
    tooltip: { trigger: 'item', appendToBody: true },
    legend: { orient: 'vertical', left: 'left', textStyle: { color: muted, fontSize: 11 } },
    series: [{
      type: 'pie',
      radius: ['45%', '75%'],
      center: ['58%', '50%'],
      label: { color: muted, fontSize: 10 },
      itemStyle: { borderColor: 'var(--bg)', borderWidth: 2 },
      data: [
        { name: 'TypeScript/JavaScript', value: 6, itemStyle: { color: '#3178c6' } },
        { name: 'Go', value: 2, itemStyle: { color: '#00ADD8' } },
        { name: 'Shell', value: 1, itemStyle: { color: '#89e051' } }
      ]
    }],
    color: [accent, accent2, muted]
  });
  window.addEventListener('resize', function() { chart2.resize(); });

  // --- Chart: Integration Complexity vs Value ---
  var chart3 = echarts.init(document.getElementById('chart-bubble'), null, { renderer: 'svg' });
  var bubbleData = [
    { name: 'semantic-release', value: [7, 8, 22], color: accent },
    { name: 'goreleaser', value: [3, 9, 14], color: accent2 },
    { name: 'release-please', value: [4, 10, 5], color: '#0a84ff' },
    { name: 'changesets', value: [5, 8, 9], color: '#ff9f0a' },
    { name: 'standard-version', value: [2, 6, 9], color: '#af52de' },
    { name: 'commitlint', value: [1, 5, 17], color: '#ff375f' },
    { name: 'conventional-changelog', value: [3, 7, 8], color: '#5e5ce6' },
    { name: 'svu', value: [1, 4, 0.8], color: '#64d2ff' }
  ];
  chart3.setOption({
    animation: false,
    tooltip: {
      trigger: 'item',
      appendToBody: true,
      formatter: function(p) { return p.name + '<br/>集成复杂度: ' + p.value[0] + '/10<br/>集成价值: ' + p.value[1] + '/10<br/>Stars: ' + p.value[2] + 'k'; }
    },
    grid: { left: 60, right: 30, top: 20, bottom: 50 },
    xAxis: { name: '集成复杂度 (越低越好)', nameTextStyle: { color: muted }, min: 0, max: 10, axisLabel: { color: muted }, splitLine: { lineStyle: { color: rule } } },
    yAxis: { name: '集成价值', nameTextStyle: { color: muted }, min: 0, max: 12, axisLabel: { color: muted }, splitLine: { lineStyle: { color: rule } } },
    series: [{
      type: 'scatter',
      data: bubbleData,
      symbolSize: function(d) { return Math.max(12, d[2] * 2); },
      label: { show: true, position: 'right', formatter: function(p) { return p.name; }, color: ink, fontSize: 10 },
      itemStyle: { borderColor: 'var(--bg)', borderWidth: 1, opacity: 0.85 }
    }]
  });
  window.addEventListener('resize', function() { chart3.resize(); });

  // --- Chart: Feature Coverage Radar ---
  var chart4 = echarts.init(document.getElementById('chart-radar'), null, { renderer: 'svg' });
  chart4.setOption({
    animation: false,
    tooltip: { trigger: 'item', appendToBody: true },
    legend: { bottom: 0, textStyle: { color: muted, fontSize: 10 } },
    radar: {
      center: ['50%', '48%'],
      radius: '65%',
      indicator: [
        { name: '自动版本号', max: 5 },
        { name: 'Changelog', max: 5 },
        { name: 'Git Tag', max: 5 },
        { name: 'CI 集成', max: 5 },
        { name: 'Monorepo', max: 5 },
        { name: 'Go 原生', max: 5 }
      ],
      axisName: { color: muted },
      splitArea: { areaStyle: { color: ['transparent', bg2 + '66'] } }
    },
    series: [
      {
        type: 'radar',
        name: 'semantic-release',
        data: [{ value: [5, 5, 5, 5, 3, 0], name: 'semantic-release' }],
        itemStyle: { color: accent },
        lineStyle: { color: accent },
        areaStyle: { color: accent + '22' }
      },
      {
        type: 'radar',
        name: 'release-please',
        data: [{ value: [5, 5, 5, 5, 5, 0], name: 'release-please' }],
        itemStyle: { color: '#0a84ff' },
        lineStyle: { color: '#0a84ff' },
        areaStyle: { color: '#0a84ff22' }
      },
      {
        type: 'radar',
        name: 'goreleaser',
        data: [{ value: [3, 3, 5, 5, 2, 5], name: 'goreleaser' }],
        itemStyle: { color: accent2 },
        lineStyle: { color: accent2 },
        areaStyle: { color: accent2 + '22' }
      },
      {
        type: 'radar',
        name: 'changesets',
        data: [{ value: [4, 5, 4, 4, 5, 0], name: 'changesets' }],
        itemStyle: { color: '#ff9f0a' },
        lineStyle: { color: '#ff9f0a' },
        areaStyle: { color: '#ff9f0a22' }
      }
    ]
  });
  window.addEventListener('resize', function() { chart4.resize(); });
})();