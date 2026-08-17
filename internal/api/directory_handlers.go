//go:build server

package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// listChannels 返回当前企业的渠道摘要。
func (s *Service) listChannels(c *gin.Context) {
	principal := currentPrincipal(c)
	channels, err := s.listChannelsQuery(c.Request.Context(), principal)
	if err != nil {
		slog.Warn("读取渠道摘要失败", "organization_id", principal.Organization.ID, "user_id", principal.User.ID, "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorChannelSummaryListFailed, nil)
		return
	}
	c.JSON(http.StatusOK, gin.H{"channels": channels})
}

// listUsers 返回当前企业的团队成员目录。
func (s *Service) listUsers(c *gin.Context) {
	page, ok := positiveQueryInteger(c, "page", 1)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInteger(c, "pageSize", 50)
	if !ok {
		return
	}
	principal := currentPrincipal(c)
	output, err := s.listUsersQuery(c.Request.Context(), principal, useraction.ListInput{
		Query: c.Query("q"), Status: c.Query("status"), Role: c.Query("role"), Page: page, PageSize: pageSize,
	})
	if errors.Is(err, useraction.ErrQueryInvalid) {
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, nil)
		return
	}
	if err != nil {
		slog.Warn("读取团队成员列表失败", "organization_id", principal.Organization.ID, "user_id", principal.User.ID, "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorUserListFailed, nil)
		return
	}
	c.JSON(http.StatusOK, output)
}

// getUser 返回当前企业的团队成员详情。
func (s *Service) getUser(c *gin.Context) {
	principal := currentPrincipal(c)
	userID := c.Param("userID")
	user, err := s.getUserQuery(c.Request.Context(), principal, userID)
	if errors.Is(err, useraction.ErrNotFound) {
		writeError(c, http.StatusNotFound, "USER_NOT_FOUND", cervii18n.ErrorUserNotFound, nil)
		return
	}
	if err != nil {
		slog.Warn("读取团队成员详情失败", "organization_id", principal.Organization.ID, "user_id", principal.User.ID, "target_user_id", userID, "error", err)
		writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", cervii18n.ErrorUserReadFailed, nil)
		return
	}
	c.JSON(http.StatusOK, user)
}
