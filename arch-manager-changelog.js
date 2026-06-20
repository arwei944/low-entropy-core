/**
 * arch-manager-changelog.js
 * 架构变动日志渲染函数
 *
 * 从 arch-manager-migration.js 拆分
 *
 * 包含函数:
 *   - renderArchChangelog      架构变动日志
 *   - fetchArchChangelog       查询变动日志
 *
 * 依赖全局变量 (来自 core.js):
 *   archChangelog, archChangelogStats, esc, toast, api, renderCurrentView
 */

// ============================================================
// 架构变动日志
// ============================================================
function renderArchChangelog(container) {
  container.innerHTML = '<div class="view-title">架构变动日志</div><div class="view-desc">记录架构的所有实时变动 — 文件增删改、违规检测等</div>';
  const stats = document.createElement('div');
  stats.className = 'grid-4';
  const total = archChangelogStats ? archChangelogStats.total : archChangelog.length;
  const byCat = archChangelogStats ? archChangelogStats.by_category : {};
  const fileChanges = (byCat.file_add||0) + (byCat.file_modify||0) + (byCat.file_delete||0);
  const violations = (byCat.violation_add||0);
  stats.innerHTML = `
    <div class="stat-card"><div class="label">总变动</div><div class="value">${total}</div></div>
    <div class="stat-card"><div class="label">文件变更</div><div class="value">${fileChanges}</div></div>
    <div class="stat-card"><div class="label">违规事件</div><div class="value" style="color:${violations>0?'var(--orange)':'var(--green)'}">${violations}</div></div>
    <div class="stat-card"><div class="label">日志条目</div><div class="value">${archChangelog.length}</div></div>
  `;
  container.appendChild(stats);
  const qbar = document.createElement('div');
  qbar.className = 'card';
  qbar.style.cssText = 'padding:12px 16px;display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:12px';
  qbar.innerHTML = `
    <select id="chlogCategory" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 8px;border-radius:6px;font-size:12px">
      <option value="">全部类别</option><option value="file_add">文件新增</option><option value="file_modify">文件修改</option><option value="file_delete">文件删除</option><option value="violation_add">违规新增</option><option value="violation_resolve">违规解决</option><option value="health_change">健康分变更</option>
    </select>
    <select id="chlogSeverity" style="background:var(--bg2);border:1px solid var(--rule);color:var(--ink);padding:6px 8px;border-radius:6px;font-size:12px">
      <option value="">全部级别</option><option value="info">info</option><option value="warning">warning</option><option value="critical">critical</option>
    </select>
    <button class="btn primary" onclick="fetchArchChangelog()" style="padding:6px 14px;font-size:12px">查询</button>
    <span style="font-size:11px;color:var(--dim);margin-left:auto">共 ${archChangelog.length} 条</span>
  `;
  container.appendChild(qbar);
  const card = document.createElement('div');
  card.className = 'card';
  card.style.cssText = 'padding:0;overflow:auto;max-height:calc(100vh - 420px)';
  if (archChangelog.length === 0) {
    card.innerHTML = '<div style="padding:40px;text-align:center;color:var(--dim)">暂无架构变动记录</div>';
  } else {
    let tbl = '<table class="data-table"><thead><tr><th>时间</th><th>类别</th><th>级别</th><th>文件</th><th>详情</th></tr></thead><tbody>';
    archChangelog.slice(0, 100).forEach(e => {
      const ts = new Date(e.timestamp).toLocaleTimeString('zh-CN');
      const catLabel = {file_add:'文件新增',file_modify:'文件修改',file_delete:'文件删除',violation_add:'违规新增',violation_resolve:'违规解决',health_change:'健康分变更',symbol_add:'符号新增',symbol_remove:'符号删除',layer_change:'层级变更'}[e.category]||e.category;
      const catColor = {file_add:'var(--green)',file_modify:'var(--accent)',file_delete:'var(--red)',violation_add:'var(--orange)',violation_resolve:'var(--green)',health_change:'#f472b6'}[e.category]||'var(--muted)';
      const sevColor = {info:'var(--muted)',warning:'var(--orange)',critical:'var(--red)'}[e.severity]||'var(--muted)';
      tbl += `<tr><td class="mono" style="font-size:11px">${ts}</td><td style="color:${catColor};font-weight:500">${esc(catLabel)}</td><td style="color:${sevColor}">${esc(e.severity)}</td><td class="mono" style="font-size:11px">${esc(e.file||'--')}</td><td style="font-size:12px">${esc(e.detail)}</td></tr>`;
    });
    tbl += '</tbody></table>';
    card.innerHTML = tbl;
  }
  container.appendChild(card);
}

async function fetchArchChangelog() {
  const category = document.getElementById('chlogCategory').value;
  const severity = document.getElementById('chlogSeverity').value;
  let url = '/api/arch-changelog?limit=200';
  if (category) url += '&category=' + encodeURIComponent(category);
  if (severity) url += '&severity=' + encodeURIComponent(severity);
  try { archChangelog = await api(url); toast('查询完成: ' + archChangelog.length + ' 条', 'ok'); renderCurrentView(); }
  catch(e) { toast('查询失败: ' + e.message, 'err'); }
}
