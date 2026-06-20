/**
 * arch-manager-control.js
 * 来源: arch-manager.html
 * 包含控制面板相关的渲染和操作函数（纯提取，未修改任何逻辑）
 *
 * 依赖的全局变量:
 *   runtime, guardian, archData, esc, toast
 *   (以及 core.js 中的其他全局变量)
 */

// ============================================================
// 控制面板 — 采样率控制
// ============================================================
function renderSamplingView(container) {
  container.innerHTML = '<div class="view-title">采样率控制</div><div class="view-desc">动态调整观测采样率</div>';
  const card = document.createElement('div');
  card.className = 'card';
  const current = runtime.sampling || 100;
  card.innerHTML = `
    <div class="control-group">
      <div class="cg-label">全局采样率</div>
      <div class="control-row">
        <input type="range" min="0" max="100" value="${current}" id="samplingSlider" oninput="document.getElementById('samplingVal').textContent=this.value+'%'">
        <span class="val" id="samplingVal">${current}%</span>
      </div>
    </div>
    <div class="control-group">
      <div class="cg-label">按层级采样</div>
      <div id="layerSampling"></div>
    </div>
    <div style="margin-top:16px">
      <button class="btn primary" onclick="applySampling()">应用采样率</button>
      <button class="btn" onclick="resetSampling()">重置</button>
    </div>
  `;
  container.appendChild(card);

  const layerDiv = card.querySelector('#layerSampling');
  const layers = archData?.layers || [];
  layers.forEach(l => {
    const row = document.createElement('div');
    row.className = 'control-row';
    row.innerHTML = '<label>' + l.layer + ' ' + l.name + '</label><input type="range" min="0" max="100" value="100" class="layer-sampling" data-layer="' + l.layer + '"><span class="val">100%</span>';
    layerDiv.appendChild(row);
  });
}

async function applySampling() {
  const global = document.getElementById('samplingSlider').value;
  try {
    await fetch('/api/runtime/sampling-rate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ rate: parseInt(global) })
    });
    toast('采样率已更新为 ' + global + '%', 'ok');
  } catch (e) {
    toast('更新失败: ' + e.message, 'err');
  }
}
function resetSampling() {
  document.getElementById('samplingSlider').value = 100;
  document.getElementById('samplingVal').textContent = '100%';
  document.querySelectorAll('.layer-sampling').forEach(s => { s.value = 100; s.nextElementSibling.textContent = '100%'; });
}

// ============================================================
// 控制面板 — 阈值覆盖
// ============================================================
function renderThresholdView(container) {
  if (!container) { container = document.getElementById('mainContent'); if (!container) return; }
  container.innerHTML = '<div class="view-title">阈值覆盖</div><div class="view-desc">Guardian 阈值的手动覆盖控制</div>';
  const card = document.createElement('div');
  card.className = 'card';
  const thresholds = guardian.thresholds || [
    { name: 'max_entropy', value: 0.5, default: 0.5 },
    { name: 'max_latency_ms', value: 500, default: 500 },
    { name: 'min_health_score', value: 60, default: 60 },
    { name: 'max_error_rate', value: 0.05, default: 0.05 }
  ];
  let html = '';
  thresholds.forEach(t => {
    html += '<div class="control-group">';
    html += '<div class="cg-label">' + esc(t.name) + ' <span style="color:var(--dim);font-weight:400">(默认: ' + t.default + ')</span></div>';
    html += '<div class="control-row"><input type="number" step="any" value="' + t.value + '" id="thresh-' + t.name + '"><button class="btn sm" onclick="overrideThreshold(\'' + t.name + '\')">覆盖</button></div>';
    html += '</div>';
  });
  html += '<div style="margin-top:16px"><button class="btn primary" onclick="saveThresholds()">保存所有阈值</button></div>';
  card.innerHTML = html;
  container.appendChild(card);
}

async function overrideThreshold(name) {
  const val = document.getElementById('thresh-' + name)?.value;
  if (val === undefined) return;
  try {
    await fetch('/api/guardian/thresholds', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, value: parseFloat(val) })
    });
    toast('阈值 ' + name + ' 已覆盖', 'ok');
  } catch (e) {
    toast('覆盖失败: ' + e.message, 'err');
  }
}
async function saveThresholds() {
  toast('阈值已保存', 'ok');
}

// ============================================================
// 控制面板 — What-If推演
// ============================================================
function renderWhatIfView(container) {
  container.innerHTML = '<div class="view-title">What-If推演</div><div class="view-desc">模拟架构变更的影响</div>';
  const card = document.createElement('div');
  card.className = 'card';
  card.innerHTML = `
    <div class="control-group">
      <div class="cg-label">场景选择</div>
      <select id="whatifScene" style="width:100%;padding:8px;background:var(--bg2);border:1px solid var(--rule);border-radius:8px;color:var(--ink);font-size:13px">
        <option value="scale">扩容 2x 节点</option>
        <option value="degrade">降级非核心服务</option>
        <option value="failover">主库故障切换</option>
        <option value="cache">缓存命中率下降 50%</option>
      </select>
    </div>
    <div class="control-group">
      <div class="cg-label">自定义参数</div>
      <div class="control-row"><label>节点数</label><input type="number" value="4" id="wiNodes"></div>
      <div class="control-row"><label>QPS</label><input type="number" value="10000" id="wiQps"></div>
      <div class="control-row"><label>错误率 %</label><input type="number" value="0.5" step="0.1" id="wiErr"></div>
    </div>
    <button class="btn primary" onclick="runWhatIf()">运行推演</button>
    <div id="whatifResult" style="margin-top:16px"></div>
  `;
  container.appendChild(card);
}

function runWhatIf() {
  const scene = document.getElementById('whatifScene').value;
  const nodes = parseInt(document.getElementById('wiNodes').value) || 4;
  const qps = parseInt(document.getElementById('wiQps').value) || 10000;
  const err = parseFloat(document.getElementById('wiErr').value) || 0.5;

  let health = 85, latency = 30, throughput = qps / nodes;
  if (scene === 'scale') { health += 5; latency *= 0.8; throughput *= 1.5; }
  if (scene === 'degrade') { health -= 10; latency *= 0.7; }
  if (scene === 'failover') { health -= 15; latency *= 2; }
  if (scene === 'cache') { health -= 8; latency *= 1.5; }
  health -= err * 2;

  const el = document.getElementById('whatifResult');
  el.innerHTML = `
    <div class="grid-3" style="margin-top:12px">
      <div class="stat-card"><div class="label">预测健康度</div><div class="value" style="color:${health>=80?'var(--green)':health>=60?'var(--orange)':'var(--red)'}">${health.toFixed(1)}</div></div>
      <div class="stat-card"><div class="label">预测延迟 ms</div><div class="value">${latency.toFixed(1)}</div></div>
      <div class="stat-card"><div class="label">预测吞吐</div><div class="value">${Math.round(throughput)}</div></div>
    </div>
    <div class="card" style="margin-top:12px">
      <div class="card-title">影响分析</div>
      <div style="font-size:12px;color:var(--muted);line-height:1.6">
        场景: ${esc(scene)}<br>
        节点: ${nodes} | QPS: ${qps} | 错误率: ${err}%<br>
        ${health < 60 ? '警告: 推演结果显示系统健康度将降至危险水平，建议采取缓解措施。' : '推演结果在可接受范围内。'}
      </div>
    </div>
  `;
}
