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
	websiteVisitorHeader      = appservice.WebsiteVisitorTokenHeader
	websiteVisitorTokenSize   = 32
	websiteVisitorBodyLimit   = 16 * 1024
	websiteVisitorCookieAge   = 365 * 24 * 60 * 60
	websiteVisitorExternalKey = "cervi_website_visitor_external_id"
	websiteVisitorTokenKey    = "cervi_website_visitor_token"
)

// registerWebsiteVisitorRoutes 注册匿名网站 Messenger 路由。
func (s *Service) registerWebsiteVisitorRoutes(router *gin.Engine) {
	if s.websiteVisitor == nil {
		return
	}
	const messengerPath = "/public/website-channels/:channelID/messenger"
	const messagesPath = "/public/website-channels/:channelID/messages"
	const directoryPath = "/public/website-channels/:channelID/conversations"
	const historyPath = "/public/website-channels/:channelID/conversations/:conversationID/messages"
	const typingPath = "/public/website-channels/:channelID/conversations/:conversationID/typing"
	const realtimePath = "/public/website-channels/:channelID/realtime"
	const attachmentsPath = "/public/website-channels/:channelID/attachments"
	const attachmentUploadPath = "/public/website-channels/:channelID/attachments/:fileID"
	const attachmentMessagesPath = "/public/website-channels/:channelID/attachment-messages"
	const messageAttachmentPath = "/public/website-channels/:channelID/conversations/:conversationID/messages/:messageID/attachment"
	router.GET(messengerPath, s.initializeWebsiteMessenger)
	router.POST(messagesPath, authorizeWebsiteVisitor, s.sendWebsiteVisitorMessage)
	router.GET(directoryPath, authorizeWebsiteVisitor, s.listWebsiteVisitorConversations)
	router.GET(historyPath, authorizeWebsiteVisitor, s.listWebsiteVisitorMessages)
	router.POST(attachmentsPath, authorizeWebsiteVisitor, s.createWebsiteVisitorAttachmentUpload)
	router.POST(attachmentUploadPath, authorizeWebsiteVisitor, s.completeWebsiteVisitorAttachmentUpload)
	router.POST(attachmentMessagesPath, authorizeWebsiteVisitor, s.sendWebsiteVisitorAttachmentMessage)
	router.GET(messageAttachmentPath, authorizeWebsiteVisitor, s.getWebsiteVisitorMessageAttachment)
	router.POST(typingPath, authorizeWebsiteVisitor, s.reportWebsiteVisitorTyping)
	router.Match([]string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, attachmentsPath, websiteVisitorMethodNotAllowed(http.MethodPost))
	router.Match([]string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, attachmentUploadPath, websiteVisitorMethodNotAllowed(http.MethodPost))
	router.Match([]string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, attachmentMessagesPath, websiteVisitorMethodNotAllowed(http.MethodPost))
	router.Match([]string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, messageAttachmentPath, websiteVisitorMethodNotAllowed(http.MethodGet))
	router.Match([]string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, messengerPath, websiteVisitorMethodNotAllowed(http.MethodGet))
	router.Match([]string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, messagesPath, websiteVisitorMethodNotAllowed(http.MethodPost))
	router.Match([]string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, directoryPath, websiteVisitorMethodNotAllowed(http.MethodGet))
	router.Match([]string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, historyPath, websiteVisitorMethodNotAllowed(http.MethodGet))
	router.Match([]string{http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, typingPath, websiteVisitorMethodNotAllowed(http.MethodPost))
	if s.visitorRealtime == nil {
		return
	}
	router.GET(realtimePath, authorizeWebsiteVisitor, s.serveWebsiteVisitorRealtime)
	router.Match([]string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions, http.MethodConnect, http.MethodTrace}, realtimePath, websiteVisitorMethodNotAllowed(http.MethodGet))
}

// serveWebsiteVisitorRealtime 输出网站访客实时事件流，事件流结束前由网关独占响应写入。
func (s *Service) serveWebsiteVisitorRealtime(c *gin.Context) {
	s.visitorRealtime.ServeVisitor(c.Writer, c.Request, c.Param("channelID"), c.GetString(websiteVisitorExternalKey))
}

// authorizeWebsiteVisitor 统一处理需要访客 Token 的公开路由：禁止缓存，按 Header、Cookie 读取 Token 并写入渠道外部编号，Token 缺失或格式非法按公开错误体拒绝。
func authorizeWebsiteVisitor(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	token, valid := readWebsiteVisitorToken(c, c.Param("channelID"))
	if !valid {
		writeApplicationError(c, invalidWebsiteVisitorTokenError(c))
		c.Abort()
		return
	}
	c.Set(websiteVisitorExternalKey, websiteVisitorExternalID(token))
	c.Set(websiteVisitorTokenKey, token)
}

