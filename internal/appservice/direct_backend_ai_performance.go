//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	aiperformanceaction "github.com/runforyou-ai/cervi/internal/actions/aiperformance"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// aiPerformanceOps 持有 AI 表现报表的查询。
type aiPerformanceOps struct {
	aiPerformanceOverview   *aiperformanceaction.OverviewQuery
	aiPerformanceBreakdowns *aiperformanceaction.BreakdownQuery
	aiKnowledgeGaps         *aiperformanceaction.KnowledgeGapsQuery
}

// newAIPerformanceOps 创建 AI 表现报表的业务实现依赖。
func newAIPerformanceOps(db *bun.DB) aiPerformanceOps {
	return aiPerformanceOps{
		aiPerformanceOverview:   aiperformanceaction.NewOverviewQuery(db),
		aiPerformanceBreakdowns: aiperformanceaction.NewBreakdownQuery(db),
		aiKnowledgeGaps:         aiperformanceaction.NewKnowledgeGapsQuery(db),
	}
}

// GetAIPerformanceReport 返回当前企业指定范围内的 AI 客服表现概览。
func (o *directOperations) GetAIPerformanceReport(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input AIPerformanceReportInput) (AIPerformanceReport, error) {
	if input.ChannelID != "" && !common.ValidUUID(input.ChannelID) {
		return AIPerformanceReport{}, NotFoundError(meta, cervii18n.ErrorChannelNotFound)
	}
	overview, err := o.aiPerformanceOverview.Execute(ctx, identity, aiperformanceaction.Input{Days: input.Days, ChannelID: input.ChannelID})
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
	if input.ChannelID != "" && !common.ValidUUID(input.ChannelID) {
		return AIPerformanceBreakdownList{}, NotFoundError(meta, cervii18n.ErrorChannelNotFound)
	}
	list, err := o.aiPerformanceBreakdowns.Execute(ctx, identity, aiperformanceaction.BreakdownInput{
		Input:     aiperformanceaction.Input{Days: input.Days, ChannelID: input.ChannelID},
		Dimension: domain.AIPerformanceDimension(input.Dimension), Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return AIPerformanceBreakdownList{}, aiPerformanceError(meta, err, identity.Organization.ID)
	}
	rows := make([]AIPerformanceBreakdown, 0, len(list.Rows))
	for _, row := range list.Rows {
		rows = append(rows, AIPerformanceBreakdown{ID: common.StringValue(row.ID), Name: row.Name, Closed: row.Closed, AIResolved: row.AIResolved})
	}
	return AIPerformanceBreakdownList{Rows: rows, Page: PageInfo{Number: list.Page, Size: list.PageSize, Total: list.Total}}, nil
}

// ListAIKnowledgeGaps 返回一页因知识不足或缺少依据的转人工。
func (o *directOperations) ListAIKnowledgeGaps(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input AIKnowledgeGapInput) (AIKnowledgeGapList, error) {
	if input.ChannelID != "" && !common.ValidUUID(input.ChannelID) {
		return AIKnowledgeGapList{}, NotFoundError(meta, cervii18n.ErrorChannelNotFound)
	}
	list, err := o.aiKnowledgeGaps.Execute(ctx, identity, aiperformanceaction.KnowledgeGapInput{
		Input: aiperformanceaction.Input{Days: input.Days, ChannelID: input.ChannelID}, Page: input.Page, PageSize: input.PageSize,
	})
	if err != nil {
		return AIKnowledgeGapList{}, aiPerformanceError(meta, err, identity.Organization.ID)
	}
	gaps := make([]AIKnowledgeGap, 0, len(list.Gaps))
	for _, gap := range list.Gaps {
		gaps = append(gaps, AIKnowledgeGap{
			EventID: gap.EventID, ConversationID: gap.ConversationID, MessageID: common.StringValue(gap.MessageID), Question: gap.Question,
			Reason: AgentHandoffReason(gap.Reason), CategoryName: common.StringValue(gap.CategoryName), OccurredAt: gap.OccurredAt,
		})
	}
	return AIKnowledgeGapList{Gaps: gaps, Page: PageInfo{Number: list.Page, Size: list.PageSize, Total: list.Total}}, nil
}

// aiPerformanceError 把报表查询错误转换为结构化、本地化错误。
func aiPerformanceError(meta RequestMeta, err error, organizationID string) error {
	if errors.Is(err, aiperformanceaction.ErrPageSizeInvalid) || errors.Is(err, aiperformanceaction.ErrDimensionInvalid) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, nil)
	}
	slog.Warn("读取 AI 表现报表失败", "organization_id", organizationID, "error", err)
	return FailedError(meta, cervii18n.ErrorAIPerformanceReportLoadFailed)
}
