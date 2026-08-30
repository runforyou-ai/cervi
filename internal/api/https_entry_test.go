//go:build server

package api

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"golang.org/x/crypto/acme/autocert"
)

type fixedTenantResolver string

// Resolve 只返回测试指定的企业访问地址。
func (r fixedTenantResolver) Resolve(_ context.Context, accessHost string) (tenant.Scope, error) {
	if accessHost == string(r) {
		return tenant.Scope{OrganizationID: "organization-id"}, nil
	}
	return tenant.Scope{}, tenant.ErrNotFound
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
	service := &HTTPSEntry{cache: autocert.DirCache(t.TempDir())}
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

// TestNewHTTPSEntryExternalDoesNotCreateListeners 验证外部模式不会创建自动 HTTPS 监听器。
func TestNewHTTPSEntryExternalDoesNotCreateListeners(t *testing.T) {
	service := NewHTTPSEntry(
		serverconfig.TLSConfig{Mode: "external"},
		serverconfig.ServerConfig{Host: "127.0.0.1", Port: 8080},
		nil,
		nil,
	)
	if service.mode != modeExternal || service.httpServer != nil || service.httpsServer != nil {
		t.Fatal("expected external mode without HTTP or HTTPS listeners")
	}
}

// TestCertificateRequestLimiterBoundsNewAttempts 验证新证书请求受滑动时间窗口限制。
func TestCertificateRequestLimiterBoundsNewAttempts(t *testing.T) {
	limiter := &certificateRequestLimiter{}
	now := time.Now()
	for index := range certificateRequestLimit {
		if !limiter.allow("host-"+big.NewInt(int64(index)).String()+".example.com", now) {
			t.Fatalf("request %d rejected before the limit", index+1)
		}
	}
	if limiter.allow("overflow.example.com", now) {
		t.Fatal("expected request beyond the limit to be rejected")
	}
	if !limiter.allow("host-0.example.com", now.Add(certificateRequestRetryWindow-time.Second)) {
		t.Fatal("expected duplicate host inside retry window to share the attempt")
	}
	if !limiter.allow("after-window.example.com", now.Add(certificateRequestWindow+time.Second)) {
		t.Fatal("expected request after the sliding window to be accepted")
	}
}

// TestProxyHostFollowsServerListener 验证 HTTPS 反代使用可访问的监听地址。
func TestProxyHostFollowsServerListener(t *testing.T) {
	tests := map[string]string{
		"0.0.0.0":   "127.0.0.1",
		"[::]":      "::1",
		"[::1]":     "::1",
		"localhost": "localhost",
	}
	for host, expected := range tests {
		if actual := proxyHost(host); actual != expected {
			t.Errorf("proxyHost(%q) = %q, want %q", host, actual, expected)
		}
	}
}

// TestAllowCertificateRestoresCachedDomain 验证重启后可以从持久化证书恢复域名许可。
func TestAllowCertificateRestoresCachedDomain(t *testing.T) {
	const host = "cached.runforyou.app"
	cache := autocert.DirCache(t.TempDir())
	if err := cache.Put(t.Context(), host, cachedCertificate(t, host, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))); err != nil {
		t.Fatalf("cache certificate: %v", err)
	}
	service := &HTTPSEntry{cache: cache}
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

// TestAllowCertificateRejectsExpiredCachedDomain 验证过期缓存不会绕过新证书限制。
func TestAllowCertificateRejectsExpiredCachedDomain(t *testing.T) {
	const host = "expired.runforyou.app"
	cache := autocert.DirCache(t.TempDir())
	data := cachedCertificate(t, host, time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	if err := cache.Put(t.Context(), host, data); err != nil {
		t.Fatalf("cache expired certificate: %v", err)
	}
	service := &HTTPSEntry{cache: cache}
	if err := service.allowCertificate(t.Context(), host); err == nil {
		t.Fatal("expected expired cached domain without HTTP entry to be rejected")
	}
	if _, ok := service.certified.Load(host); ok {
		t.Fatal("expected expired cached domain not to be marked as certified")
	}
}

// TestAllowCertificateRestoresTenantDomain 验证已绑定企业的域名可以从 HTTPS 直接触发签发。
func TestAllowCertificateRestoresTenantDomain(t *testing.T) {
	const host = "tenant.runforyou.app"
	service := &HTTPSEntry{
		cache:  autocert.DirCache(t.TempDir()),
		tenant: fixedTenantResolver(host),
	}
	if err := service.allowCertificate(t.Context(), host); err != nil {
		t.Fatalf("tenant domain rejected: %v", err)
	}
	if _, ok := service.allowed.Load(host); !ok {
		t.Fatal("expected tenant domain to be restored in memory")
	}
	if err := service.allowCertificate(t.Context(), "unknown.runforyou.app"); err == nil {
		t.Fatal("expected unknown domain without HTTP entry to be rejected")
	}
}

// cachedCertificate 创建符合 autocert 缓存格式的测试证书。
func cachedCertificate(t *testing.T, host string, notBefore, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     []string{host},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
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
