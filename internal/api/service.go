//go:build server

// Package api 将统一应用服务公开为企业服务端 HTTP API。
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Kind    appservice.ErrorKind    `json:"kind,omitempty"`
	State   appservice.SessionState `json:"state,omitempty"`
	Message string                  `json:"message"`
	Fields  map[string]string       `json:"fields,omitempty"`
	Reason  string                  `json:"reason,omitempty"`
}

// Service 是企业服务端对外提供的 Gin HTTP 适配器。
type Service struct {
	application         *appservice.Service
	websiteVisitor      *appservice.WebsiteVisitorService
	telegramWebhook     TelegramWebhookReceiver
	trustForwardedProto bool
	router              *gin.Engine
}

// ServiceOption 配置企业服务端 HTTP API 的独立能力。
type ServiceOption func(*Service)

// WithWebsiteVisitor 注入匿名网站访客应用服务。
func WithWebsiteVisitor(visitor *appservice.WebsiteVisitorService, trustForwardedProto bool) ServiceOption {
	return func(service *Service) {
		service.websiteVisitor = visitor
		service.trustForwardedProto = trustForwardedProto
	}
}

// WithTelegramWebhook 注入 Telegram 公开回调接收能力。
func WithTelegramWebhook(receiver TelegramWebhookReceiver) ServiceOption {
	return func(service *Service) {
		service.telegramWebhook = receiver
	}
}

// NewService 创建企业服务端 HTTP API。
func NewService(application *appservice.Service, options ...ServiceOption) *Service {
	service := &Service{application: application}
	for _, option := range options {
		option(service)
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	service.registerGeneratedRoutes(router)
	router.POST("/install", service.install)
	router.GET("/contacts", service.listContacts)
	router.GET("/contacts/trash", service.listDeletedContacts)
	service.registerWebsiteVisitorRoutes(router)
	service.registerTelegramWebhookRoutes(router)

	service.router = router
	return service
}

// ServeHTTP 将 HTTP 请求交给 Gin 路由处理。
func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.router.ServeHTTP(writer, request)
}

// install 创建企业管理员并返回登录令牌。
func (s *Service) install(c *gin.Context) {
	var input appservice.InstallWorkspaceInput
	if !bindJSON(c, &input) {
		return
	}
	auth, err := s.application.InstallWorkspace(c.Request.Context(), requestMeta(c), input)
	if writeApplicationError(c, err) {
		return
	}
	c.JSON(http.StatusCreated, auth)
}

// listContacts 返回联系人列表。
func (s *Service) listContacts(c *gin.Context) {
	s.writeContactList(c, false)
}

// listDeletedContacts 返回回收站中的联系人列表。
func (s *Service) listDeletedContacts(c *gin.Context) {
	s.writeContactList(c, true)
}

// writeContactList 按回收站开关返回联系人列表。
func (s *Service) writeContactList(c *gin.Context, deleted bool) {
	page, ok := positiveQueryInteger(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInteger(c, "pageSize", 50)
	if !ok {
		return
	}
	contacts, err := s.application.ListContacts(c.Request.Context(), requestMeta(c), appservice.ContactListInput{
		Query: c.Query("query"), Stage: optionalEnum[appservice.ContactStage](c.Query("stage")), ChannelID: c.Query("channelId"), MethodType: optionalEnum[appservice.ContactMethodType](c.Query("methodType")),
		Sort: appservice.ContactSort(c.Query("sort")), Page: page, PageSize: pageSize, Deleted: deleted,
	})
	writeResult(c, http.StatusOK, contacts, err)
}

// optionalEnum 把非空查询值转换成对应枚举指针，空值返回 nil。
func optionalEnum[T ~string](value string) *T {
	if value == "" {
		return nil
	}
	typed := T(value)
	return &typed
}

// requestMeta 从请求头提取令牌和语言，构造应用服务请求元数据。
func requestMeta(c *gin.Context) appservice.RequestMeta {
	return appservice.RequestMeta{Token: bearerToken(c.GetHeader("Authorization")), Locale: appservice.Locale(c.GetHeader("Accept-Language"))}
}

// bearerToken 从 Authorization 头解析 Bearer 令牌，格式不符时返回空串。
func bearerToken(authorization string) string {
	scheme, token, found := strings.Cut(strings.TrimSpace(authorization), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// bindJSON 绑定 JSON 请求体，失败时写入校验错误响应并返回 false。
func bindJSON(c *gin.Context, output any) bool {
	if err := c.ShouldBindJSON(output); err != nil {
		writeApplicationError(c, appservice.InvalidError(requestMeta(c), cervii18n.ErrorValidationFailed, nil))
		return false
	}
	return true
}

// positiveQueryInteger 解析正整数查询参数，缺省时返回默认值，非法时写入校验错误响应。
func positiveQueryInteger(c *gin.Context, name string, defaultValue int) (int, bool) {
	value := c.Query(name)
	if value == "" {
		return defaultValue, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		writeApplicationError(c, appservice.InvalidError(requestMeta(c), cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{name: cervii18n.FieldQueryPositiveInteger}))
		return 0, false
	}
	return parsed, true
}

// writeResult 无错误时按状态码写入 JSON 结果，否则写入错误响应。
func writeResult(c *gin.Context, status int, result any, err error) {
	if writeApplicationError(c, err) {
		return
	}
	c.JSON(status, result)
}

// writeEmpty 无错误时返回 204 空响应，否则写入错误响应。
func writeEmpty(c *gin.Context, err error) {
	if writeApplicationError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// writeApplicationError 把应用服务错误写成结构化错误响应，返回是否已处理错误。
func writeApplicationError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if c.Request.Context().Err() != nil {
		return true
	}
	if applicationError, ok := errors.AsType[*appservice.Error](err); ok {
		writeErrorBody(c, applicationError)
		return true
	}
	slog.Warn("应用服务调用失败", "error", err)
	writeErrorBody(c, appservice.FailedError(requestMeta(c), cervii18n.ErrorInternal))
	return true
}

// writeErrorBody 按请求语言写入本地化的错误响应体。
func writeErrorBody(c *gin.Context, applicationError *appservice.Error) {
	_, language := cervii18n.Localize(c.GetHeader("Accept-Language"), cervii18n.ErrorInternal)
	c.Header("Content-Language", language)
	c.Header("Vary", "Accept-Language")
	c.JSON(applicationError.HTTPStatus(), errorBody{Error: apiError{
		Kind: applicationError.Kind, State: applicationError.State, Message: applicationError.Message,
		Fields: applicationError.Fields, Reason: applicationError.Reason,
	}})
}
