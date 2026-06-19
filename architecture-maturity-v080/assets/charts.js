(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();
  var green = '#34d399';
  var red = '#ef4444';
  var purple = '#a78bfa';

  // --- Chart 1: Radar ---
  var radar = echarts.init(document.getElementById('chart-radar'), null, { renderer: 'svg' });
  radar.setOption({
    animation: false,
    backgroundColor: 'transparent',
    radar: {
      center: ['50%', '55%'],
      radius: '65%',
      indicator: [
        { name: '架构设计', max: 5 },
        { name: '实现质量', max: 5 },
        { name: '测试覆盖', max: 5 },
        { name: '可观测性', max: 5 },
        { name: '安全性', max: 5 },
        { name: '文档完备', max: 5 },
        { name: '生产就绪', max: 5 }
      ],
      axisName: { color: muted, fontSize: 12 },
      splitArea: { areaStyle: { color: ['transparent', 'rgba(56,189,248,0.03)'] } },
      splitLine: { lineStyle: { color: rule } },
      axisLine: { lineStyle: { color: rule } }
    },
    series: [{
      type: 'radar',
      data: [{
        value: [5.0, 4.0, 3.5, 2.5, 2.0, 3.0, 2.0],
        name: 'v0.8.0',
        areaStyle: { color: accent + '33' },
        lineStyle: { color: accent, width: 2 },
        itemStyle: { color: accent },
        symbol: 'circle',
        symbolSize: 6
      }]
    }],
    tooltip: { appendToBody: true }
  });
  window.addEventListener('resize', function() { radar.resize(); });

  // --- Chart 2: Bar Chart ---
  var bars = echarts.init(document.getElementById('chart-bars'), null, { renderer: 'svg' });
  var scores = [4.5, 2.5, 3.5, 4.0, 3.5, 2.5, 0.0, 0.0];
  var colors = scores.map(function(s) {
    if (s >= 4.0) return green;
    if (s >= 3.0) return accent;
    if (s >= 2.0) return accent2;
    return red;
  });
  bars.setOption({
    animation: false,
    backgroundColor: 'transparent',
    grid: { left: '8%', right: '8%', top: '8%', bottom: '8%' },
    xAxis: {
      type: 'category',
      data: ['L0\n内核', 'L1\n微服务', 'L2\n中型服务', 'L3\n大型服务', 'L4\n平台', 'L5\n企业平台', 'L6\n全球分布', 'L7\n联邦自治'],
      axisLabel: { color: muted, fontSize: 11 },
      axisLine: { lineStyle: { color: rule } },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: 5,
      interval: 1,
      axisLabel: { color: muted, fontSize: 11 },
      splitLine: { lineStyle: { color: rule } },
      name: '成熟度评分',
      nameTextStyle: { color: muted, fontSize: 11 }
    },
    series: [{
      type: 'bar',
      data: scores.map(function(s, i) {
        return { value: s, itemStyle: { color: colors[i], borderRadius: [4, 4, 0, 0] } };
      }),
      barWidth: '50%',
      label: {
        show: true,
        position: 'top',
        color: ink,
        fontSize: 12,
        fontWeight: 'bold',
        formatter: function(p) { return p.value > 0 ? p.value.toFixed(1) : ''; }
      }
    }],
    tooltip: {
      appendToBody: true,
      formatter: function(p) { return p.name + ': <b>' + p.value.toFixed(1) + '</b> / 5.0'; }
    }
  });
  window.addEventListener('resize', function() { bars.resize(); });

  // --- Mermaid ---
  mermaid.initialize({ startOnLoad: true, theme: 'neutral', securityLevel: 'loose' });
})();