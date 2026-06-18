(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg = style.getPropertyValue('--bg').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();
  var success = style.getPropertyValue('--success').trim();
  var warn = style.getPropertyValue('--warn').trim();
  var danger = style.getPropertyValue('--danger').trim();

  // --- Chart 1: Scaling Heatmap ---
  var chart1 = echarts.init(document.getElementById('chart-scaling-heatmap'), null, { renderer: 'svg' });
  var yData = ['ShardedLock', 'AtomicState', 'BatchedUUIDGen', 'TDigest', 'ShardedEventStore', 'DependencyGraph', 'ShardedObs', 'ModuleEntropy', 'DistRateLimiter', 'CircuitBreaker', 'ResilienceChain'];
  var xData = ['1M ops/s', '10M ops/s', '100M ops/s', '1B ops/s'];
  var heatData = [
    [0, 0, 3], [0, 1, 2], [0, 2, 1], [0, 3, 0],   // ShardedLock: green → yellow → yellow → red (hot key)
    [1, 0, 3], [1, 1, 3], [1, 2, 3], [1, 3, 3],    // AtomicState: green all the way
    [2, 0, 3], [2, 1, 2], [2, 2, 0], [2, 3, 0],    // BatchedUUIDGen: green → yellow → red → red
    [3, 0, 3], [3, 1, 3], [3, 2, 2], [3, 3, 1],    // TDigest: green → green → yellow → yellow (merge)
    [4, 0, 3], [4, 1, 2], [4, 2, 0], [4, 3, 0],    // ShardedEventStore: green → yellow → red → red (memory)
    [5, 0, 2], [5, 1, 1], [5, 2, 0], [5, 3, 0],    // DependencyGraph: yellow → yellow → red → red (O(V³))
    [6, 0, 3], [6, 1, 1], [6, 2, 0], [6, 3, 0],    // ShardedObs: green → yellow → red → red (ring buffer)
    [7, 0, 3], [7, 1, 3], [7, 2, 2], [7, 3, 1],    // ModuleEntropy: green → green → yellow → yellow
    [8, 0, 3], [8, 1, 3], [8, 2, 2], [8, 3, 1],    // DistRateLimiter: green → green → yellow → yellow
    [9, 0, 3], [9, 1, 3], [9, 2, 2], [9, 3, 2],    // CircuitBreaker: green → green → yellow → yellow
    [10, 0, 3], [10, 1, 3], [10, 2, 3], [10, 3, 2] // ResilienceChain: green → green → green → yellow
  ];
  chart1.setOption({
    animation: false,
    tooltip: {
      appendToBody: true,
      backgroundColor: bg2,
      borderColor: rule,
      textStyle: { color: ink },
      formatter: function(p) {
        var levels = ['瓶颈/不可用', '需优化/降级', '可接受', '理想'];
        return p.value[1] + ' × ' + p.value[0] + '<br/>状态: <strong>' + levels[3 - p.value[2]] + '</strong>';
      }
    },
    grid: { left: '15%', right: '5%', bottom: '10%', top: 30 },
    xAxis: {
      type: 'category',
      data: xData,
      position: 'bottom',
      axisLabel: { color: ink, fontSize: 12, fontWeight: 700 },
      axisLine: { lineStyle: { color: rule } },
      splitArea: { show: false }
    },
    yAxis: {
      type: 'category',
      data: yData,
      axisLabel: { color: ink, fontSize: 11, fontFamily: 'JetBrains Mono, monospace' },
      axisLine: { lineStyle: { color: rule } },
      splitArea: { show: false }
    },
    visualMap: {
      show: false,
      min: 0,
      max: 3,
      inRange: { color: [danger, warn, success + 'aa', success] }
    },
    series: [{
      type: 'heatmap',
      data: heatData,
      label: {
        show: true,
        fontSize: 11,
        fontWeight: 700,
        formatter: function(p) {
          var labels = ['✗', '!', '~', '✓'];
          return labels[3 - p.value[2]];
        },
        color: ink
      },
      emphasis: { itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.5)' } }
    }]
  });
  window.addEventListener('resize', function() { chart1.resize(); });

  // --- Chart 2: Complexity Growth Comparison ---
  var chart2 = echarts.init(document.getElementById('chart-complexity-growth'), null, { renderer: 'svg' });
  var dims = ['并发竞争', '内存管理', '可观测性', '依赖管理', '变更风险', '测试覆盖', '部署复杂度', '故障定位'];
  chart2.setOption({
    animation: false,
    tooltip: {
      trigger: 'axis',
      appendToBody: true,
      backgroundColor: bg2,
      borderColor: rule,
      textStyle: { color: ink }
    },
    legend: {
      data: ['传统架构 百万行', '传统架构 亿级', 'Low-Entropy v4.0 百万行', 'Low-Entropy v4.0 亿级'],
      textStyle: { color: muted, fontSize: 11 },
      top: 0,
      type: 'scroll'
    },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 60, containLabel: true },
    xAxis: {
      type: 'category',
      data: dims,
      axisLabel: { color: ink, fontSize: 11 },
      axisLine: { lineStyle: { color: rule } },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      name: '相对复杂度',
      max: 10,
      axisLabel: { color: muted },
      axisLine: { lineStyle: { color: rule } },
      splitLine: { lineStyle: { color: rule + '44' } },
      nameTextStyle: { color: muted }
    },
    series: [
      {
        name: '传统架构 百万行',
        type: 'bar',
        data: [3, 2, 2, 3, 4, 3, 2, 3],
        itemStyle: { color: muted + '88' },
        barWidth: '18%',
        barGap: '10%'
      },
      {
        name: '传统架构 亿级',
        type: 'bar',
        data: [9, 8, 9, 9, 10, 9, 8, 10],
        itemStyle: { color: danger }
      },
      {
        name: 'Low-Entropy v4.0 百万行',
        type: 'bar',
        data: [1, 1, 1, 1, 2, 1, 1, 1],
        itemStyle: { color: accent + '88' }
      },
      {
        name: 'Low-Entropy v4.0 亿级',
        type: 'bar',
        data: [2, 2, 2, 3, 3, 2, 2, 2],
        itemStyle: { color: accent }
      }
    ]
  });
  window.addEventListener('resize', function() { chart2.resize(); });

  // --- Chart 3: Bottleneck Radar ---
  var chart3 = echarts.init(document.getElementById('chart-bottleneck-radar'), null, { renderer: 'svg' });
  chart3.setOption({
    animation: false,
    tooltip: {
      appendToBody: true,
      backgroundColor: bg2,
      borderColor: rule,
      textStyle: { color: ink }
    },
    legend: {
      data: ['百万行', '亿级', '十亿级'],
      textStyle: { color: muted },
      top: 0
    },
    radar: {
      center: ['50%', '55%'],
      radius: '65%',
      indicator: [
        { name: '并发竞争', max: 10 },
        { name: '内存压力', max: 10 },
        { name: 'ID 生成', max: 10 },
        { name: '事件存储', max: 10 },
        { name: '依赖分析', max: 10 },
        { name: '观测覆盖', max: 10 }
      ],
      axisName: { color: ink, fontSize: 11 },
      splitArea: { areaStyle: { color: [bg2, bg] } },
      splitLine: { lineStyle: { color: rule } },
      axisLine: { lineStyle: { color: rule } }
    },
    series: [{
      type: 'radar',
      data: [
        { value: [1, 1, 1, 1, 2, 2], name: '百万行', areaStyle: { color: success + '33' }, lineStyle: { color: success }, itemStyle: { color: success } },
        { value: [3, 4, 5, 5, 6, 4], name: '亿级', areaStyle: { color: warn + '33' }, lineStyle: { color: warn }, itemStyle: { color: warn } },
        { value: [5, 8, 9, 9, 9, 7], name: '十亿级', areaStyle: { color: danger + '33' }, lineStyle: { color: danger }, itemStyle: { color: danger } }
      ]
    }]
  });
  window.addEventListener('resize', function() { chart3.resize(); });
})();