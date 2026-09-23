//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	customerserviceaction "github.com/runforyou-ai/cervi/internal/actions/customerservice"
	servicecategoryaction "github.com/runforyou-ai/cervi/internal/actions/servicecategory"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// customerServiceOps 持有企业客服设置的业务实现依赖。
type customerServiceOps struct {
	getBusinessHours      *customerserviceaction.GetBusinessHoursQuery
	updateBusinessHours   *customerserviceaction.UpdateBusinessHoursAction
	getServiceTimeouts    *customerserviceaction.GetServiceTimeoutsQuery
	updateServiceTimeouts *customerserviceaction.UpdateServiceTimeoutsAction
	listCategories        *servicecategoryaction.ListQuery
	createCategory        *servicecategoryaction.CreateAction
	updateCategory        *servicecategoryaction.UpdateAction
	archiveCategory       *servicecategoryaction.ArchiveAction
	getIdentitySecret     *customerserviceaction.GetCustomerIdentitySecretQuery
	regenerateSecret      *customerserviceaction.RegenerateCustomerIdentitySecretAction
	getCustomerProfile    *contactaction.GetCustomerProfileQuery
	listBusinessQueries   *conversationaction.ListBusinessQueriesQuery
}

// newCustomerServiceOps 创建企业客服设置的业务实现依赖。
func newCustomerServiceOps(db *bun.DB) customerServiceOps {
	return customerServiceOps{
		getBusinessHours:      customerserviceaction.NewGetBusinessHoursQuery(db),
		updateBusinessHours:   customerserviceaction.NewUpdateBusinessHoursAction(db),
		getServiceTimeouts:    customerserviceaction.NewGetServiceTimeoutsQuery(db),
		updateServiceTimeouts: customerserviceaction.NewUpdateServiceTimeoutsAction(db),
		listCategories:        servicecategoryaction.NewListQuery(db),
		createCategory:        servicecategoryaction.NewCreateAction(db),
		updateCategory:        servicecategoryaction.NewUpdateAction(db),
		archiveCategory:       servicecategoryaction.NewArchiveAction(db),
		getIdentitySecret:     customerserviceaction.NewGetCustomerIdentitySecretQuery(db),
		regenerateSecret:      customerserviceaction.NewRegenerateCustomerIdentitySecretAction(db),
		getCustomerProfile:    contactaction.NewGetCustomerProfileQuery(db),
		listBusinessQueries:   conversationaction.NewListBusinessQueriesQuery(db),
	}
}

// GetCustomerIdentitySecret 读取当前企业的客户身份密钥，未生成时为空。
func (o *directOperations) GetCustomerIdentitySecret(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (CustomerIdentitySecret, error) {
	secret, err := o.getIdentitySecret.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return CustomerIdentitySecret{}, ctx.Err()
		}
		slog.Warn("读取客户身份密钥失败", "organization_id", identity.Organization.ID, "error", err)
		return CustomerIdentitySecret{}, FailedError(meta, cervii18n.ErrorCustomerIdentitySecretLoadFailed)
	}
	return CustomerIdentitySecret{Secret: secret}, nil
}

