/**
 * arch-manager-migration.js
 * 从 arch-manager.html 提取的迁移引擎渲染函数
 *
 * 包含函数:
 *   - renderMigStatus         引擎状态
 *   - triggerMigrateAnalyze   触发分析
 *   - triggerMigrateValidate  触发验证
 *   - renderMigPatternMap     模式分类
 *   - renderMigGateChain       约束门链
 *   - renderMigLog            迁移日志
 *   - fetchMigLogs             查询日志
 *   - exportMigLogs            导出日志
 *   - renderMigSessions        会话历史
 *
 * 依赖全局变量 (来自 core.js):
 *   migStatus, migSessions, migLogs, migPatternMap, migGateChain,
 *   charts, echarts, esc, toast, api, renderCurrentView
 */

// ============================================================
// 迁移引擎渲染函数
// ============================================================
function renderMigStatus(container) {
  container.innerHTML = '<div class="view-title">引擎状态</div><div class="view-desc">迁移引擎全局状态概览 — 支持触发分析和验证</div>';
  const stats = document.createElement('div');
  stats.className = 'grid-4';
  stats.innerHTML = `
    <div class="stat-card"><div class="label">活跃会话</div><div class="value">${migStatus ? migStatus.active_sessions : 0}</div></div>
    <div class="stat-card"><div class="label">历史会话</div><div class="value">${migSessions.length}</div></div>
    <div class="stat-card"><div class="label">日志条目</div><div class="value">${migLogs.length}</div></div>
    <div class="stat-card"><div class="label">引擎状态</div><div class="value" style="color:var(--green)">就绪</div></div>
  `;
  container.appendChild(stats);
  const actions = document.createElement('div');
  actions.className = 'card';
  actions.style.cssText = 'padding:12px 16px;display:flex;gap:10px;align-items:center;margin-bottom:12px';
  actions.innerHTML = `
    <input type="text" id="migDirInput" placeholder="目标目录 (如 ./demo-project)" value="./demo-project" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 10px;border-radius:6px;font-size:12px;width:200px">
    <button class="btn primary" onclick="triggerMigrateAnalyze()" style="padding:6px 14px;font-size:12px">分析目录</button>
    <button class="btn" onclick="triggerMigrateValidate()" style="padding:6px 14px;font-size:12px">运行验证</button>
  `;
  container.appendChild(actions);
  const card = document.createElement('div');
  card.className = 'card';
  card.style.cssText = 'padding:0;overflow:auto;max-height:calc(100vh - 400px)';
  if (migLogs.length === 0) {
    card.innerHTML = '<div style="padding:40px;text-align:center;color:var(--dim)">暂无迁移事件（请在上方触发分析或验证）</div>';
  } else {
    let tbl = '<table class="data-table"><thead><tr><th>时间</th><th>类型</th><th>消息</th></tr></thead><tbody>';
    migLogs.slice(0, 50).forEach(e => {
      const ts = e.timestamp ? new Date(e.timestamp).toLocaleTimeString('zh-CN') : '--';
      const typeColor = {analyze_done:'var(--green)',validate_done:'var(--accent)',log_append:'var(--muted)'}[e.type] || 'var(--muted)';
      tbl += `<tr><td class="mono" style="font-size:11px">${ts}</td><td style="color:${typeColor}">${esc(e.type||'')}</td><td>${esc(e.message||'')}</td></tr>`;
    });
    tbl += '</tbody></table>';
    card.innerHTML = tbl;
  }
  container.appendChild(card);
}

async function triggerMigrateAnalyze() {
  const dir = document.getElementById('migDirInput').value;
  try {
    const result = await api('/api/migrate/analyze', { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({dir:dir}) });
    migPatternMap = result.pattern_map;
    toast('分析完成: ' + result.file_count + ' 文件', 'ok');
    renderCurrentView();
  } catch(e) { toast('分析失败: ' + e.message, 'err'); }
}

async function triggerMigrateValidate() {
  const dir = document.getElementById('migDirInput').value;
  try {
    const result = await api('/api/migrate/validate', { method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({dir:dir}) });
    migGateChain = result;
    toast('验证完成: ' + (result.pass ? '全部通过' : '存在阻断'), result.pass ? 'ok' : 'err');
    renderCurrentView();
  } catch(e) { toast('验证失败: ' + e.message, 'err'); }
}

