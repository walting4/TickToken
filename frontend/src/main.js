let timelineChart = null;
let aggregateChart = null;
let trendChart = null;
let currentDimension = 'model';
let currentGranularity = 'hour';

// ============ 初始化 ============
window.addEventListener('DOMContentLoaded', () => {
  initApp();
});

async function initApp() {
  try {
    // 加载语言设置
    const langInfo = await window.go.main.App.GetLanguage();
    window.i18n.setLang(langInfo.current);
    document.getElementById('langSelect').value = langInfo.current;
    window.i18n.applyTranslations();

    const status = await window.go.main.App.GetStatus();
    document.getElementById('modeBadge').textContent =
      status.mode === 'passive' ? window.i18n.t('passive') : window.i18n.t('relay');
    document.getElementById('modeBadge').classList.toggle('relay', status.mode === 'relay');
    document.getElementById('proxyInfo').textContent = 'Proxy: ' + status.proxyAddr;
  } catch (e) {
    console.error('init failed:', e);
  }

  // 监听实时 token 事件
  window.runtime.EventsOn('token:event', (event) => {
    addLiveEvent(event);
    refreshSummary();
    // 异常事件同步加入异常列表
    if (event.IsAnomaly) {
      addAnomalyEvent(event);
    }
  });

  // 监听后端推送的异常事件
  window.runtime.EventsOn('anomaly:event', (info) => {
    refreshAnomalyList();
  });

  refreshAll();

  // 定时刷新代理状态（每 5 秒）
  setInterval(refreshProxyStatus, 5000);
}

// ============ 语言切换 ============
async function onLanguageChange() {
  const lang = document.getElementById('langSelect').value;
  try {
    await window.go.main.App.SetLanguage(lang);
    window.i18n.setLang(lang);
    window.i18n.applyTranslations();
    // 重新刷新所有面板以应用新语言
    refreshAll();
    refreshVerificationStats();
    refreshAPIKeyMonitor();
  } catch (e) {
    console.error('SetLanguage failed:', e);
  }
}

// ============ 全局刷新 ============
function refreshAll() {
  refreshTimeline();
  refreshTrend();
  refreshAggregate();
  refreshSummary();
  refreshVerificationStats();
  refreshAPIKeyMonitor();
  refreshAnomalyList();
  refreshProxyStatus();
}

// ============ 汇总 ============
async function refreshSummary() {
  const hours = parseInt(document.getElementById('timeRange').value) || 24;
  try {
    const stats = await window.go.main.App.GetSummary(hours);
    document.getElementById('totalPrompt').textContent = stats.totalPrompt.toLocaleString();
    document.getElementById('totalCompletion').textContent = stats.totalCompletion.toLocaleString();
    document.getElementById('totalCacheHit').textContent = stats.totalCacheHit.toLocaleString();
    document.getElementById('totalEvents').textContent = stats.totalEvents.toLocaleString();
  } catch (e) {
    console.error('GetSummary failed:', e);
  }
}

// ============ 校验统计 ============
async function refreshVerificationStats() {
  try {
    const s = await window.go.main.App.GetVerificationStats();
    document.getElementById('vsTotal').textContent = s.totalVerified.toLocaleString();
    document.getElementById('vsAccuracy').textContent = s.accuracyRate.toFixed(2) + '%';
    document.getElementById('vsDeviation').textContent = s.avgDeviationPct.toFixed(2) + '%';
    document.getElementById('vsAnomalies').textContent = s.anomalies.toLocaleString();
    document.getElementById('vsWithUsage').textContent = s.withResponseUsage.toLocaleString();
    document.getElementById('vsLocalOnly').textContent = s.withLocalOnly.toLocaleString();
    document.getElementById('vsAvgLatency').textContent = s.avgLatencyMs + ' ' + window.i18n.t('ms');
    document.getElementById('vsMaxLatency').textContent = s.maxLatencyMs + ' ' + window.i18n.t('ms');
  } catch (e) {
    console.error('GetVerificationStats failed:', e);
  }
}

// ============ API Key 监控 ============
async function refreshAPIKeyMonitor() {
  try {
    const s = await window.go.main.App.GetAPIKeyMonitorStats();
    document.getElementById('amTotal').textContent = s.totalRequests.toLocaleString();
    document.getElementById('amCompleted').textContent = s.completedRequests.toLocaleString();
    document.getElementById('amFailed').textContent = s.failedRequests.toLocaleString();
    document.getElementById('amAnomaly').textContent = s.anomalyRequests.toLocaleString();
    document.getElementById('amInflight').textContent = s.inflightCount.toLocaleString();
  } catch (e) {
    console.error('GetAPIKeyMonitorStats failed:', e);
  }
}

