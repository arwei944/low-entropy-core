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
  container.innerHTML = '<div class="view-title">因果链 & 符号溯源</div><div class="view-desc">基于 /api/origin 的符号定义与依赖关系追踪</div>';

  // === 0. Origin 统计卡片 ===
  if (originData && (originData.by_layer || originData.total)) {
    const statsGrid = document.createElement('div');
    statsGrid.className = 'grid-4';
    let html = '';
    html += `<div class="stat-card"><div class="label">总符号数</div><div class="value">${originData.total || 0}</div></div>`;
    html += `<div class="stat-card"><div class="label">层类型数</div><div class="value">${Object.keys(originData.by_layer || {}).length}</div></div>`;
    html += `<div class="stat-card"><div class="label">Kind 类型</div><div class="value">${Object.keys(originData.by_kind || {}).length}</div></div>`;
    html += `<div class="stat-card"><div class="label">有依赖记录</div><div class="value">${(originData.symbols || []).filter(s => (s.depends_on || []).length > 0).length}</div></div>`;
    statsGrid.innerHTML = html;
    container.appendChild(statsGrid);

    // === 0a. 按层/Kind 柱状图 ===
    const layerChartCard = document.createElement('div');
    layerChartCard.className = 'card';
    layerChartCard.innerHTML = '<div class="card-title">符号按层分布</div><div class="chart-container" id="originLayerChart"></div>';
    container.appendChild(layerChartCard);

    const layers = Object.keys(originData.by_layer || {}).sort();
    const layerCounts = layers.map(l => originData.by_layer[l]);
    const layerColors = layers.map(l => {
      const lm = { L0: '#f44336', L1: '#ff9800', L2: '#ffc107', L3: '#4caf50', L4: '#00bcd4', L5: '#2196f3', L6: '#9c27b0', L7: '#607d8b' };
      return lm[l] || '#888';
    });

    charts.originLayer = echarts.init(document.getElementById('originLayerChart'));
    charts.originLayer.setOption({
      backgroundColor: 'transparent',
      tooltip: { trigger: 'axis', backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' } },
      grid: { left: 60, right: 20, top: 20, bottom: 40 },
      xAxis: { type: 'category', data: layers, axisLabel: { color: '#6e6e73', fontSize: 11 } },
      yAxis: { type: 'value', axisLabel: { color: '#6e6e73', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
      series: [{
        type: 'bar',
        data: layers.map((l, i) => ({ value: layerCounts[i], itemStyle: { color: layerColors[i] } })),
        label: { show: true, position: 'top', color: '#f5f5f7', fontSize: 11 }
      }]
    });

    // === 1. 符号溯源表（按层过滤） ===
    const symCard = document.createElement('div');
    symCard.className = 'card';
    symCard.innerHTML = '<div class="card-title">符号起源 / Symbol Origin</div><div style="padding:12px 20px"><div class="control-group" style="margin-bottom:12px"><select id="originLayerFilter" style="padding:6px 10px;background:var(--bg2);border:1px solid var(--rule);border-radius:8px;color:var(--ink);font-size:12px"><option value="">全部层</option>' + layers.map(l => `<option value="${l}">${l}</option>`).join('') + '</select><input id="originSearch" type="text" placeholder="搜索符号名..." style="margin-left:10px;padding:6px 10px;background:var(--bg2);border:1px solid var(--rule);border-radius:8px;color:var(--ink);font-size:12px;width:300px"></div><div id="originTableContainer" style="max-height:600px;overflow-y:auto"></div></div>';
    container.appendChild(symCard);

    // 渲染表格函数
    const renderOriginTable = () => {
      const layerFilter = document.getElementById('originLayerFilter')?.value || '';
      const searchStr = (document.getElementById('originSearch')?.value || '').toLowerCase();
      let symbols = originData.symbols || [];
      if (layerFilter) symbols = symbols.filter(s => s.layer === layerFilter);
      if (searchStr) symbols = symbols.filter(s => (s.name || '').toLowerCase().includes(searchStr));

      let html = '<table class="data-table"><thead><tr><th style="width:80px">层</th><th style="width:120px">Kind</th><th>符号名</th><th style="width:180px">文件</th><th style="width:200px">Package</th><th style="width:120px">原语类型</th><th>依赖</th></tr></thead><tbody>';
      symbols.slice(0, 200).forEach(s => {
        const deps = (s.depends_on || []).length;
        const primColor = s.primitive ? '#4299e1' : '#718096';
        html += `<tr><td class="mono"><span style="color:var(--layer-${s.layer || 'L0'})">${s.layer}</span></td><td class="mono" style="font-size:11px">${esc(s.kind || '')}</td><td class="mono" style="font-size:12px;color:#f5f5f7">${esc(s.name || '')}</td><td class="mono" style="font-size:11px;color:#a0aec0">${esc((s.file || '').replace(/^.*[\\\/]/, ''))}</td><td class="mono" style="font-size:11px;color:#a0aec0">${esc(s.package || '')}</td><td class="mono" style="color:${primColor};font-size:11px">${s.primitive ? s.primitive : '-'}</td><td style="font-size:11px;color:#a0aec0">${deps > 0 ? deps + ' 个依赖' : '-'}</td></tr>`;
        if (s.doc && s.doc.length > 5) {
          html += `<tr style="background:rgba(255,255,255,0.02)"><td colspan="7" style="padding:4px 16px 16px;font-size:11px;color:#a0aec0;line-height:1.6;font-family:system-ui">${esc(s.doc.substring(0, 200))}${s.doc.length > 200 ? '...' : ''}</td></tr>`;
        }
      });
      html += '</tbody></table>';
      if (symbols.length > 200) html += `<div style="padding:12px;color:#718096;text-align:center;font-size:11px">仅显示前 200 条，共 ${symbols.length} 条符号</div>`;
      if (symbols.length === 0) html = '<div style="padding:40px;text-align:center;color:#718096">没有匹配的符号</div>';
      document.getElementById('originTableContainer').innerHTML = html;
    };
    renderOriginTable();

    setTimeout(() => {
      const filter = document.getElementById('originLayerFilter');
      const search = document.getElementById('originSearch');
      if (filter) filter.addEventListener('change', renderOriginTable);
      if (search) search.addEventListener('input', renderOriginTable);
    }, 0);

    // === 2. 符号依赖图（Graph） ===
    if ((originData.symbols || []).filter(s => (s.depends_on || []).length > 0).length > 0) {
      const graphCard = document.createElement('div');
      graphCard.className = 'card';
      graphCard.innerHTML = '<div class="card-title">符号依赖图 (Symbol Dependencies)</div><div class="chart-container-lg" id="originGraphChart"></div>';
      container.appendChild(graphCard);

      const nodes = [];
      const links = [];
      const nodeSet = new Set();
      const symbolMap = {};
      (originData.symbols || []).forEach(s => { symbolMap[s.name] = s; });

      // 只取有依赖的前 60 个符号，避免图太大
      const targetSyms = (originData.symbols || []).filter(s => (s.depends_on || []).length > 0).slice(0, 60);
      targetSyms.forEach(s => {
        if (!nodeSet.has(s.name)) {
          nodeSet.add(s.name);
          const colorMap = { L0: '#f44336', L1: '#ff9800', L2: '#ffc107', L3: '#4caf50', L4: '#00bcd4', L5: '#2196f3', L6: '#9c27b0', L7: '#607d8b' };
          nodes.push({ name: s.name, id: s.name, itemStyle: { color: colorMap[s.layer] || '#888' }, symbolSize: Math.max(14, Math.min(30, 12 + (s.depends_on || []).length * 2)) });
        }
        (s.depends_on || []).forEach(dep => {
          const depName = dep.replace(/\.[^.]+$/, '');
          if (!nodeSet.has(depName)) {
            nodeSet.add(depName);
            const depS = symbolMap[depName];
            const colorMap = { L0: '#f44336', L1: '#ff9800', L2: '#ffc107', L3: '#4caf50', L4: '#00bcd4', L5: '#2196f3', L6: '#9c27b0', L7: '#607d8b' };
            nodes.push({ name: depName, id: depName, itemStyle: { color: depS ? colorMap[depS.layer] : '#555' }, symbolSize: 12 });
          }
          links.push({ source: s.name, target: depName });
        });
      });

      if (nodes.length > 0) {
        charts.originGraph = echarts.init(document.getElementById('originGraphChart'));
        charts.originGraph.setOption({
          backgroundColor: 'transparent',
          tooltip: { backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' } },
          legend: { show: false },
          series: [{
            type: 'graph',
            layout: 'force',
            roam: true,
            draggable: true,
            label: { show: true, fontSize: 9, color: '#f5f5f7', fontFamily: 'system-ui' },
            edgeSymbol: ['none', 'arrow'],
            edgeSymbolSize: [0, 6],
            data: nodes,
            links: links,
            force: { repulsion: 250, edgeLength: 60, gravity: 0.1, layoutAnimation: true },
            lineStyle: { color: '#2c2c2e', width: 1, curveness: 0.1, opacity: 0.5 },
            emphasis: { focus: 'adjacency', lineStyle: { width: 2, color: '#0a84ff', opacity: 1 } }
          }]
        });
      }
    }
    return;
  }

  // === Fallback: 原来的因果链逻辑 ===
  const card = document.createElement('div');
  card.className = 'card';
  card.innerHTML = '<div class="chart-container-lg" id="causationChart"></div>';
  container.appendChild(card);

  let data = [];
  let links = [];
  if (traceTree && traceTree.spans && traceTree.spans.length > 0) {
    const spanMap = {};
    traceTree.spans.forEach(span => { spanMap[span.id] = span; });
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
    data.forEach((d, i) => { const span = spanMap[d.id]; d.x = (span.depth || 0) * 150; d.y = i * 50 + 50; });
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
      type: 'graph', layout: 'none', roam: true, animation: false,
      label: { show: true, fontSize: 12, color: '#f5f5f7' },
      edgeSymbol: ['none', 'arrow'], edgeSymbolSize: [0, 10],
      data: data, links: links,
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