function renderMigPatternMap(container) {
  container.innerHTML = '<div class="view-title">模式分类</div><div class="view-desc">四原语模式自动分类结果 — Atom/Port/Adapter/Composer</div>';
  if (!migPatternMap) {
    container.innerHTML += '<div style="padding:40px;text-align:center;color:var(--dim)">暂无分类数据（请先在"引擎状态"面板触发分析）</div>';
    return;
  }
  const grid = document.createElement('div');
  grid.className = 'grid-2';
  grid.innerHTML = '<div class="card"><div class="card-title">模式分布</div><div class="chart-container" id="migPieChart" style="height:280px"></div></div><div class="card"><div class="card-title">置信度分布</div><div class="chart-container" id="migConfChart" style="height:280px"></div></div>';
  container.appendChild(grid);
  const atoms = migPatternMap.atoms ? migPatternMap.atoms.length : 0;
  const ports = migPatternMap.ports ? migPatternMap.ports.length : 0;
  const adapters = migPatternMap.adapters ? migPatternMap.adapters.length : 0;
  const composers = migPatternMap.composers ? migPatternMap.composers.length : 0;
  const unknowns = migPatternMap.unknowns ? migPatternMap.unknowns.length : 0;
  const pieEl = document.getElementById('migPieChart');
  if (pieEl) {
    charts.migPie = echarts.init(pieEl);
    charts.migPie.setOption({
      backgroundColor:'transparent',
      tooltip:{trigger:'item',backgroundColor:'#1c1c1e',borderColor:'#2c2c2e',textStyle:{color:'#f5f5f7'}},
      series:[{type:'pie',radius:['40%','70%'],data:[
        {value:atoms,name:'Atom',itemStyle:{color:'#30d158'}},
        {value:ports,name:'Port',itemStyle:{color:'#0a84ff'}},
        {value:adapters,name:'Adapter',itemStyle:{color:'#ff9f0a'}},
        {value:composers,name:'Composer',itemStyle:{color:'#f472b6'}},
        {value:unknowns,name:'Unknown',itemStyle:{color:'#636366'}}
      ],label:{color:'#98989d',fontSize:11},itemStyle:{borderColor:'#1c1c1e',borderWidth:2}}]
    });
  }
  const confEl = document.getElementById('migConfChart');
  if (confEl) {
    const allMatches = [...(migPatternMap.atoms||[]),...(migPatternMap.ports||[]),...(migPatternMap.adapters||[]),...(migPatternMap.composers||[])];
    const buckets = [0,0,0,0,0];
    allMatches.forEach(m => { const c = (m.confidence||0)*100; if(c>=80) buckets[4]++; else if(c>=60) buckets[3]++; else if(c>=40) buckets[2]++; else if(c>=20) buckets[1]++; else buckets[0]++; });
    charts.migConf = echarts.init(confEl);
    charts.migConf.setOption({
      backgroundColor:'transparent',
      tooltip:{trigger:'axis',backgroundColor:'#1c1c1e',borderColor:'#2c2c2e',textStyle:{color:'#f5f5f7'}},
      grid:{left:50,right:10,top:10,bottom:30},
      xAxis:{type:'category',data:['0-20%','20-40%','40-60%','60-80%','80-100%'],axisLabel:{color:'#6e6e73',fontSize:10},axisLine:{lineStyle:{color:'#2c2c2e'}}},
      yAxis:{type:'value',axisLabel:{color:'#6e6e73',fontSize:10},splitLine:{lineStyle:{color:'#2c2c2e'}}},
      series:[{type:'bar',data:buckets.map((v,i)=>({value:v,itemStyle:{color:['#636366','#ff9f0a','#ff9f0a','#30d158','#30d158'][i]}}))}]
    });
  }
  const card = document.createElement('div');
  card.className = 'card';
  card.style.cssText = 'padding:0;overflow:auto;max-height:calc(100vh - 520px)';
  const allMatches = [...(migPatternMap.atoms||[]),...(migPatternMap.ports||[]),...(migPatternMap.adapters||[]),...(migPatternMap.composers||[]),...(migPatternMap.unknowns||[])];
  let tbl = '<table class="data-table"><thead><tr><th>函数</th><th>文件</th><th>模式</th><th>置信度</th></tr></thead><tbody>';
  allMatches.forEach(m => {
    const color = {atom:'var(--green)',port:'var(--accent)',adapter:'var(--orange)',composer:'#f472b6',unknown:'var(--dim)'}[m.pattern]||'var(--muted)';
    tbl += `<tr><td class="mono">${esc(m.func_name||m.FuncName||'')}</td><td style="font-size:11px">${esc(m.file||m.File||'')}</td><td style="color:${color};font-weight:500">${esc(m.pattern||'')}</td><td>${((m.confidence||m.Confidence||0)*100).toFixed(0)}%</td></tr>`;
  });
  tbl += '</tbody></table>';
  card.innerHTML = tbl;
  container.appendChild(card);
}

