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
)

// PayloadHandler 处理捕获的请求/响应 payload
type PayloadHandler func(host string, reqBody []byte, respBody []byte, req *http.Request, resp *http.Response)

// MITMProxy HTTPS MITM 代理
type MITMProxy struct {
	addr    string
	certMgr *CertManager
	handler PayloadHandler
}

// NewMITMProxy 创建 MITM 代理
func NewMITMProxy(addr string, certMgr *CertManager, handler PayloadHandler) *MITMProxy {
	return &MITMProxy{
		addr:    addr,
		certMgr: certMgr,
		handler: handler,
	}
}

// Start 启动代理
func (p *MITMProxy) Start() error {
	ln, err := net.Listen("tcp", p.addr)
	if err != nil {
		return fmt.Errorf("代理监听失败: %w", err)
	}
	log.Printf("[Proxy] MITM 代理监听于 %s", p.addr)

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
		// 普通 HTTP 代理请求
		p.handleHTTP(conn, req)
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
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	tlsConn := tls.Server(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("[Proxy] TLS 握手失败 %s: %v", host, err)
		return
	}
	defer tlsConn.Close()

	// 在 TLS 隧道上读取 HTTP 请求
	tlsBr := bufio.NewReader(tlsConn)
	for {
		req, err := http.ReadRequest(tlsBr)
		if err != nil {
			break
		}
		p.handleHTTPSRequest(tlsConn, host, req)
	}
}

// handleHTTPSRequest 处理 TLS 隧道内的 HTTP 请求
func (p *MITMProxy) handleHTTPSRequest(clientConn net.Conn, host string, req *http.Request) {
	// 读取请求体
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}

	// 转发请求到真实服务器
	targetAddr := host
	if _, _, err := net.SplitHostPort(targetAddr); err != nil {
		targetAddr = net.JoinHostPort(targetAddr, "443")
	}

	upstream, err := tls.Dial("tcp", targetAddr, &tls.Config{
		InsecureSkipVerify: false,
	})
	if err != nil {
		log.Printf("[Proxy] 连接上游失败 %s: %v", targetAddr, err)
		return
	}
	defer upstream.Close()

	// 重写请求发送给上游
	req.URL.Scheme = "https"
	req.URL.Host = host
	req.RequestURI = ""

	// 恢复请求体
	if reqBody != nil {
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
		req.ContentLength = int64(len(reqBody))
	}

	if err := req.Write(upstream); err != nil {
		log.Printf("[Proxy] 转发请求失败 %s: %v", host, err)
		return
	}

	// 读取上游响应
	resp, err := http.ReadResponse(bufio.NewReader(upstream), req)
	if err != nil {
		log.Printf("[Proxy] 读取响应失败 %s: %v", host, err)
		return
	}

	var respBody []byte
	if resp.Body != nil {
		respBody, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
	}

	// 调用 payload 处理器
	if p.handler != nil {
		p.handler(host, reqBody, respBody, req, resp)
	}

	// 恢复响应体返回给客户端
	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	if err := resp.Write(clientConn); err != nil {
		log.Printf("[Proxy] 返回响应失败 %s: %v", host, err)
	}
}

// handleHTTP 处理普通 HTTP 代理请求
func (p *MITMProxy) handleHTTP(conn net.Conn, req *http.Request) {
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}

	// 转发请求
	client := &http.Client{}
	req.RequestURI = ""
	if reqBody != nil {
		req.Body = io.NopCloser(bytes.NewReader(reqBody))
		req.ContentLength = int64(len(reqBody))
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Proxy] HTTP 请求失败: %v", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if p.handler != nil {
		p.handler(req.URL.Host, reqBody, respBody, req, resp)
	}

	resp.Body = io.NopCloser(bytes.NewReader(respBody))
	_ = resp.Write(conn)
}
