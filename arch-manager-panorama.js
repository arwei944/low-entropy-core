/**
 * arch-manager-panorama.js
 * 架构全景视图 — 渲染函数集
 *
 * 提取自: arch-manager.html (第306行 ~ 第726行)
 *
 * 包含函数:
 *   - renderFileTreeView    文件树
 *   - showFileDetail        文件详情面板
 *   - renderViolationsView  违规看板
 *   - renderPrimitivesView  原语分布 (旭日图)
 *   - renderLayerMatrix     层级矩阵 (堆叠柱状图)
 *
 * 依赖全局变量 (来自 core.js):
 *   archData, violations, primitives, charts, esc
 */

// ============================================================
// 文件树
// ============================================================
function renderFileTreeView(container) {
  container.innerHTML = '<div class="view-title">文件树</div><div class="view-desc">按层级分组浏览所有文件，点击查看详情</div>';

  const files = archData?.files || [];
  const layers = archData?.layers || [];

  if (files.length === 0) {
    const empty = document.createElement('div');
    empty.className = 'empty-state';
    empty.innerHTML = '<svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/></svg><p>暂无文件数据</p>';
    container.appendChild(empty);
    return;
  }

  // Group files by layer
  const layerMap = {};
  files.forEach(f => {
    const layer = f.layer || 'UNKNOWN';
    if (!layerMap[layer]) layerMap[layer] = [];
    layerMap[layer].push(f);
  });

  // Sort layers by layer number
  const sortedLayers = Object.keys(layerMap).sort((a, b) => {
    const na = parseInt(a.replace('L', '')) || 99;
    const nb = parseInt(b.replace('L', '')) || 99;
    return na - nb;
  });

  const treeRoot = document.createElement('div');
  treeRoot.className = 'file-tree-root';

  sortedLayers.forEach(layerKey => {
    const layerFiles = layerMap[layerKey];
    const layerInfo = layers.find(l => l.layer === layerKey);
    const layerColor = layerInfo?.color || '#888';
    const layerName = layerInfo ? (layerKey + ' ' + layerInfo.name) : layerKey;

    const section = document.createElement('div');
    section.className = 'file-tree-section';

    const header = document.createElement('div');
    header.className = 'file-tree-layer-header';
    header.innerHTML = `
      <svg class="ft-chevron" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="9 18 15 12 9 6"/></svg>
      <span class="ft-color" style="background:${layerColor}"></span>
      <span class="ft-name">${esc(layerName)}</span>
      <span class="ft-count">${layerFiles.length} 文件</span>
    `;

    const filesList = document.createElement('div');
    filesList.className = 'file-tree-files';
    filesList.style.maxHeight = (layerFiles.length * 36 + 20) + 'px';

    layerFiles.sort((a, b) => (a.name || '').localeCompare(b.name || '')).forEach(f => {
      const item = document.createElement('div');
      item.className = 'file-tree-item';
      const ext = (f.name || '').split('.').pop();
      const extColor = ext === 'go' ? '#00ADD8' : ext === 'ts' ? '#3178C6' : ext === 'js' ? '#F7DF1E' : ext === 'py' ? '#3776AB' : ext === 'rs' ? '#DEA584' : 'var(--muted)';
      item.innerHTML = `
        <svg class="ft-file-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>
        <span class="ft-file-name" style="color:${extColor}" title="${esc(f.name)}">${esc(f.name)}</span>
        <span class="ft-file-meta">${f.lines || 0}行 ${(Array.isArray(f.symbols) ? f.symbols.length : (f.symbols || 0))}符号</span>
      `;
      item.addEventListener('click', () => showFileDetail(f));
      filesList.appendChild(item);
    });

    header.addEventListener('click', () => {
      const chev = header.querySelector('.ft-chevron');
      const isOpen = !filesList.classList.contains('collapsed');
      if (isOpen) {
        filesList.classList.add('collapsed');
        filesList.style.maxHeight = '0';
        chev.classList.remove('open');
      } else {
        filesList.classList.remove('collapsed');
        filesList.style.maxHeight = (layerFiles.length * 36 + 20) + 'px';
        chev.classList.add('open');
      }
    });
    // Start open
    header.querySelector('.ft-chevron').classList.add('open');

    section.appendChild(header);
    section.appendChild(filesList);
    treeRoot.appendChild(section);
  });

  container.appendChild(treeRoot);
}

function showFileDetail(f) {
  const body = document.getElementById('rightPanelBody');
  const layerInfo = archData?.layers?.find(l => l.layer === f.layer);
  const layerColor = layerInfo?.color || '#888';

  let html = '<div class="detail-field"><div class="dl">文件路径</div><div class="dv mono">' + esc(f.path || f.name) + '</div></div>';
  html += '<div class="detail-field"><div class="dl">层级</div><div class="dv"><span style="display:inline-block;width:10px;height:10px;border-radius:3px;background:' + layerColor + ';margin-right:6px"></span>' + esc(f.layer + (layerInfo ? ' ' + layerInfo.name : '')) + '</div></div>';
  html += '<div class="detail-field"><div class="dl">行数</div><div class="dv">' + (f.lines || 0) + '</div></div>';
  html += '<div class="detail-field"><div class="dl">符号数</div><div class="dv">' + (Array.isArray(f.symbols) ? f.symbols.length : (f.symbols || 0)) + '</div></div>';

  if (f.package) {
    html += '<div class="detail-field"><div class="dl">包名</div><div class="dv mono">' + esc(f.package) + '</div></div>';
  }

  if (f.imports && f.imports.length > 0) {
    html += '<div class="detail-field"><div class="dl">导入 (' + f.imports.length + ')</div><div class="dd">' + f.imports.map(i => esc(typeof i === 'string' ? i : i.path || i)).join('\n') + '</div></div>';
  }

  const syms = f.symbols || f.symbols_list;
  if (Array.isArray(syms) && syms.length > 0) {
    html += '<div class="detail-field"><div class="dl">符号列表 (' + syms.length + ')</div><div class="dd">' + syms.map(s => esc(typeof s === 'string' ? s : (s.name || '') + (s.kind ? ' (' + s.kind + ')' : ''))).join('\n') + '</div></div>';
  }

  if (f.depends_on && f.depends_on.length > 0) {
    html += '<div class="detail-field"><div class="dl">依赖 (' + f.depends_on.length + ')</div><div class="dd">' + f.depends_on.map(d => esc(d)).join('\n') + '</div></div>';
  }

  body.innerHTML = html;
}