// initializeWebsiteMessenger 签发或恢复访客 Token 并返回会话列表。
func (s *Service) initializeWebsiteMessenger(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
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
	var input appservice.WebsiteVisitorTextMessageInput
	if !bindWebsiteVisitorJSON(c, &input) {
		return
	}
	result, err := s.websiteVisitor.SendTextMessage(c.Request.Context(), websiteVisitorMeta(c), c.Param("channelID"), c.GetString(websiteVisitorExternalKey), input)
	if writeApplicationError(c, err) {
		return
	}
	writeWebsiteVisitorResult(c, http.StatusOK, result)
}

// createWebsiteVisitorAttachmentUpload 创建网站访客附件的上传请求。
func (s *Service) createWebsiteVisitorAttachmentUpload(c *gin.Context) {
	var input appservice.WebsiteVisitorUploadInput
	if !bindWebsiteVisitorJSON(c, &input) {
		return
	}
	result, err := s.websiteVisitor.CreateAttachmentUpload(c.Request.Context(), websiteVisitorMeta(c), c.Param("channelID"), c.GetString(websiteVisitorExternalKey), input)
	if writeApplicationError(c, err) {
		return
	}
	writeWebsiteVisitorResult(c, http.StatusOK, result)
}

// reportWebsiteVisitorTyping 向企业客服发布网站访客的输入状态。
func (s *Service) reportWebsiteVisitorTyping(c *gin.Context) {
	var input appservice.WebsiteVisitorTypingInput
	if !bindWebsiteVisitorJSON(c, &input) {
		return
	}
	err := s.websiteVisitor.ReportTyping(c.Request.Context(), websiteVisitorMeta(c), c.Param("channelID"), c.GetString(websiteVisitorExternalKey), c.Param("conversationID"), input)
	if writeApplicationError(c, err) {
		return
	}
	writeWebsiteVisitorResult(c, http.StatusOK, struct{}{})
}

// completeWebsiteVisitorAttachmentUpload 核验网站访客上传的附件内容。
func (s *Service) completeWebsiteVisitorAttachmentUpload(c *gin.Context) {
	err := s.websiteVisitor.CompleteAttachmentUpload(c.Request.Context(), websiteVisitorMeta(c), c.Param("channelID"), c.GetString(websiteVisitorExternalKey), c.Param("fileID"))
	if writeApplicationError(c, err) {
		return
	}
	writeWebsiteVisitorResult(c, http.StatusOK, struct{}{})
}

// sendWebsiteVisitorAttachmentMessage 接收网站访客附件消息。
func (s *Service) sendWebsiteVisitorAttachmentMessage(c *gin.Context) {
	var input appservice.WebsiteVisitorAttachmentMessageInput
	if !bindWebsiteVisitorJSON(c, &input) {
		return
	}
	result, err := s.websiteVisitor.SendAttachmentMessage(c.Request.Context(), websiteVisitorMeta(c), c.Param("channelID"), c.GetString(websiteVisitorExternalKey), input)
	if writeApplicationError(c, err) {
		return
	}
	writeWebsiteVisitorResult(c, http.StatusOK, result)
}

// getWebsiteVisitorMessageAttachment 重新签发网站访客消息附件的预览与下载地址。
func (s *Service) getWebsiteVisitorMessageAttachment(c *gin.Context) {
	result, err := s.websiteVisitor.GetMessageAttachment(c.Request.Context(), websiteVisitorMeta(c), c.Param("channelID"), c.GetString(websiteVisitorExternalKey), c.Param("conversationID"), c.Param("messageID"))
	if writeApplicationError(c, err) {
		return
	}
	writeWebsiteVisitorResult(c, http.StatusOK, result)
}

// listWebsiteVisitorConversations 返回网站访客当前渠道身份的线程目录。
func (s *Service) listWebsiteVisitorConversations(c *gin.Context) {
	result, err := s.websiteVisitor.ListConversations(c.Request.Context(), websiteVisitorMeta(c), c.Param("channelID"), c.GetString(websiteVisitorExternalKey))
	if writeApplicationError(c, err) {
		return
	}
	writeWebsiteVisitorResult(c, http.StatusOK, result)
}

// listWebsiteVisitorMessages 返回网站访客指定线程的消息历史。
func (s *Service) listWebsiteVisitorMessages(c *gin.Context) {
	result, err := s.websiteVisitor.ListMessages(c.Request.Context(), websiteVisitorMeta(c), c.Param("channelID"), c.GetString(websiteVisitorExternalKey), c.Param("conversationID"), appservice.WebsiteVisitorMessageHistoryInput{
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
	return appservice.WebsiteVisitorMeta{Locale: appservice.Locale(c.GetHeader("Accept-Language")), Token: c.GetString(websiteVisitorTokenKey)}
}

// websiteVisitorRequestMeta 构造公开错误本地化所需的应用元信息。
func websiteVisitorRequestMeta(c *gin.Context) appservice.RequestMeta {
	return appservice.RequestMeta{Locale: appservice.Locale(c.GetHeader("Accept-Language"))}
}

// invalidWebsiteVisitorTokenError 返回缺失或非法访客 Token 错误。
func invalidWebsiteVisitorTokenError(c *gin.Context) *appservice.Error {
	return appservice.InvalidError(websiteVisitorRequestMeta(c), cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"visitorToken": cervii18n.FieldVisitorTokenInvalid})
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
