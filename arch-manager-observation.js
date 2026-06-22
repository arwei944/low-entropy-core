/**
 * arch-manager-observation.js
 * 来源: arch-manager.html
 * 包含可观测性相关的渲染函数（纯提取，未修改任何逻辑）
 *
 * 包含函数:
 *   - renderObsSteps         执行步骤
 *   - renderObsTraceDetail   Trace 详情
 *   - renderObsAggregates    聚合指标
 *   - renderObsPipelines     Pipeline 状态
 *   - renderObsArch          架构快照
 *
 * 依赖的全局变量:
 *   obsSteps, obsStepsQuery, obsErrors, obsTraceDetail,
 *   obsAggregates, obsAggQuery, obsPipelines, obsArch,
 *   charts, echarts, esc
 *   (以及 core.js 中的其他全局变量)
 */

// ============================================================
// 可观测性 — 执行步骤
// ============================================================
function renderObsSteps(container) {
  container.innerHTML = '<div class="view-title">执行步骤</div><div class="view-desc">Observation 层记录的 ExecutionStep 列表 — 支持按条件查询、只看错误、Trace 钻取</div>';

  // Query bar
  const qbar = document.createElement('div');
  qbar.className = 'card';
  qbar.style.cssText = 'padding:12px 16px;display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:12px';
  qbar.innerHTML = `
    <input type="text" id="obsStepsPattern" placeholder="Pattern" value="${esc(obsStepsQuery.pattern)}" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 10px;border-radius:6px;font-size:12px;width:120px">
    <input type="text" id="obsStepsUnit" placeholder="Unit" value="${esc(obsStepsQuery.unit)}" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 10px;border-radius:6px;font-size:12px;width:120px">
    <label style="display:flex;align-items:center;gap:6px;font-size:12px;color:var(--muted);cursor:pointer">
      <input type="checkbox" id="obsStepsErrorOnly" ${obsStepsQuery.error_only ? 'checked' : ''}> 只看错误
    </label>
    <select id="obsStepsLimit" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 8px;border-radius:6px;font-size:12px">
      <option value="20" ${obsStepsQuery.limit===20?'selected':''}>20条</option>
      <option value="50" ${obsStepsQuery.limit===50?'selected':''}>50条</option>
      <option value="100" ${obsStepsQuery.limit===100?'selected':''}>100条</option>
      <option value="200" ${obsStepsQuery.limit===200?'selected':''}>200条</option>
    </select>
    <button class="btn primary" onclick="obsStepsQuery.pattern=document.getElementById('obsStepsPattern').value;obsStepsQuery.unit=document.getElementById('obsStepsUnit').value;obsStepsQuery.error_only=document.getElementById('obsStepsErrorOnly').checked;obsStepsQuery.limit=parseInt(document.getElementById('obsStepsLimit').value);queryObsSteps().then(()=>renderCurrentView())" style="padding:6px 14px;font-size:12px">查询</button>
    <button class="btn" onclick="fetchObsErrors().then(()=>renderCurrentView())" style="padding:6px 14px;font-size:12px">加载错误步骤</button>
    <span style="font-size:11px;color:var(--dim);margin-left:auto">共 ${obsSteps.length} 条 / 错误 ${obsErrors.length} 条</span>
  `;
  container.appendChild(qbar);

  const steps = obsSteps || [];
  const card = document.createElement('div');
  card.className = 'card';
  card.style.padding = '0';
  card.style.overflow = 'auto';
  card.style.maxHeight = 'calc(100vh - 380px)';

  if (steps.length === 0 && obsErrors.length === 0) {
    card.innerHTML = '<div style="padding:40px;text-align:center;color:var(--dim)">暂无执行步骤数据（需要运行 Observation Pipeline）</div>';
  } else {
    const displaySteps = steps.length > 0 ? steps : obsErrors;
    let tbl = '<table class="data-table"><thead><tr><th>TraceID</th><th>Pattern</th><th>Unit</th><th>耗时</th><th>状态</th><th>时间</th></tr></thead><tbody>';
    displaySteps.slice(0, 200).forEach((s, idx) => {
      const dur = s.duration_ms ? s.duration_ms.toFixed(1) + 'ms' : (s.duration_us ? (s.duration_us / 1000).toFixed(1) + 'ms' : '--');
      const status = s.error ? '<span style="color:var(--red)">失败</span>' : '<span style="color:var(--green)">成功</span>';
      const ts = s.timestamp ? new Date(s.timestamp).toLocaleTimeString('zh-CN') : '--';
      const tid = (s.trace_id || '');
      const tidShort = tid.substring(0, 8);
      const traceIdAttr = esc(tid);
      tbl += `<tr><td class="mono" style="cursor:pointer;color:var(--accent)" onclick="obsTraceDetail=null;fetchObsTrace('${traceIdAttr}').then(()=>{if(obsTraceDetail)renderObsTraceDetail(document.getElementById('mainContent'))})" title="点击查看 Trace 详情">${esc(tidShort)}</td><td>${esc(s.pattern || '--')}</td><td class="mono">${esc(s.unit || '--')}</td><td>${dur}</td><td>${status}</td><td class="mono">${ts}</td></tr>`;
    });
    tbl += '</tbody></table>';
    card.innerHTML = tbl;
  }
  container.appendChild(card);
}

