//go:build server

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// deviceModelMaxRequestBytes 是设备单次模型请求体的上限，覆盖随消息直传的附件。
const deviceModelMaxRequestBytes = 64 << 20

// deviceModelProxyOrigin 是计算品牌入口路径前缀时使用的占位源地址。
const deviceModelProxyOrigin = "http://device-model-proxy"

// DeviceModelAuthorizer 校验设备模型代理请求并返回运行锁定的上游模型服务。
type DeviceModelAuthorizer interface {
	// AuthorizeDeviceModelRequest 校验请求来自持有该运行有效租约的本人未撤销设备，并返回上游模型服务。
	AuthorizeDeviceModelRequest(ctx context.Context, meta appservice.RequestMeta, runID string) (appservice.DeviceModelUpstream, error)
}

// WithDeviceModelProxy 注入设备模型代理的请求授权。
func WithDeviceModelProxy(authorizer DeviceModelAuthorizer) ServiceOption {
	return func(service *Service) {
		service.deviceModels = authorizer
	}
}

// proxyDeviceModel 把设备运行的模型请求转发给运行锁定的上游模型服务：换成供应商凭据，要求请求的模型与配置版本一致，流式响应逐块透传。
func (s *Service) proxyDeviceModel(c *gin.Context) {
	meta := requestMeta(c)
	upstream, err := s.deviceModels.AuthorizeDeviceModelRequest(c.Request.Context(), meta, c.Param("runID"))
	if writeApplicationError(c, err) {
		return
	}
	target, err := url.Parse(upstream.BaseURL)
	if err != nil {
		writeApplicationError(c, err)
		return
	}
	// 设备按同一品牌规则拼接入口，去掉品牌附加的路径前缀后接到上游入口。
	proxyBase, err := common.CompatibleModelBaseURL(upstream.Brand, deviceModelProxyOrigin)
	if err != nil {
		writeApplicationError(c, err)
		return
	}
	endpoint, ok := strings.CutPrefix(c.Param("path"), strings.TrimPrefix(proxyBase, deviceModelProxyOrigin))
	if !ok || endpoint == "" {
		writeApplicationError(c, appservice.InvalidError(meta, cervii18n.ErrorValidationFailed, nil))
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, deviceModelMaxRequestBytes))
	if err != nil {
		writeApplicationError(c, appservice.InvalidError(meta, cervii18n.ErrorValidationFailed, nil))
		return
	}
	if !deviceModelRequestMatches(upstream, endpoint, body) {
		slog.Warn("设备模型请求的模型与运行配置不一致", "agent_run_id", c.Param("runID"), "brand", upstream.Brand)
		writeApplicationError(c, appservice.InvalidError(meta, cervii18n.ErrorValidationFailed, nil))
		return
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.Out.URL.Scheme, request.Out.URL.Host, request.Out.Host = target.Scheme, target.Host, target.Host
			request.Out.URL.Path = strings.TrimRight(target.Path, "/") + endpoint
			request.Out.URL.RawPath = ""
			query := request.In.URL.Query()
			query.Del("key")
			request.Out.URL.RawQuery = query.Encode()
			request.Out.Body = io.NopCloser(bytes.NewReader(body))
			request.Out.ContentLength = int64(len(body))
			// 去掉设备登录凭据，按品牌写入供应商凭据。
			for _, header := range []string{"Authorization", appservice.DeviceHeader, "X-Api-Key", "X-Goog-Api-Key", "Api-Key", "Cookie"} {
				request.Out.Header.Del(header)
			}
			if upstream.APIKey == "" {
				return
			}
			switch domain.AIProviderBrand(upstream.Brand) {
			case domain.AIProviderBrandAnthropic:
				request.Out.Header.Set("X-Api-Key", upstream.APIKey)
			case domain.AIProviderBrandGoogle:
				request.Out.Header.Set("X-Goog-Api-Key", upstream.APIKey)
			default:
				request.Out.Header.Set("Authorization", "Bearer "+upstream.APIKey)
			}
		},
		FlushInterval: -1,
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			if request.Context().Err() != nil {
				return
			}
			slog.Warn("设备模型请求转发失败", "agent_run_id", c.Param("runID"), "brand", upstream.Brand, "error", err)
			writeApplicationError(c, appservice.UnavailableError(meta, cervii18n.ErrorDeviceRunRequestFailed, nil))
		},
	}
	// 只向代理暴露写入与刷新能力，连接断开由请求 context 感知。
	proxy.ServeHTTP(struct {
		http.ResponseWriter
		http.Flusher
	}{c.Writer, c.Writer}, c.Request)
}

// deviceModelRequestMatches 判断请求的模型是否为运行配置版本锁定的模型：Google 按路径中的模型名判断，其余品牌按请求体的 model 字段判断。
func deviceModelRequestMatches(upstream appservice.DeviceModelUpstream, endpoint string, body []byte) bool {
	if domain.AIProviderBrand(upstream.Brand) == domain.AIProviderBrandGoogle {
		return strings.Contains(endpoint, "/models/"+upstream.Identifier+":")
	}
	var payload struct {
		Model *string `json:"model"`
	}
	return json.Unmarshal(body, &payload) == nil && payload.Model != nil && *payload.Model == upstream.Identifier
}
