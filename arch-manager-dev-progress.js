/**
 * arch-manager-dev-progress.js
 * 开发进度渲染函数
 *
 * 从 arch-manager-observation.js 拆分
 *
 * 包含函数:
 *   - renderDevProgress       开发进度
 *
 * 依赖全局变量 (来自 core.js):
 *   devEvents, esc
 */

// ============================================================
// 开发进度
// ============================================================
function renderDevProgress(container) {
  container.innerHTML = '<div class="view-title">开发进度</div><div class="view-desc">实时监控代码变更、架构刷新和违规检测</div>';

  // Stats row
  const stats = document.createElement('div');
  stats.className = 'grid-4';
  const fileChanges = devEvents.filter(e => e.type === 'file_changed').length;
  const buildDones = devEvents.filter(e => e.type === 'build_done').length;
  const violationsDuring = devEvents.filter(e => e.type === 'violation_found').length;
  const lastBuild = devEvents.find(e => e.type === 'build_done');
  stats.innerHTML = `
    <div class="stat-card"><div class="label">文件变更</div><div class="value">${fileChanges}</div></div>
    <div class="stat-card"><div class="label">架构刷新</div><div class="value">${buildDones}</div></div>
    <div class="stat-card"><div class="label">违规检测</div><div class="value" style="color:${violationsDuring > 0 ? 'var(--orange)' : 'var(--green)'}">${violationsDuring}</div></div>
    <div class="stat-card"><div class="label">最后刷新</div><div class="value" style="font-size:14px">${lastBuild ? new Date(lastBuild.timestamp).toLocaleTimeString('zh-CN') : '--'}</div></div>
  `;
  container.appendChild(stats);

  // Event log
  const card = document.createElement('div');
  card.className = 'card';
  card.style.padding = '0';
  card.style.overflow = 'auto';
  card.style.maxHeight = 'calc(100vh - 350px)';

  if (devEvents.length === 0) {
    card.innerHTML = '<div style="padding:40px;text-align:center;color:var(--dim)">等待开发事件...（需使用 --watch 启动 arch-manager，然后修改 .go 文件）</div>';
  } else {
    let tbl = '<table class="data-table"><thead><tr><th style="width:80px">时间</th><th style="width:80px">类型</th><th>详情</th></tr></thead><tbody>';
    devEvents.slice(0, 100).forEach(e => {
      const time = new Date(e.timestamp).toLocaleTimeString('zh-CN');
      const typeLabel = { file_changed: '文件变更', build_start: '开始刷新', build_done: '刷新完成', violation_found: '违规检测', connected: '已连接' }[e.type] || e.type;
      const typeColor = { file_changed: 'var(--accent)', build_done: 'var(--green)', violation_found: 'var(--orange)', build_start: 'var(--dim)' }[e.type] || 'var(--muted)';
      const icon = e.action === 'created' ? ' +' : e.action === 'deleted' ? ' -' : e.action === 'modified' ? ' ~' : '';
      const detail = e.file ? esc(e.file) + icon : esc(e.message || '');
      tbl += `<tr><td class="mono" style="font-size:11px">${time}</td><td style="color:${typeColor};font-weight:500">${typeLabel}</td><td style="font-size:12px">${detail}</td></tr>`;
    });
    tbl += '</tbody></table>';
    card.innerHTML = tbl;
  }
  container.appendChild(card);

  // Update badge
  const badge = document.getElementById('devEventBadge');
  if (badge && devEvents.length > 0) {
    badge.style.display = 'inline';
    badge.textContent = devEvents.filter(e => e.type !== 'connected').length;
  }
}
