package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CertManager 管理 CA 证书和动态签发
type CertManager struct {
	caDir     string
	caCert    *x509.Certificate
	caKey     *rsa.PrivateKey
	caTLS     tls.Certificate
	mu        sync.RWMutex
	certCache map[string]tls.Certificate
}

// NewCertManager 创建证书管理器
func NewCertManager(caDir string) (*CertManager, error) {
	if err := os.MkdirAll(caDir, 0755); err != nil {
		return nil, fmt.Errorf("无法创建 CA 目录: %w", err)
	}

	cm := &CertManager{
		caDir:     caDir,
		certCache: make(map[string]tls.Certificate),
	}

	if err := cm.loadOrGenerateCA(); err != nil {
		return nil, err
	}

	return cm, nil
}

// loadOrGenerateCA 加载或生成 CA 根证书
func (cm *CertManager) loadOrGenerateCA() error {
	certPath := filepath.Join(cm.caDir, "ca.crt")
	keyPath := filepath.Join(cm.caDir, "ca.key")

	// 尝试加载已有证书
	if certData, err := os.ReadFile(certPath); err == nil {
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("读取 CA 私钥失败: %w", err)
		}

		cert, err := tls.X509KeyPair(certData, keyData)
		if err != nil {
			return fmt.Errorf("解析 CA 证书失败: %w", err)
		}

		parsedCert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return fmt.Errorf("解析 CA 证书失败: %w", err)
		}

		key, ok := cert.PrivateKey.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("CA 私钥不是 RSA 类型")
		}

		cm.caCert = parsedCert
		cm.caKey = key
		cm.caTLS = cert
		return nil
	}

	// 生成新 CA
	return cm.generateCA(certPath, keyPath)
}

// generateCA 生成新的 CA 根证书
func (cm *CertManager) generateCA(certPath, keyPath string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("生成 CA 密钥失败: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("生成序列号失败: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"TickToken"},
			CommonName:   "TickToken CA",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("创建 CA 证书失败: %w", err)
	}

	// 保存证书
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("保存 CA 证书失败: %w", err)
	}

	// 保存私钥
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("保存 CA 私钥失败: %w", err)
	}

	parsedCert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return fmt.Errorf("解析生成的 CA 证书失败: %w", err)
	}

	cm.caCert = parsedCert
	cm.caKey = key
	cm.caTLS = tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}

	fmt.Printf("[TickToken] 已生成 CA 根证书: %s\n", certPath)
	fmt.Printf("[TickToken] 请将此证书安装到系统信任存储中\n")

	return nil
}

// GetCACertPath 返回 CA 证书路径
func (cm *CertManager) GetCACertPath() string {
	return filepath.Join(cm.caDir, "ca.crt")
}

// SignHost 为指定域名动态签发证书
func (cm *CertManager) SignHost(host string) (tls.Certificate, error) {
	cm.mu.RLock()
	if cert, ok := cm.certCache[host]; ok {
		cm.mu.RUnlock()
		return cert, nil
	}
	cm.mu.RUnlock()

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 双重检查
	if cert, ok := cm.certCache[host]; ok {
		return cert, nil
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("生成主机密钥失败: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("生成序列号失败: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"TickToken"},
			CommonName:   host,
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{host},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, cm.caCert, &key.PublicKey, cm.caKey)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("签发证书失败: %w", err)
	}

	cert := tls.Certificate{
		Certificate: [][]byte{certDER, cm.caCert.Raw},
		PrivateKey:  key,
	}

	cm.certCache[host] = cert
	return cert, nil
}
