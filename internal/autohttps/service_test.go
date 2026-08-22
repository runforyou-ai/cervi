//go:build server

package autohttps

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// TestHTTPSModeFromEnv 验证显式模式和统一默认模式。
func TestHTTPSModeFromEnv(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      httpsMode
		wantError bool
	}{
		{name: "empty defaults to off", want: modeOff},
		{name: "explicit auto", value: "auto", want: modeAuto},
		{name: "explicit external", value: "external", want: modeExternal},
		{name: "explicit off", value: "off", want: modeOff},
		{name: "case insensitive", value: " EXTERNAL ", want: modeExternal},
		{name: "invalid", value: "proxy", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("CERVI_HTTPS_MODE", test.value)
			mode, err := httpsModeFromEnv()
			if test.wantError {
				if err == nil {
					t.Fatal("expected invalid mode error")
				}
				return
			}
			if err != nil || mode != test.want {
				t.Fatalf("httpsModeFromEnv() = (%q, %v), want (%q, nil)", mode, err, test.want)
			}
		})
	}
}

// TestRequestHostKeepsLocalAddressesOnHTTP 验证本地和内网地址不会申请公网证书。
func TestRequestHostKeepsLocalAddressesOnHTTP(t *testing.T) {
	tests := []struct {
		value string
		host  string
		local bool
	}{
		{value: "localhost", host: "localhost", local: true},
		{value: "localhost:8080", host: "localhost", local: true},
		{value: "127.0.0.1:8080", host: "127.0.0.1", local: true},
		{value: "[::1]:8080", host: "::1", local: true},
		{value: "192.168.1.10", host: "192.168.1.10", local: true},
		{value: "47.239.49.135", host: "47.239.49.135", local: true},
		{value: "cervi.internal", host: "cervi.internal", local: true},
		{value: "test-https.runforyou.app", host: "test-https.runforyou.app", local: false},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			host, local := requestHost(test.value)
			if host != test.host || local != test.local {
				t.Fatalf("requestHost(%q) = (%q, %t), want (%q, %t)", test.value, host, local, test.host, test.local)
			}
		})
	}
}

// TestAllowCertificateRequiresHTTPEntry 验证无缓存的公网域名必须先通过 HTTP 入口访问。
func TestAllowCertificateRequiresHTTPEntry(t *testing.T) {
	service := &Service{}
	const host = "test-https.runforyou.app"
	if err := service.allowCertificate(t.Context(), host); err == nil {
		t.Fatal("expected unapproved domain to be rejected")
	}
	service.allowed.Store(host, struct{}{})
	if err := service.allowCertificate(t.Context(), host); err != nil {
		t.Fatalf("approved domain rejected: %v", err)
	}
	service.allowed.Store("localhost", struct{}{})
	if err := service.allowCertificate(t.Context(), "localhost"); err == nil {
		t.Fatal("expected localhost to be rejected")
	}
}

// TestNewServiceExternalDoesNotCreateListeners 验证外部模式不会创建自动 HTTPS 监听器。
func TestNewServiceExternalDoesNotCreateListeners(t *testing.T) {
	t.Setenv("CERVI_HTTPS_MODE", "external")
	service, err := NewService()
	if err != nil {
		t.Fatalf("new external service: %v", err)
	}
	if service.mode != modeExternal || service.httpServer != nil || service.httpsServer != nil {
		t.Fatal("expected external mode without HTTP or HTTPS listeners")
	}
}

// TestAllowCertificateRestoresCachedDomain 验证重启后可以从持久化证书恢复域名许可。
func TestAllowCertificateRestoresCachedDomain(t *testing.T) {
	const host = "cached.runforyou.app"
	cache := autocert.DirCache(t.TempDir())
	if err := cache.Put(t.Context(), host, cachedCertificate(t, host)); err != nil {
		t.Fatalf("cache certificate: %v", err)
	}
	service := &Service{cache: cache}
	if err := service.allowCertificate(t.Context(), host); err != nil {
		t.Fatalf("cached domain rejected: %v", err)
	}
	if _, ok := service.allowed.Load(host); !ok {
		t.Fatal("expected cached domain to be restored in memory")
	}
	if err := service.allowCertificate(t.Context(), "other.runforyou.app"); err == nil {
		t.Fatal("expected domain without matching certificate to be rejected")
	}
}

// cachedCertificate 创建符合 autocert 缓存格式的测试证书。
func cachedCertificate(t *testing.T, host string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     []string{host},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certificateDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	var data bytes.Buffer
	if err := pem.Encode(&data, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatalf("encode private key: %v", err)
	}
	if err := pem.Encode(&data, &pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}); err != nil {
		t.Fatalf("encode certificate: %v", err)
	}
	return data.Bytes()
}