// RegenerateCustomerIdentitySecret 生成或重新生成当前企业的客户身份密钥，旧密钥立即失效。
func (o *directOperations) RegenerateCustomerIdentitySecret(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (CustomerIdentitySecret, error) {
	secret, err := o.regenerateSecret.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return CustomerIdentitySecret{}, ctx.Err()
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return CustomerIdentitySecret{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("生成客户身份密钥失败", "organization_id", identity.Organization.ID, "error", err)
		return CustomerIdentitySecret{}, FailedError(meta, cervii18n.ErrorIdentitySecretRegenerateFailed)
	}
	slog.Info("客户身份密钥已重新生成", "organization_id", identity.Organization.ID, "user_id", identity.User.ID)
	return CustomerIdentitySecret{Secret: secret}, nil
}

// GetCustomerProfile 返回客户会话的客户身份与当前周期访客上下文。
func (o *directOperations) GetCustomerProfile(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string) (CustomerProfile, error) {
	profile, err := o.getCustomerProfile.Execute(ctx, identity, conversationID)
	if err != nil {
		if ctx.Err() != nil {
			return CustomerProfile{}, ctx.Err()
		}
		if errors.Is(err, contactaction.ErrNotFound) {
			return CustomerProfile{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound)
		}
		slog.Warn("读取客户资料失败", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "error", err)
		return CustomerProfile{}, FailedError(meta, cervii18n.ErrorCustomerProfileLoadFailed)
	}
	result := CustomerProfile{IdentityVerified: profile.IdentityVerified, ExternalUserID: profile.ExternalUserID, Email: profile.Email}
	if visit := profile.VisitorContext; visit != nil {
		result.Visit = &CustomerVisit{
			ReferrerURL: visit.ReferrerURL, PageURL: visit.PageURL, PageTitle: visit.PageTitle, Browser: visit.Browser, OS: visit.OS,
			DeviceType: visit.DeviceType, Language: visit.Language, TimeZone: visit.TimeZone, Country: visit.Country,
		}
	}
	return result, nil
}

// ListCustomerBusinessQueries 返回客户会话当前客服周期内 AI 客服查询业务系统的记录。
func (o *directOperations) ListCustomerBusinessQueries(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, conversationID string) (CustomerBusinessQueryList, error) {
	queries, err := o.listBusinessQueries.Execute(ctx, identity, conversationID)
	if err != nil {
		if ctx.Err() != nil {
			return CustomerBusinessQueryList{}, ctx.Err()
		}
		if errors.Is(err, conversationaction.ErrConversationNotFound) {
			return CustomerBusinessQueryList{}, NotFoundError(meta, cervii18n.ErrorConversationNotFound)
		}
		slog.Warn("读取业务查询记录失败", "organization_id", identity.Organization.ID, "conversation_id", conversationID, "error", err)
		return CustomerBusinessQueryList{}, FailedError(meta, cervii18n.ErrorBusinessQueriesLoadFailed)
	}
	result := CustomerBusinessQueryList{Queries: make([]CustomerBusinessQuery, 0, len(queries))}
	for _, query := range queries {
		call := query.ToolCall
		result.Queries = append(result.Queries, CustomerBusinessQuery{
			ID: query.ID, MCPServer: call.MCPServer, ToolName: call.Name, Arguments: call.Arguments, Result: call.Result, Error: call.Error,
			Status: AgentToolCallStatus(call.Status), Evidence: call.Evidence, CalledAt: query.CalledAt,
		})
	}
	return result, nil
}

// GetBusinessHours 读取当前企业的客服工作时间。
func (o *directOperations) GetBusinessHours(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (BusinessHours, error) {
	hours, err := o.getBusinessHours.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return BusinessHours{}, ctx.Err()
		}
		slog.Warn("读取客服工作时间失败", "organization_id", identity.Organization.ID, "error", err)
		return BusinessHours{}, FailedError(meta, cervii18n.ErrorBusinessHoursLoadFailed)
	}
	return businessHoursFromDomain(hours), nil
}

// UpdateBusinessHours 修改当前企业的客服工作时间。
func (o *directOperations) UpdateBusinessHours(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input BusinessHours) (BusinessHours, error) {
	// 每周时段固定 7 项，按周一到周日转换为领域值。
	if len(input.Weekly) != 7 {
		return BusinessHours{}, InvalidError(meta, cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{"weekly": cervii18n.FieldBusinessHoursWeeklyInvalid})
	}
	hours := domain.BusinessHours{Enabled: input.Enabled, TimeZone: input.TimeZone, Overrides: make([]domain.BusinessHoursOverride, 0, len(input.Overrides))}
	for day, periods := range input.Weekly {
		hours.Weekly[day] = businessHoursPeriodsToDomain(periods)
	}
	for _, override := range input.Overrides {
		hours.Overrides = append(hours.Overrides, domain.BusinessHoursOverride{Date: override.Date, Periods: businessHoursPeriodsToDomain(override.Periods)})
	}
	saved, err := o.updateBusinessHours.Execute(ctx, identity, hours)
	if err != nil {
		if ctx.Err() != nil {
			return BusinessHours{}, ctx.Err()
		}
		if validationError, ok := errors.AsType[*common.FieldError](err); ok {
			// 把客服工作时间校验错误码映射为本地化文案键。
			keys := map[common.FieldCode]cervii18n.Key{
				customerserviceaction.ValidationTimeZoneInvalid: cervii18n.FieldTimeZoneInvalid,
				customerserviceaction.ValidationWeeklyInvalid:   cervii18n.FieldBusinessHoursWeeklyInvalid,
				customerserviceaction.ValidationOverrideInvalid: cervii18n.FieldBusinessHoursOverrideInvalid,
			}
			return BusinessHours{}, InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, keys))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return BusinessHours{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("修改客服工作时间失败", "organization_id", identity.Organization.ID, "error", err)
		return BusinessHours{}, FailedError(meta, cervii18n.ErrorBusinessHoursUpdateFailed)
	}
	slog.Info("客服工作时间已更新", "organization_id", identity.Organization.ID, "enabled", saved.Enabled, "time_zone", saved.TimeZone)
	return businessHoursFromDomain(saved), nil
}

