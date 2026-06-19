(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();

  // --- Chart 1: Throughput ---
  (function() {
    var el = document.getElementById('chart-throughput');
    if (!el) return;
    var chart = echarts.init(el, null, { renderer: 'svg' });

    var categories = [
      'ShardedRateLimiter', 'TDigest_Add', 'ShardedLock_Read', 'ShardedLock_Write',
      'StepStore_Record', 'RateLimiter', 'UUIDGen', 'ShardedEventStore',
      'FastPipeline', 'CircuitBreaker', 'Pipeline_Simple', 'Pipeline_100Steps', 'StepStore_Query'
    ];
    var data = [332794958, 24800305, 21092966, 20339947, 15310593, 8723871, 2422466, 1265284, 617214, 280620, 163586, 4605, 955];

    chart.setOption({
      animation: false,
      tooltip: {
        trigger: 'axis',
        backgroundColor: bg2,
        borderColor: rule,
        textStyle: { color: ink, fontSize: 13 },
        appendToBody: true,
        formatter: function(p) {
          return p[0].name + '<br/>' + Number(p[0].value).toLocaleString() + ' ops/s';
        }
      },
      grid: { left: 20, right: 40, top: 10, bottom: 20, containLabel: true },
      xAxis: {
        type: 'category',
        data: categories,
        axisLabel: { color: muted, fontSize: 10, rotate: 35, fontFamily: 'JetBrains Mono, monospace' },
        axisLine: { lineStyle: { color: rule } },
        axisTick: { show: false }
      },
      yAxis: {
        type: 'log',
        axisLabel: {
          color: muted,
          fontSize: 11,
          fontFamily: 'JetBrains Mono, monospace',
          formatter: function(v) { return v >= 1e6 ? (v/1e6).toFixed(0) + 'M' : v >= 1e3 ? (v/1e3).toFixed(0) + 'K' : v; }
        },
        splitLine: { lineStyle: { color: rule } },
        axisLine: { lineStyle: { color: rule } }
      },
      series: [{
        type: 'bar',
        data: data,
        itemStyle: {
          color: function(p) {
            return p.dataIndex === data.length - 1 ? '#f85149' : accent;
          },
          borderRadius: [4, 4, 0, 0]
        },
        barMaxWidth: 28
      }]
    });

    window.addEventListener('resize', function() { chart.resize(); });
  })();

  // --- Chart 2: Stress Test Results ---
  (function() {
    var el = document.getElementById('chart-stress');
    if (!el) return;
    var chart = echarts.init(el, null, { renderer: 'svg' });

    chart.setOption({
      animation: false,
      tooltip: {
        trigger: 'item',
        backgroundColor: bg2,
        borderColor: rule,
        textStyle: { color: ink, fontSize: 13 },
        appendToBody: true
      },
      series: [{
        type: 'pie',
        radius: ['55%', '78%'],
        center: ['50%', '50%'],
        avoidLabelOverlap: false,
        label: { show: false },
        emphasis: { label: { show: true, fontSize: 18, fontWeight: 'bold' } },
        data: [
          { value: 24, name: '通过 (PASS)', itemStyle: { color: accent2 } },
          { value: 0, name: '失败 (FAIL)', itemStyle: { color: '#f85149' } }
        ],
        labelLine: { show: false }
      }],
      graphic: [
        {
          type: 'text',
          left: 'center',
          top: 'center',
          style: {
            text: '24/24\n100%',
            textAlign: 'center',
            fill: accent2,
            fontSize: 28,
            fontWeight: 'bold',
            fontFamily: 'JetBrains Mono, monospace'
          }
        }
      ]
    });

    window.addEventListener('resize', function() { chart.resize(); });
  })();

  // --- Chart 3: Scenario Results ---
  (function() {
    var el = document.getElementById('chart-scenario');
    if (!el) return;
    var chart = echarts.init(el, null, { renderer: 'svg' });

    chart.setOption({
      animation: false,
      tooltip: {
        trigger: 'axis',
        backgroundColor: bg2,
        borderColor: rule,
        textStyle: { color: ink, fontSize: 13 },
        appendToBody: true
      },
      grid: { left: 20, right: 30, top: 10, bottom: 20, containLabel: true },
      xAxis: {
        type: 'category',
        data: ['订单处理', '多Agent协同', '流式处理', '多租户限流', '交易引擎', '浸泡测试'],
        axisLabel: { color: muted, fontSize: 12 },
        axisLine: { lineStyle: { color: rule } },
        axisTick: { show: false }
      },
      yAxis: {
        type: 'value',
        max: 1.5,
        axisLabel: {
          color: muted,
          fontSize: 11,
          fontFamily: 'JetBrains Mono, monospace',
          formatter: function(v) { return v === 1 ? 'PASS' : 'FAIL'; }
        },
        splitLine: { lineStyle: { color: rule } },
        axisLine: { lineStyle: { color: rule } }
      },
      series: [{
        type: 'bar',
        data: [1, 1, 1, 1, 1, 1],
        itemStyle: {
          color: accent2,
          borderRadius: [6, 6, 0, 0]
        },
        barMaxWidth: 60,
        label: {
          show: true,
          position: 'top',
          color: accent2,
          fontSize: 14,
          fontWeight: 'bold',
          fontFamily: 'JetBrains Mono, monospace',
          formatter: 'PASS'
        }
      }]
    });

    window.addEventListener('resize', function() { chart.resize(); });
  })();
})();