// ============================================================
// 可观测性 — Trace 详情
// ============================================================
function renderObsTraceDetail(container) {
  const td = obsTraceDetail;
  if (!td) { container.innerHTML = '<div class="view-title">Trace 详情</div><div class="view-desc">Trace 数据加载失败</div>'; return; }
  container.innerHTML = '<div class="view-title">Trace 详情</div><div class="view-desc">TraceID: ' + esc(td.trace_id || '--') + '</div>';

  const grid = document.createElement('div');
  grid.className = 'grid-4';
  grid.innerHTML = `
    <div class="stat-card"><div class="label">SpanID</div><div class="value mono" style="font-size:14px">${esc((td.span_id || '--').substring(0,8))}</div></div>
    <div class="stat-card"><div class="label">Pattern</div><div class="value">${esc(td.pattern || '--')}</div></div>
    <div class="stat-card"><div class="label">Unit</div><div class="value mono">${esc(td.unit || '--')}</div></div>
    <div class="stat-card"><div class="label">耗时</div><div class="value">${td.duration_ms ? td.duration_ms.toFixed(1)+'ms' : '--'}</div></div>
  `;
  container.appendChild(grid);

  if (td.error) {
    const errCard = document.createElement('div');
    errCard.className = 'card';
    errCard.innerHTML = '<div class="card-title">错误信息</div><div style="color:var(--red);font-family:var(--mono);font-size:12px;white-space:pre-wrap">' + esc(typeof td.error === 'string' ? td.error : JSON.stringify(td.error, null, 2)) + '</div>';
    container.appendChild(errCard);
  }

  if (td.metadata) {
    const metaCard = document.createElement('div');
    metaCard.className = 'card';
    metaCard.innerHTML = '<div class="card-title">元数据</div><div style="font-family:var(--mono);font-size:12px;white-space:pre-wrap;color:var(--muted)">' + esc(JSON.stringify(td.metadata, null, 2)) + '</div>';
    container.appendChild(metaCard);
  }

  // Back button
  const backBtn = document.createElement('button');
  backBtn.className = 'btn';
  backBtn.textContent = '← 返回执行步骤';
  backBtn.style.marginTop = '12px';
  backBtn.onclick = () => { currentView = 'obsSteps'; renderCurrentView(); };
  container.appendChild(backBtn);
}

