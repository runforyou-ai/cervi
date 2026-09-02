//go:build server

package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

const (
	websiteVisitorHeader    = "X-Cervi-Visitor-Token"
	websiteVisitorTokenSize = 32
	websiteVisitorBodyLimit = 16 * 1024
	websiteVisitorCookieAge = 365 * 24 * 60 * 60
)

// registerWebsiteVisitorRoutes 注册匿名网站 Messenger 路由。
func (s *Service) registerWebsiteVisitorRoutes(router *gin.Engine) {
	if s.websiteVisitor == nil {
		return
	}
	const messengerPath = "/public/website-channels/:channelID/messenger"
	const messagesPath = "/public/website-channels/:channelID/messages"
	const historyPath = "/public/website-channels/:channelID/conversations/:conversationID/messages"
	router.GET(messengerPath, s.initializeWebsiteMessenger)
	router.POST(messagesPath, s.sendWebsiteVisitorMessage)
	router.GET(historyPath, s.listWebsiteVisitorMessages)
	router.Match([]string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, messengerPath, websiteVisitorMethodNotAllowed(http.MethodGet))
	router.Match([]string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, messagesPath, websiteVisitorMethodNotAllowed(http.MethodPost))
	router.Match([]string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, historyPath, websiteVisitorMethodNotAllowed(http.MethodGet))
}

// initializeWebsiteMessenger 签发或恢复访客 Token 并返回会话列表。
func (s *Service) initializeWebsiteMessenger(c *gin.Context) {
	s.prepareWebsiteVisitorResponse(c)
	channelID := c.Param("channelID")
	token, valid := readWebsiteVisitorToken(c, channelID)
	issued := false
	if !valid {
		var err error
		token, err = generateWebsiteVisitorToken()
		if err != nil {
			slog.Warn("生成网站访客令牌失败", "channel_id", channelID, "error", err)
			writeApplicationError(c, appservice.FailedError(websiteVisitorRequestMeta(c), cervii18n.ErrorWebsiteMessengerLoadFailed))
			return
		}
		issued = true
	}
	result, err := s.websiteVisitor.InitializeMessenger(c.Request.Context(), websiteVisitorMeta(c), channelID, websiteVisitorExternalID(token), token)
	if writeApplicationError(c, err) {
		return
	}
	if issued {
		// 设置渠道级长期访客 Cookie。
		secure := c.Request.TLS != nil || (s.trustForwardedProto && strings.EqualFold(strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0]), "https"))
		sameSite := http.SameSiteLaxMode
		if secure {
			sameSite = http.SameSiteNoneMode
		}
		http.SetCookie(c.Writer, &http.Cookie{
			Name: websiteVisitorCookieName(channelID), Value: token, Path: "/", HttpOnly: true,
			Secure: secure, SameSite: sameSite, MaxAge: websiteVisitorCookieAge,
		})
	}
	writeWebsiteVisitorResult(c, http.StatusOK, result)
}

// sendWebsiteVisitorMessage 接收网站访客文本消息。
func (s *Service) sendWebsiteVisitorMessage(c *gin.Context) {
	s.prepareWebsiteVisitorResponse(c)
	channelID := c.Param("channelID")
	token, valid := readWebsiteVisitorToken(c, channelID)
	if !valid {
		writeApplicationError(c, invalidWebsiteVisitorTokenError(c))
		return
	}
	var input appservice.WebsiteVisitorTextMessageInput
	if !bindWebsiteVisitorJSON(c, &input) {
		return
	}
	result, err := s.websiteVisitor.SendTextMessage(c.Request.Context(), websiteVisitorMeta(c), channelID, websiteVisitorExternalID(token), input)
	if writeApplicationError(c, err) {
		return
	}
	writeWebsiteVisitorResult(c, http.StatusOK, result)
}

// listWebsiteVisitorMessages 返回网站访客指定线程的消息历史。
func (s *Service) listWebsiteVisitorMessages(c *gin.Context) {
	s.prepareWebsiteVisitorResponse(c)
	channelID := c.Param("channelID")
	token, valid := readWebsiteVisitorToken(c, channelID)
	if !valid {
		writeApplicationError(c, invalidWebsiteVisitorTokenError(c))
		return
	}
	result, err := s.websiteVisitor.ListMessages(c.Request.Context(), websiteVisitorMeta(c), channelID, websiteVisitorExternalID(token), c.Param("conversationID"), appservice.WebsiteVisitorMessageHistoryInput{
		Before: c.Query("before"), After: c.Query("after"),
	})
	if writeApplicationError(c, err) {
		return
	}
	writeWebsiteVisitorResult(c, http.StatusOK, result)
}

