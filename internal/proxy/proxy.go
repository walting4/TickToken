package proxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 最大请求/响应体大小（50MB，防止 OOM）
const maxBodySize = 50 * 1024 * 1024

// PayloadHandler 处理捕获的请求/响应 payload
type PayloadHandler func(host string, reqBody []byte, respBody []byte, req *http.Request, resp *http.Response)

// MITMProxy HTTPS MITM 代理
type MITMProxy struct {
	addr    string
	certMgr *CertManager
	handler PayloadHandler

	// 上游 HTTP 客户端（支持 HTTP/2，带超时）
	upstreamClient *http.Client

	// 统计
	mu             sync.Mutex
	totalRequests  int64
	totalCaptured  int64
	totalErrors    int64
	totalSSEStream int64
}

// NewMITMProxy 创建 MITM 代理
func NewMITMProxy(addr string, certMgr *CertManager, handler PayloadHandler) *MITMProxy {
	// 上游客户端：支持 HTTP/2，30s 超时，自定义 Transport
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			// 上游证书正常校验
			InsecureSkipVerify: false,
		},
		// 强制使用 HTTP/2
		ForceAttemptHTTP2: true,
		// 连接池
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		// 拨号超时
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		// 响应头超时
		ResponseHeaderTimeout: 30 * time.Second,
	}

	return &MITMProxy{
		addr:    addr,
		certMgr: certMgr,
		handler: handler,
		upstreamClient: &http.Client{
			Transport: transport,
			// 不设整体超时，因为 SSE 流式可能持续很久
			// 超时由 Transport 层控制
		},
	}
}

// Start 启动代理
func (p *MITMProxy) Start() error {
	ln, err := net.Listen("tcp", p.addr)
	if err != nil {
		return fmt.Errorf("代理监听失败: %w", err)
	}
	log.Printf("[Proxy] MITM 代理监听于 %s（支持 HTTP/2 上游 + SSE 流式）", p.addr)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("[Proxy] 接受连接失败: %v", err)
				continue
			}
			go p.handleConn(conn)
		}
	}()

	return nil
}

// GetStats 获取代理统计
func (p *MITMProxy) GetStats() (total, captured, errors, sse int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.totalRequests, p.totalCaptured, p.totalErrors, p.totalSSEStream
}

// handleConn 处理单个连接
func (p *MITMProxy) handleConn(conn net.Conn) {
	defer conn.Close()

	br := bufio.NewReader(conn)
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}

	if req.Method == http.MethodConnect {
		p.handleConnect(conn, req)
	} else {
		p.handleHTTP(conn, req, br)
	}
}

// handleConnect 处理 HTTPS CONNECT 隧道（MITM 核心）
func (p *MITMProxy) handleConnect(conn net.Conn, req *http.Request) {
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}

	// 告诉客户端隧道已建立
	_, _ = conn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// 为目标域名签发证书
	cert, err := p.certMgr.SignHost(host)
	if err != nil {
		log.Printf("[Proxy] 签发证书失败 %s: %v", host, err)
		return
	}

	// TLS 握手（伪装成目标服务器）
	// 关键：NextProtos 强制只协商 HTTP/1.1，避免客户端协商 H2 后发送二进制帧
	// 代理到上游时再用 http.Client 自动协商 H2
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"http/1.1"}, // 强制 HTTP/1.1，不协商 h2
		MinVersion:   tls.VersionTLS12,
	}

	tlsConn := tls.Server(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("[Proxy] TLS 握手失败 %s: %v", host, err)
		return
	}
	defer tlsConn.Close()

	// 在 TLS 隧道上用 http.Server 处理请求（支持 keep-alive 多请求）
	// 标准库的 http.Server 没有 ServeConn 方法，使用单连接 Listener 包装 tlsConn
	// 再调用 http.Server.Serve(listener) 即可复用标准库的 HTTP/1.1 解析、keep-alive、pipeline
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.handleMITMRequest(w, r, host)
	})

	srv := &http.Server{
		Handler:      handler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 300 * time.Second, // SSE 流式可能持续较久
		IdleTimeout:  120 * time.Second,
	}

	// singleConnListener 只接受一次连接，Accept 返回 tlsConn 后下次 Accept 返回错误
	// 这样 http.Server.Serve 会在处理完该连接的所有 keep-alive 请求后退出
	_ = srv.Serve(newSingleConnListener(tlsConn))
}

// singleConnListener 包装单个已建立的 net.Conn，使其表现为 net.Listener
// 用于在 TLS 隧道上跑 http.Server.Serve()，复用标准库的 HTTP/1.1 keep-alive 处理
// 关键设计：Accept 在连接关闭前一直阻塞，确保 Serve 不会在连接处理 goroutine
// 还在运行时就返回（避免外层 defer 提前关闭 tlsConn）
type singleConnListener struct {
	conn     net.Conn
	accepted bool
	done     chan struct{} // 连接关闭后关闭此 channel
	once     sync.Once
}

func newSingleConnListener(conn net.Conn) *singleConnListener {
	return &singleConnListener{
		conn: conn,
		done: make(chan struct{}),
	}
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	// 第一次 Accept 返回连接
	if !l.accepted {
		l.accepted = true
		return &singleConn{Conn: l.conn, listener: l}, nil
	}
	// 后续 Accept 阻塞直到连接关闭，然后返回错误让 Serve 退出
	<-l.done
	return nil, net.ErrClosed
}

