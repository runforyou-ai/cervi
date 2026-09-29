//go:build server

package main

import (
	"errors"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	customerchataction "github.com/runforyou-ai/cervi/internal/actions/customerchat"
	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	"github.com/runforyou-ai/cervi/internal/actions/customernotify"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/actions/filemaintenance"
	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/actions/knowledgegap"
	mcpserveraction "github.com/runforyou-ai/cervi/internal/actions/mcpserver"
	"github.com/runforyou-ai/cervi/internal/actions/serviceassignment"
	"github.com/runforyou-ai/cervi/internal/actions/servicesummary"
	"github.com/runforyou-ai/cervi/internal/actions/servicetimeout"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/decision"
	"github.com/runforyou-ai/cervi/internal/integration/documentconvert"
	mcpintegration "github.com/runforyou-ai/cervi/internal/integration/mcp"
	telegramintegration "github.com/runforyou-ai/cervi/internal/integration/telegram"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/runforyou-ai/cervi/pkg/embedding"
	"github.com/runforyou-ai/cervi/pkg/webfetch"
	"github.com/uptrace/bun"
)

// serverTaskDeps 定义后台任务处理器共用的存储、模型运行时与外部服务。
type serverTaskDeps struct {
	db            *bun.DB
	tasks         *servertask.Runtime
	publicURL     string
	localFiles    *serverfilecontent.LocalStore
	fileS3        serverfilecontent.S3Config
	fileReader    *serverfilecontent.Reader
	emailSender   customernotify.Sender
	agentRuntime  *agentruntime.EinoRuntime
	agentSchedule *agentrunaction.Scheduler
	agentRun      *agentrunaction.ExecuteAction
	telegramAPI   *telegramintegration.Client
}

