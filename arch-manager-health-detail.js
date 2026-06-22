/**
 * arch-manager-health-detail.js
 * 健康评分详情面板 — 从 arch-manager-panorama-charts.js 拆分
 *
 * 渲染内容:
 *   - renderHealthDetail(container, healthScore)  // 等级说明、项目统计、扣分原因、改进建议
 *
 * 依赖全局变量 (来自 core.js):
 *   esc
 */

// 五维因子 key -> 中文显示名
const HEALTH_FACTOR_NAMES = {
  layer_balance: '层级平衡',
  file_granularity: '文件粒度',
  symbol_density: '符号密度',
  dependency_depth: '依赖深度',
  interface_ratio: '接口率'
};

// 评分 -> 颜色
function scoreColor(score) {
  if (score >= 80) return '#30d158';
  if (score >= 70) return '#ff9f0a';
  if (score >= 50) return '#ff9f0a';
  return '#ff453a';
}

// impact -> emoji
function impactEmoji(impact) {
  if (!impact) return '💡';
  const s = String(impact).toLowerCase();
  if (s === 'critical' || s === 'high' || s.includes('严重') || s.includes('火')) return '🔥';
  if (s === 'warning' || s === 'medium' || s.includes('警告') || s.includes('中')) return '⚠️';
  return '💡';
}

// 从建议文本推断 impact 级别（若无 impact 字段时回退）
function guessImpactFromText(text) {
  const s = String(text);
  if (/(严重|critical|优先|立即|🔥|错误|违规)/i.test(s)) return 'high';
  if (/(警告|warning|建议|⚠️|较多)/i.test(s)) return 'medium';
  return 'low';
}

