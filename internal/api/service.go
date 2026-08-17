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
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
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

var channelFieldMessageKeys = map[channelaction.ValidationCode]cervii18n.Key{
	channelaction.ValidationNameRequired:         cervii18n.FieldChannelNameRequired,
	channelaction.ValidationNameTooLong:          cervii18n.FieldChannelNameTooLong,
	channelaction.ValidationDescriptionTooLong:   cervii18n.FieldChannelDescriptionTooLong,
	channelaction.ValidationDefaultLocaleInvalid: cervii18n.FieldChannelDefaultLocaleInvalid,
}

var s3SettingFieldMessageKeys = map[settingaction.ValidationCode]cervii18n.Key{
	settingaction.ValidationEndpointRequired:        cervii18n.FieldS3EndpointRequired,
	settingaction.ValidationEndpointInvalid:         cervii18n.FieldS3EndpointInvalid,
	settingaction.ValidationProviderInvalid:         cervii18n.FieldS3ProviderInvalid,
	settingaction.ValidationRegionRequired:          cervii18n.FieldS3RegionRequired,
	settingaction.ValidationBucketRequired:          cervii18n.FieldS3BucketRequired,
	settingaction.ValidationAccessKeyIDRequired:     cervii18n.FieldS3AccessKeyIDRequired,
	settingaction.ValidationSecretAccessKeyRequired: cervii18n.FieldS3SecretAccessKeyRequired,
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

type websiteChannelRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultLocale string `json:"defaultLocale"`
}

type s3SettingRequest struct {
	Enabled         bool   `json:"enabled"`
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	ForcePathStyle  bool   `json:"forcePathStyle"`
}

// Dependencies 定义 HTTP API 使用的 Action 和 Query。
type Dependencies struct {
	InstallWorkspace      func(context.Context, installationaction.InstallWorkspaceInput) (installationaction.InstallWorkspaceOutput, error)
	Login                 func(context.Context, authaction.LoginInput) (authaction.LoginOutput, error)
	Logout                func(context.Context, string) error
	ResolveSession        func(context.Context, string) (*servermodels.Principal, error)
	Installation          func(context.Context) (bool, error)
	LoadInbox             func(context.Context, *servermodels.Principal) inboxaction.LoadInboxOutput
	ListWebsiteChannels   func(context.Context, *servermodels.Principal, bool) ([]servermodels.Channel, error)
	GetWebsiteChannel     func(context.Context, *servermodels.Principal, string) (*servermodels.Channel, error)
	CreateWebsiteChannel  func(context.Context, *servermodels.Principal, channelaction.WebsiteChannelInput) (*servermodels.Channel, error)
	UpdateWebsiteChannel  func(context.Context, *servermodels.Principal, string, channelaction.WebsiteChannelInput) (*servermodels.Channel, error)
	DeleteWebsiteChannel  func(context.Context, *servermodels.Principal, string) error
	RestoreWebsiteChannel func(context.Context, *servermodels.Principal, string) (*servermodels.Channel, error)
	GetS3Setting          func(context.Context, *servermodels.Principal) (settingaction.S3Setting, error)
	SaveS3Setting         func(context.Context, *servermodels.Principal, settingaction.S3Setting) (settingaction.S3Setting, error)
	TestS3Setting         func(context.Context, settingaction.S3Setting) error
}

// Service 是挂载到 Wails /api 路径的 Gin 服务。
type Service struct {
	router                      *gin.Engine
	installWorkspace            func(context.Context, installationaction.InstallWorkspaceInput) (installationaction.InstallWorkspaceOutput, error)
	loginAction                 func(context.Context, authaction.LoginInput) (authaction.LoginOutput, error)
	logoutAction                func(context.Context, string) error
	sessionQuery                func(context.Context, string) (*servermodels.Principal, error)
	installation                func(context.Context) (bool, error)
	loadInbox                   func(context.Context, *servermodels.Principal) inboxaction.LoadInboxOutput
	listWebsiteChannelsQuery    func(context.Context, *servermodels.Principal, bool) ([]servermodels.Channel, error)
	getWebsiteChannelQuery      func(context.Context, *servermodels.Principal, string) (*servermodels.Channel, error)
	createWebsiteChannelAction  func(context.Context, *servermodels.Principal, channelaction.WebsiteChannelInput) (*servermodels.Channel, error)
	updateWebsiteChannelAction  func(context.Context, *servermodels.Principal, string, channelaction.WebsiteChannelInput) (*servermodels.Channel, error)
	deleteWebsiteChannelAction  func(context.Context, *servermodels.Principal, string) error
	restoreWebsiteChannelAction func(context.Context, *servermodels.Principal, string) (*servermodels.Channel, error)
	getS3SettingQuery           func(context.Context, *servermodels.Principal) (settingaction.S3Setting, error)
	saveS3SettingAction         func(context.Context, *servermodels.Principal, settingaction.S3Setting) (settingaction.S3Setting, error)
	testS3SettingAction         func(context.Context, settingaction.S3Setting) error
}