// registerServerTasks 注册服务端全部后台任务处理器与定时计划。
func registerServerTasks(deps serverTaskDeps) error {
	registry, db := deps.tasks.Registry(), deps.db

	// 知识库文档处理、问答索引与 MCP 工具目录更新在最终失败时写入失败状态。
	processDocument := knowledgeaction.NewProcessDocumentAction(db, documentconvert.NewConverter(), embedding.NewClient(), deps.fileReader, webfetch.NewClient(common.WebFetchUserAgent()))
	processQAEntry := knowledgeaction.NewProcessQAEntryAction(db, embedding.NewClient())
	updateMCPTools := mcpserveraction.NewUpdateToolsAction(db, mcpintegration.NewClient())
	if err := errors.Join(
		registry.RegisterJSONWithTerminalFailure(knowledgeaction.ProcessDocumentActionName, processDocument.Execute, processDocument.FinalizeFailure),
		registry.RegisterJSONWithTerminalFailure(knowledgeaction.ProcessQAEntryActionName, processQAEntry.Execute, processQAEntry.FinalizeFailure),
		registry.RegisterJSONWithTerminalFailure(mcpserveraction.RefreshToolsActionName, updateMCPTools.Execute, updateMCPTools.FinalizeFailure),
	); err != nil {
		return err
	}

	// 部署配置了 SMTP 时向转人工后离开的网站访客发送客服回复通知，每 30 秒扫描一次到达检查时间的客户会话。
	if deps.emailSender != nil {
		customerNotify := customernotify.NewWorker(db, deps.tasks, deps.emailSender, deps.publicURL)
		if err := errors.Join(
			registry.RegisterJSON(customernotify.ScanActionName, customerNotify.Scan),
			registry.RegisterJSONWithTerminalFailure(customernotify.NotifyActionName, customerNotify.Execute, customerNotify.FinalizeFailure),
		); err != nil {
			return err
		}
		deps.tasks.RegisterSchedule(maintenanceSchedule(customernotify.ScheduleKey, customernotify.ScanActionName, "@every 30s"))
	}

	// Agent 运行执行、会话标题、助理记忆与退回转人工；设备运行收敛扫描每 15 秒把租约过期或失去执行条件的设备运行标记失败。
	agentChatTitle := agentrunaction.NewGenerateAgentChatTitleAction(db, deps.agentRuntime)
	assistantMemory := agentrunaction.NewExtractAssistantMemoryAction(db, deps.tasks, deps.agentRuntime)
	if err := errors.Join(
		registry.RegisterJSONWithTerminalFailure(agentrunaction.RunActionName, deps.agentRun.Execute, deps.agentRun.FinalizeFailure),
		registry.RegisterJSON(agentrunaction.AgentChatTitleActionName, agentChatTitle.Execute),
		registry.RegisterJSON(agentrunaction.AssistantMemoryActionName, assistantMemory.Execute),
		registry.RegisterJSONWithTerminalFailure(agentrunaction.ReturnedHandoffActionName, deps.agentRun.HandOffReturnedSession, deps.agentRun.FinalizeReturnedHandoffFailure),
		registry.RegisterJSON(agentrunaction.DeviceRunSweepActionName, deps.agentRun.SweepDeviceRuns),
	); err != nil {
		return err
	}
	deps.tasks.RegisterSchedule(maintenanceSchedule("agent-device-run-sweep", agentrunaction.DeviceRunSweepActionName, "@every 15s"))

	// 过期文件每小时扫描一次，逐个删除。
	scanExpired := filemaintenance.NewScanExpiredAction(db, deps.tasks)
	deleteExpired := filemaintenance.NewDeleteExpiredAction(db, serverfilecontent.NewDeleter(deps.localFiles, deps.fileS3))
	if err := errors.Join(
		registry.RegisterJSON(filemaintenance.ScanExpiredActionName, scanExpired.Execute),
		registry.RegisterJSON(filemaintenance.DeleteExpiredActionName, deleteExpired.Execute),
	); err != nil {
		return err
	}
	cleanup := maintenanceSchedule(filemaintenance.CleanupScheduleKey, filemaintenance.ScanExpiredActionName, "@hourly")
	cleanup.Payload, cleanup.MaxAttempts = filemaintenance.ScanExpiredInput{}, 5
	deps.tasks.RegisterSchedule(cleanup)

	// 客服处理周期的自动分配与补分配，小结、质检、交接摘要、联系人资料抽取与待补知识起草，以及每 30 秒扫描一次的超时处理；AI 超时跟进经 Agent 调度器追加输入。
	serviceAssignment := serviceassignment.NewWorker(db)
	serviceSummary := servicesummary.NewWorker(db, deps.tasks, decision.NewClient(), deps.agentRuntime)
	serviceTimeout := servicetimeout.NewWorker(db, deps.tasks, deps.agentSchedule)
	if err := errors.Join(
		registry.RegisterJSON(serviceassignment.AssignActionName, serviceAssignment.Assign),
		registry.RegisterJSON(serviceassignment.BackfillActionName, serviceAssignment.Backfill),
		registry.RegisterJSONWithTerminalFailure(servicesummary.SummarizeActionName, serviceSummary.Summarize, serviceSummary.FinalizeSummarizeFailure),
		registry.RegisterJSON(servicesummary.ReviewActionName, serviceSummary.Review),
		registry.RegisterJSON(servicesummary.HandoffSummaryActionName, serviceSummary.HandoffSummary),
		registry.RegisterJSON(servicesummary.ExtractContactProfileActionName, serviceSummary.ExtractContactProfile),
		registry.RegisterJSONWithTerminalFailure(knowledgegap.DraftActionName, serviceSummary.DraftKnowledgeGap, serviceSummary.FinalizeKnowledgeGapDraftFailure),
		registry.RegisterJSON(servicetimeout.ScanActionName, serviceTimeout.Scan),
		registry.RegisterJSON(servicetimeout.ProcessActionName, serviceTimeout.Process),
	); err != nil {
		return err
	}
	deps.tasks.RegisterSchedule(maintenanceSchedule(servicetimeout.ScheduleKey, servicetimeout.ScanActionName, "@every 30s"))

	// 客户消息每五秒扫描一次待投递消息；Telegram 头像与入站媒体按部署级存储配置导入，媒体取回最终失败时写入附件终态。
	deliveryWorker := deliveryaction.NewWorker(db, deps.telegramAPI, deps.fileReader, deps.tasks)
	fileWriter := serverfilecontent.NewWriter(deps.localFiles, deps.fileS3)
	retrieveTelegramMedia := customerchataction.NewRetrieveTelegramMediaAction(db, deps.telegramAPI, fileWriter, deps.agentSchedule)
	refreshTelegramAvatar := channelaction.NewRefreshTelegramContactAvatarAction(db, deps.telegramAPI, fileaction.NewImportAction(db, deps.fileS3.Backend(), fileWriter))
	if err := errors.Join(
		registry.RegisterJSON(deliveryaction.SendActionName, deliveryWorker.Execute),
		registry.RegisterJSON(deliveryaction.ScanActionName, deliveryWorker.Scan),
		registry.RegisterJSONWithTerminalFailure(customerchataction.RetrieveTelegramMediaActionName, retrieveTelegramMedia.Execute, retrieveTelegramMedia.FinalizeFailure),
		registry.RegisterJSON(channelaction.RefreshTelegramContactAvatarActionName, refreshTelegramAvatar.Execute),
	); err != nil {
		return err
	}
	deps.tasks.RegisterSchedule(maintenanceSchedule("customer-delivery-scan", deliveryaction.ScanActionName, "@every 5s"))
	return nil
}

// maintenanceSchedule 返回维护队列中按 UTC 周期触发、启动时立即执行一次、失败不重试的定时计划。
func maintenanceSchedule(key, actionName, cron string) servertask.ScheduleDefinition {
	return servertask.ScheduleDefinition{
		Key: key, ActionName: actionName, Queue: "maintenance", Payload: struct{}{}, CronExpression: cron,
		Timezone: "UTC", Enabled: true, MaxAttempts: 1, StartImmediately: true,
	}
}