function renderMigGateChain(container) {
  container.innerHTML = '<div class="view-title">约束门链</div><div class="view-desc">六门约束链 (G1-G6) 执行状态 — 迁移前必须全部通过</div>';
  if (!migGateChain) {
    container.innerHTML += '<div style="padding:40px;text-align:center;color:var(--dim)">暂无验证数据（请先在"引擎状态"面板运行验证）</div>';
    return;
  }
  const overallPass = migGateChain.pass !== false;
  const pipeline = document.createElement('div');
  pipeline.className = 'card';
  pipeline.style.cssText = 'padding:16px;display:flex;gap:8px;align-items:center;overflow-x:auto;margin-bottom:12px';
  const gateNames = ['G1','G2','G3','G4','G5','G6'];
  const gateLabels = ['解析覆盖','全部分类','未知比例','原子日志','编译检查','日志完整'];
  gateNames.forEach((g, i) => {
    const pass = overallPass;
    const color = pass ? 'var(--green)' : 'var(--red)';
    pipeline.innerHTML += `
      <div style="flex-shrink:0;text-align:center">
        <div style="width:48px;height:48px;border-radius:8px;background:${pass?'rgba(48,209,88,0.1)':'rgba(255,69,58,0.1)'};border:1px solid ${color};display:flex;align-items:center;justify-content:center;font-weight:700;color:${color};font-size:14px">${g}</div>
        <div style="font-size:9px;color:var(--dim);margin-top:4px">${gateLabels[i]}</div>
        <div style="font-size:10px;color:${color};margin-top:2px">${pass?'PASS':'FAIL'}</div>
      </div>
      ${i < gateNames.length - 1 ? '<div style="color:var(--rule);font-size:16px;flex-shrink:0">&rarr;</div>' : ''}
    `;
  });
  container.appendChild(pipeline);
  if (migGateChain.blocked && migGateChain.blocked.length > 0) {
    const card = document.createElement('div');
    card.className = 'card';
    card.style.cssText = 'padding:0;overflow:auto;max-height:calc(100vh - 450px)';
    let tbl = '<table class="data-table"><thead><tr><th>阻断规则</th></tr></thead><tbody>';
    migGateChain.blocked.forEach(r => { tbl += `<tr><td style="color:var(--red)">${esc(r)}</td></tr>`; });
    tbl += '</tbody></table>';
    card.innerHTML = tbl;
    container.appendChild(card);
  }
  if (migGateChain.warnings && migGateChain.warnings.length > 0) {
    const card = document.createElement('div');
    card.className = 'card';
    card.style.cssText = 'padding:0;overflow:auto;margin-top:12px';
    let tbl = '<table class="data-table"><thead><tr><th>警告</th></tr></thead><tbody>';
    migGateChain.warnings.forEach(w => { tbl += `<tr><td style="color:var(--orange)">${esc(w)}</td></tr>`; });
    tbl += '</tbody></table>';
    card.innerHTML = tbl;
    container.appendChild(card);
  }
}

