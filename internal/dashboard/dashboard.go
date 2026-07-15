package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"ticktoken/internal/storage"

	"nhooyr.io/websocket"
)

// Server Web 仪表盘服务器
type Server struct {
	addr    string
	store   *storage.Store
	mu      sync.RWMutex
	clients map[*websocket.Conn]context.CancelFunc
}

// NewServer 创建仪表盘服务器
func NewServer(addr string, store *storage.Store) *Server {
	return &Server{
		addr:    addr,
		store:   store,
		clients: make(map[*websocket.Conn]context.CancelFunc),
	}
}

// Start 启动仪表盘
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/timeseries", s.handleTimeSeries)
	mux.HandleFunc("/api/aggregate", s.handleAggregate)
	mux.HandleFunc("/ws", s.handleWebSocket)

	srv := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	log.Printf("[Dashboard] 仪表盘监听于 http://%s", s.addr)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Dashboard] 服务器错误: %v", err)
		}
	}()

	return nil
}

// handleIndex 返回仪表盘页面
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

// handleTimeSeries 时间序列数据接口
func (s *Server) handleTimeSeries(w http.ResponseWriter, r *http.Request) {
	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if _, err := fmt.Sscanf(h, "%d", &hours); err == nil && hours > 0 && hours <= 720 {
			// ok
		}
	}

	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)

	points, err := s.store.QueryTimeSeries(start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(points)
}

// handleAggregate 聚合数据接口
func (s *Server) handleAggregate(w http.ResponseWriter, r *http.Request) {
	dimension := r.URL.Query().Get("dimension")
	if dimension != "model" && dimension != "tool" {
		dimension = "model"
	}

	hours := 24
	if h := r.URL.Query().Get("hours"); h != "" {
		if _, err := fmt.Sscanf(h, "%d", &hours); err == nil && hours > 0 && hours <= 720 {
			// ok
		}
	}

	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)

	items, err := s.store.QueryByDimension(dimension, start, end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// handleWebSocket WebSocket 实时推送
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("[Dashboard] WebSocket 接受失败: %v", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())

	s.mu.Lock()
	s.clients[conn] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		cancel()
		s.mu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "")
	}()

	// 保持连接，等待推送
	<-ctx.Done()
}