// HTML 转义
function escHtml(s) {
  return String(s == null ? '' : s)
    .replace(/&/g, '&amp;').replace(/</g, '&lt;')
    .replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// 渲染等级说明 & 项目统计
function renderGradeAndStats(hs) {
  const grade = hs.grade || '—';
  const gradeDesc = hs.grade_description || '暂无等级说明';
  const overall = Math.round(hs.overall || 0);
  const stats = hs.stats || {};
  return `
    <div class="card" style="margin-bottom:12px">
      <div class="card-title">评分等级</div>
      <div style="display:flex;gap:16px;align-items:flex-start;flex-wrap:wrap">
        <div style="font-size:48px;font-weight:700;color:${scoreColor(overall)};line-height:1">${grade}</div>
        <div style="flex:1;min-width:240px">
          <div style="font-size:16px;color:#f5f5f7;margin-bottom:6px">综合分数 ${overall}</div>
          <div style="font-size:13px;color:#98989d;line-height:1.6">${escHtml(gradeDesc)}</div>
        </div>
      </div>
      <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(130px,1fr));gap:8px;margin-top:12px;padding-top:12px;border-top:1px solid #2c2c2e">
        <div><div style="color:#6e6e73;font-size:11px">文件总数</div><div style="color:#f5f5f7;font-size:16px;font-weight:600">${stats.total_files ?? 0}</div></div>
        <div><div style="color:#6e6e73;font-size:11px">代码行数</div><div style="color:#f5f5f7;font-size:16px;font-weight:600">${stats.total_lines ?? 0}</div></div>
        <div><div style="color:#6e6e73;font-size:11px">平均行数/文件</div><div style="color:#f5f5f7;font-size:16px;font-weight:600">${(stats.avg_lines_per_file ?? 0).toFixed(1)}</div></div>
        <div><div style="color:#6e6e73;font-size:11px">平均符号/文件</div><div style="color:#f5f5f7;font-size:16px;font-weight:600">${(stats.avg_symbols_per_file ?? 0).toFixed(1)}</div></div>
        <div><div style="color:#6e6e73;font-size:11px">原语数量</div><div style="color:#f5f5f7;font-size:16px;font-weight:600">${stats.primitive_count ?? 0}</div></div>
      </div>
    </div>`;
}

// 渲染因子详情（扣分原因面板）
function renderFactorDetails(hs) {
  const factors = Array.isArray(hs.factor_details) && hs.factor_details.length > 0
    ? hs.factor_details
    : Object.keys(hs.factors || {}).map(k => ({
        key: k, name: HEALTH_FACTOR_NAMES[k] || k,
        score: hs.factors[k], explanation: '—', raw_value: '—', threshold: '—', suggestion: '继续保持'
      }));

  // 过滤出 score < 100 的；按 score 升序（最差的排前）
  const problem = factors
    .filter(f => (f.score ?? 0) < 100)
    .sort((a, b) => (a.score ?? 0) - (b.score ?? 0));

  if (problem.length === 0) {
    return `<div class="card" style="margin-bottom:12px">
      <div class="card-title">因子详情</div>
      <div style="color:#30d158;font-size:13px;padding:8px 0">✅ 所有五维因子均处于理想水平，无需改进</div>
    </div>`;
  }

  const rows = problem.map(f => {
    const s = Math.round(f.score ?? 0);
    const isError = s < 70;
    const color = scoreColor(s);
    const name = f.name || HEALTH_FACTOR_NAMES[f.key] || f.key || '—';
    const borderCls = isError ? 'border-left:3px solid #ff453a;padding-left:10px;margin-left:-13px' : 'border-left:3px solid #ff9f0a;padding-left:10px;margin-left:-13px';
    return `
      <div style="padding:10px 0;border-bottom:1px dashed #2c2c2e;${borderCls}">
        <div style="display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:4px">
          <div style="font-size:14px;font-weight:600;color:#f5f5f7">${isError ? '⚠️ ' : ''}${escHtml(name)} <span style="color:#6e6e73;font-weight:400;font-size:11px">(${escHtml(f.key || '')})</span></div>
          <div style="font-size:14px;font-weight:700;color:${color}">${s} 分</div>
        </div>
        <div style="font-size:12px;color:#98989d;line-height:1.6;margin-bottom:4px">
          <b style="color:#c7c7cc">原因：</b>${escHtml(f.explanation || '—')}
        </div>
        <div style="font-size:12px;color:#98989d;line-height:1.6;margin-bottom:4px">
          <b style="color:#c7c7cc">当前值：</b>${escHtml(f.raw_value || '—')} &nbsp;·&nbsp; <b style="color:#c7c7cc">阈值：</b>${escHtml(f.threshold || '—')}
        </div>
        <div style="font-size:12px;color:#ffd426;line-height:1.6">
          💡 ${escHtml(f.suggestion || '建议优化该因子')}
        </div>
      </div>`;
  }).join('');

  return `<div class="card" style="margin-bottom:12px">
    <div class="card-title">扣分原因（${problem.length} 项待改进）</div>
    ${rows}
  </div>`;
}

// 渲染改进建议列表
function renderSuggestions(hs) {
  const raw = Array.isArray(hs.suggestions) ? hs.suggestions : [];
  if (raw.length === 0) {
    return `<div class="card" style="margin-bottom:12px">
      <div class="card-title">改进建议</div>
      <div style="color:#98989d;font-size:13px;padding:8px 0">暂无改进建议</div>
    </div>`;
  }

  // 解析：支持纯字符串数组，也支持 {"impact":"high","text":"..."}
  const items = raw.map((s, idx) => {
    if (s && typeof s === 'object' && s.text) {
      return { impact: s.impact || guessImpactFromText(s.text), text: s.text };
    }
    return { impact: guessImpactFromText(String(s)), text: String(s) };
  });

  // 排序：🔥 > ⚠️ > 💡
  const rank = { high: 0, medium: 1, low: 2 };
  items.sort((a, b) => (rank[a.impact] ?? 9) - (rank[b.impact] ?? 9));

  const ol = items.map(it => {
    const emoji = impactEmoji(it.impact);
    return `<li style="padding:6px 0;font-size:13px;color:#f5f5f7;line-height:1.6;list-style:none;counter-increment:sug">
      <span style="color:#6e6e73;font-variant-numeric:tabular-nums">${emoji} </span>${escHtml(it.text)}
    </li>`;
  }).join('');

  return `<div class="card" style="margin-bottom:12px">
    <div class="card-title">改进建议（${items.length} 条）</div>
    <ol style="margin:0;padding:0;counter-reset:sug">${ol}</ol>
  </div>`;
}

// 主入口：在给定 container 末尾渲染详情面板
function renderHealthDetail(container, healthScore) {
  if (!healthScore) return '';
  const html = renderGradeAndStats(healthScore) + renderFactorDetails(healthScore) + renderSuggestions(healthScore);
  const wrap = document.createElement('div');
  wrap.style.cssText = 'margin-bottom:12px';
  wrap.innerHTML = html;
  container.appendChild(wrap);
  return html;
}

// 雷达图 tooltip formatter：显示因子详情
function radarTooltipFormatter(factorDetails, factorsMap) {
  const detailByKey = {};
  (Array.isArray(factorDetails) ? factorDetails : []).forEach(f => {
    if (f && f.key) detailByKey[f.key] = f;
    else if (f && f.name) {
      const reverseMap = {};
      Object.keys(HEALTH_FACTOR_NAMES).forEach(k => { reverseMap[HEALTH_FACTOR_NAMES[k]] = k; });
      const k = reverseMap[f.name] || f.name;
      detailByKey[k] = f;
    }
  });

  return function(params) {
    if (!params || !params.value) return '';
    // 雷达图 hover 一个顶点 — params.name 是中文名称（如"层级平衡"）
    const name = params.name;
    const score = typeof params.value === 'number' ? params.value : (params.value && params.value[0]);
    const reverseMap = {};
    Object.keys(HEALTH_FACTOR_NAMES).forEach(k => { reverseMap[HEALTH_FACTOR_NAMES[k]] = k; });
    const key = reverseMap[name] || name;
    const detail = detailByKey[key];
    const color = scoreColor(score);
    let html = `<div style="font-size:13px;font-weight:600;color:#f5f5f7;margin-bottom:4px">${escHtml(name)}</div>`;
    html += `<div style="font-size:12px;color:${color};margin-bottom:6px">得分：${Math.round(score)} / 100</div>`;
    if (detail) {
      if (detail.raw_value) html += `<div style="font-size:11px;color:#98989d;margin-bottom:2px">当前值：${escHtml(detail.raw_value)}</div>`;
      if (detail.threshold) html += `<div style="font-size:11px;color:#98989d;margin-bottom:2px">阈值：${escHtml(detail.threshold)}</div>`;
      if (detail.explanation) html += `<div style="font-size:11px;color:#98989d;margin-top:4px">${escHtml(detail.explanation)}</div>`;
      if (detail.suggestion) html += `<div style="font-size:11px;color:#ffd426;margin-top:4px">💡 ${escHtml(detail.suggestion)}</div>`;
    } else {
      html += `<div style="font-size:11px;color:#6e6e73;margin-top:4px">暂无详细数据</div>`;
    }
    return html;
  };
}