// ============================================================
// 可观测性 — 聚合指标
// ============================================================
function renderObsAggregates(container) {
  container.innerHTML = '<div class="view-title">聚合指标</div><div class="view-desc">P50/P99 延迟、错误率等聚合窗口数据 — 支持按条件查询</div>';

  // Query bar
  const qbar = document.createElement('div');
  qbar.className = 'card';
  qbar.style.cssText = 'padding:12px 16px;display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:12px';
  qbar.innerHTML = `
    <input type="text" id="obsAggPattern" placeholder="Pattern" value="${esc(obsAggQuery.pattern)}" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 10px;border-radius:6px;font-size:12px;width:120px">
    <input type="text" id="obsAggUnit" placeholder="Unit" value="${esc(obsAggQuery.unit)}" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 10px;border-radius:6px;font-size:12px;width:120px">
    <input type="text" id="obsAggWindow" placeholder="Window" value="${esc(obsAggQuery.window)}" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 10px;border-radius:6px;font-size:12px;width:120px">
    <select id="obsAggLimit" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 8px;border-radius:6px;font-size:12px">
      <option value="10" ${obsAggQuery.limit===10?'selected':''}>10条</option>
      <option value="20" ${obsAggQuery.limit===20?'selected':''}>20条</option>
      <option value="50" ${obsAggQuery.limit===50?'selected':''}>50条</option>
    </select>
    <button class="btn primary" onclick="obsAggQuery.pattern=document.getElementById('obsAggPattern').value;obsAggQuery.unit=document.getElementById('obsAggUnit').value;obsAggQuery.window=document.getElementById('obsAggWindow').value;obsAggQuery.limit=parseInt(document.getElementById('obsAggLimit').value);queryObsAggregates().then(()=>renderCurrentView())" style="padding:6px 14px;font-size:12px">查询</button>
    <span style="font-size:11px;color:var(--dim);margin-left:auto">共 ${obsAggregates.length} 条聚合记录</span>
  `;
  container.appendChild(qbar);

  const aggs = obsAggregates || [];
  const grid = document.createElement('div');
  grid.className = 'grid-2';
  grid.innerHTML = `
    <div class="card"><div class="card-title">P50/P99 延迟</div><div class="chart-container" id="pLatencyChart"></div></div>
    <div class="card"><div class="card-title">错误率趋势</div><div class="chart-container" id="pErrorChart"></div></div>
  `;
  container.appendChild(grid);

  const stats = document.createElement('div');
  stats.className = 'grid-4';
  const lastAgg = aggs.length > 0 ? aggs[aggs.length - 1] : {};
  const totalSteps = aggs.reduce((s, a) => s + (a.count || 0), 0);
  const totalErrors = aggs.reduce((s, a) => s + (a.error_count || 0), 0);
  const avgP50 = aggs.length ? (aggs.reduce((s, a) => s + (a.p50 || 0), 0) / aggs.length).toFixed(1) : '--';
  const avgP99 = aggs.length ? (aggs.reduce((s, a) => s + (a.p99 || 0), 0) / aggs.length).toFixed(1) : '--';
  stats.innerHTML = `
    <div class="stat-card"><div class="label">总步骤</div><div class="value">${totalSteps}</div></div>
    <div class="stat-card"><div class="label">总错误</div><div class="value" style="color:${totalErrors > 0 ? 'var(--orange)' : 'var(--green)'}">${totalErrors}</div></div>
    <div class="stat-card"><div class="label">平均 P50</div><div class="value">${avgP50}ms</div></div>
    <div class="stat-card"><div class="label">平均 P99</div><div class="value">${avgP99}ms</div></div>
  `;
  container.appendChild(stats);

  const p50Data = aggs.map(a => a.p50 || 0);
  const p99Data = aggs.map(a => a.p99 || 0);
  const errData = aggs.map(a => a.error_count || 0);
  const labels = aggs.map((_, i) => 'W' + (i + 1));

  charts.pLatency = echarts.init(document.getElementById('pLatencyChart'));
  charts.pLatency.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' } },
    legend: { textStyle: { color: '#98989d', fontSize: 10 }, top: 0 },
    grid: { left: 50, right: 10, top: 30, bottom: 20 },
    xAxis: { type: 'category', data: labels, axisLabel: { color: '#6e6e73', fontSize: 10 }, axisLine: { lineStyle: { color: '#2c2c2e' } } },
    yAxis: { type: 'value', axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
    series: [
      { name: 'P50', type: 'line', data: p50Data, smooth: true, itemStyle: { color: '#0a84ff' }, lineStyle: { width: 2 } },
      { name: 'P99', type: 'line', data: p99Data, smooth: true, itemStyle: { color: '#ff9f0a' }, lineStyle: { width: 2 } }
    ]
  });

  charts.pError = echarts.init(document.getElementById('pErrorChart'));
  charts.pError.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' } },
    grid: { left: 50, right: 10, top: 10, bottom: 20 },
    xAxis: { type: 'category', data: labels, axisLabel: { color: '#6e6e73', fontSize: 10 }, axisLine: { lineStyle: { color: '#2c2c2e' } } },
    yAxis: { type: 'value', axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
    series: [{
      name: '错误', type: 'bar', data: errData,
      itemStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1, colorStops: [{ offset: 0, color: '#ff453a' }, { offset: 1, color: '#ff9f0a' }] } }
    }]
  });
}