func (l *singleConnListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }

// singleConn 包装 net.Conn
// Close 时通知 listener 连接已结束，但不真正关闭底层 tlsConn（由外层 defer 统一关闭）
type singleConn struct {
	net.Conn
	listener *singleConnListener
	closed   bool
	mu       sync.Mutex
}

func (c *singleConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	// 通知 listener：连接处理完毕，可以让 Serve 退出了
	c.listener.Close()
	// 不关闭底层 conn，由 handleConnect 的 defer tlsConn.Close() 统一关闭
	return nil
}

// handleMITMRequest 处理 MITM 隧道内的 HTTP 请求
// 这是核心方法：转发请求到上游 + 捕获 payload + 处理 SSE 流式
func (p *MITMProxy) handleMITMRequest(w http.ResponseWriter, r *http.Request, host string) {
	p.mu.Lock()
	p.totalRequests++
	p.mu.Unlock()

	// 读取请求体（限制大小）
	var reqBody []byte
	if r.Body != nil {
		reqBody, _ = io.ReadAll(io.LimitReader(r.Body, maxBodySize))
		r.Body.Close()
	}

	// 构建转发到上游的请求
	targetURL := "https://" + host + r.RequestURI
	upstreamReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, bytes.NewReader(reqBody))
	if err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		p.mu.Lock()
		p.totalErrors++
		p.mu.Unlock()
		return
	}

	// 复制请求头
	for key, values := range r.Header {
		for _, v := range values {
			upstreamReq.Header.Add(key, v)
		}
	}
	upstreamReq.Header.Set("Host", host)
	// 强制不压缩响应，便于 SSE 按行解析和 token 提取
	upstreamReq.Header.Set("Accept-Encoding", "identity")
	if reqBody != nil {
		upstreamReq.ContentLength = int64(len(reqBody))
	}

	// 发送到上游（http.Client 自动协商 H2）
	resp, err := p.upstreamClient.Do(upstreamReq)
	if err != nil {
		log.Printf("[Proxy] 上游请求失败 %s: %v", host, err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		p.mu.Lock()
		p.totalErrors++
		p.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	// 检测 SSE 流式响应
	contentType := resp.Header.Get("Content-Type")
	isSSE := strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "application/x-ndjson")

	// 复制响应头到客户端
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if isSSE {
		p.handleSSEResponse(w, resp, host, reqBody, r)
	} else {
		p.handleNormalResponse(w, resp, host, reqBody, r)
	}
}

// handleSSEResponse 处理 SSE 流式响应
// 边转发给客户端边缓冲完整内容，流结束后提取 token
func (p *MITMProxy) handleSSEResponse(w http.ResponseWriter, resp *http.Response, host string, reqBody []byte, req *http.Request) {
	p.mu.Lock()
	p.totalSSEStream++
	p.mu.Unlock()

	flusher, _ := w.(http.Flusher)
	buf := &bytes.Buffer{}

	// 使用 bufio.Scanner 按行读取 SSE
	scanner := bufio.NewScanner(resp.Body)
	// 增大 scanner buffer 防止超长行
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		// SSE 每行末尾加 \n
		out := append(line, '\n')
		// 写给客户端
		w.Write(out)
		// 缓冲完整内容用于后续 token 提取
		buf.Write(out)
		// 立即 flush 让客户端收到
		if flusher != nil {
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		log.Printf("[Proxy] SSE 读取异常 %s: %v", host, err)
	}

	// 流结束，调用 payload 处理器提取 token
	respBody := buf.Bytes()
	p.mu.Lock()
	p.totalCaptured++
	p.mu.Unlock()

	if p.handler != nil {
		// 重建 resp.Body 供 handler 读取
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		// 异步处理避免阻塞响应返回
		go p.handler(host, reqBody, respBody, req, resp)
	}
}

// handleNormalResponse 处理普通（非流式）响应
func (p *MITMProxy) handleNormalResponse(w http.ResponseWriter, resp *http.Response, host string, reqBody []byte, req *http.Request) {
	// 读取响应体（限制大小）
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))

	// 写给客户端
	w.Write(respBody)

	p.mu.Lock()
	p.totalCaptured++
	p.mu.Unlock()

	if p.handler != nil {
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		// 异步处理
		go p.handler(host, reqBody, respBody, req, resp)
	}
}

// handleHTTP 处理普通 HTTP 代理请求（非 HTTPS）
func (p *MITMProxy) handleHTTP(conn net.Conn, req *http.Request, br *bufio.Reader) {
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(io.LimitReader(req.Body, maxBodySize))
		req.Body.Close()
	}

	// 复用上游客户端（支持连接池和 H2）
	req.RequestURI = ""
	req.Header.Set("Accept-Encoding", "identity")
	if reqBody != nil {
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
		req.ContentLength = int64(len(reqBody))
	}

	resp, err := p.upstreamClient.Do(req)
	if err != nil {
		log.Printf("[Proxy] HTTP 请求失败: %v", err)
		conn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))

	if p.handler != nil {
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		go p.handler(req.URL.Host, reqBody, respBody, req, resp)
	}

	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	_ = resp.Write(conn)
}
