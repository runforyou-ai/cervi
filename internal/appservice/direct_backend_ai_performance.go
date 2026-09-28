//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	aiperformanceaction "github.com/runforyou-ai/cervi/internal/actions/aiperformance"
	identityaction "github.com/runforyou-ai/cervi/internal/actions/identity"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// aiPerformanceOps 持有 AI 表现报表、问题会话与 AI 员工服务记录的查询。
type aiPerformanceOps struct {
	aiPerformanceOverview    *aiperformanceaction.OverviewQuery
	aiPerformanceBreakdowns  *aiperformanceaction.BreakdownQuery
	agentServiceSessionsList *aiperformanceaction.ServiceSessionListQuery
	aiPerformanceIssues      *aiperformanceaction.IssueListQuery
	aiPerformanceIssue       *aiperformanceaction.IssueQuery
}

// newAIPerformanceOps 创建 AI 表现报表的业务实现依赖。
func newAIPerformanceOps(db *bun.DB) aiPerformanceOps {
	return aiPerformanceOps{
		aiPerformanceOverview:    aiperformanceaction.NewOverviewQuery(db),
		aiPerformanceBreakdowns:  aiperformanceaction.NewBreakdownQuery(db),
		agentServiceSessionsList: aiperformanceaction.NewServiceSessionListQuery(db),
		aiPerformanceIssues:      aiperformanceaction.NewIssueListQuery(db),
		aiPerformanceIssue:       aiperformanceaction.NewIssueQuery(db),
	}
}

// GetAIPerformanceReport 返回当前企业指定范围内的 AI 客服表现概览。
func (o *directOperations) GetAIPerformanceReport(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input AIPerformanceReportInput) (AIPerformanceReport, error) {
	agents, err := reportAgentScope(meta, identity, input.ChannelID, input.AgentID, input.Mine)
	if err != nil {
		return AIPerformanceReport{}, err
	}
	overview, err := o.aiPerformanceOverview.Execute(ctx, identity, aiperformanceaction.Input{Days: input.Days, ChannelID: input.ChannelID, Agents: agents})
	if err != nil {
		return AIPerformanceReport{}, aiPerformanceError(meta, err, identity.Organization.ID)
	}
	output := AIPerformanceReport{
		Summary:           AIPerformanceSummary(overview.Summary),
		HandoffReasons:    make([]AIHandoffReasonCount, 0, len(overview.HandoffReasons)),
		KnowledgeGapTotal: overview.KnowledgeGapTotal,
	}
	for _, reason := range overview.HandoffReasons {
		output.HandoffReasons = append(output.HandoffReasons, AIHandoffReasonCount{Reason: AgentHandoffReason(reason.Reason), Count: reason.Count})
	}
	return output, nil
}

