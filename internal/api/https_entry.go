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
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

const (
	defaultBackendPort = 8080
	httpAddress        = ":80"
	httpsAddress       = ":443"
	modeAuto           = httpsMode("auto")
	modeExternal       = httpsMode("external")
	modeOff            = httpsMode("off")
)

type httpsMode string

// HTTPSEntry 根据部署模式代理 Wails server 并管理 HTTPS。
type HTTPSEntry struct {
	mode        httpsMode
	cache       autocert.Cache
	cachePath   string
	manager     *autocert.Manager
	proxy       *httputil.ReverseProxy
	httpServer  *http.Server
	httpsServer *http.Server
	allowed     sync.Map
}

// NewHTTPSEntry 根据部署模式创建 HTTPS 入口。
func NewHTTPSEntry() (*HTTPSEntry, error) {
	mode, err := httpsModeFromEnv()
	if err != nil {
		return nil, err
	}
	service := &HTTPSEntry{mode: mode}
	if mode != modeAuto {
		return service, nil
	}

	backendURL := &url.URL{Scheme: "http", Host: net.JoinHostPort("127.0.0.1", strconv.Itoa(serverPort()))}
	service.proxy = httputil.NewSingleHostReverseProxy(backendURL)
	service.proxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
		slog.Warn("转发 HTTPS 请求失败", "host", request.Host, "path", request.URL.Path, "error", err)
		http.Error(writer, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
	}

	cachePath := strings.TrimSpace(os.Getenv("CERVI_TLS_DATA_DIR"))
	if cachePath == "" {
		cachePath = "data/tls"
	}
	service.cachePath = cachePath
	service.cache = autocert.DirCache(cachePath)
	service.manager = &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      service.cache,
		HostPolicy: service.allowCertificate,
		Email:      strings.TrimSpace(os.Getenv("CERVI_ACME_EMAIL")),
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
	return service, nil
}

// Start 根据部署模式启动自动 HTTPS 入口。
func (s *HTTPSEntry) Start(ctx context.Context) error {
	if s.mode == modeExternal {
		slog.Info("HTTPS 由外部入口管理", "server_port", serverPort())
		return nil
	}
	if s.mode != modeAuto {
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
	s.allowed.Store(host, struct{}{})
	redirectToHTTPS(writer, request, host)
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
		s.allowed.Store(host, struct{}{})
		slog.Info("已从证书缓存恢复 HTTPS 域名", "domain", host)
		return nil
	}
	return fmt.Errorf("host has not entered through HTTP")
}

// cachedCertificateMatches 判断持久化缓存中是否存在属于该域名的证书。
func (s *HTTPSEntry) cachedCertificateMatches(ctx context.Context, host string) bool {
	if s.cache == nil {
		return false
	}
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
	host := strings.TrimSpace(value)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
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

// serverPort 返回 Wails server 实际使用的端口。
func serverPort() int {
	value := os.Getenv("WAILS_SERVER_PORT")
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return defaultBackendPort
	}
	return port
}

// httpsModeFromEnv 返回配置的 HTTPS 模式，留空时关闭 HTTPS 入口。
func httpsModeFromEnv() (httpsMode, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("CERVI_HTTPS_MODE")))
	if value == "" {
		return modeOff, nil
	}
	mode := httpsMode(value)
	switch mode {
	case modeAuto, modeExternal, modeOff:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid CERVI_HTTPS_MODE %q: expected auto, external, or off", value)
	}
}