// ============================================================
// 可观测性 — Pipeline 状态
// ============================================================
function renderObsPipelines(container) {
  container.innerHTML = '<div class="view-title">Pipeline 状态</div><div class="view-desc">Observation Pipeline 注册表及运行状态 — 来自 /api/observation/pipeline</div>';

  // === 优先: 使用 obsPipelineData (来自 /api/observation/pipeline) ===
  if (obsPipelineData && obsPipelineData.snapshot) {
    const snap = obsPipelineData.snapshot;
    const stats = obsPipelineData.aggregate_stats || {};
    const stepSumm = obsPipelineData.step_summary || {};

    // 1. 顶部: Pipeline 元信息卡片
    const metaGrid = document.createElement('div');
    metaGrid.className = 'grid-4';
    metaGrid.innerHTML = `
      <div class="stat-card"><div class="label">架构</div><div class="value" style="font-size:14px">${esc(snap.architecture || '--')}</div></div>
      <div class="stat-card"><div class="label">版本</div><div class="value" style="font-size:14px">${esc(snap.version || '--')}</div></div>
      <div class="stat-card"><div class="label">总步骤</div><div class="value">${snap.total_steps || 0}</div></div>
      <div class="stat-card"><div class="label">完成/进行/待定</div><div class="value" style="font-size:14px">${stepSumm.completed || 0} / ${stepSumm.in_progress || 0} / ${stepSumm.pending || 0}</div></div>
    `;
    container.appendChild(metaGrid);

    // 2. 8层 Pipeline 步骤详细图
    const stepCard = document.createElement('div');
    stepCard.className = 'card';
    stepCard.innerHTML = '<div class="card-title">8层 Pipeline 执行步骤</div><div class="chart-container" id="obsPipelineStepsChart"></div>';
    container.appendChild(stepCard);

    const steps = snap.steps || [];
    const labels = steps.map(s => `${s.layer} · ${s.name}`);
    const durations = steps.map(s => s.duration_ms || 0);
    const colors = steps.map(s => {
      if (s.status === 'completed') return '#48bb78';
      if (s.status === 'in_progress') return '#ecc94b';
      if (s.status === 'failed') return '#f56565';
      return '#4a5568';
    });

    charts.obsPipelineSteps = echarts.init(document.getElementById('obsPipelineStepsChart'));
    charts.obsPipelineSteps.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' },
        formatter: function(params) {
          const p = params[0];
          const s = steps[p.dataIndex];
          let html = `<b>${s.layer} · ${esc(s.name || 'n/a')}</b><br/>状态: ${s.status}<br/>耗时: ${s.duration_ms || 0}ms`;
          if (s.input) html += `<br/>输入: ${esc(String(s.input).substring(0, 80))}`;
          if (s.output) html += `<br/>输出: ${esc(String(s.output).substring(0, 80))}`;
          if (s.source) html += `<br/>来源: ${esc(String(s.source).substring(0, 80))}`;
          return html;
        }
      },
      grid: { left: 130, right: 80, top: 20, bottom: 40 },
      xAxis: { type: 'value', axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
      yAxis: { type: 'category', data: labels, axisLabel: { color: '#f5f5f7', fontSize: 11 } },
      series: [{
        type: 'bar',
        data: steps.map((s, i) => ({ value: s.duration_ms || 0, itemStyle: { color: colors[i] } })),
        label: { show: true, position: 'right', color: '#f5f5f7', fontSize: 11, formatter: (p) => `${p.value}ms [${steps[p.dataIndex].status}]` }
      }]
    });

    // 3. Pipeline Trace 记录（如果有）
    if (obsPipelineData.recent_traces && obsPipelineData.recent_traces.length > 0) {
      const traceCard = document.createElement('div');
      traceCard.className = 'card';
      traceCard.innerHTML = '<div class="card-title">Recent Pipeline Traces</div>';
      let traceHtml = '<table class="data-table"><thead><tr><th>Trace ID</th><th style="width:120px">耗时</th><th style="width:100px">步骤数</th><th style="width:100px">状态</th><th style="width:200px">开始时间</th></tr></thead><tbody>';
      obsPipelineData.recent_traces.slice(0, 20).forEach(t => {
        const statusColor = t.status === 'completed' ? 'var(--green)' : t.status === 'in_progress' ? 'var(--orange)' : 'var(--red)';
        traceHtml += `<tr><td class="mono" style="font-size:11px;color:#4299e1">${esc(t.trace_id || 'n/a')}</td><td>${(t.total_time_ms || 0).toLocaleString()} ms</td><td>${(t.steps || []).length}</td><td style="color:${statusColor};font-size:11px">${t.status || '-'}</td><td class="mono" style="font-size:11px;color:#a0aec0">${esc(t.start_time || '-')}</td></tr>`;
      });
      traceHtml += '</tbody></table>';
      const body = document.createElement('div');
      body.style.padding = '0 20px 20px';
      body.innerHTML = traceHtml;
      traceCard.appendChild(body);
      container.appendChild(traceCard);
    }

    // === 附带: 原有的 obsPipelines 数据 ===
    if (obsPipelines && obsPipelines.length > 0) {
      const pipeCard = document.createElement('div');
      pipeCard.className = 'card';
      pipeCard.innerHTML = '<div class="card-title">Pipeline Registry</div>';
      let tbl = '<table class="data-table"><thead><tr><th>名称</th><th>状态</th><th>缓冲区</th><th>采样率</th><th>步骤数</th><th>丢弃数</th></tr></thead><tbody>';
      obsPipelines.forEach(p => {
        const status = p.running ? '<span style="color:var(--green)">运行中</span>' : '<span style="color:var(--dim)">已停止</span>';
        const buf = p.buffer_size || p.buffer || '--';
        const rate = p.sampling_rate != null ? (p.sampling_rate * 100).toFixed(0) + '%' : '--';
        const stepCount = p.total_steps || p.steps || 0;
        const dropped = p.dropped || p.sampler_dropped || 0;
        tbl += `<tr><td class="mono">${esc(p.name || p.id || '--')}</td><td>${status}</td><td>${buf}</td><td>${rate}</td><td>${stepCount}</td><td>${dropped}</td></tr>`;
      });
      tbl += '</tbody></table>';
      const body = document.createElement('div');
      body.style.padding = '0 20px 20px';
      body.innerHTML = tbl;
      pipeCard.appendChild(body);
      container.appendChild(pipeCard);
    }
    return;
  }

  // === Fallback: 原有的 obsPipelines 数据 ===
  const pipes = obsPipelines || [];
  const card = document.createElement('div');
  card.className = 'card';
  card.style.padding = '0';
  card.style.overflow = 'auto';
  if (pipes.length === 0) {
    card.innerHTML = '<div style="padding:40px;text-align:center;color:var(--dim)">暂无 Pipeline 数据（需要启动 Observation Pipeline）</div>';
  } else {
    let tbl = '<table class="data-table"><thead><tr><th>名称</th><th>状态</th><th>缓冲区</th><th>采样率</th><th>步骤数</th><th>丢弃数</th></tr></thead><tbody>';
    pipes.forEach(p => {
      const status = p.running ? '<span style="color:var(--green)">运行中</span>' : '<span style="color:var(--dim)">已停止</span>';
      const buf = p.buffer_size || p.buffer || '--';
      const rate = p.sampling_rate != null ? (p.sampling_rate * 100).toFixed(0) + '%' : '--';
      const stepCount = p.total_steps || p.steps || 0;
      const dropped = p.dropped || p.sampler_dropped || 0;
      tbl += `<tr><td class="mono">${esc(p.name || p.id || '--')}</td><td>${status}</td><td>${buf}</td><td>${rate}</td><td>${stepCount}</td><td>${dropped}</td></tr>`;
    });
    tbl += '</tbody></table>';
    card.innerHTML = tbl;
  }
  container.appendChild(card);
}