// ============ 时间序列 ============
async function refreshTimeline() {
  const hours = parseInt(document.getElementById('timeRange').value) || 24;
  try {
    const data = await window.go.main.App.GetTimeSeries(hours);
    if (!data || data.length === 0) {
      if (timelineChart) { timelineChart.destroy(); timelineChart = null; }
      return;
    }

    const labels = data.map(d => new Date(d.Timestamp).toLocaleString());
    const prompt = data.map(d => d.PromptTokens);
    const completion = data.map(d => d.CompletionTokens);

    if (timelineChart) timelineChart.destroy();
    timelineChart = new Chart(document.getElementById('timelineChart'), {
      type: 'line',
      data: {
        labels: labels,
        datasets: [
          { label: window.i18n.t('promptTokens'), data: prompt, borderColor: '#58a6ff', backgroundColor: 'rgba(88,166,255,0.1)', fill: true, tension: 0.3 },
          { label: window.i18n.t('completionTokens'), data: completion, borderColor: '#3fb950', backgroundColor: 'rgba(63,185,80,0.1)', fill: true, tension: 0.3 }
        ]
      },
      options: {
        responsive: true,
        scales: {
          x: { ticks: { color: '#8b949e', maxRotation: 45 }, grid: { color: '#21262d' } },
          y: { ticks: { color: '#8b949e' }, grid: { color: '#21262d' } }
        },
        plugins: { legend: { labels: { color: '#e1e4e8' } } }
      }
    });
  } catch (e) {
    console.error('GetTimeSeries failed:', e);
  }
}

// ============ 趋势分析 ============
function setGranularity(g) {
  currentGranularity = g;
  document.getElementById('btn-hour').classList.toggle('active', g === 'hour');
  document.getElementById('btn-day').classList.toggle('active', g === 'day');
  refreshTrend();
}

async function refreshTrend() {
  const hours = parseInt(document.getElementById('timeRange').value) || 24;
  try {
    const data = await window.go.main.App.GetTrend(hours, currentGranularity);
    if (!data || data.length === 0) {
      if (trendChart) { trendChart.destroy(); trendChart = null; }
      return;
    }

    const labels = data.map(d => new Date(d.Timestamp).toLocaleString());
    const totalTokens = data.map(d => d.TotalTokens);
    const eventCount = data.map(d => d.EventCount);
    const anomalyCount = data.map(d => d.AnomalyCount);

    if (trendChart) trendChart.destroy();
    trendChart = new Chart(document.getElementById('trendChart'), {
      type: 'line',
      data: {
        labels: labels,
        datasets: [
          { label: window.i18n.t('totalTokens'), data: totalTokens, borderColor: '#58a6ff', backgroundColor: 'rgba(88,166,255,0.1)', fill: true, tension: 0.3, yAxisID: 'y' },
          { label: window.i18n.t('eventCount'), data: eventCount, borderColor: '#3fb950', backgroundColor: 'rgba(63,185,80,0.1)', tension: 0.3, yAxisID: 'y1' },
          { label: window.i18n.t('anomalyCount'), data: anomalyCount, borderColor: '#f85149', backgroundColor: 'rgba(248,81,73,0.1)', tension: 0.3, yAxisID: 'y1' }
        ]
      },
      options: {
        responsive: true,
        scales: {
          x: { ticks: { color: '#8b949e', maxRotation: 45 }, grid: { color: '#21262d' } },
          y: { type: 'linear', position: 'left', ticks: { color: '#8b949e' }, grid: { color: '#21262d' } },
          y1: { type: 'linear', position: 'right', ticks: { color: '#8b949e' }, grid: { drawOnChartArea: false } }
        },
        plugins: { legend: { labels: { color: '#e1e4e8' } } }
      }
    });
  } catch (e) {
    console.error('GetTrend failed:', e);
  }
}

// ============ 分类聚合 ============
function setDimension(dim) {
  currentDimension = dim;
  document.getElementById('btn-model').classList.toggle('active', dim === 'model');
  document.getElementById('btn-tool').classList.toggle('active', dim === 'tool');
  refreshAggregate();
}

