(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg = style.getPropertyValue('--bg').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();

  var palette = [accent, accent2, muted, accent + '99', accent2 + '99'];

  // --- Chart 1: Phase Task Completion ---
  var chart1 = echarts.init(document.getElementById('chart-phase-tasks'), null, { renderer: 'svg' });
  chart1.setOption({
    animation: false,
    tooltip: {
      trigger: 'axis',
      appendToBody: true,
      axisPointer: { type: 'shadow' },
      backgroundColor: bg2,
      borderColor: rule,
      textStyle: { color: ink }
    },
    legend: {
      data: ['已完成', '任务总数'],
      textStyle: { color: muted },
      top: 0
    },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 40, containLabel: true },
    xAxis: {
      type: 'value',
      axisLabel: { color: muted },
      axisLine: { lineStyle: { color: rule } },
      splitLine: { lineStyle: { color: rule + '44' } }
    },
    yAxis: {
      type: 'category',
      data: ['P1 性能核心', 'P2 监督层细粒度', 'P3 观测层智能聚合', 'P4 依赖图+快照', 'P5 分布式韧性', 'P6 事件溯源升级', 'P7 测试验收'],
      axisLabel: { color: ink, fontSize: 12 },
      axisLine: { lineStyle: { color: rule } }
    },
    series: [
      {
        name: '已完成',
        type: 'bar',
        data: [12, 6, 5, 4, 4, 5, 4],
        itemStyle: { color: accent },
        barWidth: 14,
        label: { show: true, position: 'right', color: ink, fontSize: 11 }
      },
      {
        name: '任务总数',
        type: 'bar',
        data: [12, 6, 5, 4, 4, 5, 4],
        itemStyle: { color: rule },
        barWidth: 14,
        barGap: '0%',
        label: { show: true, position: 'right', color: muted, fontSize: 11 }
      }
    ]
  });
  window.addEventListener('resize', function() { chart1.resize(); });

  // --- Chart 2: Performance Comparison ---
  var chart2 = echarts.init(document.getElementById('chart-perf-compare'), null, { renderer: 'svg' });
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
      data: ['v3.0', 'v4.0'],
      textStyle: { color: muted },
      top: 0
    },
    grid: { left: '3%', right: '4%', bottom: '3%', top: 40, containLabel: true },
    xAxis: {
      type: 'category',
      data: ['并发写入\n(ops/s)', 'UUID生成\n(M/s)', '分位数查询\n(ns)', '事件存储\n(ops/s)', '内存占用\n(per step)'],
      axisLabel: { color: ink, fontSize: 11 },
      axisLine: { lineStyle: { color: rule } },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: muted },
      axisLine: { lineStyle: { color: rule } },
      splitLine: { lineStyle: { color: rule + '44' } }
    },
    series: [
      {
        name: 'v3.0',
        type: 'bar',
        data: [1, 0.5, 10000, 0.8, 400],
        itemStyle: { color: muted },
        barWidth: '35%',
        label: { show: false }
      },
      {
        name: 'v4.0',
        type: 'bar',
        data: [8, 5, 100, 6, 200],
        itemStyle: { color: accent },
        barWidth: '35%',
        label: { show: false }
      }
    ]
  });
  window.addEventListener('resize', function() { chart2.resize(); });

  // --- Chart 3: Module Distribution ---
  var chart3 = echarts.init(document.getElementById('chart-module-dist'), null, { renderer: 'svg' });
  chart3.setOption({
    animation: false,
    tooltip: {
      trigger: 'item',
      appendToBody: true,
      backgroundColor: bg2,
      borderColor: rule,
      textStyle: { color: ink },
      formatter: '{b}: {c} 文件 ({d}%)'
    },
    series: [
      {
        type: 'sunburst',
        data: [
          {
            name: '核心原语',
            itemStyle: { color: accent },
            children: [
              { name: 'Atom', value: 8, itemStyle: { color: accent + 'dd' } },
              { name: 'Port', value: 6, itemStyle: { color: accent + 'bb' } },
              { name: 'Adapter', value: 5, itemStyle: { color: accent + '99' } },
              { name: 'Composer', value: 4, itemStyle: { color: accent + '77' } }
            ]
          },
          {
            name: 'Guardian',
            itemStyle: { color: accent2 },
            children: [
              { name: '决策引擎', value: 3, itemStyle: { color: accent2 + 'dd' } },
              { name: '架构守卫', value: 3, itemStyle: { color: accent2 + 'bb' } },
              { name: '漂移检测', value: 2, itemStyle: { color: accent2 + '99' } },
              { name: '告警系统', value: 2, itemStyle: { color: accent2 + '77' } },
              { name: '模块熵', value: 2, itemStyle: { color: accent2 + '55' } }
            ]
          },
          {
            name: 'Observation',
            itemStyle: { color: '#5ce0d4' },
            children: [
              { name: '管道', value: 3, itemStyle: { color: '#5ce0d4dd' } },
              { name: '采样', value: 2, itemStyle: { color: '#5ce0d4bb' } },
              { name: '聚合', value: 2, itemStyle: { color: '#5ce0d499' } },
              { name: 'API', value: 1, itemStyle: { color: '#5ce0d477' } },
              { name: 'TDigest', value: 1, itemStyle: { color: '#5ce0d455' } }
            ]
          },
          {
            name: 'Patterns',
            itemStyle: { color: '#a78bfa' },
            children: [
              { name: '韧性', value: 3, itemStyle: { color: '#a78bfadd' } },
              { name: '分布式', value: 2, itemStyle: { color: '#a78bfabb' } },
              { name: '降级', value: 1, itemStyle: { color: '#a78bfa99' } }
            ]
          },
          {
            name: 'EventStore',
            itemStyle: { color: '#f59e0b' },
            children: [
              { name: '事件存储', value: 2, itemStyle: { color: '#f59e0bdd' } },
              { name: '事件总线', value: 2, itemStyle: { color: '#f59e0bbb' } },
              { name: '快照', value: 1, itemStyle: { color: '#f59e0b99' } },
              { name: '投影', value: 1, itemStyle: { color: '#f59e0b77' } }
            ]
          }
        ],
        radius: [0, '90%'],
        label: {
          color: '#fff',
          fontSize: 11
        },
        itemStyle: {
          borderColor: bg,
          borderWidth: 2
        }
      }
    ]
  });
  window.addEventListener('resize', function() { chart3.resize(); });
})();