function renderMigLog(container) {
  container.innerHTML = '<div class="view-title">迁移日志</div><div class="view-desc">原子级不可变迁移日志 — 按 Phase 过滤，支持导出</div>';
  const qbar = document.createElement('div');
  qbar.className = 'card';
  qbar.style.cssText = 'padding:12px 16px;display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:12px';
  qbar.innerHTML = `
    <select id="migLogPhase" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 8px;border-radius:6px;font-size:12px">
      <option value="">全部 Phase</option><option value="parse">parse</option><option value="pattern">pattern</option><option value="transform">transform</option><option value="shim">shim</option><option value="validate">validate</option>
    </select>
    <button class="btn primary" onclick="fetchMigLogs()" style="padding:6px 14px;font-size:12px">查询</button>
    <button class="btn" onclick="exportMigLogs('json')" style="padding:6px 14px;font-size:12px">导出 JSON</button>
    <button class="btn" onclick="exportMigLogs('md')" style="padding:6px 14px;font-size:12px">导出 MD</button>
    <span style="font-size:11px;color:var(--dim);margin-left:auto">共 ${migLogs.length} 条</span>
  `;
  container.appendChild(qbar);
  const card = document.createElement('div');
  card.className = 'card';
  card.style.cssText = 'padding:0;overflow:auto;max-height:calc(100vh - 380px)';
  if (migLogs.length === 0) {
    card.innerHTML = '<div style="padding:40px;text-align:center;color:var(--dim)">暂无迁移日志</div>';
  } else {
    let tbl = '<table class="data-table"><thead><tr><th>SeqNo</th><th>Phase</th><th>Action</th><th>File</th><th>Line</th></tr></thead><tbody>';
    migLogs.slice(0, 200).forEach(e => {
      const phaseColor = {parse:'var(--accent)',pattern:'#f472b6',transform:'var(--green)',shim:'var(--orange)',validate:'var(--muted)'}[e.phase]||'var(--dim)';
      tbl += `<tr><td class="mono">${e.seq_no||e.SeqNo||'--'}</td><td style="color:${phaseColor}">${esc(e.phase||'')}</td><td class="mono">${esc(e.action_type||e.ActionType||'')}</td><td class="mono" style="font-size:11px">${esc(e.file_path||e.FilePath||'--')}</td><td class="mono">${(e.line_start||e.LineStart||0) > 0 ? (e.line_start||e.LineStart)+'-'+(e.line_end||e.LineEnd) : '--'}</td></tr>`;
    });
    tbl += '</tbody></table>';
    card.innerHTML = tbl;
  }
  container.appendChild(card);
}

async function fetchMigLogs() {
  const phase = document.getElementById('migLogPhase').value;
  let url = '/api/migrate/logs';
  if (phase) url += '?phase=' + encodeURIComponent(phase);
  try { migLogs = await api(url); toast('查询完成: ' + migLogs.length + ' 条', 'ok'); renderCurrentView(); }
  catch(e) { toast('查询失败: ' + e.message, 'err'); }
}

async function exportMigLogs(format) {
  try {
    const blob = await fetch('/api/migrate/logs/export?format=' + format).then(r => r.blob());
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'migration-log.' + (format === 'json' ? 'json' : 'md');
    a.click();
    toast('导出成功', 'ok');
  } catch(e) { toast('导出失败: ' + e.message, 'err'); }
}

function renderMigSessions(container) {
  container.innerHTML = '<div class="view-title">会话历史</div><div class="view-desc">所有迁移分析/验证会话记录</div>';
  const card = document.createElement('div');
  card.className = 'card';
  card.style.cssText = 'padding:0;overflow:auto;max-height:calc(100vh - 300px)';
  if (migSessions.length === 0) {
    card.innerHTML = '<div style="padding:40px;text-align:center;color:var(--dim)">暂无会话记录</div>';
  } else {
    let tbl = '<table class="data-table"><thead><tr><th>SessionID</th><th>语言</th><th>文件数</th><th>状态</th><th>开始时间</th></tr></thead><tbody>';
    migSessions.forEach(s => {
      const statusColor = s.status === 'completed' ? 'var(--green)' : s.status === 'failed' ? 'var(--red)' : 'var(--orange)';
      const ts = new Date(s.started_at).toLocaleString('zh-CN');
      tbl += `<tr><td class="mono" style="font-size:11px">${esc(s.session_id)}</td><td>${esc(s.language)}</td><td>${s.file_count}</td><td style="color:${statusColor};font-weight:500">${esc(s.status)}</td><td class="mono" style="font-size:11px">${ts}</td></tr>`;
    });
    tbl += '</tbody></table>';
    card.innerHTML = tbl;
  }
  container.appendChild(card);
}
