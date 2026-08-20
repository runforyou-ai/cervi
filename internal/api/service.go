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
}

// Service 是企业服务端对外提供的 Gin HTTP 适配器。
type Service struct {
	application *appservice.Service
	router      *gin.Engine
}

// NewService 创建企业服务端 HTTP API。
func NewService(application *appservice.Service) *Service {
	service := &Service{application: application}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/installation/status", service.installationStatus)
	router.POST("/install", service.install)
	router.POST("/auth/login", service.login)
	router.POST("/auth/logout", service.logout)
	router.GET("/auth/identity", service.loadIdentity)
	router.PATCH("/profile", service.updateProfile)
	router.PATCH("/password", service.changePassword)
	router.GET("/inbox", service.loadInbox)
	router.GET("/channels/website", service.listWebsiteChannels)
	router.GET("/channels/website/trash", service.listDeletedWebsiteChannels)
	router.POST("/channels/website", service.createWebsiteChannel)
	router.GET("/channels/website/:channelID", service.getWebsiteChannel)
	router.PATCH("/channels/website/:channelID", service.updateWebsiteChannel)
	router.PATCH("/channels/website/:channelID/chat-interface", service.updateWebsiteChannelChatInterface)
	router.DELETE("/channels/website/:channelID", service.deleteWebsiteChannel)
	router.POST("/channels/website/:channelID/restore", service.restoreWebsiteChannel)
	router.GET("/channels", service.listChannels)
	router.GET("/users", service.listUsers)
	router.GET("/users/:userID", service.getUser)
	router.GET("/contacts", service.listContacts)
	router.GET("/contacts/trash", service.listDeletedContacts)
	router.POST("/contacts", service.createContact)
	router.GET("/contacts/:contactID", service.getContact)
	router.PATCH("/contacts/:contactID", service.updateContact)
	router.DELETE("/contacts/:contactID", service.deleteContact)
	router.POST("/contacts/:contactID/restore", service.restoreContact)
	router.GET("/settings/storage/s3", service.getS3Setting)
	router.PUT("/settings/storage/s3", service.saveS3Setting)
	router.POST("/settings/storage/s3/test", service.testS3Setting)

	service.router = router
	return service
}

// ServeHTTP 将 HTTP 请求交给 Gin 路由处理。
func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.router.ServeHTTP(writer, request)
}

func (s *Service) installationStatus(c *gin.Context) {
	status, err := s.application.InstallationStatus(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, status, err)
}

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

func (s *Service) login(c *gin.Context) {
	var input appservice.LoginInput
	if !bindJSON(c, &input) {
		return
	}
	auth, err := s.application.Login(c.Request.Context(), requestMeta(c), input)
	if writeApplicationError(c, err) {
		return
	}
	c.JSON(http.StatusOK, auth)
}