async function refreshAggregate() {
  const hours = parseInt(document.getElementById('timeRange').value) || 24;
  try {
    const data = await window.go.main.App.GetAggregate(currentDimension, hours);
    if (!data || data.length === 0) {
      if (aggregateChart) { aggregateChart.destroy(); aggregateChart = null; }
      return;
    }

    const labels = data.map(d => d.Key || 'unknown');
    const hit = data.map(d => d.CacheHit);
    const miss = data.map(d => d.CacheMiss);
    const creation = data.map(d => d.CacheCreation);
    const completion = data.map(d => d.CompletionTokens);

    if (aggregateChart) aggregateChart.destroy();
    aggregateChart = new Chart(document.getElementById('aggregateChart'), {
      type: 'bar',
      data: {
        labels: labels,
        datasets: [
          { label: window.i18n.t('cacheHit'), data: hit, backgroundColor: '#d29922' },
          { label: 'Cache Miss', data: miss, backgroundColor: '#f85149' },
          { label: 'Cache Creation', data: creation, backgroundColor: '#bc8cff' },
          { label: window.i18n.t('completionTokens'), data: completion, backgroundColor: '#3fb950' }
        ]
      },
      options: {
        responsive: true,
        scales: {
          x: { stacked: true, ticks: { color: '#8b949e' }, grid: { color: '#21262d' } },
          y: { stacked: true, ticks: { color: '#8b949e' }, grid: { color: '#21262d' } }
        },
        plugins: { legend: { labels: { color: '#e1e4e8' } } }
      }
    });
  } catch (e) {
    console.error('GetAggregate failed:', e);
  }
}

// ============ 异常事件列表 ============
async function refreshAnomalyList() {
  try {
    const events = await window.go.main.App.GetAnomalies(50);
    const container = document.getElementById('anomalyList');
    if (!events || events.length === 0) {
      container.innerHTML = '<div class="empty">' + window.i18n.t('noAnomalies') + '</div>';
      return;
    }
    container.innerHTML = '';
    events.forEach(ev => addAnomalyEvent(ev, container));
  } catch (e) {
    console.error('GetAnomalies failed:', e);
  }
}

function addAnomalyEvent(event, container) {
  const list = container || document.getElementById('anomalyList');
  if (list.querySelector('.empty')) list.innerHTML = '';

  const row = document.createElement('div');
  row.className = 'event-row anomaly-row';
  row.innerHTML =
    '<span class="time">' + new Date(event.Timestamp).toLocaleTimeString() + '</span>' +
    '<span class="tool">' + (event.Tool || 'unknown') + '</span>' +
    '<span class="model" title="' + (event.Model || '') + '">' + (event.Model || '-') + '</span>' +
    '<span class="anomaly-type">' + window.i18n.tAnomaly(event.AnomalyType) + '</span>' +
    '<span class="deviation">' + (event.DeviationPct || 0).toFixed(1) + '%</span>' +
    '<span class="provider">' + (event.Provider || '-') + '</span>';
  list.insertBefore(row, list.firstChild);

  while (list.children.length > 50) {
    list.removeChild(list.lastChild);
  }
}

// ============ 实时事件 ============
function addLiveEvent(event) {
  const container = document.getElementById('liveEvents');
  if (container.querySelector('.empty')) container.innerHTML = '';

  const row = document.createElement('div');
  row.className = 'event-row' + (event.IsAnomaly ? ' has-anomaly' : '');
  row.innerHTML =
    '<span class="time">' + new Date(event.Timestamp).toLocaleTimeString() + '</span>' +
    '<span class="tool">' + (event.Tool || 'unknown') + '</span>' +
    '<span class="model" title="' + (event.Model || '') + '">' + (event.Model || 'unknown') + '</span>' +
    '<span>P:' + event.PromptTokens + '</span>' +
    '<span>C:' + event.CompletionTokens + '</span>' +
    '<span class="src">' + (event.Source || '') + '</span>';
  container.insertBefore(row, container.firstChild);

  while (container.children.length > 50) {
    container.removeChild(container.lastChild);
  }
}

// ============ 文件管理器 ============
async function openCACert() {
  try { await window.go.main.App.OpenCACert(); }
  catch (e) { console.error('OpenCACert failed:', e); }
}

async function openDataDir() {
  try { await window.go.main.App.OpenDataDir(); }
  catch (e) { console.error('OpenDataDir failed:', e); }
}

// ============ 代理配置 ============
function setProxyIndicator(id, ok, okText, failText) {
  const el = document.getElementById(id);
  if (!el) return;
  if (ok) {
    el.textContent = '✓ ' + (okText || '');
    el.classList.add('green');
    el.classList.remove('red');
  } else {
    el.textContent = '✗ ' + (failText || '');
    el.classList.add('red');
    el.classList.remove('green');
  }
}

