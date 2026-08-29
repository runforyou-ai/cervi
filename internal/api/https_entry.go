//go:build server

package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"golang.org/x/crypto/acme/autocert"
)

const (
	httpAddress  = ":80"
	httpsAddress = ":443"
	modeAuto     = tlsMode("auto")
	modeExternal = tlsMode("external")
	modeOff      = tlsMode("off")
	// maxAllowedHostEntries 限制已确认公网域名缓存的条目数，防止恶意 Host 头刷量造成内存无界增长。
	maxAllowedHostEntries = 1024
)

type tlsMode string

// HTTPSEntry 按 TLS 模式代理 Wails server。
type HTTPSEntry struct {
	mode         tlsMode
	cache        autocert.Cache
	cachePath    string
	manager      *autocert.Manager
	proxy        *httputil.ReverseProxy
	httpServer   *http.Server
	httpsServer  *http.Server
	allowed      sync.Map
	allowedMutex sync.Mutex
	allowedCount int
}

// NewHTTPSEntry 根据 TLS 配置创建 HTTPS 入口。
func NewHTTPSEntry(config serverconfig.TLSConfig, backend serverconfig.ServerConfig) *HTTPSEntry {
	mode := tlsMode(config.Mode)
	service := &HTTPSEntry{mode: mode}
	if mode != modeAuto {
		return service
	}
	if config.ACMEEmail == "" {
		slog.Warn("自动 HTTPS 未配置 ACME 联系邮箱")
	}

	backendURL := &url.URL{Scheme: "http", Host: net.JoinHostPort(proxyHost(backend.Host), strconv.Itoa(backend.Port))}
	service.proxy = httputil.NewSingleHostReverseProxy(backendURL)
	director := service.proxy.Director
	service.proxy.Director = func(request *http.Request) {
		director(request)
		protocol := "http"
		if request.TLS != nil {
			protocol = "https"
		}
		request.Header.Set("X-Forwarded-Proto", protocol)
	}
	service.proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
		slog.Warn("转发 HTTPS 请求失败", "host", request.Host, "path", request.URL.Path, "error", err)
		http.Error(writer, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
	}

	service.cachePath = config.DataDirectory
	service.cache = autocert.DirCache(config.DataDirectory)
	service.manager = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      service.cache,
		HostPolicy: service.allowCertificate,
		Email:      config.ACMEEmail,
	}
	service.httpServer = &http.Server{
		Addr:              httpAddress,
		Handler:           service.manager.HTTPHandler(http.HandlerFunc(service.serveHTTP)),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	service.httpsServer = &http.Server{
		Addr:              httpsAddress,
		Handler:           service.proxy,
		TLSConfig:         service.manager.TLSConfig(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return service
}

// proxyHost 返回 HTTPS 入口访问服务监听器使用的地址。
func proxyHost(host string) string {
	host = strings.Trim(host, "[]")
	switch host {
	case "0.0.0.0":
		return "127.0.0.1"
	case "::":
		return "::1"
	default:
		return host
	}
}

// Start 启动 HTTPS 入口。
func (s *HTTPSEntry) Start(ctx context.Context) error {
	if s.mode == modeExternal {
		slog.Info("TLS 由外部入口终止")
		return nil
	}
	if s.mode == modeOff {
		slog.Info("TLS 入口已关闭")
		return nil
	}
	httpListener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP on %s: %w", s.httpServer.Addr, err)
	}
	httpsListener, err := net.Listen("tcp", s.httpsServer.Addr)
	if err != nil {
		httpListener.Close()
		return fmt.Errorf("listen HTTPS on %s: %w", s.httpsServer.Addr, err)
	}

	go s.serve("HTTP", func() error { return s.httpServer.Serve(httpListener) })
	go s.serve("HTTPS", func() error { return s.httpsServer.ServeTLS(httpsListener, "", "") })
	go func() {
		<-ctx.Done()
		if err := s.Shutdown(); err != nil {
			slog.Warn("关闭自动 HTTPS 服务失败", "error", err)
		}
	}()
	slog.Info("自动 HTTPS 入口已启动", "http", httpAddress, "https", httpsAddress, "certificate_cache", s.cachePath)
	return nil
}

// Shutdown 关闭 HTTP 和 HTTPS 监听器。
func (s *HTTPSEntry) Shutdown() error {
	if s.mode != modeAuto {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpErr := s.httpServer.Shutdown(ctx)
	httpsErr := s.httpsServer.Shutdown(ctx)
	return errors.Join(httpErr, httpsErr)
}

// serve 运行一个入口并记录无法恢复的监听错误。
func (s *HTTPSEntry) serve(protocol string, run func() error) {
	if err := run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("自动 HTTPS 入口停止", "protocol", protocol, "error", err)
	}
}

// serveHTTP 为本地地址保留 HTTP，并为公网域名启用 HTTPS。
func (s *HTTPSEntry) serveHTTP(writer http.ResponseWriter, request *http.Request) {
	host, local := requestHost(request.Host)
	if local {
		s.proxy.ServeHTTP(writer, request)
		return
	}
	if host == "" {
		http.NotFound(writer, request)
		return
	}
	s.rememberAllowedHost(host)
	redirectToHTTPS(writer, request, host)
}

// rememberAllowedHost 缓存已确认的公网域名；达到上限时整体清空重建，避免恶意 Host 头刷量挤掉合法新域名的签发授权。
func (s *HTTPSEntry) rememberAllowedHost(host string) {
	s.allowedMutex.Lock()
	defer s.allowedMutex.Unlock()
	if _, ok := s.allowed.Load(host); ok {
		return
	}
	if s.allowedCount >= maxAllowedHostEntries {
		s.allowed.Clear()
		s.allowedCount = 0
	}
	s.allowed.Store(host, struct{}{})
	s.allowedCount++
}

// allowCertificate 只允许通过 HTTP 入口或已有证书缓存确认的公网域名。
func (s *HTTPSEntry) allowCertificate(ctx context.Context, host string) error {
	host, local := requestHost(host)
	if host == "" || local {
		return fmt.Errorf("host is not a public domain")
	}
	if _, ok := s.allowed.Load(host); ok {
		return nil
	}
	if s.cachedCertificateMatches(ctx, host) {
		s.rememberAllowedHost(host)
		slog.Info("已从证书缓存恢复 HTTPS 域名", "domain", host)
		return nil
	}
	return fmt.Errorf("host has not entered through HTTP")
}

// cachedCertificateMatches 判断持久化缓存中是否存在属于该域名的证书。
func (s *HTTPSEntry) cachedCertificateMatches(ctx context.Context, host string) bool {
	for _, key := range []string{host, host + "+rsa"} {
		data, err := s.cache.Get(ctx, key)
		if errors.Is(err, autocert.ErrCacheMiss) {
			continue
		}
		if err != nil {
			slog.Warn("读取证书缓存失败", "domain", host, "cache_key", key, "error", err)
			continue
		}
		certificate, err := tls.X509KeyPair(data, data)
		if err != nil {
			slog.Warn("证书缓存内容无效", "domain", host, "cache_key", key, "error", err)
			continue
		}
		if len(certificate.Certificate) == 0 {
			slog.Warn("证书缓存缺少证书链", "domain", host, "cache_key", key)
			continue
		}
		leaf, err := x509.ParseCertificate(certificate.Certificate[0])
		if err != nil {
			slog.Warn("解析缓存证书失败", "domain", host, "cache_key", key, "error", err)
			continue
		}
		if err := leaf.VerifyHostname(host); err == nil {
			return true
		}
		slog.Warn("缓存证书域名不匹配", "domain", host, "cache_key", key)
	}
	return false
}

// requestHost 规范化请求域名并判断是否应当保留 HTTP。
func requestHost(value string) (string, bool) {
	host := tenant.NormalizeHostname(value)
	if host == "" {
		return "", false
	}
	if ip := net.ParseIP(host); ip != nil {
		return host, true
	}
	local := host == "localhost" ||
		!strings.Contains(host, ".") ||
		strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".home.arpa")
	return host, local
}

// redirectToHTTPS 把公网 HTTP 请求跳转到相同域名和路径的 HTTPS 地址。
func redirectToHTTPS(writer http.ResponseWriter, request *http.Request, host string) {
	target := url.URL{Scheme: "https", Host: host, Path: request.URL.Path, RawQuery: request.URL.RawQuery}
	http.Redirect(writer, request, target.String(), http.StatusTemporaryRedirect)
}
