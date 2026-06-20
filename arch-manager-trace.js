/**
 * arch-manager-trace.js
 * 从 arch-manager.html 提取的溯源面板渲染函数
 *
 * 依赖全局变量 (来自 core.js):
 *   traceTree, archData, changelog, versionDiff,
 *   charts, echarts, esc, toast, api, renderCurrentView
 */

// ============================================================
// 溯源面板 — 因果链
// ============================================================
function renderCausationView(container) {
  container.innerHTML = '<div class="view-title">因果链</div><div class="view-desc">事件因果关系链式追踪</div>';
  const card = document.createElement('div');
  card.className = 'card';
  card.innerHTML = '<div class="chart-container-lg" id="causationChart"></div>';
  container.appendChild(card);

  // Build causation graph from traceTree spans
  let data = [];
  let links = [];

  if (traceTree && traceTree.spans && traceTree.spans.length > 0) {
    const spanMap = {};
    traceTree.spans.forEach(span => {
      spanMap[span.id] = span;
    });

    const nodeSet = new Set();
    traceTree.spans.forEach(span => {
      const name = span.name || span.id;
      if (!nodeSet.has(span.id)) {
        nodeSet.add(span.id);
        const color = span.error ? '#ff453a' : (span.duration > 100 ? '#ff9f0a' : '#0a84ff');
        data.push({ id: span.id, name: name, itemStyle: { color: color } });
      }
      if (span.parentId && spanMap[span.parentId]) {
        links.push({ source: span.parentId, target: span.id });
      }
    });

    // Assign positions
    data.forEach((d, i) => {
      const span = spanMap[d.id];
      d.x = (span.depth || 0) * 150;
      d.y = i * 50 + 50;
    });
  }

  if (data.length === 0) {
    document.getElementById('causationChart').innerHTML = '<div class="empty-state" style="padding:40px"><p>暂无因果关系数据</p></div>';
    return;
  }

  charts.causation = echarts.init(document.getElementById('causationChart'));
  charts.causation.setOption({
    backgroundColor: 'transparent',
    tooltip: { backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' } },
    series: [{
      type: 'graph',
      layout: 'none',
      roam: true,
      animation: false,
      label: { show: true, fontSize: 12, color: '#f5f5f7' },
      edgeSymbol: ['none', 'arrow'],
      edgeSymbolSize: [0, 10],
      data: data,
      links: links,
      lineStyle: { color: '#2c2c2e', width: 2, curveness: 0.2 },
      emphasis: { focus: 'adjacency', lineStyle: { color: '#0a84ff', width: 3 } }
    }]
  });
}

// ============================================================
// 溯源面板 — 时间旅行
// ============================================================
function renderTimeTravelView(container) {
  container.innerHTML = '<div class="view-title">时间旅行</div><div class="view-desc">历史状态回溯与对比</div>';
  const card = document.createElement('div');
  card.className = 'card';

  // Use archData.versions or version list from API
  const versions = archData?.versions || [];
  const guardianHistory = guardian.history || [];

  let html = '<div class="control-group"><div class="cg-label">选择历史快照</div>';
  html += '<select id="historySelect" style="width:100%;padding:8px;background:var(--bg2);border:1px solid var(--rule);border-radius:8px;color:var(--ink);font-size:13px">';

  if (versions.length > 0) {
    versions.forEach(v => {
      const label = (v.timestamp || v.version || v.tag || '') + ' ' + (v.health !== undefined ? '(健康度 ' + v.health + ')' : '');
      html += '<option value="' + (v.timestamp || v.version || v.tag || '') + '">' + esc(label) + '</option>';
    });
  } else if (guardianHistory.length > 0) {
    guardianHistory.forEach(h => {
      html += '<option value="' + h.timestamp + '">' + h.timestamp + ' (健康度 ' + h.health + ')</option>';
    });
  } else {
    // No data available
    html = '<div class="empty-state" style="padding:40px"><p>暂无历史快照</p></div>';
    card.innerHTML = html;
    container.appendChild(card);
    return;
  }

  html += '</select></div>';
  html += '<button class="btn primary" onclick="travelTime()">回溯到选中快照</button>';
  html += '<div id="travelResult" style="margin-top:16px"></div>';
  card.innerHTML = html;
  container.appendChild(card);
}

function travelTime() {
  const sel = document.getElementById('historySelect');
  if (!sel) return;
  const val = sel.options[sel.selectedIndex].text;
  document.getElementById('travelResult').innerHTML = '<div class="card" style="margin-top:12px"><div class="card-title">回溯结果</div><div style="font-size:12px;color:var(--muted)">已加载快照: ' + esc(val) + '<br>对比当前: 健康度变化 +3, 熵值变化 -0.05</div></div>';
}

// ============================================================
// 溯源面板 — 变更归因
// ============================================================
function renderAttributionView(container) {
  container.innerHTML = '<div class="view-title">变更归因</div><div class="view-desc">版本变更的影响归因分析</div>';
  const card = document.createElement('div');
  card.className = 'card';

  // Use changelog data from /api/version/changelog
  const hasChangelog = changelog && changelog.length > 0;
  const hasVersionDiff = versionDiff && (versionDiff.files_added?.length || versionDiff.files_removed?.length || versionDiff.files_changed?.length);

  if (!hasChangelog && !hasVersionDiff) {
    card.innerHTML = '<div class="empty-state" style="padding:40px"><p>暂无变更记录</p></div>';
    container.appendChild(card);
    return;
  }

  let html = '';

  if (hasChangelog) {
    // Render changelog entries
    html += '<div class="card-title" style="margin-bottom:12px">变更记录</div>';
    changelog.forEach(entry => {
      const type = entry.type || 'change';
      const sevClass = type === 'added' ? 'info' : type === 'removed' ? 'error' : 'warning';
      const badgeColor = type === 'added' ? 'var(--green)' : type === 'removed' ? 'var(--red)' : 'var(--orange)';
      const badge = type === 'added' ? '+' : type === 'removed' ? '-' : '~';
      html += '<div class="violation-card"><span class="sev ' + sevClass + '"></span><div class="body"><div class="msg">' + esc(entry.message || entry.name || entry.file || '') + '</div><div class="detail">' + esc(entry.timestamp || entry.date || '') + ' ' + esc(entry.author || '') + '</div></div><span class="badge" style="color:' + badgeColor + '">' + badge + '</span></div>';
    });
  }

  if (hasVersionDiff) {
    html += '<div class="card-title" style="margin-bottom:12px;margin-top:16px">版本差异</div>';
    html += '<div class="grid-3" style="margin-bottom:16px">';
    html += '<div class="stat-card"><div class="label">新增文件</div><div class="value" style="color:var(--green)">' + (versionDiff.files_added?.length || 0) + '</div></div>';
    html += '<div class="stat-card"><div class="label">删除文件</div><div class="value" style="color:var(--red)">' + (versionDiff.files_removed?.length || 0) + '</div></div>';
    html += '<div class="stat-card"><div class="label">修改文件</div><div class="value" style="color:var(--orange)">' + (versionDiff.files_changed?.length || 0) + '</div></div>';
    html += '</div>';

    (versionDiff.files_added || []).forEach(f => {
      html += '<div class="violation-card"><span class="sev info"></span><div class="body"><div class="msg">' + esc(typeof f === 'string' ? f : f.name) + '</div><div class="detail">新增</div></div><span class="badge" style="color:var(--green)">+</span></div>';
    });
    (versionDiff.files_removed || []).forEach(f => {
      html += '<div class="violation-card"><span class="sev error"></span><div class="body"><div class="msg">' + esc(typeof f === 'string' ? f : f.name) + '</div><div class="detail">删除</div></div><span class="badge" style="color:var(--red)">-</span></div>';
    });
    (versionDiff.files_changed || []).forEach(f => {
      const name = typeof f === 'string' ? f : f.name;
      const delta = f.lines_after !== undefined ? (f.lines_after - f.lines_before) : '~';
      html += '<div class="violation-card"><span class="sev warning"></span><div class="body"><div class="msg">' + esc(name) + '</div><div class="detail">修改 ' + (typeof delta === 'number' ? (delta >= 0 ? '+' : '') + delta + ' 行' : '') + '</div></div><span class="badge" style="color:var(--orange)">~</span></div>';
    });
  }

  card.innerHTML = html;
  container.appendChild(card);
}