// NewService 创建并配置企业服务端 HTTP API。
func NewService(dependencies Dependencies) *Service {
	service := &Service{
		installWorkspace:            dependencies.InstallWorkspace,
		loginAction:                 dependencies.Login,
		logoutAction:                dependencies.Logout,
		sessionQuery:                dependencies.ResolveSession,
		installation:                dependencies.Installation,
		loadInbox:                   dependencies.LoadInbox,
		listWebsiteChannelsQuery:    dependencies.ListWebsiteChannels,
		getWebsiteChannelQuery:      dependencies.GetWebsiteChannel,
		createWebsiteChannelAction:  dependencies.CreateWebsiteChannel,
		updateWebsiteChannelAction:  dependencies.UpdateWebsiteChannel,
		deleteWebsiteChannelAction:  dependencies.DeleteWebsiteChannel,
		restoreWebsiteChannelAction: dependencies.RestoreWebsiteChannel,
		getS3SettingQuery:           dependencies.GetS3Setting,
		saveS3SettingAction:         dependencies.SaveS3Setting,
		testS3SettingAction:         dependencies.TestS3Setting,
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
	protected.GET("/auth/session", service.session)
	protected.GET("/inbox", service.inbox)
	protected.GET("/channels/website", service.listWebsiteChannels)
	protected.GET("/channels/website/trash", service.listDeletedWebsiteChannels)
	protected.POST("/channels/website", service.createWebsiteChannel)
	protected.GET("/channels/website/:channelID", service.getWebsiteChannel)
	protected.PATCH("/channels/website/:channelID", service.updateWebsiteChannel)
	protected.DELETE("/channels/website/:channelID", service.deleteWebsiteChannel)
	protected.POST("/channels/website/:channelID/restore", service.restoreWebsiteChannel)
	protected.GET("/settings/storage/s3", service.getS3Setting)
	protected.PUT("/settings/storage/s3", service.saveS3Setting)
	protected.POST("/settings/storage/s3/test", service.testS3Setting)

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

// session 返回当前登录用户和企业信息。
func (s *Service) session(c *gin.Context) {
	c.JSON(http.StatusOK, c.MustGet(principalKey).(*servermodels.Principal))
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

// listWebsiteChannels 返回当前企业未删除的网站渠道。
func (s *Service) listWebsiteChannels(c *gin.Context) {
	s.writeWebsiteChannelList(c, false)
}

// listDeletedWebsiteChannels 返回当前企业回收站中的网站渠道。
func (s *Service) listDeletedWebsiteChannels(c *gin.Context) {
	s.writeWebsiteChannelList(c, true)
}

// writeWebsiteChannelList 按软删除状态返回网站渠道列表。
func (s *Service) writeWebsiteChannelList(c *gin.Context, deleted bool) {
	channels, err := s.listWebsiteChannelsQuery(c.Request.Context(), currentPrincipal(c), deleted)
	if err != nil {
		slog.Warn("读取网站渠道列表失败", "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorChannelListFailed, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"channels": channels})
}

// getWebsiteChannel 返回当前企业的网站渠道详情。
func (s *Service) getWebsiteChannel(c *gin.Context) {
	channel, err := s.getWebsiteChannelQuery(c.Request.Context(), currentPrincipal(c), c.Param("channelID"))
	if s.writeWebsiteChannelError(c, err, cervii18n.ErrorChannelReadFailed) {
		return
	}
	c.JSON(http.StatusOK, channel)
}

// createWebsiteChannel 创建当前企业的网站渠道。
func (s *Service) createWebsiteChannel(c *gin.Context) {
	request, ok := bindWebsiteChannelRequest(c)
	if !ok {
		return
	}
	channel, err := s.createWebsiteChannelAction(c.Request.Context(), currentPrincipal(c), request.websiteChannelInput())
	if s.writeWebsiteChannelMutationError(c, err, cervii18n.ErrorChannelCreateFailed) {
		return
	}
	c.JSON(http.StatusCreated, channel)
}

// updateWebsiteChannel 修改当前企业的网站渠道。
func (s *Service) updateWebsiteChannel(c *gin.Context) {
	request, ok := bindWebsiteChannelRequest(c)
	if !ok {
		return
	}
	channel, err := s.updateWebsiteChannelAction(c.Request.Context(), currentPrincipal(c), c.Param("channelID"), request.websiteChannelInput())
	if s.writeWebsiteChannelMutationError(c, err, cervii18n.ErrorChannelUpdateFailed) {
		return
	}
	c.JSON(http.StatusOK, channel)
}

// deleteWebsiteChannel 将当前企业的网站渠道移入回收站。
func (s *Service) deleteWebsiteChannel(c *gin.Context) {
	err := s.deleteWebsiteChannelAction(c.Request.Context(), currentPrincipal(c), c.Param("channelID"))
	if s.writeWebsiteChannelError(c, err, cervii18n.ErrorChannelDeleteFailed) {
		return
	}
	c.Status(http.StatusNoContent)
}

// restoreWebsiteChannel 恢复当前企业回收站中的网站渠道。
func (s *Service) restoreWebsiteChannel(c *gin.Context) {
	channel, err := s.restoreWebsiteChannelAction(c.Request.Context(), currentPrincipal(c), c.Param("channelID"))
	if s.writeWebsiteChannelError(c, err, cervii18n.ErrorChannelRestoreFailed) {
		return
	}
	c.JSON(http.StatusOK, channel)
}

// getS3Setting 返回当前企业的 S3 对象存储配置。
func (s *Service) getS3Setting(c *gin.Context) {
	principal := currentPrincipal(c)
	setting, err := s.getS3SettingQuery(c.Request.Context(), principal)
	if err != nil {
		slog.Warn("读取 S3 配置失败", "organization_id", principal.Organization.ID, "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorS3SettingReadFailed, nil)
		return
	}
	c.JSON(http.StatusOK, setting)
}

// saveS3Setting 保存当前企业的 S3 对象存储配置。
func (s *Service) saveS3Setting(c *gin.Context) {
	request, ok := bindS3SettingRequest(c)
	if !ok {
		return
	}
	principal := currentPrincipal(c)
	input := request.s3SettingInput()
	setting, err := s.saveS3SettingAction(c.Request.Context(), principal, input)
	if err != nil {
		writeS3SettingError(c, err, cervii18n.ErrorS3SettingSaveFailed, "save", principal.Organization.ID, input)
		return
	}
	slog.Info("S3 配置已保存",
		"organization_id", principal.Organization.ID,
		"enabled", setting.Enabled,
		"provider", setting.Provider,
		"bucket", setting.Bucket,
	)
	c.JSON(http.StatusOK, setting)
}

// testS3Setting 测试请求中的 S3 对象存储配置。
func (s *Service) testS3Setting(c *gin.Context) {
	request, ok := bindS3SettingRequest(c)
	if !ok {
		return
	}
	principal := currentPrincipal(c)
	setting := request.s3SettingInput()
	err := s.testS3SettingAction(c.Request.Context(), setting)
	if err != nil {
		writeS3SettingError(c, err, cervii18n.ErrorS3ConnectionTestFailed, "test", principal.Organization.ID, setting)
		return
	}
	slog.Info("S3 连接测试成功",
		"organization_id", principal.Organization.ID,
		"provider", setting.Provider,
		"bucket", setting.Bucket,
	)
	c.Status(http.StatusNoContent)
}

// currentPrincipal 返回请求中已认证的企业用户。
func currentPrincipal(c *gin.Context) *servermodels.Principal {
	return c.MustGet(principalKey).(*servermodels.Principal)
}

// bindWebsiteChannelRequest 解析网站渠道表单请求。
func bindWebsiteChannelRequest(c *gin.Context) (websiteChannelRequest, bool) {
	var request websiteChannelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, nil)
		return websiteChannelRequest{}, false
	}
	return request, true
}

// websiteChannelInput 将 HTTP 请求转换为网站渠道输入。
func (r websiteChannelRequest) websiteChannelInput() channelaction.WebsiteChannelInput {
	return channelaction.WebsiteChannelInput{
		Name:          r.Name,
		Description:   r.Description,
		DefaultLocale: r.DefaultLocale,
	}
}

// bindS3SettingRequest 解析 S3 配置表单请求。
func bindS3SettingRequest(c *gin.Context) (s3SettingRequest, bool) {
	var request s3SettingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, nil)
		return s3SettingRequest{}, false
	}
	return request, true
}

// s3SettingInput 将 HTTP 请求转换为 S3 配置输入。
func (r s3SettingRequest) s3SettingInput() settingaction.S3Setting {
	return settingaction.S3Setting{
		Enabled:         r.Enabled,
		Provider:        r.Provider,
		Endpoint:        r.Endpoint,
		Region:          r.Region,
		Bucket:          r.Bucket,
		AccessKeyID:     r.AccessKeyID,
		SecretAccessKey: r.SecretAccessKey,
		ForcePathStyle:  r.ForcePathStyle,
	}
}

// writeS3SettingError 记录并返回 S3 配置操作错误。
func writeS3SettingError(c *gin.Context, err error, failureKey cervii18n.Key, operation string, organizationID string, setting settingaction.S3Setting) {
	var validationError *settingaction.ValidationError
	if errors.As(err, &validationError) {
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, s3SettingFieldKeys(validationError.Fields))
		return
	}
	if errors.Is(err, settingaction.ErrPrincipalInvalid) {
		writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", cervii18n.ErrorAuthenticationRequired, nil)
		return
	}
	if errors.Is(err, settingaction.ErrS3ConnectionFailed) {
		slog.Info("S3 连接测试失败",
			"organization_id", organizationID,
			"provider", setting.Provider,
			"bucket", setting.Bucket,
			"error", err,
		)
		writeError(c, http.StatusUnprocessableEntity, "S3_CONNECTION_FAILED", cervii18n.ErrorS3ConnectionTestFailed, nil)
		return
	}
	slog.Warn("S3 配置操作失败",
		"operation", operation,
		"organization_id", organizationID,
		"provider", setting.Provider,
		"bucket", setting.Bucket,
		"error", err,
	)
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", failureKey, nil)
}

// writeWebsiteChannelMutationError 返回网站渠道写入错误。
func (s *Service) writeWebsiteChannelMutationError(c *gin.Context, err error, failureKey cervii18n.Key) bool {
	var validationError *channelaction.ValidationError
	if errors.As(err, &validationError) {
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, channelFieldKeys(validationError.Fields))
		return true
	}
	return s.writeWebsiteChannelError(c, err, failureKey)
}