// ============================================================
// 可观测性 — 架构快照
// ============================================================
function renderObsArch(container) {
  container.innerHTML = '<div class="view-title">架构快照</div><div class="view-desc">Observation 层完整架构快照（total_steps / sampler / aggregates / pipelines）</div>';
  const oa = obsArch || {};
  const grid = document.createElement('div');
  grid.className = 'grid-4';
  const errRate = oa.total_steps > 0 ? ((oa.error_steps || 0) / oa.total_steps * 100).toFixed(2) : '0';
  grid.innerHTML = `
    <div class="stat-card"><div class="label">总步骤</div><div class="value">${oa.total_steps || 0}</div></div>
    <div class="stat-card"><div class="label">错误步骤</div><div class="value" style="color:${(oa.error_steps || 0) > 0 ? 'var(--orange)' : 'var(--green)'}">${oa.error_steps || 0}</div></div>
    <div class="stat-card"><div class="label">错误率</div><div class="value">${errRate}%</div></div>
    <div class="stat-card"><div class="label">采样丢弃</div><div class="value">${oa.sampler_dropped || 0}</div></div>
  `;
  container.appendChild(grid);

  if (oa.aggregates && oa.aggregates.length > 0) {
    const aggsCard = document.createElement('div');
    aggsCard.className = 'card';
    aggsCard.innerHTML = '<div class="card-title">聚合器状态</div><div class="chart-container" id="obsAggChart"></div>';
    container.appendChild(aggsCard);

    const aggLabels = oa.aggregates.map((a, i) => 'W' + (i + 1));
    const aggCounts = oa.aggregates.map(a => a.count || 0);
    charts.obsAgg = echarts.init(document.getElementById('obsAggChart'));
    charts.obsAgg.setOption({
      backgroundColor: 'transparent',
      tooltip: { trigger: 'axis', backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' } },
      grid: { left: 50, right: 10, top: 10, bottom: 20 },
      xAxis: { type: 'category', data: aggLabels, axisLabel: { color: '#6e6e73', fontSize: 10 } },
      yAxis: { type: 'value', axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
      series: [{ name: '步骤数', type: 'bar', data: aggCounts, itemStyle: { color: '#34d399' } }]
    });
  }

  if (oa.pipelines && oa.pipelines.length > 0) {
    const pipeCard = document.createElement('div');
    pipeCard.className = 'card';
    pipeCard.style.padding = '0';
    pipeCard.style.overflow = 'auto';
    let tbl = '<div class="card-title" style="padding:16px">Pipeline 注册表</div><table class="data-table"><thead><tr><th>名称</th><th>状态</th><th>步骤</th><th>丢弃</th></tr></thead><tbody>';
    oa.pipelines.forEach(p => {
      tbl += `<tr><td class="mono">${esc(p.name || p.id)}</td><td>${p.running ? '<span style="color:var(--green)">运行中</span>' : '<span style="color:var(--dim)">已停止</span>'}</td><td>${p.steps || 0}</td><td>${p.dropped || 0}</td></tr>`;
    });
    tbl += '</tbody></table>';
    pipeCard.innerHTML = tbl;
    container.appendChild(pipeCard);
  }
}
