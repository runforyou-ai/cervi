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
	application         *appservice.Service
	websiteVisitor      *appservice.WebsiteVisitorService
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

// NewService 创建企业服务端 HTTP API。
func NewService(application *appservice.Service, options ...ServiceOption) *Service {
	service := &Service{application: application}
	for _, option := range options {
		option(service)
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/installation/status", service.installationStatus)
	router.POST("/install", service.install)
	router.POST("/auth/login", service.login)
	router.POST("/auth/logout", service.logout)
	router.GET("/auth/identity", service.loadIdentity)
	router.PATCH("/profile", service.updateProfile)
	router.POST("/files/uploads", service.createFileUpload)
	router.POST("/files/:fileID/complete", service.completeFileUpload)
	router.PATCH("/password", service.changePassword)
	router.PATCH("/preferences", service.updateUserPreferences)
	router.PATCH("/work-status", service.updateUserWorkStatus)
	router.GET("/inbox", service.loadInbox)
	router.GET("/channels", service.listMessageChannels)
	router.POST("/channels", service.createMessageChannel)
	router.GET("/channels/:channelID", service.getMessageChannel)
	router.PATCH("/channels/:channelID", service.updateMessageChannel)
	router.POST("/channels/:channelID/deactivate", service.deactivateMessageChannel)
	router.POST("/channels/:channelID/activate", service.activateMessageChannel)
	router.GET("/channels/website/:channelID", service.getWebsiteChannel)
	router.PATCH("/channels/website/:channelID/chat-interface", service.updateWebsiteChannelChatInterface)
	router.PATCH("/channels/website/:channelID/access", service.updateWebsiteChannelAccess)
	router.GET("/channel-options", service.listChannelOptions)
	router.GET("/members/options", service.listMemberOptions)
	router.GET("/agents/model-options", service.listAgentModelOptions)
	router.GET("/agents", service.listAgents)
	router.POST("/agents", service.createAgent)
	router.GET("/agents/:agentID", service.getAgent)
	router.PATCH("/agents/:agentID", service.updateAgent)
	router.PATCH("/agents/:agentID/execution", service.updateAgentExecution)
	router.PATCH("/agents/:agentID/work-status", service.updateAgentWorkStatus)
	router.POST("/agents/:agentID/deactivate", service.deactivateAgent)
	router.POST("/agents/:agentID/reactivate", service.reactivateAgent)
	router.GET("/users", service.listUsers)
	router.POST("/users", service.createUser)
	router.PATCH("/users/roles", service.updateUserRoles)
	router.GET("/users/:userID", service.getUser)
	router.PATCH("/users/:userID", service.updateUser)
	router.POST("/users/:userID/deactivate", service.deactivateUser)
	router.POST("/users/:userID/reactivate", service.reactivateUser)
	router.GET("/teams", service.listTeams)
	router.POST("/teams", service.createTeam)
	router.PATCH("/teams/:teamID", service.updateTeam)
	router.DELETE("/teams/:teamID", service.deleteTeam)
	router.GET("/knowledge-bases", service.listKnowledgeBases)
	router.POST("/knowledge-bases", service.createKnowledgeBase)
	router.GET("/knowledge-bases/:knowledgeBaseID", service.getKnowledgeBase)
	router.PATCH("/knowledge-bases/:knowledgeBaseID", service.updateKnowledgeBase)
	router.DELETE("/knowledge-bases/:knowledgeBaseID", service.deleteKnowledgeBase)
	router.POST("/knowledge-bases/:knowledgeBaseID/groups", service.createKnowledgeGroup)
	router.PATCH("/knowledge-bases/:knowledgeBaseID/groups/:groupID", service.updateKnowledgeGroup)
	router.DELETE("/knowledge-bases/:knowledgeBaseID/groups/:groupID", service.deleteKnowledgeGroup)
	router.GET("/teams/:teamID/members", service.listTeamMembers)
	router.GET("/teams/:teamID/member-candidates", service.listTeamMemberCandidates)
	router.POST("/teams/:teamID/members", service.addTeamMembers)
	router.DELETE("/teams/:teamID/members", service.removeTeamMembers)
	router.GET("/contacts", service.listContacts)
	router.GET("/contacts/trash", service.listDeletedContacts)
	router.POST("/contacts", service.createContact)
	router.GET("/contacts/:contactID", service.getContact)
	router.PATCH("/contacts/:contactID", service.updateContact)
	router.DELETE("/contacts/:contactID", service.deleteContact)
	router.POST("/contacts/:contactID/restore", service.restoreContact)
	router.GET("/settings/roles", service.listRoles)
	router.POST("/settings/roles", service.createRole)
	router.GET("/settings/roles/:roleID", service.getRole)
	router.PUT("/settings/roles/:roleID", service.updateRole)
	router.DELETE("/settings/roles/:roleID", service.deleteRole)
	router.GET("/integrations/model-services/models", service.listAvailableAIModels)
	router.GET("/integrations/model-services", service.listAIProviders)
	router.POST("/integrations/model-services", service.createAIProvider)
	router.GET("/integrations/model-services/:providerID", service.getAIProvider)
	router.PUT("/integrations/model-services/:providerID", service.updateAIProvider)
	router.DELETE("/integrations/model-services/:providerID", service.deleteAIProvider)
	router.PUT("/settings/organization", service.updateOrganization)
	router.GET("/settings/storage/s3", service.getS3Setting)
	router.PUT("/settings/storage/s3", service.saveS3Setting)
	router.POST("/settings/storage/s3/test", service.testS3Setting)
	service.registerWebsiteVisitorRoutes(router)

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

// createFileUpload 创建文件上传请求。
func (s *Service) createFileUpload(c *gin.Context) {
	var input appservice.FileUploadInput
	if !bindJSON(c, &input) {
		return
	}
	upload, err := s.application.CreateFileUpload(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusCreated, upload, err)
}

// completeFileUpload 核验并完成文件上传。
func (s *Service) completeFileUpload(c *gin.Context) {
	file, err := s.application.CompleteFileUpload(c.Request.Context(), requestMeta(c), c.Param("fileID"))
	writeResult(c, http.StatusOK, file, err)
}

// changePassword 修改当前用户的登录密码。
func (s *Service) changePassword(c *gin.Context) {
	var input appservice.ChangePasswordInput
	if !bindJSON(c, &input) {
		return
	}
	writeEmpty(c, s.application.ChangePassword(c.Request.Context(), requestMeta(c), input))
}

// updateUserPreferences 保存当前用户的偏好设置。
func (s *Service) updateUserPreferences(c *gin.Context) {
	var input appservice.UserPreferencesInput
	if !bindJSON(c, &input) {
		return
	}
	user, err := s.application.UpdateUserPreferences(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusOK, user, err)
}

// updateUserWorkStatus 保存当前用户主动设置的工作状态。
func (s *Service) updateUserWorkStatus(c *gin.Context) {
	var input appservice.UserWorkStatusInput
	if !bindJSON(c, &input) {
		return
	}
	user, err := s.application.UpdateUserWorkStatus(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusOK, user, err)
}

func (s *Service) loadInbox(c *gin.Context) {
	inbox, err := s.application.LoadInbox(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, inbox, err)
}

// listMessageChannels 返回消息渠道列表。
func (s *Service) listMessageChannels(c *gin.Context) {
	list, err := s.application.ListMessageChannels(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, list, err)
}

// getWebsiteChannel 返回网站渠道详情。
func (s *Service) getWebsiteChannel(c *gin.Context) {
	channel, err := s.application.GetWebsiteChannel(c.Request.Context(), requestMeta(c), c.Param("channelID"))
	writeResult(c, http.StatusOK, channel, err)
}

// getMessageChannel 返回消息渠道基础信息。
func (s *Service) getMessageChannel(c *gin.Context) {
	channel, err := s.application.GetMessageChannel(c.Request.Context(), requestMeta(c), c.Param("channelID"))
	writeResult(c, http.StatusOK, channel, err)
}

// createMessageChannel 创建消息渠道。
func (s *Service) createMessageChannel(c *gin.Context) {
	var input appservice.CreateMessageChannelInput
	if !bindJSON(c, &input) {
		return
	}
	channel, err := s.application.CreateMessageChannel(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusCreated, channel, err)
}

// updateMessageChannel 修改消息渠道基础信息。
func (s *Service) updateMessageChannel(c *gin.Context) {
	var input appservice.MessageChannelInput
	if !bindJSON(c, &input) {
		return
	}
	channel, err := s.application.UpdateMessageChannel(c.Request.Context(), requestMeta(c), c.Param("channelID"), input)
	writeResult(c, http.StatusOK, channel, err)
}

// updateWebsiteChannelChatInterface 修改网站渠道聊天界面。
func (s *Service) updateWebsiteChannelChatInterface(c *gin.Context) {
	var input appservice.WebsiteChannelChatInterfaceInput
	if !bindJSON(c, &input) {
		return
	}
	setting, err := s.application.UpdateWebsiteChannelChatInterface(c.Request.Context(), requestMeta(c), c.Param("channelID"), input)
	writeResult(c, http.StatusOK, setting, err)
}

// updateWebsiteChannelAccess 修改网站渠道允许使用的网站。
func (s *Service) updateWebsiteChannelAccess(c *gin.Context) {
	var input appservice.WebsiteChannelAccessInput
	if !bindJSON(c, &input) {
		return
	}
	access, err := s.application.UpdateWebsiteChannelAccess(c.Request.Context(), requestMeta(c), c.Param("channelID"), input)
	writeResult(c, http.StatusOK, access, err)
}

// deactivateMessageChannel 停用消息渠道。
func (s *Service) deactivateMessageChannel(c *gin.Context) {
	channel, err := s.application.DeactivateMessageChannel(c.Request.Context(), requestMeta(c), c.Param("channelID"))
	writeResult(c, http.StatusOK, channel, err)
}

// activateMessageChannel 启用消息渠道。
func (s *Service) activateMessageChannel(c *gin.Context) {
	channel, err := s.application.ActivateMessageChannel(c.Request.Context(), requestMeta(c), c.Param("channelID"))
	writeResult(c, http.StatusOK, channel, err)
}

// listChannelOptions 返回可用渠道选项。
func (s *Service) listChannelOptions(c *gin.Context) {
	list, err := s.application.ListChannelOptions(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, list, err)
}

// listMemberOptions 返回可分配的企业成员。
func (s *Service) listMemberOptions(c *gin.Context) {
	page, ok := positiveQueryInteger(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInteger(c, "pageSize", 50)
	if !ok {
		return
	}
	list, err := s.application.ListMemberOptions(c.Request.Context(), requestMeta(c), appservice.MemberOptionListInput{
		Query: c.Query("query"), Page: page, PageSize: pageSize,
	})
	writeResult(c, http.StatusOK, list, err)
}

// createAgent 创建企业 AI 员工。
func (s *Service) createAgent(c *gin.Context) {
	var input appservice.CreateAgentInput
	if !bindJSON(c, &input) {
		return
	}
	agent, err := s.application.CreateAgent(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusCreated, agent, err)
}

// listAgentModelOptions 返回 AI 员工可使用的对话模型。
func (s *Service) listAgentModelOptions(c *gin.Context) {
	models, err := s.application.ListAgentModelOptions(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, models, err)
}

// listAgents 返回企业 AI 员工目录。
func (s *Service) listAgents(c *gin.Context) {
	page, ok := positiveQueryInteger(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInteger(c, "pageSize", 50)
	if !ok {
		return
	}
	list, err := s.application.ListAgents(c.Request.Context(), requestMeta(c), appservice.AgentListInput{
		Query: c.Query("query"), Status: optionalEnum[appservice.UserStatus](c.Query("status")), Page: page, PageSize: pageSize,
	})
	writeResult(c, http.StatusOK, list, err)
}

// getAgent 返回企业 AI 员工详情。
func (s *Service) getAgent(c *gin.Context) {
	agent, err := s.application.GetAgent(c.Request.Context(), requestMeta(c), c.Param("agentID"))
	writeResult(c, http.StatusOK, agent, err)
}

// updateAgent 修改企业 AI 员工。
func (s *Service) updateAgent(c *gin.Context) {
	var input appservice.UpdateAgentInput
	if !bindJSON(c, &input) {
		return
	}
	agent, err := s.application.UpdateAgent(c.Request.Context(), requestMeta(c), c.Param("agentID"), input)
	writeResult(c, http.StatusOK, agent, err)
}

// updateAgentExecution 修改企业 AI 员工执行配置。
func (s *Service) updateAgentExecution(c *gin.Context) {
	var input appservice.AgentExecutionInput
	if !bindJSON(c, &input) {
		return
	}
	agent, err := s.application.UpdateAgentExecution(c.Request.Context(), requestMeta(c), c.Param("agentID"), input)
	writeResult(c, http.StatusOK, agent, err)
}

// updateAgentWorkStatus 修改企业 AI 员工工作状态。
func (s *Service) updateAgentWorkStatus(c *gin.Context) {
	var input appservice.AgentWorkStatusInput
	if !bindJSON(c, &input) {
		return
	}
	agent, err := s.application.UpdateAgentWorkStatus(c.Request.Context(), requestMeta(c), c.Param("agentID"), input)
	writeResult(c, http.StatusOK, agent, err)
}

// deactivateAgent 禁用企业 AI 员工账号。
func (s *Service) deactivateAgent(c *gin.Context) {
	agent, err := s.application.DeactivateAgent(c.Request.Context(), requestMeta(c), c.Param("agentID"))
	writeResult(c, http.StatusOK, agent, err)
}

// reactivateAgent 恢复企业 AI 员工。
func (s *Service) reactivateAgent(c *gin.Context) {
	agent, err := s.application.ReactivateAgent(c.Request.Context(), requestMeta(c), c.Param("agentID"))
	writeResult(c, http.StatusOK, agent, err)
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
		Query: c.Query("query"), Status: optionalEnum[appservice.UserStatus](c.Query("status")), RoleID: c.Query("roleId"), TeamID: c.Query("teamId"), Page: page, PageSize: pageSize,
	})
	writeResult(c, http.StatusOK, users, err)
}

func (s *Service) getUser(c *gin.Context) {
	user, err := s.application.GetUser(c.Request.Context(), requestMeta(c), c.Param("userID"))
	writeResult(c, http.StatusOK, user, err)
}

func (s *Service) createUser(c *gin.Context) {
	var input appservice.CreateUserInput
	if !bindJSON(c, &input) {
		return
	}
	user, err := s.application.CreateUser(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusCreated, user, err)
}

func (s *Service) updateUser(c *gin.Context) {
	var input appservice.UpdateUserInput
	if !bindJSON(c, &input) {
		return
	}
	user, err := s.application.UpdateUser(c.Request.Context(), requestMeta(c), c.Param("userID"), input)
	writeResult(c, http.StatusOK, user, err)
}

// updateUserRoles 在一个事务中批量调整企业成员角色。
func (s *Service) updateUserRoles(c *gin.Context) {
	var input appservice.UserRoleChangesInput
	if !bindJSON(c, &input) {
		return
	}
	writeEmpty(c, s.application.UpdateUserRoles(c.Request.Context(), requestMeta(c), input))
}

// deactivateUser 禁用企业成员账号。
func (s *Service) deactivateUser(c *gin.Context) {
	user, err := s.application.DeactivateUser(c.Request.Context(), requestMeta(c), c.Param("userID"))
	writeResult(c, http.StatusOK, user, err)
}

// reactivateUser 恢复企业成员账号。
func (s *Service) reactivateUser(c *gin.Context) {
	user, err := s.application.ReactivateUser(c.Request.Context(), requestMeta(c), c.Param("userID"))
	writeResult(c, http.StatusOK, user, err)
}

func (s *Service) listTeams(c *gin.Context) {
	page, ok := positiveQueryInteger(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInteger(c, "pageSize", 50)
	if !ok {
		return
	}
	teams, err := s.application.ListTeams(c.Request.Context(), requestMeta(c), appservice.TeamListInput{Query: c.Query("query"), Page: page, PageSize: pageSize})
	writeResult(c, http.StatusOK, teams, err)
}

func (s *Service) createTeam(c *gin.Context) {
	var input appservice.TeamInput
	if !bindJSON(c, &input) {
		return
	}
	team, err := s.application.CreateTeam(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusCreated, team, err)
}

func (s *Service) updateTeam(c *gin.Context) {
	var input appservice.TeamInput
	if !bindJSON(c, &input) {
		return
	}
	team, err := s.application.UpdateTeam(c.Request.Context(), requestMeta(c), c.Param("teamID"), input)
	writeResult(c, http.StatusOK, team, err)
}

func (s *Service) deleteTeam(c *gin.Context) {
	writeEmpty(c, s.application.DeleteTeam(c.Request.Context(), requestMeta(c), c.Param("teamID")))
}

// listKnowledgeBases 返回企业知识库列表。
func (s *Service) listKnowledgeBases(c *gin.Context) {
	knowledgeBases, err := s.application.ListKnowledgeBases(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, knowledgeBases, err)
}

// getKnowledgeBase 返回企业知识库详情。
func (s *Service) getKnowledgeBase(c *gin.Context) {
	knowledgeBase, err := s.application.GetKnowledgeBase(c.Request.Context(), requestMeta(c), c.Param("knowledgeBaseID"))
	writeResult(c, http.StatusOK, knowledgeBase, err)
}

// createKnowledgeBase 创建企业知识库。
func (s *Service) createKnowledgeBase(c *gin.Context) {
	var input appservice.KnowledgeBaseInput
	if !bindJSON(c, &input) {
		return
	}
	knowledgeBase, err := s.application.CreateKnowledgeBase(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusCreated, knowledgeBase, err)
}

// updateKnowledgeBase 修改企业知识库。
func (s *Service) updateKnowledgeBase(c *gin.Context) {
	var input appservice.KnowledgeBaseInput
	if !bindJSON(c, &input) {
		return
	}
	knowledgeBase, err := s.application.UpdateKnowledgeBase(c.Request.Context(), requestMeta(c), c.Param("knowledgeBaseID"), input)
	writeResult(c, http.StatusOK, knowledgeBase, err)
}

// deleteKnowledgeBase 删除企业知识库。
func (s *Service) deleteKnowledgeBase(c *gin.Context) {
	writeEmpty(c, s.application.DeleteKnowledgeBase(c.Request.Context(), requestMeta(c), c.Param("knowledgeBaseID")))
}

// createKnowledgeGroup 创建知识库分组。
func (s *Service) createKnowledgeGroup(c *gin.Context) {
	var input appservice.KnowledgeGroupInput
	if !bindJSON(c, &input) {
		return
	}
	knowledgeBase, err := s.application.CreateKnowledgeGroup(c.Request.Context(), requestMeta(c), c.Param("knowledgeBaseID"), input)
	writeResult(c, http.StatusCreated, knowledgeBase, err)
}

// updateKnowledgeGroup 修改知识库分组。
func (s *Service) updateKnowledgeGroup(c *gin.Context) {
	var input appservice.KnowledgeGroupInput
	if !bindJSON(c, &input) {
		return
	}
	knowledgeBase, err := s.application.UpdateKnowledgeGroup(c.Request.Context(), requestMeta(c), c.Param("knowledgeBaseID"), c.Param("groupID"), input)
	writeResult(c, http.StatusOK, knowledgeBase, err)
}

// deleteKnowledgeGroup 删除不含子分组的知识库分组。
func (s *Service) deleteKnowledgeGroup(c *gin.Context) {
	knowledgeBase, err := s.application.DeleteKnowledgeGroup(c.Request.Context(), requestMeta(c), c.Param("knowledgeBaseID"), c.Param("groupID"))
	writeResult(c, http.StatusOK, knowledgeBase, err)
}

// listTeamMembers 返回团队成员列表。
func (s *Service) listTeamMembers(c *gin.Context) {
	page, ok := positiveQueryInteger(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInteger(c, "pageSize", 50)
	if !ok {
		return
	}
	list, err := s.application.ListTeamMembers(c.Request.Context(), requestMeta(c), c.Param("teamID"), appservice.TeamMemberListInput{
		Query: c.Query("query"), WorkStatus: optionalEnum[appservice.WorkStatus](c.Query("workStatus")), Page: page, PageSize: pageSize,
	})
	writeResult(c, http.StatusOK, list, err)
}

// listTeamMemberCandidates 返回尚未加入团队的企业成员。
func (s *Service) listTeamMemberCandidates(c *gin.Context) {
	page, ok := positiveQueryInteger(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInteger(c, "pageSize", 50)
	if !ok {
		return
	}
	members, err := s.application.ListTeamMemberCandidates(c.Request.Context(), requestMeta(c), c.Param("teamID"), appservice.TeamMemberCandidateInput{Query: c.Query("query"), Page: page, PageSize: pageSize})
	writeResult(c, http.StatusOK, members, err)
}

// addTeamMembers 将企业身份批量加入团队。
func (s *Service) addTeamMembers(c *gin.Context) {
	var input appservice.TeamMemberInput
	if !bindJSON(c, &input) {
		return
	}
	team, err := s.application.AddTeamMembers(c.Request.Context(), requestMeta(c), c.Param("teamID"), input)
	writeResult(c, http.StatusOK, team, err)
}

// removeTeamMembers 将企业身份批量移出团队。
func (s *Service) removeTeamMembers(c *gin.Context) {
	var input appservice.TeamMemberInput
	if !bindJSON(c, &input) {
		return
	}
	team, err := s.application.RemoveTeamMembers(c.Request.Context(), requestMeta(c), c.Param("teamID"), input)
	writeResult(c, http.StatusOK, team, err)
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

// listRoles 返回企业角色和预定义权限目录。
func (s *Service) listRoles(c *gin.Context) {
	roles, err := s.application.ListRoles(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, roles, err)
}

// getRole 返回企业角色详情。
func (s *Service) getRole(c *gin.Context) {
	role, err := s.application.GetRole(c.Request.Context(), requestMeta(c), c.Param("roleID"))
	writeResult(c, http.StatusOK, role, err)
}

// createRole 创建自定义角色。
func (s *Service) createRole(c *gin.Context) {
	var input appservice.RoleInput
	if !bindJSON(c, &input) {
		return
	}
	role, err := s.application.CreateRole(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusCreated, role, err)
}

// updateRole 修改角色信息和权限。
func (s *Service) updateRole(c *gin.Context) {
	var input appservice.RoleInput
	if !bindJSON(c, &input) {
		return
	}
	role, err := s.application.UpdateRole(c.Request.Context(), requestMeta(c), c.Param("roleID"), input)
	writeResult(c, http.StatusOK, role, err)
}

// deleteRole 删除自定义角色。
func (s *Service) deleteRole(c *gin.Context) {
	writeEmpty(c, s.application.DeleteRole(c.Request.Context(), requestMeta(c), c.Param("roleID")))
}

// listAIProviders 返回模型服务供应商列表。
func (s *Service) listAIProviders(c *gin.Context) {
	providers, err := s.application.ListAIProviders(c.Request.Context(), requestMeta(c))
	writeResult(c, http.StatusOK, providers, err)
}

// getAIProvider 返回模型服务供应商详情。
func (s *Service) getAIProvider(c *gin.Context) {
	provider, err := s.application.GetAIProvider(c.Request.Context(), requestMeta(c), c.Param("providerID"))
	writeResult(c, http.StatusOK, provider, err)
}

// listAvailableAIModels 返回指定品牌的预设模型目录。
func (s *Service) listAvailableAIModels(c *gin.Context) {
	models, err := s.application.ListAvailableAIModels(c.Request.Context(), requestMeta(c), appservice.AIProviderBrand(c.Query("brand")))
	writeResult(c, http.StatusOK, models, err)
}

// createAIProvider 创建模型服务供应商。
func (s *Service) createAIProvider(c *gin.Context) {
	var input appservice.AIProviderInput
	if !bindJSON(c, &input) {
		return
	}
	provider, err := s.application.CreateAIProvider(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusCreated, provider, err)
}

// updateAIProvider 修改模型服务供应商。
func (s *Service) updateAIProvider(c *gin.Context) {
	var input appservice.AIProviderInput
	if !bindJSON(c, &input) {
		return
	}
	provider, err := s.application.UpdateAIProvider(c.Request.Context(), requestMeta(c), c.Param("providerID"), input)
	writeResult(c, http.StatusOK, provider, err)
}

// deleteAIProvider 删除模型服务供应商。
func (s *Service) deleteAIProvider(c *gin.Context) {
	writeEmpty(c, s.application.DeleteAIProvider(c.Request.Context(), requestMeta(c), c.Param("providerID")))
}

// updateOrganization 修改当前企业名称。
func (s *Service) updateOrganization(c *gin.Context) {
	var input appservice.OrganizationInput
	if !bindJSON(c, &input) {
		return
	}
	organization, err := s.application.UpdateOrganization(c.Request.Context(), requestMeta(c), input)
	writeResult(c, http.StatusOK, organization, err)
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