// writeWebsiteChannelError 返回网站渠道通用操作错误。
func (s *Service) writeWebsiteChannelError(c *gin.Context, err error, failureKey cervii18n.Key) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, channelaction.ErrPrincipalInvalid) {
		writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", cervii18n.ErrorAuthenticationRequired, nil)
		return true
	}
	if errors.Is(err, channelaction.ErrNotFound) {
		writeError(c, http.StatusNotFound, "CHANNEL_NOT_FOUND", cervii18n.ErrorChannelNotFound, nil)
		return true
	}
	slog.Warn("网站渠道操作失败", "error", err)
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", failureKey, nil)
	return true
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

// channelFieldKeys 将网站渠道校验码转换为本地化文案键。
func channelFieldKeys(fields map[string]channelaction.ValidationCode) map[string]cervii18n.Key {
	keys := make(map[string]cervii18n.Key, len(fields))
	for field, code := range fields {
		keys[field] = channelFieldMessageKeys[code]
	}
	return keys
}

// s3SettingFieldKeys 将 S3 配置校验码转换为本地化文案键。
func s3SettingFieldKeys(fields map[string]settingaction.ValidationCode) map[string]cervii18n.Key {
	keys := make(map[string]cervii18n.Key, len(fields))
	for field, code := range fields {
		keys[field] = s3SettingFieldMessageKeys[code]
	}
	return keys
}
