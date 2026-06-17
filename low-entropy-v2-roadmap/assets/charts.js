// assets/charts.js - v2.0 Roadmap Charts
(function() {
  var style = getComputedStyle(document.documentElement);
  var accent = style.getPropertyValue('--accent').trim();
  var accent2 = style.getPropertyValue('--accent2').trim();
  var ink = style.getPropertyValue('--ink').trim();
  var muted = style.getPropertyValue('--muted').trim();
  var rule = style.getPropertyValue('--rule').trim();
  var bg2 = style.getPropertyValue('--bg2').trim();
  var green = style.getPropertyValue('--green').trim();
  var red = style.getPropertyValue('--red').trim();
  var teal = style.getPropertyValue('--teal').trim();

  // --- Bar Chart: Entropy Metrics v1.x vs v2.0 ---
  var chartDom = document.getElementById('chart-entropy');
  if (chartDom) {
    var chart = echarts.init(chartDom, null, { renderer: 'svg' });

    var categories = ['抽象种类数\n(越低越好)', '步骤增长率\n(越低越好)', '类型断言密度\n(越低越好)', 'Composer扇出\n(越低越好)', '原语均衡度\n(越高越好)', '测试覆盖率\n(越高越好)'];

    chart.setOption({
      animation: false,
      tooltip: {
        appendToBody: true,
        backgroundColor: bg2,
        borderColor: rule,
        textStyle: { color: ink }
      },
      legend: {
        bottom: 0,
        textStyle: { color: muted, fontSize: 12 },
        data: ['v1.x 当前', 'v2.0 目标']
      },
      grid: {
        left: '3%', right: '4%', bottom: '12%', top: '8%',
        containLabel: true
      },
      xAxis: {
        type: 'category',
        data: categories,
        axisLabel: { color: muted, fontSize: 10, interval: 0 },
        axisLine: { lineStyle: { color: rule } },
        axisTick: { show: false }
      },
      yAxis: {
        type: 'value',
        name: '评分 (0-10)',
        min: 0, max: 10,
        nameTextStyle: { color: muted, fontSize: 11 },
        axisLabel: { color: muted, fontSize: 11 },
        splitLine: { lineStyle: { color: rule } },
        axisLine: { lineStyle: { color: rule } }
      },
      series: [
        {
          name: 'v1.x 当前',
          type: 'bar',
          data: [
            { value: 1, itemStyle: { color: red } },
            { value: 2, itemStyle: { color: red } },
            { value: 1, itemStyle: { color: red } },
            { value: 3, itemStyle: { color: red } },
            { value: 3, itemStyle: { color: red } },
            { value: 0, itemStyle: { color: red } }
          ],
          barWidth: '35%',
          label: {
            show: true,
            position: 'top',
            color: muted,
            fontSize: 10,
            formatter: function(p) { return p.value === 0 ? '0' : p.value; }
          }
        },
        {
          name: 'v2.0 目标',
          type: 'bar',
          data: [
            { value: 9, itemStyle: { color: teal } },
            { value: 8, itemStyle: { color: teal } },
            { value: 9, itemStyle: { color: teal } },
            { value: 8, itemStyle: { color: teal } },
            { value: 8, itemStyle: { color: teal } },
            { value: 8, itemStyle: { color: teal } }
          ],
          barWidth: '35%',
          label: {
            show: true,
            position: 'top',
            color: muted,
            fontSize: 10,
            formatter: function(p) { return p.value; }
          }
        }
      ]
    });
    window.addEventListener('resize', function() { chart.resize(); });
  }
})();