// bindWebsiteVisitorJSON 限制并严格解析公开 JSON 请求体。
func bindWebsiteVisitorJSON(c *gin.Context, output any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, websiteVisitorBodyLimit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeApplicationError(c, appservice.InvalidError(websiteVisitorRequestMeta(c), cervii18n.ErrorValidationFailed, nil).WithStatus(http.StatusRequestEntityTooLarge))
			return false
		}
		writeApplicationError(c, appservice.InvalidError(websiteVisitorRequestMeta(c), cervii18n.ErrorValidationFailed, nil))
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeApplicationError(c, appservice.InvalidError(websiteVisitorRequestMeta(c), cervii18n.ErrorValidationFailed, nil))
		return false
	}
	return true
}

// readWebsiteVisitorToken 按 Header、Cookie 顺序读取访客 Token。
func readWebsiteVisitorToken(c *gin.Context, channelID string) (string, bool) {
	if value := strings.TrimSpace(c.GetHeader(websiteVisitorHeader)); value != "" {
		return value, validWebsiteVisitorToken(value)
	}
	cookie, err := c.Request.Cookie(websiteVisitorCookieName(channelID))
	if err != nil {
		return "", false
	}
	value := strings.TrimSpace(cookie.Value)
	return value, validWebsiteVisitorToken(value)
}

// validWebsiteVisitorToken 校验访客 Token 格式。
func validWebsiteVisitorToken(value string) bool {
	if len(value) != websiteVisitorTokenSize {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'f') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

// generateWebsiteVisitorToken 生成 32 位小写十六进制访客 Token。
func generateWebsiteVisitorToken() (string, error) {
	value := make([]byte, websiteVisitorTokenSize/2)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

// websiteVisitorCookieName 返回渠道级访客 Cookie 名称。
func websiteVisitorCookieName(channelID string) string {
	return "cervi_visitor_" + channelID
}

// websiteVisitorExternalID 把裸 Token 规范化为渠道外部编号。
func websiteVisitorExternalID(token string) string { return "web-session:" + token }

// websiteVisitorMeta 构造不含成员认证的访客调用元信息。
func websiteVisitorMeta(c *gin.Context) appservice.WebsiteVisitorMeta {
	return appservice.WebsiteVisitorMeta{Locale: appservice.Locale(c.GetHeader("Accept-Language"))}
}

// websiteVisitorRequestMeta 构造仅用于本地化公开错误的应用元信息。
func websiteVisitorRequestMeta(c *gin.Context) appservice.RequestMeta {
	return appservice.RequestMeta{Locale: appservice.Locale(c.GetHeader("Accept-Language"))}
}

// invalidWebsiteVisitorTokenError 返回缺失或非法访客 Token 错误。
func invalidWebsiteVisitorTokenError(c *gin.Context) *appservice.Error {
	return appservice.InvalidError(websiteVisitorRequestMeta(c), cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"visitorToken": cervii18n.FieldVisitorTokenInvalid})
}

// prepareWebsiteVisitorResponse 禁止公开 Messenger 响应缓存。
func (s *Service) prepareWebsiteVisitorResponse(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
}

// writeWebsiteVisitorResult 写入公开 Messenger 成功响应和本地化语言。
func writeWebsiteVisitorResult(c *gin.Context, status int, value any) {
	_, language := cervii18n.Localize(c.GetHeader("Accept-Language"), cervii18n.ErrorInternal)
	c.Header("Content-Language", language)
	c.Header("Vary", "Accept-Language")
	c.JSON(status, value)
}

// websiteVisitorMethodNotAllowed 返回公开路由的显式方法错误。
func websiteVisitorMethodNotAllowed(allowed string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("Allow", allowed)
		writeApplicationError(c, appservice.FailedError(websiteVisitorRequestMeta(c), cervii18n.ErrorMethodNotAllowed).WithStatus(http.StatusMethodNotAllowed))
	}
}
