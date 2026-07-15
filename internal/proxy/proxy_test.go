package proxy

import (
	"bytes"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestSingleConnListener 验证单连接 Listener 的基本行为
func TestSingleConnListener(t *testing.T) {
	// 创建一对连接
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	listener := newSingleConnListener(serverConn)

	// 第一次 Accept 应该返回连接
	conn1, err := listener.Accept()
	if err != nil {
		t.Fatalf("第一次 Accept 失败: %v", err)
	}
	if conn1 == nil {
		t.Fatal("返回的连接为 nil")
	}

	// 在另一个 goroutine 中等待第二次 Accept（会阻塞直到连接关闭）
	acceptDone := make(chan error, 1)
	go func() {
		_, err := listener.Accept()
		acceptDone <- err
	}()

	// 关闭连接，应该让第二次 Accept 返回错误
	conn1.Close()

	select {
	case err := <-acceptDone:
		if err == nil {
			t.Log("第二次 Accept 返回 nil 错误（可接受）")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("第二次 Accept 超时，listener 没有正确响应连接关闭")
	}
}

// TestMITMProxyHTTP 验证 MITM 代理转发普通 HTTP 请求
func TestMITMProxyHTTP(t *testing.T) {
	// 创建上游测试服务器
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
	}))
	defer upstream.Close()

	// 提取上游 host:port
	upstreamURL := upstream.URL
	upstreamHost := strings.TrimPrefix(upstreamURL, "http://")

	// 创建捕获处理器
	var capturedHost string
	var capturedRespBody []byte
	handler := func(host string, reqBody []byte, respBody []byte, req *http.Request, resp *http.Response) {
		capturedHost = host
		capturedRespBody = respBody
	}

	// 创建代理（不需要 CA 证书，只测试 HTTP）
	proxy := &MITMProxy{
		addr:           "127.0.0.1:0",
		handler:        handler,
		upstreamClient: upstream.Client(),
	}

	// 启动代理监听
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	proxyAddr := ln.Addr().String()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go proxy.handleConn(conn)
		}
	}()

	// 通过代理发送 HTTP 请求
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse("http://" + proxyAddr)
			},
		},
	}

	resp, err := client.Get(upstreamURL + "/test")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("prompt_tokens")) {
		t.Errorf("响应体不包含预期内容: %s", string(body))
	}

	// 等待异步处理器完成
	time.Sleep(100 * time.Millisecond)

	if capturedHost != upstreamHost {
		t.Errorf("捕获的 host 不匹配: got %s, want %s", capturedHost, upstreamHost)
	}
	if !bytes.Contains(capturedRespBody, []byte("prompt_tokens")) {
		t.Errorf("捕获的响应体不包含预期内容: %s", string(capturedRespBody))
	}
}

// TestMITMProxySSE 验证 MITM 代理正确转发 SSE 流式响应
func TestMITMProxySSE(t *testing.T) {
	// 创建上游 SSE 服务器
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher := w.(http.Flusher)
		// 发送多个 SSE 事件
		events := []string{
			"data: {\"token\":\"hello\"}\n\n",
			"data: {\"token\":\"world\"}\n\n",
			"data: [DONE]\n\n",
		}
		for _, evt := range events {
			w.Write([]byte(evt))
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	// 创建捕获处理器
	var capturedRespBody []byte
	handler := func(host string, reqBody []byte, respBody []byte, req *http.Request, resp *http.Response) {
		capturedRespBody = respBody
	}

	// 创建代理
	proxy := &MITMProxy{
		addr:           "127.0.0.1:0",
		handler:        handler,
		upstreamClient: upstream.Client(),
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	proxyAddr := ln.Addr().String()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go proxy.handleConn(conn)
		}
	}()

	// 通过代理发送请求
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				return url.Parse("http://" + proxyAddr)
			},
		},
	}

	resp, err := client.Get(upstream.URL + "/stream")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取完整 SSE 流
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读取响应失败: %v", err)
	}

	// 验证所有事件都被转发
	if !bytes.Contains(body, []byte("hello")) {
		t.Errorf("SSE 流缺少 'hello' 事件: %s", string(body))
	}
	if !bytes.Contains(body, []byte("world")) {
		t.Errorf("SSE 流缺少 'world' 事件: %s", string(body))
	}
	if !bytes.Contains(body, []byte("[DONE]")) {
		t.Errorf("SSE 流缺少 [DONE] 事件: %s", string(body))
	}

	// 等待异步处理器完成
	time.Sleep(100 * time.Millisecond)

	// 验证捕获的响应体
	if !bytes.Contains(capturedRespBody, []byte("[DONE]")) {
		t.Errorf("捕获的 SSE 响应体缺少 [DONE]: %s", string(capturedRespBody))
	}
}

// TestCertManagerSignHost 验证证书签发
func TestCertManagerSignHost(t *testing.T) {
	tmpDir := t.TempDir()
	cm, err := NewCertManager(tmpDir)
	if err != nil {
		t.Fatalf("创建证书管理器失败: %v", err)
	}

	// 签发证书
	cert, err := cm.SignHost("api.example.com")
	if err != nil {
		t.Fatalf("签发证书失败: %v", err)
	}

	// 验证证书
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("解析证书失败: %v", err)
	}

	if parsed.DNSNames[0] != "api.example.com" {
		t.Errorf("证书 DNS 名称不匹配: got %s, want api.example.com", parsed.DNSNames[0])
	}

	// 验证缓存（第二次签发应该返回缓存的证书）
	cert2, err := cm.SignHost("api.example.com")
	if err != nil {
		t.Fatalf("第二次签发证书失败: %v", err)
	}
	if !bytes.Equal(cert.Certificate[0], cert2.Certificate[0]) {
		t.Error("第二次签发没有返回缓存的证书")
	}
}

// TestHandleMITMRequestHeaders 验证 MITM 请求头处理
func TestHandleMITMRequestHeaders(t *testing.T) {
	// 创建 HTTPS 上游服务器验证收到的请求头
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") != "identity" {
			t.Errorf("上游没有收到 Accept-Encoding: identity，got: %s", r.Header.Get("Accept-Encoding"))
		}
		if r.Host == "" {
			t.Error("上游没有收到 Host 头")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	upstreamHost := strings.TrimPrefix(upstream.URL, "https://")

	// 上游客户端跳过 TLS 校验（测试用）
	upstreamClient := upstream.Client()
	upstreamClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true

	proxy := &MITMProxy{
		handler:        func(string, []byte, []byte, *http.Request, *http.Response) {},
		upstreamClient: upstreamClient,
	}

	// 模拟 MITM 请求
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte(`{"model":"gpt-4"}`)))
	req.Host = upstreamHost
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	proxy.handleMITMRequest(w, req, upstreamHost)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，got %d", w.Code)
	}
}

// 辅助：用于 TestMITMProxyHTTP 和 TestMITMProxySSE 中的 url 导入
// （单独放在这里以便测试文件自包含）