async function refreshProxyStatus() {
  try {
    const s = await window.go.main.App.GetProxyStatus();
    if (!s) return;

    setProxyIndicator('psSystemProxy', s.systemProxySet,
      window.i18n.t('configured'), window.i18n.t('notConfigured'));
    setProxyIndicator('psCaCert', s.cacertInstalled,
      window.i18n.t('caCertInstalled'), window.i18n.t('notConfigured'));
    setProxyIndicator('psTraeProxy', s.traeProxySet,
      window.i18n.t('configured'), window.i18n.t('notConfigured'));
    setProxyIndicator('psListening', s.isListening,
      window.i18n.t('listening'), window.i18n.t('notListening'));

    const ps = s.proxyStats || {};
    document.getElementById('psTotalRequests').textContent = (ps.totalRequests || 0).toLocaleString();
    document.getElementById('psCaptured').textContent = (ps.totalCaptured || 0).toLocaleString();
    document.getElementById('psSseStreams').textContent = (ps.totalSSEStream || 0).toLocaleString();
    document.getElementById('psErrors').textContent = (ps.totalErrors || 0).toLocaleString();
  } catch (e) {
    console.error('GetProxyStatus failed:', e);
  }
}

async function setupProxyAndCert() {
  const resultEl = document.getElementById('proxyResult');
  try {
    const res = await window.go.main.App.SetupProxyAndCert();
    if (res && res.success) {
      resultEl.textContent = res.message || window.i18n.t('setupSuccess');
      resultEl.className = 'proxy-result success';
    } else {
      resultEl.textContent = res.message || window.i18n.t('setupPartial');
      resultEl.className = 'proxy-result warn';
    }
    refreshProxyStatus();
  } catch (e) {
    console.error('SetupProxyAndCert failed:', e);
    resultEl.textContent = String(e);
    resultEl.className = 'proxy-result error';
  }
}

async function removeProxyAndCert() {
  const resultEl = document.getElementById('proxyResult');
  try {
    await window.go.main.App.RemoveProxyAndCert();
    resultEl.textContent = window.i18n.t('removeSuccess');
    resultEl.className = 'proxy-result success';
    refreshProxyStatus();
  } catch (e) {
    console.error('RemoveProxyAndCert failed:', e);
    resultEl.textContent = String(e);
    resultEl.className = 'proxy-result error';
  }
}

// ============ 代理诊断 ============
async function diagnoseProxy() {
  const reportEl = document.getElementById('diagnosticReport');
  reportEl.innerHTML = '<div class="diag-running">' + window.i18n.t('diagnosing') + '</div>';
  try {
    const result = await window.go.main.App.DiagnoseProxy();
    renderDiagnosticReport(result);
  } catch (e) {
    console.error('DiagnoseProxy failed:', e);
    reportEl.innerHTML = '<div class="diag-item fail">' + String(e) + '</div>';
  }
}

function renderDiagnosticReport(result) {
  const reportEl = document.getElementById('diagnosticReport');
  if (!result || !result.checks) {
    reportEl.innerHTML = '';
    return;
  }

  let html = '<div class="diag-summary ' + (result.success ? 'pass' : 'fail') + '">' +
    '<strong>' + (result.success ? '✓' : '⚠') + '</strong> ' + (result.message || '') +
    '</div>';

  result.checks.forEach(check => {
    const status = check.passed ? 'pass' : 'fail';
    const icon = check.passed ? '✓' : '✗';
    html += '<div class="diag-item ' + status + '">' +
      '<span class="diag-icon">' + icon + '</span>' +
      '<div class="diag-content">' +
      '<div class="diag-name">' + window.i18n.t('diag_' + check.name) + '</div>' +
      '<div class="diag-detail">' + (check.detail || '') + '</div>' +
      (check.hint ? '<div class="diag-hint">' + window.i18n.t('hint') + ': ' + check.hint + '</div>' : '') +
      '</div></div>';
  });

  reportEl.innerHTML = html;
}

document.getElementById('timeRange').addEventListener('change', refreshAll);

// ============ CSV 导出 ============
async function exportCSV() {
  const hours = parseInt(document.getElementById('timeRange').value) || 24;
  try {
    const path = await window.go.main.App.ExportCSV(hours);
    if (path) {
      alert(window.i18n.t('exportSuccess') + ': ' + path);
    } else {
      alert(window.i18n.t('exportEmpty'));
    }
  } catch (e) {
    console.error('ExportCSV failed:', e);
    alert(window.i18n.t('exportFailed') + ': ' + String(e));
  }
}
