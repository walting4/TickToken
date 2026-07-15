let timelineChart = null;
let aggregateChart = null;
let currentDimension = 'model';

// Wails 启动后初始化
window.addEventListener('DOMContentLoaded', () => {
  initApp();
});

async function initApp() {
  try {
    const status = await window.go.main.App.GetStatus();
    document.getElementById('modeBadge').textContent = status.mode === 'passive' ? 'Passive' : 'Relay';
    document.getElementById('modeBadge').classList.toggle('relay', status.mode === 'relay');
    document.getElementById('proxyInfo').textContent = 'Proxy: ' + status.proxyAddr;
  } catch (e) {
    console.error('GetStatus failed:', e);
  }

  // 监听实时事件
  window.runtime.EventsOn('token:event', (event) => {
    addLiveEvent(event);
    refreshSummary();
  });

  refreshAll();
}

function refreshAll() {
  refreshTimeline();
  refreshAggregate();
  refreshSummary();
}

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

async function refreshTimeline() {
  const hours = parseInt(document.getElementById('timeRange').value) || 24;
  try {
    const data = await window.go.main.App.GetTimeSeries(hours);
    if (!data || data.length === 0) return;

    const labels = data.map(d => new Date(d.Timestamp).toLocaleString());
    const prompt = data.map(d => d.PromptTokens);
    const completion = data.map(d => d.CompletionTokens);

    if (timelineChart) timelineChart.destroy();
    timelineChart = new Chart(document.getElementById('timelineChart'), {
      type: 'line',
      data: {
        labels: labels,
        datasets: [
          { label: 'Prompt', data: prompt, borderColor: '#58a6ff', backgroundColor: 'rgba(88,166,255,0.1)', fill: true, tension: 0.3 },
          { label: 'Completion', data: completion, borderColor: '#3fb950', backgroundColor: 'rgba(63,185,80,0.1)', fill: true, tension: 0.3 }
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
    if (!data || data.length === 0) return;

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
          { label: 'Cache Hit', data: hit, backgroundColor: '#d29922' },
          { label: 'Cache Miss', data: miss, backgroundColor: '#f85149' },
          { label: 'Cache Creation', data: creation, backgroundColor: '#bc8cff' },
          { label: 'Completion', data: completion, backgroundColor: '#3fb950' }
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

function addLiveEvent(event) {
  const container = document.getElementById('liveEvents');
  if (container.querySelector('.empty')) container.innerHTML = '';

  const row = document.createElement('div');
  row.className = 'event-row';
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

async function openCACert() {
  try {
    await window.go.main.App.OpenCACert();
  } catch (e) {
    console.error('OpenCACert failed:', e);
  }
}

async function openDataDir() {
  try {
    await window.go.main.App.OpenDataDir();
  } catch (e) {
    console.error('OpenDataDir failed:', e);
  }
}

document.getElementById('timeRange').addEventListener('change', refreshAll);
