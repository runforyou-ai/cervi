//go:build server

// Package api 提供企业服务端的 HTTP API。
package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

const (
	sessionCookieName = "cervi_session"
	principalKey      = "cervi_principal"
	sessionTokenKey   = "cervi_session_token"
)

var installationFieldMessageKeys = map[installationaction.ValidationCode]cervii18n.Key{
	installationaction.ValidationOrganizationNameRequired: cervii18n.FieldOrganizationNameRequired,
	installationaction.ValidationDisplayNameRequired:      cervii18n.FieldDisplayNameRequired,
	installationaction.ValidationEmailInvalid:             cervii18n.FieldEmailInvalid,
	installationaction.ValidationPasswordTooShort:         cervii18n.FieldPasswordTooShort,
	installationaction.ValidationPasswordTooLong:          cervii18n.FieldPasswordTooLong,
}

type errorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type installRequest struct {
	OrganizationName string `json:"organizationName"`
	DisplayName      string `json:"displayName"`
	Email            string `json:"email"`
	Password         string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Dependencies 定义 HTTP API 使用的 Action 和 Query。
type Dependencies struct {
	InstallWorkspace func(context.Context, installationaction.InstallWorkspaceInput) (installationaction.InstallWorkspaceOutput, error)
	Login            func(context.Context, authaction.LoginInput) (authaction.LoginOutput, error)
	Logout           func(context.Context, string) error
	ResolveSession   func(context.Context, string) (*servermodels.Principal, error)
	Installation     func(context.Context) (bool, error)
	LoadInbox        func(context.Context, *servermodels.Principal) inboxaction.LoadInboxOutput
}

// Service 是挂载到 Wails /api 路径的 Gin 服务。
type Service struct {
	router           *gin.Engine
	installWorkspace func(context.Context, installationaction.InstallWorkspaceInput) (installationaction.InstallWorkspaceOutput, error)
	loginAction      func(context.Context, authaction.LoginInput) (authaction.LoginOutput, error)
	logoutAction     func(context.Context, string) error
	sessionQuery     func(context.Context, string) (*servermodels.Principal, error)
	installation     func(context.Context) (bool, error)
	loadInbox        func(context.Context, *servermodels.Principal) inboxaction.LoadInboxOutput
}

// NewService 创建并配置企业服务端 HTTP API。
func NewService(dependencies Dependencies) *Service {
	service := &Service{
		installWorkspace: dependencies.InstallWorkspace,
		loginAction:      dependencies.Login,
		logoutAction:     dependencies.Logout,
		sessionQuery:     dependencies.ResolveSession,
		installation:     dependencies.Installation,
		loadInbox:        dependencies.LoadInbox,
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	router.POST("/install", service.requireUninitialized(), service.install)
	router.POST("/auth/login", service.requireInitialized(), service.login)

	protected := router.Group("")
	protected.Use(
		service.requireInitialized(),
		service.resolveSession(),
		service.requireAuthenticated(),
	)
	protected.POST("/auth/logout", service.logout)
	protected.GET("/inbox", service.inbox)

	service.router = router
	return service
}

// ServeHTTP 将 HTTP 请求交给 Gin 路由处理。
func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.router.ServeHTTP(writer, request)
}

// requireUninitialized 仅允许未初始化实例继续处理请求。
func (s *Service) requireUninitialized() gin.HandlerFunc {
	return func(c *gin.Context) {
		installed, err := s.installation(c.Request.Context())
		if err != nil {
			slog.Warn("读取安装状态失败", "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorInstallationStatusReadFailed, nil)
			return
		}
		if installed {
			writeError(c, http.StatusConflict, "ALREADY_INITIALIZED", cervii18n.ErrorAlreadyInitialized, nil)
			return
		}
		c.Next()
	}
}

// requireInitialized 仅允许已初始化实例继续处理请求。
func (s *Service) requireInitialized() gin.HandlerFunc {
	return func(c *gin.Context) {
		installed, err := s.installation(c.Request.Context())
		if err != nil {
			slog.Warn("读取安装状态失败", "error", err)
			writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorInstallationStatusReadFailed, nil)
			return
		}
		if !installed {
			writeError(c, http.StatusConflict, "INSTALLATION_REQUIRED", cervii18n.ErrorInstallationRequired, nil)
			return
		}
		c.Next()
	}
}

// resolveSession 从会话 Cookie 解析当前用户身份。
func (s *Service) resolveSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(sessionCookieName)
		if err == nil && token != "" {
			principal, lookupErr := s.sessionQuery(c.Request.Context(), token)
			if lookupErr == nil && principal != nil {
				c.Set(principalKey, principal)
				c.Set(sessionTokenKey, token)
			} else if lookupErr != nil {
				slog.Warn("读取登录会话失败", "error", lookupErr)
				writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorAuthenticationStatusFailed, nil)
				return
			}
		}
		c.Next()
	}
}