// GetServiceTimeouts 读取当前企业的客服超时时长。
func (o *directOperations) GetServiceTimeouts(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (ServiceTimeouts, error) {
	timeouts, err := o.getServiceTimeouts.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return ServiceTimeouts{}, ctx.Err()
		}
		slog.Warn("读取客服超时时长失败", "organization_id", identity.Organization.ID, "error", err)
		return ServiceTimeouts{}, FailedError(meta, cervii18n.ErrorServiceTimeoutsLoadFailed)
	}
	return ServiceTimeouts(timeouts), nil
}

// UpdateServiceTimeouts 修改当前企业的客服超时时长。
func (o *directOperations) UpdateServiceTimeouts(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input ServiceTimeouts) (ServiceTimeouts, error) {
	saved, err := o.updateServiceTimeouts.Execute(ctx, identity, domain.ServiceTimeouts(input))
	if err != nil {
		if ctx.Err() != nil {
			return ServiceTimeouts{}, ctx.Err()
		}
		if validationError, ok := errors.AsType[*common.FieldError](err); ok {
			// 把客服超时时长校验错误码映射为本地化文案键。
			keys := map[common.FieldCode]cervii18n.Key{
				customerserviceaction.ValidationTimeoutMinutesInvalid: cervii18n.FieldServiceTimeoutMinutesInvalid,
				customerserviceaction.ValidationReclaimNotAfterRemind: cervii18n.FieldServiceReclaimNotAfterReminder,
			}
			return ServiceTimeouts{}, InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, keys))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return ServiceTimeouts{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("修改客服超时时长失败", "organization_id", identity.Organization.ID, "error", err)
		return ServiceTimeouts{}, FailedError(meta, cervii18n.ErrorServiceTimeoutsUpdateFailed)
	}
	slog.Info("客服超时时长已更新", "organization_id", identity.Organization.ID,
		"response_reminder_minutes", saved.ResponseReminderMinutes, "response_reclaim_minutes", saved.ResponseReclaimMinutes,
		"queue_reminder_minutes", saved.QueueReminderMinutes, "ai_follow_up_minutes", saved.AIFollowUpMinutes, "ai_close_minutes", saved.AICloseMinutes)
	return ServiceTimeouts(saved), nil
}