// BroadcastEvent 广播新事件到所有 WebSocket 客户端
func (s *Server) BroadcastEvent(event *storage.TokenEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	s.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.RUnlock()

	for _, conn := range clients {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := conn.Write(ctx, websocket.MessageText, data)
		cancel()
		if err != nil {
			s.mu.Lock()
			if _, ok := s.clients[conn]; ok {
				if c := s.clients[conn]; c != nil {
					c()
				}
				delete(s.clients, conn)
			}
			s.mu.Unlock()
		}
	}
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>TickToken - Token 计数器</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0f1117; color: #e1e4e8; }
.header { background: #161b22; padding: 16px 24px; border-bottom: 1px solid #30363d; display: flex; align-items: center; justify-content: space-between; }
.header h1 { font-size: 20px; color: #58a6ff; }
.header .mode { font-size: 13px; color: #8b949e; padding: 4px 10px; border: 1px solid #30363d; border-radius: 12px; }
.container { max-width: 1400px; margin: 0 auto; padding: 24px; }
.summary { display: grid; grid-template-columns: repeat(4, 1fr); gap: 16px; margin-bottom: 24px; }
.card { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px; }
.card .label { font-size: 12px; color: #8b949e; text-transform: uppercase; margin-bottom: 8px; }
.card .value { font-size: 28px; font-weight: 600; color: #e1e4e8; }
.card .value.blue { color: #58a6ff; }
.card .value.green { color: #3fb950; }
.card .value.yellow { color: #d29922; }
.card .value.purple { color: #bc8cff; }
.section { background: #161b22; border: 1px solid #30363d; border-radius: 8px; padding: 20px; margin-bottom: 24px; }
.section h2 { font-size: 16px; color: #e1e4e8; margin-bottom: 16px; }
.controls { display: flex; gap: 12px; align-items: center; margin-bottom: 16px; flex-wrap: wrap; }
.controls select, .controls button { background: #21262d; border: 1px solid #30363d; color: #e1e4e8; padding: 6px 12px; border-radius: 6px; font-size: 13px; cursor: pointer; }
.controls button:hover { border-color: #58a6ff; }
.controls button.active { border-color: #58a6ff; color: #58a6ff; }
.chart-container { width: 100%; overflow-x: auto; }
#timelineChart, #aggregateChart { width: 100%; height: 300px; }
.live-events { max-height: 300px; overflow-y: auto; }
.event-row { display: grid; grid-template-columns: 150px 120px 150px 80px 80px 100px; gap: 12px; padding: 8px 0; border-bottom: 1px solid #21262d; font-size: 13px; }
.event-row .time { color: #8b949e; }
.event-row .tool { color: #58a6ff; }
.event-row .model { color: #bc8cff; }
.empty { text-align: center; color: #8b949e; padding: 40px; }
</style>
</head>
<body>
<div class="header">
  <h1>TickToken Dashboard</h1>
  <div class="mode" id="mode">Loading...</div>
</div>
<div class="container">
  <div class="summary" id="summary">
    <div class="card"><div class="label">Prompt Tokens</div><div class="value blue" id="totalPrompt">0</div></div>
    <div class="card"><div class="label">Completion Tokens</div><div class="value green" id="totalCompletion">0</div></div>
    <div class="card"><div class="label">Cache Hit</div><div class="value yellow" id="totalCacheHit">0</div></div>
    <div class="card"><div class="label">Total Events</div><div class="value purple" id="totalEvents">0</div></div>
  </div>

  <div class="section">
    <h2>时间序列</h2>
    <div class="controls">
      <select id="timeRange">
        <option value="1">1 hour</option>
        <option value="6">6 hours</option>
        <option value="24" selected>24 hours</option>
        <option value="168">7 days</option>
      </select>
      <button onclick="refreshTimeline()">Refresh</button>
    </div>
    <div class="chart-container"><canvas id="timelineChart"></canvas></div>
  </div>

  <div class="section">
    <h2>分类聚合</h2>
    <div class="controls">
      <button id="btn-model" class="active" onclick="setDimension('model')">By Model</button>
      <button id="btn-tool" onclick="setDimension('tool')">By Tool</button>
    </div>
    <div class="chart-container"><canvas id="aggregateChart"></canvas></div>
  </div>

  <div class="section">
    <h2>实时事件</h2>
    <div class="live-events" id="liveEvents">
      <div class="empty">等待事件...</div>
    </div>
  </div>
</div>

<script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
<script>
let timelineChart = null, aggregateChart = null;
let currentDimension = 'model';

function fetchJSON(url, cb) {
  fetch(url).then(r => r.json()).then(cb).catch(err => console.error(err));
}

function refreshTimeline() {
  const hours = document.getElementById('timeRange').value;
  fetchJSON('/api/timeseries?hours=' + hours, function(data) {
    if (!data || !data.length) return;
    const labels = data.map(d => new Date(d.Timestamp).toLocaleString());
    const prompt = data.map(d => d.PromptTokens);
    const completion = data.map(d => d.CompletionTokens);

    let totalP = 0, totalC = 0, totalH = 0;
    data.forEach(d => { totalP += d.PromptTokens; totalC += d.CompletionTokens; totalH += d.CacheHit; });
    document.getElementById('totalPrompt').textContent = totalP.toLocaleString();
    document.getElementById('totalCompletion').textContent = totalC.toLocaleString();
    document.getElementById('totalCacheHit').textContent = totalH.toLocaleString();
    document.getElementById('totalEvents').textContent = data.length;

    if (timelineChart) timelineChart.destroy();
    timelineChart = new Chart(document.getElementById('timelineChart'), {
      type: 'line',
      data: { labels: labels, datasets: [
        { label: 'Prompt', data: prompt, borderColor: '#58a6ff', backgroundColor: 'rgba(88,166,255,0.1)', fill: true },
        { label: 'Completion', data: completion, borderColor: '#3fb950', backgroundColor: 'rgba(63,185,80,0.1)', fill: true }
      ]},
      options: { responsive: true, scales: { x: { ticks: { color: '#8b949e' }, grid: { color: '#21262d' } }, y: { ticks: { color: '#8b949e' }, grid: { color: '#21262d' } } }, plugins: { legend: { labels: { color: '#e1e4e8' } } } }
    });
  });
}

function setDimension(dim) {
  currentDimension = dim;
  document.getElementById('btn-model').classList.toggle('active', dim === 'model');
  document.getElementById('btn-tool').classList.toggle('active', dim === 'tool');
  refreshAggregate();
}

function refreshAggregate() {
  const hours = document.getElementById('timeRange').value;
  fetchJSON('/api/aggregate?dimension=' + currentDimension + '&hours=' + hours, function(data) {
    if (!data || !data.length) return;
    const labels = data.map(d => d.Key || 'unknown');
    const hit = data.map(d => d.CacheHit);
    const miss = data.map(d => d.CacheMiss);
    const creation = data.map(d => d.CacheCreation);
    const completion = data.map(d => d.CompletionTokens);

    if (aggregateChart) aggregateChart.destroy();
    aggregateChart = new Chart(document.getElementById('aggregateChart'), {
      type: 'bar',
      data: { labels: labels, datasets: [
        { label: 'Cache Hit', data: hit, backgroundColor: '#d29922' },
        { label: 'Cache Miss', data: miss, backgroundColor: '#f85149' },
        { label: 'Cache Creation', data: creation, backgroundColor: '#bc8cff' },
        { label: 'Completion', data: completion, backgroundColor: '#3fb950' }
      ]},
      options: { responsive: true, scales: { x: { stacked: true, ticks: { color: '#8b949e' }, grid: { color: '#21262d' } }, y: { stacked: true, ticks: { color: '#8b949e' }, grid: { color: '#21262d' } } }, plugins: { legend: { labels: { color: '#e1e4e8' } } } }
    });
  });
}

function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  const ws = new WebSocket(proto + '://' + location.host + '/ws');
  ws.onmessage = function(ev) {
    const event = JSON.parse(ev.data);
    const container = document.getElementById('liveEvents');
    if (container.querySelector('.empty')) container.innerHTML = '';
    const row = document.createElement('div');
    row.className = 'event-row';
    row.innerHTML = '<span class="time">' + new Date(event.Timestamp).toLocaleTimeString() + '</span>' +
      '<span class="tool">' + (event.Tool || 'unknown') + '</span>' +
      '<span class="model">' + (event.Model || 'unknown') + '</span>' +
      '<span>P:' + event.PromptTokens + '</span>' +
      '<span>C:' + event.CompletionTokens + '</span>' +
      '<span>' + (event.Source || '') + '</span>';
    container.insertBefore(row, container.firstChild);
    while (container.children.length > 50) container.removeChild(container.lastChild);
  };
}

document.getElementById('timeRange').addEventListener('change', function() { refreshTimeline(); refreshAggregate(); });
refreshTimeline();
refreshAggregate();
connectWS();
</script>
</body>
</html>`