// requireAuthenticated 仅允许已登录用户继续处理请求。
func (s *Service) requireAuthenticated() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get(principalKey); !exists {
			writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", cervii18n.ErrorAuthenticationRequired, nil)
			return
		}
		c.Next()
	}
}

// install 创建企业所有者并建立初始登录会话。
func (s *Service) install(c *gin.Context) {
	var request installRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, nil)
		return
	}

	output, err := s.installWorkspace(c.Request.Context(), installationaction.InstallWorkspaceInput{
		OrganizationName: request.OrganizationName,
		DisplayName:      request.DisplayName,
		Email:            request.Email,
		Password:         request.Password,
	})
	var validationError *installationaction.ValidationError
	if errors.As(err, &validationError) {
		writeError(
			c,
			http.StatusBadRequest,
			"VALIDATION_FAILED",
			cervii18n.ErrorValidationFailed,
			installationFieldKeys(validationError.Fields),
		)
		return
	}
	if errors.Is(err, installationaction.ErrAlreadyInstalled) {
		writeError(c, http.StatusConflict, "ALREADY_INITIALIZED", cervii18n.ErrorAlreadyInitialized, nil)
		return
	}
	if err != nil {
		slog.Warn("初始化企业失败", "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorInstallationFailed, nil)
		return
	}

	setSessionCookie(c, output.Token, output.ExpiresAt)
	slog.Info("企业初始化完成",
		"organization_id", output.Principal.Organization.ID,
		"owner_id", output.Principal.User.ID,
	)
	c.JSON(http.StatusCreated, output.Principal)
}

// login 校验账号密码并创建登录会话。
func (s *Service) login(c *gin.Context) {
	var request loginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorAuthenticationInputInvalid, nil)
		return
	}

	output, err := s.loginAction(c.Request.Context(), authaction.LoginInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if errors.Is(err, authaction.ErrInvalidCredentials) {
		writeError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", cervii18n.ErrorInvalidCredentials, nil)
		return
	}
	if err != nil {
		slog.Warn("用户登录失败", "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorLoginFailed, nil)
		return
	}

	setSessionCookie(c, output.Token, output.ExpiresAt)
	slog.Info("用户登录成功", "user_id", output.Principal.User.ID)
	c.JSON(http.StatusOK, output.Principal)
}

// logout 删除当前登录会话并清除 Cookie。
func (s *Service) logout(c *gin.Context) {
	principal := c.MustGet(principalKey).(*servermodels.Principal)
	token := c.MustGet(sessionTokenKey).(string)
	if err := s.logoutAction(c.Request.Context(), token); err != nil {
		slog.Warn("删除登录会话失败", "user_id", principal.User.ID, "error", err)
		writeError(c, http.StatusInternalServerError, "LOGOUT_FAILED", cervii18n.ErrorLogoutFailed, nil)
		return
	}
	clearSessionCookie(c)
	slog.Info("用户退出登录", "user_id", principal.User.ID)
	c.Status(http.StatusNoContent)
}

// inbox 返回当前用户可访问的统一收件箱。
func (s *Service) inbox(c *gin.Context) {
	principal := c.MustGet(principalKey).(*servermodels.Principal)
	output := s.loadInbox(c.Request.Context(), principal)
	c.JSON(http.StatusOK, gin.H{
		"organization":  output.Organization,
		"user":          output.User,
		"conversations": output.Conversations,
	})
}

// setSessionCookie 写入安全的登录会话 Cookie。
func setSessionCookie(c *gin.Context, token string, expiresAt time.Time) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		sessionCookieName,
		token,
		int(time.Until(expiresAt).Seconds()),
		"/",
		"",
		requestIsSecure(c.Request),
		true,
	)
}

// clearSessionCookie 清除当前登录会话 Cookie。
func clearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, "/", "", requestIsSecure(c.Request), true)
}

// requestIsSecure 判断请求是否通过 HTTPS 到达服务端。
func requestIsSecure(request *http.Request) bool {
	return request.TLS != nil || strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https")
}

// writeError 返回统一格式的 API 错误。
func writeError(c *gin.Context, status int, code string, messageKey cervii18n.Key, fields map[string]cervii18n.Key) {
	acceptLanguage := c.GetHeader("Accept-Language")
	message, language := cervii18n.Localize(acceptLanguage, messageKey)
	c.Header("Content-Language", language)
	c.Header("Vary", "Accept-Language")
	c.AbortWithStatusJSON(status, errorBody{Error: apiError{
		Code:    code,
		Message: message,
		Fields:  cervii18n.LocalizeMap(acceptLanguage, fields),
	}})
}

// installationFieldKeys 将企业初始化校验码转换为本地化文案键。
func installationFieldKeys(fields map[string]installationaction.ValidationCode) map[string]cervii18n.Key {
	keys := make(map[string]cervii18n.Key, len(fields))
	for field, code := range fields {
		keys[field] = installationFieldMessageKeys[code]
	}
	return keys
}