// ============================================================
// 架构全景 — 违规看板
// ============================================================
function renderViolationsView(container) {
  container.innerHTML = '<div class="view-title">违规看板</div><div class="view-desc">按严重程度分组的架构违规</div>';
  const groups = { error: [], warning: [], info: [] };
  violations.forEach(v => {
    const sev = v.severity || 'info';
    if (groups[sev]) groups[sev].push(v);
    else groups.info.push(v);
  });

  ['error', 'warning', 'info'].forEach(sev => {
    const list = groups[sev];
    if (list.length === 0) return;
    const card = document.createElement('div');
    card.className = 'card';
    const titleColor = sev === 'error' ? 'var(--red)' : sev === 'warning' ? 'var(--orange)' : 'var(--accent)';
    card.innerHTML = '<div class="card-title" style="color:' + titleColor + '">' + (sev === 'error' ? '严重' : sev === 'warning' ? '警告' : '提示') + ' (' + list.length + ')</div>';
    list.forEach(v => {
      const item = document.createElement('div');
      item.className = 'violation-card';
      item.innerHTML = '<span class="sev ' + sev + '"></span><div class="body"><div class="msg">' + esc(v.message) + '</div><div class="detail">' + esc(v.detail || '') + '</div></div><span class="badge">' + (v.rule || 'RULE') + '</span>';
      card.appendChild(item);
    });
    container.appendChild(card);
  });
  if (violations.length === 0) {
    container.innerHTML += '<div class="card" style="text-align:center;padding:60px;color:var(--dim)"><svg width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="#30d158" stroke-width="1.5"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg><p style="margin-top:12px;font-size:13px">架构无违规，状态良好</p></div>';
  }
}

// ============================================================
// 架构全景 — 原语分布
// ============================================================
function renderPrimitivesView(container) {
  container.innerHTML = '<div class="view-title">原语分布</div><div class="view-desc">架构原语的层级旭日图分布</div>';
  const card = document.createElement('div');
  card.className = 'card';
  card.innerHTML = '<div class="chart-container-lg" id="sunburstChart"></div>';
  container.appendChild(card);

  const data = [];
  const layers = archData?.layers || [];
  layers.forEach(l => {
    const layerPrims = primitives.filter(p => p.layer === l.layer);
    const children = layerPrims.map(p => ({ name: p.name, value: p.count || 1, itemStyle: { color: l.color } }));
    if (children.length === 0) children.push({ name: '(无)', value: 1, itemStyle: { color: l.color, opacity: 0.3 } });
    data.push({ name: l.layer + ' ' + l.name, itemStyle: { color: l.color }, children });
  });

  charts.sunburst = echarts.init(document.getElementById('sunburstChart'));
  charts.sunburst.setOption({
    backgroundColor: 'transparent',
    series: [{
      type: 'sunburst',
      data: data.length ? data : [{ name: '暂无数据', value: 1, itemStyle: { color: '#2c2c2e' } }],
      radius: ['15%', '80%'],
      label: { color: '#f5f5f7', fontSize: 11 },
      itemStyle: { borderColor: '#0a0a0a', borderWidth: 2 },
      emphasis: { focus: 'ancestor' }
    }]
  });
}

// ============================================================
// 架构全景 — 层级矩阵
// ============================================================
function renderLayerMatrix(container) {
  container.innerHTML = '<div class="view-title">层级矩阵</div><div class="view-desc">各层级的文件、行数、符号堆叠分布</div>';
  const card = document.createElement('div');
  card.className = 'card';
  card.innerHTML = '<div class="chart-container-lg" id="layerMatrixChart"></div>';
  container.appendChild(card);

  const layers = archData?.layers || [];
  const cats = layers.map(l => l.layer + ' ' + l.name);
  const files = layers.map(l => l.files);
  const lines = layers.map(l => Math.round(l.lines / 100));
  const symbols = layers.map(l => l.symbols);

  charts.layerMatrix = echarts.init(document.getElementById('layerMatrixChart'));
  charts.layerMatrix.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', backgroundColor: '#1c1c1e', borderColor: '#2c2c2e', textStyle: { color: '#f5f5f7' } },
    legend: { textStyle: { color: '#98989d', fontSize: 11 }, top: 0 },
    grid: { left: 50, right: 20, top: 30, bottom: 60 },
    xAxis: { type: 'category', data: cats, axisLabel: { color: '#98989d', fontSize: 10, rotate: 30 }, axisLine: { lineStyle: { color: '#2c2c2e' } } },
    yAxis: { type: 'value', axisLabel: { color: '#98989d', fontSize: 10 }, splitLine: { lineStyle: { color: '#2c2c2e' } } },
    series: [
      { name: '文件', type: 'bar', stack: 'total', data: files, itemStyle: { color: '#0a84ff' } },
      { name: '行(x100)', type: 'bar', stack: 'total', data: lines, itemStyle: { color: '#30d158' } },
      { name: '符号', type: 'bar', stack: 'total', data: symbols, itemStyle: { color: '#ff9f0a' } }
    ]
  });
}
