//go:build server

package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

type contactMethodRequest struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Label     string `json:"label"`
	IsPrimary bool   `json:"isPrimary"`
}

type contactRequest struct {
	DisplayName string                 `json:"displayName"`
	ChannelID   string                 `json:"channelId"`
	Stage       string                 `json:"stage"`
	Notes       string                 `json:"notes"`
	Methods     []contactMethodRequest `json:"methods"`
}

// listContacts 返回当前企业未删除的联系人。
func (s *Service) listContacts(c *gin.Context) {
	s.writeContactList(c, false)
}

// listDeletedContacts 返回当前企业回收站中的联系人。
func (s *Service) listDeletedContacts(c *gin.Context) {
	s.writeContactList(c, true)
}

// writeContactList 返回指定回收状态的联系人列表。
func (s *Service) writeContactList(c *gin.Context, deleted bool) {
	input, ok := contactListInput(c, deleted)
	if !ok {
		return
	}
	output, err := s.listContactsQuery(c.Request.Context(), currentPrincipal(c), input)
	if s.writeContactInputError(c, err, cervii18n.ErrorContactListFailed, "list") {
		return
	}
	c.JSON(http.StatusOK, output)
}

// getContact 返回当前企业的联系人详情。
func (s *Service) getContact(c *gin.Context) {
	contact, err := s.getContactQuery(c.Request.Context(), currentPrincipal(c), c.Param("contactID"))
	if s.writeContactError(c, err, cervii18n.ErrorContactReadFailed, "read") {
		return
	}
	c.JSON(http.StatusOK, contact)
}

// createContact 创建当前企业的外部联系人。
func (s *Service) createContact(c *gin.Context) {
	request, ok := bindContactRequest(c)
	if !ok {
		return
	}
	principal := currentPrincipal(c)
	contact, err := s.createContactAction(c.Request.Context(), principal, request.contactInput())
	if s.writeContactInputError(c, err, cervii18n.ErrorContactCreateFailed, "create") {
		return
	}
	slog.Info("联系人创建完成", "organization_id", principal.Organization.ID, "user_id", principal.User.ID, "contact_id", contact.Contact.ID)
	c.JSON(http.StatusCreated, contact)
}

// updateContact 修改当前企业的外部联系人。
func (s *Service) updateContact(c *gin.Context) {
	request, ok := bindContactRequest(c)
	if !ok {
		return
	}
	principal := currentPrincipal(c)
	contactID := c.Param("contactID")
	contact, err := s.updateContactAction(c.Request.Context(), principal, contactID, request.contactInput())
	if s.writeContactInputError(c, err, cervii18n.ErrorContactUpdateFailed, "update") {
		return
	}
	slog.Info("联系人更新完成", "organization_id", principal.Organization.ID, "user_id", principal.User.ID, "contact_id", contactID)
	c.JSON(http.StatusOK, contact)
}

// deleteContact 将当前企业的联系人移入回收站。
func (s *Service) deleteContact(c *gin.Context) {
	principal := currentPrincipal(c)
	contactID := c.Param("contactID")
	err := s.deleteContactAction(c.Request.Context(), principal, contactID)
	if s.writeContactError(c, err, cervii18n.ErrorContactDeleteFailed, "delete") {
		return
	}
	slog.Info("联系人已移入回收站", "organization_id", principal.Organization.ID, "user_id", principal.User.ID, "contact_id", contactID)
	c.Status(http.StatusNoContent)
}

// restoreContact 恢复当前企业回收站中的联系人。
func (s *Service) restoreContact(c *gin.Context) {
	principal := currentPrincipal(c)
	contactID := c.Param("contactID")
	contact, err := s.restoreContactAction(c.Request.Context(), principal, contactID)
	if s.writeContactError(c, err, cervii18n.ErrorContactRestoreFailed, "restore") {
		return
	}
	slog.Info("联系人恢复完成", "organization_id", principal.Organization.ID, "user_id", principal.User.ID, "contact_id", contactID)
	c.JSON(http.StatusOK, contact)
}

// bindContactRequest 解析联系人请求。
func bindContactRequest(c *gin.Context) (contactRequest, bool) {
	var request contactRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, nil)
		return contactRequest{}, false
	}
	return request, true
}

// contactInput 将 HTTP 请求转换为联系人输入。
func (r contactRequest) contactInput() contactaction.ContactInput {
	methods := make([]contactaction.MethodInput, 0, len(r.Methods))
	for _, method := range r.Methods {
		methods = append(methods, contactaction.MethodInput{
			Type: method.Type, Value: method.Value, Label: method.Label, IsPrimary: method.IsPrimary,
		})
	}
	return contactaction.ContactInput{
		DisplayName: r.DisplayName,
		ChannelID:   r.ChannelID,
		Stage:       r.Stage,
		Notes:       r.Notes,
		Methods:     methods,
	}
}

// contactListInput 解析联系人列表参数。
func contactListInput(c *gin.Context, deleted bool) (contactaction.ListInput, bool) {
	page, ok := positiveQueryInteger(c, "page", 1)
	if !ok {
		return contactaction.ListInput{}, false
	}
	pageSize, ok := positiveQueryInteger(c, "pageSize", 50)
	if !ok {
		return contactaction.ListInput{}, false
	}
	return contactaction.ListInput{
		Query:      c.Query("q"),
		Stage:      c.Query("stage"),
		ChannelID:  c.Query("channelId"),
		MethodType: c.Query("methodType"),
		Sort:       c.Query("sort"),
		Page:       page,
		PageSize:   pageSize,
		Deleted:    deleted,
	}, true
}

// writeContactInputError 处理联系人校验和执行错误。
func (s *Service) writeContactInputError(c *gin.Context, err error, failureKey cervii18n.Key, operation string) bool {
	var validationError *contactaction.ValidationError
	if errors.As(err, &validationError) {
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, contactFieldKeys(validationError.Fields))
		return true
	}
	return s.writeContactError(c, err, failureKey, operation)
}

// writeContactError 处理联系人操作错误。
func (s *Service) writeContactError(c *gin.Context, err error, failureKey cervii18n.Key, operation string) bool {
	if err == nil {
		return false
	}
	principal := currentPrincipal(c)
	logger := slog.With("operation", operation, "organization_id", principal.Organization.ID, "user_id", principal.User.ID)
	if contactID := c.Param("contactID"); contactID != "" {
		logger = logger.With("contact_id", contactID)
	}
	if errors.Is(err, contactaction.ErrPrincipalInvalid) {
		logger.Warn("联系人操作被拒绝")
		writeError(c, http.StatusUnauthorized, "AUTH_REQUIRED", cervii18n.ErrorAuthenticationRequired, nil)
		return true
	}
	if errors.Is(err, contactaction.ErrNotFound) {
		writeError(c, http.StatusNotFound, "CONTACT_NOT_FOUND", cervii18n.ErrorContactNotFound, nil)
		return true
	}
	logger.Warn("联系人操作失败", "error", err)
	writeError(c, http.StatusInternalServerError, "INTERNAL_ERROR", failureKey, nil)
	return true
}