// ListServiceCategories 返回当前企业的咨询分类目录。
func (o *directOperations) ListServiceCategories(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (ServiceCategoryList, error) {
	records, err := o.listCategories.Execute(ctx, identity)
	if err != nil {
		return ServiceCategoryList{}, serviceCategoryError(ctx, meta, err, cervii18n.ErrorServiceCategoryListFailed, identity.Organization.ID, "")
	}
	categories := make([]ServiceCategory, 0, len(records))
	for _, record := range records {
		categories = append(categories, serviceCategoryFromAction(record))
	}
	return ServiceCategoryList{Categories: categories}, nil
}

// CreateServiceCategory 新增咨询分类。
func (o *directOperations) CreateServiceCategory(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input ServiceCategoryInput) (ServiceCategory, error) {
	record, err := o.createCategory.Execute(ctx, identity, servicecategoryaction.Input{Name: input.Name, Description: input.Description, TeamID: input.TeamID})
	if err != nil {
		return ServiceCategory{}, serviceCategoryError(ctx, meta, err, cervii18n.ErrorServiceCategoryCreateFailed, identity.Organization.ID, "")
	}
	slog.Info("咨询分类已新增", "organization_id", identity.Organization.ID, "service_category_id", record.ID)
	return serviceCategoryFromAction(*record), nil
}

// UpdateServiceCategory 修改咨询分类。
func (o *directOperations) UpdateServiceCategory(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, categoryID string, input ServiceCategoryInput) (ServiceCategory, error) {
	record, err := o.updateCategory.Execute(ctx, identity, categoryID, servicecategoryaction.Input{Name: input.Name, Description: input.Description, TeamID: input.TeamID})
	if err != nil {
		return ServiceCategory{}, serviceCategoryError(ctx, meta, err, cervii18n.ErrorServiceCategoryUpdateFailed, identity.Organization.ID, categoryID)
	}
	slog.Info("咨询分类已更新", "organization_id", identity.Organization.ID, "service_category_id", categoryID)
	return serviceCategoryFromAction(*record), nil
}

// DeleteServiceCategory 归档咨询分类。
func (o *directOperations) DeleteServiceCategory(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, categoryID string) error {
	if err := o.archiveCategory.Execute(ctx, identity, categoryID); err != nil {
		return serviceCategoryError(ctx, meta, err, cervii18n.ErrorServiceCategoryDeleteFailed, identity.Organization.ID, categoryID)
	}
	slog.Info("咨询分类已删除", "organization_id", identity.Organization.ID, "service_category_id", categoryID)
	return nil
}

// serviceCategoryError 把咨询分类操作错误转换为结构化、本地化错误。
func serviceCategoryError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID, categoryID string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		// 把咨询分类校验错误码映射为本地化文案键。
		keys := map[common.FieldCode]cervii18n.Key{
			servicecategoryaction.ValidationNameRequired:       cervii18n.FieldServiceCategoryNameRequired,
			servicecategoryaction.ValidationNameTooLong:        cervii18n.FieldServiceCategoryNameTooLong,
			servicecategoryaction.ValidationNameDuplicate:      cervii18n.FieldServiceCategoryNameDuplicate,
			servicecategoryaction.ValidationDescriptionTooLong: cervii18n.FieldServiceCategoryDescTooLong,
			servicecategoryaction.ValidationTeamInvalid:        cervii18n.FieldTeamInvalid,
		}
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, keys))
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, servicecategoryaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorServiceCategoryNotFound)
	}
	if errors.Is(err, servicecategoryaction.ErrLimitReached) {
		return InvalidError(meta, cervii18n.ErrorServiceCategoryLimitReached, nil)
	}
	slog.Warn("咨询分类操作失败", "organization_id", organizationID, "service_category_id", categoryID, "failure", failureKey, "error", err)
	return FailedError(meta, failureKey)
}

// serviceCategoryFromAction 转换咨询分类契约。
func serviceCategoryFromAction(record servicecategoryaction.Record) ServiceCategory {
	category := ServiceCategory{ID: record.ID, Name: record.Name, Description: record.Description, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
	if record.TeamID != nil && record.TeamName != nil {
		category.Team = &TeamSummary{ID: *record.TeamID, Name: *record.TeamName}
	}
	return category
}

// businessHoursFromDomain 把领域工作时间转换为传输结构。
func businessHoursFromDomain(hours domain.BusinessHours) BusinessHours {
	output := BusinessHours{Enabled: hours.Enabled, TimeZone: hours.TimeZone, Weekly: make([][]BusinessHoursPeriod, 0, len(hours.Weekly)), Overrides: make([]BusinessHoursOverride, 0, len(hours.Overrides))}
	for _, periods := range hours.Weekly {
		output.Weekly = append(output.Weekly, businessHoursPeriodsFromDomain(periods))
	}
	for _, override := range hours.Overrides {
		output.Overrides = append(output.Overrides, BusinessHoursOverride{Date: override.Date, Periods: businessHoursPeriodsFromDomain(override.Periods)})
	}
	return output
}

// businessHoursPeriodsFromDomain 把领域工作时段转换为传输结构。
func businessHoursPeriodsFromDomain(periods []domain.BusinessHoursPeriod) []BusinessHoursPeriod {
	output := make([]BusinessHoursPeriod, 0, len(periods))
	for _, period := range periods {
		output = append(output, BusinessHoursPeriod{Start: period.Start, End: period.End})
	}
	return output
}

// businessHoursPeriodsToDomain 把传输结构的工作时段转换为领域值。
func businessHoursPeriodsToDomain(periods []BusinessHoursPeriod) []domain.BusinessHoursPeriod {
	output := make([]domain.BusinessHoursPeriod, 0, len(periods))
	for _, period := range periods {
		output = append(output, domain.BusinessHoursPeriod{Start: period.Start, End: period.End})
	}
	return output
}