func (s *Service) logout(c *gin.Context) {
	err := s.application.Logout(c.Request.Context(), requestMeta(c))
	if writeApplicationError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (s *Service) loadIdentity(c *gin.Context) {
	identity, err := s.application.LoadIdentity(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, identity, err)
}

// updateProfile 保存当前用户的个人资料。
func (s *Service) updateProfile(c *gin.Context) {
	var input appservice.ProfileInput
	if !bindJSON(c, &input) {
		return
	}
	user, err := s.application.UpdateProfile(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusOK, user, err)
}

// changePassword 修改当前用户的登录密码。
func (s *Service) changePassword(c *gin.Context) {
	var input appservice.ChangePasswordInput
	if !bindJSON(c, &input) {
		return
	}
	writeEmpty(c, s.application.ChangePassword(c.Request.Context(), requestMeta(c), input))
}

func (s *Service) loadInbox(c *gin.Context) {
	inbox, err := s.application.LoadInbox(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, inbox, err)
}

func (s *Service) listWebsiteChannels(c *gin.Context) {
	s.writeWebsiteChannelList(c, false)
}

func (s *Service) listDeletedWebsiteChannels(c *gin.Context) {
	s.writeWebsiteChannelList(c, true)
}

func (s *Service) writeWebsiteChannelList(c *gin.Context, deleted bool) {
	list, err := s.application.ListWebsiteChannels(c.Request.Context(), requestMeta(c), deleted)
	writeResult(c, http.StatusOK, list, err)
}

func (s *Service) getWebsiteChannel(c *gin.Context) {
	channel, err := s.application.GetWebsiteChannel(c.Request.Context(), requestMeta(c), c.Param("channelID"))
	writeResult(c, http.StatusOK, channel, err)
}

func (s *Service) createWebsiteChannel(c *gin.Context) {
	var input appservice.WebsiteChannelInput
	if !bindJSON(c, &input) {
		return
	}
	channel, err := s.application.CreateWebsiteChannel(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusCreated, channel, err)
}

func (s *Service) updateWebsiteChannel(c *gin.Context) {
	var input appservice.WebsiteChannelInput
	if !bindJSON(c, &input) {
		return
	}
	channel, err := s.application.UpdateWebsiteChannel(c.Request.Context(), requestMeta(c), c.Param("channelID"), input)
	writeResult(c, http.StatusOK, channel, err)
}

func (s *Service) updateWebsiteChannelChatInterface(c *gin.Context) {
	var input appservice.WebsiteChannelChatInterfaceInput
	if !bindJSON(c, &input) {
		return
	}
	setting, err := s.application.UpdateWebsiteChannelChatInterface(c.Request.Context(), requestMeta(c), c.Param("channelID"), input)
	writeResult(c, http.StatusOK, setting, err)
}

func (s *Service) deleteWebsiteChannel(c *gin.Context) {
	err := s.application.DeleteWebsiteChannel(c.Request.Context(), requestMeta(c), c.Param("channelID"))
	writeEmpty(c, err)
}

func (s *Service) restoreWebsiteChannel(c *gin.Context) {
	channel, err := s.application.RestoreWebsiteChannel(c.Request.Context(), requestMeta(c), c.Param("channelID"))
	writeResult(c, http.StatusOK, channel, err)
}

func (s *Service) listChannels(c *gin.Context) {
	list, err := s.application.ListChannels(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, list, err)
}

func (s *Service) listUsers(c *gin.Context) {
	page, ok := positiveQueryInteger(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInteger(c, "pageSize", 50)
	if !ok {
		return
	}
	users, err := s.application.ListUsers(c.Request.Context(), requestMeta(c), appservice.UserListInput{
		Query: c.Query("query"), Status: optionalEnum[appservice.UserStatus](c.Query("status")), Role: optionalEnum[appservice.UserRole](c.Query("role")), Page: page, PageSize: pageSize,
	})
	writeResult(c, http.StatusOK, users, err)
}

func (s *Service) getUser(c *gin.Context) {
	user, err := s.application.GetUser(c.Request.Context(), requestMeta(c), c.Param("userID"))
	writeResult(c, http.StatusOK, user, err)
}

func (s *Service) listContacts(c *gin.Context) {
	s.writeContactList(c, false)
}

func (s *Service) listDeletedContacts(c *gin.Context) {
	s.writeContactList(c, true)
}

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

func (s *Service) getContact(c *gin.Context) {
	contact, err := s.application.GetContact(c.Request.Context(), requestMeta(c), c.Param("contactID"))
	writeResult(c, http.StatusOK, contact, err)
}

func (s *Service) createContact(c *gin.Context) {
	var input appservice.ContactInput
	if !bindJSON(c, &input) {
		return
	}
	contact, err := s.application.CreateContact(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusCreated, contact, err)
}

func (s *Service) updateContact(c *gin.Context) {
	var input appservice.ContactInput
	if !bindJSON(c, &input) {
		return
	}
	contact, err := s.application.UpdateContact(c.Request.Context(), requestMeta(c), c.Param("contactID"), input)
	writeResult(c, http.StatusOK, contact, err)
}

func (s *Service) deleteContact(c *gin.Context) {
	err := s.application.DeleteContact(c.Request.Context(), requestMeta(c), c.Param("contactID"))
	writeEmpty(c, err)
}

func (s *Service) restoreContact(c *gin.Context) {
	contact, err := s.application.RestoreContact(c.Request.Context(), requestMeta(c), c.Param("contactID"))
	writeResult(c, http.StatusOK, contact, err)
}

func (s *Service) getS3Setting(c *gin.Context) {
	setting, err := s.application.GetS3Setting(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, setting, err)
}

func (s *Service) saveS3Setting(c *gin.Context) {
	var input appservice.S3Setting
	if !bindJSON(c, &input) {
		return
	}
	setting, err := s.application.SaveS3Setting(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusOK, setting, err)
}

func (s *Service) testS3Setting(c *gin.Context) {
	var input appservice.S3Setting
	if !bindJSON(c, &input) {
		return
	}
	err := s.application.TestS3Setting(c.Request.Context(), requestMeta(c), input)
	writeEmpty(c, err)
}

func optionalEnum[T ~string](value string) *T {
	if value == "" {
		return nil
	}
	typed := T(value)
	return &typed
}

func requestMeta(c *gin.Context) appservice.RequestMeta {
	return appservice.RequestMeta{Token: bearerToken(c.GetHeader("Authorization")), Locale: appservice.Locale(c.GetHeader("Accept-Language"))}
}

func bearerToken(authorization string) string {
	scheme, token, found := strings.Cut(strings.TrimSpace(authorization), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

func bindJSON(c *gin.Context, output any) bool {
	if err := c.ShouldBindJSON(output); err != nil {
		writeApplicationError(c, appservice.InvalidError(requestMeta(c), cervii18n.ErrorValidationFailed, nil))
		return false
	}
	return true
}

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

func writeResult(c *gin.Context, status int, result any, err error) {
	if writeApplicationError(c, err) {
		return
	}
	c.JSON(status, result)
}

func writeEmpty(c *gin.Context, err error) {
	if writeApplicationError(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

func writeApplicationError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if c.Request.Context().Err() != nil {
		return true
	}
	var applicationError *appservice.Error
	if errors.As(err, &applicationError) {
		writeErrorBody(c, applicationError)
		return true
	}
	slog.Warn("应用服务调用失败", "error", err)
	writeErrorBody(c, appservice.FailedError(requestMeta(c), cervii18n.ErrorInternal))
	return true
}

func writeErrorBody(c *gin.Context, applicationError *appservice.Error) {
	_, language := cervii18n.Localize(c.GetHeader("Accept-Language"), cervii18n.ErrorInternal)
	c.Header("Content-Language", language)
	c.Header("Vary", "Accept-Language")
	c.JSON(applicationError.HTTPStatus(), errorBody{Error: apiError{
		Kind: applicationError.Kind, State: applicationError.State, Message: applicationError.Message, Fields: applicationError.Fields,
	}})
}