// ListAIPerformanceBreakdowns 返回按渠道或咨询分类拆分的一页 AI 客服表现。
func (o *directOperations) ListAIPerformanceBreakdowns(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input AIPerformanceBreakdownInput) (AIPerformanceBreakdownList, error) {
	agents, err := reportAgentScope(meta, identity, input.ChannelID, input.AgentID, input.Mine)
	if err != nil {
		return AIPerformanceBreakdownList{}, err
	}
	list, err := o.aiPerformanceBreakdowns.Execute(ctx, identity, aiperformanceaction.BreakdownInput{
		Input:     aiperformanceaction.Input{Days: input.Days, ChannelID: input.ChannelID, Agents: agents},
		Dimension: domain.AIPerformanceDimension(input.Dimension), Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return AIPerformanceBreakdownList{}, aiPerformanceError(meta, err, identity.Organization.ID)
	}
	rows := make([]AIPerformanceBreakdown, 0, len(list.Rows))
	for _, row := range list.Rows {
		rows = append(rows, AIPerformanceBreakdown{ID: common.StringValue(row.ID), Name: row.Name, Closed: row.Closed, Resolved: row.Resolved, AIResolved: row.AIResolved})
	}
	return AIPerformanceBreakdownList{Rows: rows, Page: PageInfo{Number: list.Page, Size: list.PageSize, Total: list.Total}}, nil
}

// ListAIPerformanceIssues 返回一页指定类型的问题会话。
func (o *directOperations) ListAIPerformanceIssues(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input AIPerformanceIssueListInput) (AIPerformanceIssueList, error) {
	agents, err := reportAgentScope(meta, identity, input.ChannelID, input.AgentID, input.Mine)
	if err != nil {
		return AIPerformanceIssueList{}, err
	}
	list, err := o.aiPerformanceIssues.Execute(ctx, identity, aiperformanceaction.IssueListInput{
		Input: aiperformanceaction.Input{Days: input.Days, ChannelID: input.ChannelID, Agents: agents},
		Issue: domain.AIPerformanceIssueType(input.Issue), Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return AIPerformanceIssueList{}, aiPerformanceError(meta, err, identity.Organization.ID)
	}
	avatarFileIDs := make([]*string, 0, len(list.Issues))
	for _, issue := range list.Issues {
		avatarFileIDs = append(avatarFileIDs, issue.RequesterAvatarFileID)
	}
	avatarURLs, err := o.optionalFileURLs(ctx, identity, avatarFileIDs...)
	if err != nil {
		return AIPerformanceIssueList{}, aiPerformanceError(meta, err, identity.Organization.ID)
	}
	issues := make([]AIPerformanceIssue, 0, len(list.Issues))
	for _, issue := range list.Issues {
		issues = append(issues, aiPerformanceIssueOutput(issue, avatarURLs))
	}
	return AIPerformanceIssueList{Issues: issues, Page: PageInfo{Number: list.Page, Size: list.PageSize, Total: list.Total}}, nil
}

// GetAIPerformanceIssue 返回客服周期的质检结论与对客沟通。
func (o *directOperations) GetAIPerformanceIssue(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, serviceSessionID string) (AIPerformanceIssueDetail, error) {
	detail, err := o.aiPerformanceIssue.Execute(ctx, identity, serviceSessionID)
	if errors.Is(err, aiperformanceaction.ErrIssueNotFound) {
		return AIPerformanceIssueDetail{}, NotFoundError(meta, cervii18n.ErrorAIPerformanceIssueNotFound)
	}
	if err != nil {
		return AIPerformanceIssueDetail{}, aiPerformanceError(meta, err, identity.Organization.ID)
	}
	avatarURLs, err := o.optionalFileURLs(ctx, identity, detail.RequesterAvatarFileID)
	if err != nil {
		return AIPerformanceIssueDetail{}, aiPerformanceError(meta, err, identity.Organization.ID)
	}
	output := AIPerformanceIssueDetail{Issue: aiPerformanceIssueOutput(detail.Issue, avatarURLs), Messages: make([]ServiceTranscriptMessage, 0, len(detail.Messages))}
	for _, message := range detail.Messages {
		output.Messages = append(output.Messages, ServiceTranscriptMessage{
			ID: message.ID, Sender: ServiceTranscriptSender(message.Sender), SenderName: message.SenderName, Body: message.Body, CreatedAt: message.CreatedAt,
		})
	}
	return output, nil
}

// aiPerformanceIssueOutput 把问题会话转换为传输结构，头像地址取自已批量生成的文件地址。
func aiPerformanceIssueOutput(issue aiperformanceaction.Issue, avatarURLs map[string]string) AIPerformanceIssue {
	return AIPerformanceIssue{
		ServiceSessionID: issue.ServiceSessionID, ConversationID: issue.ConversationID, OpeningMessageID: issue.OpeningMessageID,
		ChannelType: (*ChannelType)(issue.ChannelType), ChannelName: issue.ChannelName,
		RequesterName: common.StringValue(issue.RequesterName), RequesterAvatarURL: optionalFileURL(avatarURLs, issue.RequesterAvatarFileID),
		ClosedAt: issue.ClosedAt, Summary: issue.Summary, Preview: issue.Preview,
		Satisfaction:    (*ServiceSessionSatisfaction)(issue.Satisfaction),
		AIIncorrect:     issue.AIIncorrect != nil && *issue.AIIncorrect,
		AIMissedHandoff: issue.AIMissedHandoff != nil && *issue.AIMissedHandoff,
		AIPoorAttitude:  issue.AIPoorAttitude != nil && *issue.AIPoorAttitude,
	}
}

// ListAgentServiceSessions 返回 AI 员工接待的一页服务周期。
func (o *directOperations) ListAgentServiceSessions(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, agentID string, input AgentServiceSessionListInput) (AgentServiceSessionList, error) {
	if !common.ValidUUID(agentID) {
		return AgentServiceSessionList{}, NotFoundError(meta, cervii18n.ErrorAgentNotFound)
	}
	list, err := o.agentServiceSessionsList.Execute(ctx, identity, aiperformanceaction.ServiceSessionListInput{AgentID: agentID, Page: input.Page, PageSize: input.PageSize})
	if err != nil {
		return AgentServiceSessionList{}, agentServiceSessionsError(meta, err, identity.Organization.ID, agentID)
	}
	avatarFileIDs := make([]*string, 0, len(list.Sessions))
	for _, session := range list.Sessions {
		avatarFileIDs = append(avatarFileIDs, session.RequesterAvatarFileID)
	}
	avatarURLs, err := o.optionalFileURLs(ctx, identity, avatarFileIDs...)
	if err != nil {
		return AgentServiceSessionList{}, agentServiceSessionsError(meta, err, identity.Organization.ID, agentID)
	}
	sessions := make([]AgentServiceSession, 0, len(list.Sessions))
	for _, session := range list.Sessions {
		sessions = append(sessions, AgentServiceSession{
			ServiceSessionID: session.ID, ConversationID: session.ConversationID, OpeningMessageID: session.OpeningMessageID,
			Source: ServiceSource(session.Source), Audience: ServiceAudience(session.Audience),
			ChannelType: (*ChannelType)(session.ChannelType), ChannelName: session.ChannelName,
			RequesterName: common.StringValue(session.RequesterName), RequesterAvatarURL: optionalFileURL(avatarURLs, session.RequesterAvatarFileID),
			Status: ServiceSessionStatus(session.Status), OpenedAt: session.OpenedAt, ClosedAt: session.ClosedAt,
			CloseReason: (*ServiceSessionCloseReason)(session.CloseReason), Preview: session.Preview, Summary: session.Summary, Resolved: session.Resolved,
		})
	}
	return AgentServiceSessionList{Sessions: sessions, Page: PageInfo{Number: list.Page, Size: list.PageSize, Total: list.Total}}, nil
}

// reportAgentScope 校验报表与待补知识的渠道和 AI 员工筛选编号，并返回 AI 员工范围；mine 限定为当前成员负责的 AI 员工。
func reportAgentScope(meta RequestMeta, identity *servermodels.Identity, channelID, agentID string, mine bool) (identityaction.AgentScope, error) {
	if channelID != "" && !common.ValidUUID(channelID) {
		return identityaction.AgentScope{}, NotFoundError(meta, cervii18n.ErrorChannelNotFound)
	}
	if agentID != "" && !common.ValidUUID(agentID) {
		return identityaction.AgentScope{}, NotFoundError(meta, cervii18n.ErrorAgentNotFound)
	}
	scope := identityaction.AgentScope{AgentID: agentID}
	if mine {
		scope.ResponsibleUserID = identity.User.ID
	}
	return scope, nil
}

// agentServiceSessionsError 把服务记录查询错误转换为结构化、本地化错误。
func agentServiceSessionsError(meta RequestMeta, err error, organizationID, agentID string) error {
	if errors.Is(err, aiperformanceaction.ErrPageSizeInvalid) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	slog.Warn("读取 AI 员工服务记录失败", "organization_id", organizationID, "agent_id", agentID, "error", err)
	return FailedError(meta, cervii18n.ErrorAgentServiceSessionsLoadFailed)
}

// aiPerformanceError 把报表查询错误转换为结构化、本地化错误。
func aiPerformanceError(meta RequestMeta, err error, organizationID string) error {
	if errors.Is(err, aiperformanceaction.ErrPageSizeInvalid) || errors.Is(err, aiperformanceaction.ErrDimensionInvalid) || errors.Is(err, aiperformanceaction.ErrIssueInvalid) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	slog.Warn("读取 AI 表现报表失败", "organization_id", organizationID, "error", err)
	return FailedError(meta, cervii18n.ErrorAIPerformanceReportLoadFailed)
}
