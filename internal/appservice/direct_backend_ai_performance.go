//go:build server

package appservice

import (
	"context"
	"log/slog"

	aiperformanceaction "github.com/runforyou-ai/cervi/internal/actions/aiperformance"
	"github.com/runforyou-ai/cervi/internal/common"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// aiPerformanceOps 持有 AI 表现报表的查询。
type aiPerformanceOps struct {
	aiPerformanceReport *aiperformanceaction.ReportQuery
}

// newAIPerformanceOps 创建 AI 表现报表的业务实现依赖。
func newAIPerformanceOps(db *bun.DB) aiPerformanceOps {
	return aiPerformanceOps{aiPerformanceReport: aiperformanceaction.NewReportQuery(db)}
}

// GetAIPerformanceReport 返回当前企业指定范围内的 AI 客服表现报表。
func (o *directOperations) GetAIPerformanceReport(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input AIPerformanceReportInput) (AIPerformanceReport, error) {
	if input.ChannelID != "" && !common.ValidUUID(input.ChannelID) {
		return AIPerformanceReport{}, NotFoundError(meta, cervii18n.ErrorChannelNotFound)
	}
	report, err := o.aiPerformanceReport.Execute(ctx, identity, aiperformanceaction.Input{Days: input.Days, ChannelID: input.ChannelID})
	if err != nil {
		slog.Warn("读取 AI 表现报表失败", "organization_id", identity.Organization.ID, "error", err)
		return AIPerformanceReport{}, FailedError(meta, cervii18n.ErrorAIPerformanceReportLoadFailed)
	}
	output := AIPerformanceReport{
		Summary:           AIPerformanceSummary(report.Summary),
		Channels:          aiPerformanceBreakdowns(report.Channels),
		Categories:        aiPerformanceBreakdowns(report.Categories),
		HandoffReasons:    make([]AIHandoffReasonCount, 0, len(report.HandoffReasons)),
		KnowledgeGaps:     make([]AIKnowledgeGap, 0, len(report.KnowledgeGaps)),
		KnowledgeGapTotal: report.KnowledgeGapTotal,
	}
	for _, reason := range report.HandoffReasons {
		output.HandoffReasons = append(output.HandoffReasons, AIHandoffReasonCount{Reason: AgentHandoffReason(reason.Reason), Count: reason.Count})
	}
	for _, gap := range report.KnowledgeGaps {
		output.KnowledgeGaps = append(output.KnowledgeGaps, AIKnowledgeGap{
			EventID: gap.EventID, ConversationID: gap.ConversationID, MessageID: common.StringValue(gap.MessageID), Question: gap.Question,
			Reason: AgentHandoffReason(gap.Reason), CategoryName: common.StringValue(gap.CategoryName), OccurredAt: gap.OccurredAt,
		})
	}
	return output, nil
}

// aiPerformanceBreakdowns 转换按渠道或咨询分类拆分的报表行。
func aiPerformanceBreakdowns(records []aiperformanceaction.Breakdown) []AIPerformanceBreakdown {
	rows := make([]AIPerformanceBreakdown, 0, len(records))
	for _, record := range records {
		rows = append(rows, AIPerformanceBreakdown{ID: common.StringValue(record.ID), Name: record.Name, Closed: record.Closed, AIResolved: record.AIResolved})
	}
	return rows
}
