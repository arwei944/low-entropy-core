/**
 * arch-manager-changelog.js
 * 架构变动日志渲染函数
 *
 * 从 arch-manager-migration.js 拆分
 *
 * 包含函数:
 *   - renderArchChangelog      架构变动日志
 *   - fetchArchChangelog       查询变动日志
 *   - connectChangelogSSE      SSE 订阅
 *   - prependChangelogEntry    插入新条目
 *   - renderChangelogPagination 分页控件渲染
 *
 * 依赖全局变量 (来自 core.js):
 *   archChangelog, archChangelogStats, esc, toast, api, renderCurrentView
 */

// ============================================================
// 分页状态
// ============================================================
let changelogPage = 0;
const changelogPageSize = 50;

// ============================================================
// SSE 订阅
// ============================================================
function connectChangelogSSE() {
    const es = new EventSource('/api/sse/arch-changelog');
    es.onmessage = (e) => {
        const data = JSON.parse(e.data);
        if (data.type === 'ping') return;
        // 将新条目插入到列表顶部
        prependChangelogEntry(data);
    };
    es.onerror = () => {
        setTimeout(connectChangelogSSE, 5000); // 5秒后重连
    };
    return es;
}

function prependChangelogEntry(entry) {
    // 插入到全局数据顶部
    archChangelog.unshift(entry);
    // 限制全局数据最多100条
    if (archChangelog.length > 100) {
        archChangelog.length = 100;
    }
    // 更新统计
    if (archChangelogStats) {
        archChangelogStats.total = (archChangelogStats.total || 0) + 1;
        if (entry.category && archChangelogStats.by_category) {
            archChangelogStats.by_category[entry.category] = (archChangelogStats.by_category[entry.category] || 0) + 1;
        }
    }
    // 重新渲染当前视图
    renderCurrentView();
}

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
    archChangelog.forEach(e => {
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
  renderChangelogPagination(container);
}

async function fetchArchChangelog(page) {
  // 保持兼容性：无参数时默认第一页
  if (typeof page !== 'number') page = 0;
  changelogPage = page;
  const category = document.getElementById('chlogCategory').value;
  const severity = document.getElementById('chlogSeverity').value;
  let url = '/api/arch-changelog?offset=' + (page * changelogPageSize) + '&limit=' + changelogPageSize;
  if (category) url += '&category=' + encodeURIComponent(category);
  if (severity) url += '&severity=' + encodeURIComponent(severity);
  try {
    const result = await api(url);
    // 兼容后端返回数组或 {items,total} 对象
    if (Array.isArray(result)) {
      archChangelog = result;
    } else if (result && Array.isArray(result.items)) {
      archChangelog = result.items;
    } else {
      archChangelog = [];
    }
    toast('查询完成: ' + archChangelog.length + ' 条', 'ok');
    renderCurrentView();
  }
  catch(e) { toast('查询失败: ' + e.message, 'err'); }
}

function renderChangelogPagination(container) {
  const hasPrev = changelogPage > 0;
  const hasNext = archChangelog.length >= changelogPageSize;
  const wrap = document.createElement('div');
  wrap.style.cssText = 'display:flex;align-items:center;justify-content:center;gap:12px;padding:12px 0;';
  wrap.innerHTML = `
    <button class="btn" onclick="fetchArchChangelog(${changelogPage - 1})" ${hasPrev ? '' : 'disabled'} style="padding:6px 14px;font-size:12px">上一页</button>
    <span style="font-size:12px;color:var(--ink);">第 ${changelogPage + 1} 页</span>
    <button class="btn" onclick="fetchArchChangelog(${changelogPage + 1})" ${hasNext ? '' : 'disabled'} style="padding:6px 14px;font-size:12px">下一页</button>
  `;
  container.appendChild(wrap);
}
