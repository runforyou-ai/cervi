//go:build server

package main

import (
	"context"
	"os/signal"
	"syscall"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
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
	translationaction "github.com/runforyou-ai/cervi/internal/actions/translation"
	"github.com/runforyou-ai/cervi/internal/api"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/common/searchtext"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/ingress"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	"github.com/runforyou-ai/cervi/internal/integration/decision"
	"github.com/runforyou-ai/cervi/internal/integration/documentconvert"
	"github.com/runforyou-ai/cervi/internal/integration/embedding"
	mailintegration "github.com/runforyou-ai/cervi/internal/integration/mail"
	mcpintegration "github.com/runforyou-ai/cervi/internal/integration/mcp"
	"github.com/runforyou-ai/cervi/internal/integration/rerank"
	telegramintegration "github.com/runforyou-ai/cervi/internal/integration/telegram"
	"github.com/runforyou-ai/cervi/internal/integration/webfetch"
	"github.com/runforyou-ai/cervi/internal/publicweb"
	"github.com/runforyou-ai/cervi/internal/realtime"
	"github.com/runforyou-ai/cervi/internal/realtime/gateway"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// applicationServices 组装企业服务端入口、业务服务和后台任务，并返回处理实时事件流的资源中间件。
func applicationServices(appStorage *serverstorage.Store, config serverconfig.Config) ([]application.Service, application.Middleware, error) {
	// 按请求域名解析企业，并为 HTTPS 入口提供证书缓存。
	tenantResolver := serverstorage.NewTenantResolver(appStorage.DB())
	httpsEntry := ingress.NewHTTPSEntry(config.TLS, config.Server, serverstorage.NewACMECache(appStorage.DB()), tenantResolver)

	// 初始化本地文件存储和部署级对象存储配置。
	localFiles, err := serverfilecontent.NewLocalStore(config.Storage.LocalDirectory)
	if err != nil {
		return nil, nil, err
	}
	fileS3 := fileContentS3Config(config.Storage.S3)

	// 创建提交后发布受众通知的实时发布器，由服务生命周期统一启停。
	realtimePublisher := realtime.NewPublisher(config.NATS)

	// 创建各业务共用的可靠任务运行时，由服务生命周期统一启停。
	tasks := servertask.New(appStorage.DB(), config.NATS)

	// 注册文档处理任务及最终失败时的状态处理。
	documentConverter := documentconvert.NewClient(config.MarkitdownURL)
	fileReader := serverfilecontent.NewReader(localFiles, fileS3)
	// 上下文附件链接与企业访问入口使用同一协议。
	attachmentScheme := "http"
	if config.TLS.Mode != "off" {
		attachmentScheme = "https"
	}
	// 知识库分词词典在启动时加载一次，供分段写入与词法召回共用。
	if err := searchtext.LoadKnowledgeDictionary(); err != nil {
		return nil, nil, err
	}
	embeddingClient := embedding.NewClient()
	processDocument := knowledgeaction.NewProcessDocumentAction(appStorage.DB(), documentConverter, embeddingClient, fileReader, webfetch.NewClient())
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(knowledgeaction.ProcessDocumentActionName, processDocument.Execute, processDocument.FinalizeFailure); err != nil {
		return nil, nil, err
	}
	// 注册问答索引任务及最终失败时的状态处理。
	processQAEntry := knowledgeaction.NewProcessQAEntryAction(appStorage.DB(), embeddingClient)
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(knowledgeaction.ProcessQAEntryActionName, processQAEntry.Execute, processQAEntry.FinalizeFailure); err != nil {
		return nil, nil, err
	}

	// 注册 MCP 工具目录更新任务及最终失败时的状态处理。
	updateMCPTools := mcpserveraction.NewUpdateToolsAction(appStorage.DB(), mcpintegration.NewClient())
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(mcpserveraction.RefreshToolsActionName, updateMCPTools.Execute, updateMCPTools.FinalizeFailure); err != nil {
		return nil, nil, err
	}

	// 部署配置了 SMTP 时向转人工后离开的网站访客发送客服回复通知，注册延迟的通知检查任务。
	var emailSender customernotify.Sender
	if smtp := config.Email.SMTP; smtp.Enabled() {
		emailSender = mailintegration.NewClient(mailintegration.Config{
			Host: smtp.Host, Port: smtp.Port, Username: smtp.Username, Password: smtp.Password,
			Security: smtp.Security, FromAddress: smtp.FromAddress,
		})
	}
	customerNotify := customernotify.NewWorker(appStorage.DB(), emailSender, attachmentScheme)
	if err := tasks.Registry().RegisterJSON(customernotify.NotifyActionName, customerNotify.Execute); err != nil {
		return nil, nil, err
	}

	// 初始化智能体运行环境，注册执行任务及最终失败处理；运行期通过附件读取器读取会话附件，按配置版本绑定的知识库执行混合检索。
	agentRuntime, err := agentruntime.New()
	if err != nil {
		return nil, nil, err
	}
	agentRunScheduler := agentrunaction.NewScheduler(tasks)
	agentAttachments := agentrunaction.NewAttachmentReader(appStorage.DB(), fileReader, attachmentScheme, fileS3.PublicBaseURL)
	executeAgentRun := agentrunaction.NewExecuteAction(appStorage.DB(), tasks, agentRuntime, agentAttachments,
		knowledgeaction.NewRetrievalService(appStorage.DB(), embedding.NewClient(), rerank.NewClient()), emailSender)
	// 客服 AI 写回复复用模型构造和附件链接，以单次模型调用同步生成回复候选。
	customerReplySuggestions := agentrunaction.NewGenerateCustomerReplySuggestionsAction(appStorage.DB(), agentRuntime, agentAttachments)
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(agentrunaction.RunActionName, executeAgentRun.Execute, executeAgentRun.FinalizeFailure); err != nil {
		return nil, nil, err
	}
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(agentrunaction.ReturnedHandoffActionName, executeAgentRun.HandOffReturnedSession, executeAgentRun.FinalizeReturnedHandoffFailure); err != nil {
		return nil, nil, err
	}
	// 注册设备运行收敛扫描，每 15 秒把租约过期或失去执行条件的设备运行标记失败。
	if err := tasks.Registry().RegisterJSON(agentrunaction.DeviceRunSweepActionName, executeAgentRun.SweepDeviceRuns); err != nil {
		return nil, nil, err
	}
	tasks.RegisterSchedule(servertask.ScheduleDefinition{
		Key: "agent-device-run-sweep", ActionName: agentrunaction.DeviceRunSweepActionName, Queue: "maintenance",
		Payload: struct{}{}, CronExpression: "@every 15s", Timezone: "UTC", Enabled: true, MaxAttempts: 1, StartImmediately: true,
	})

	// 注册过期文件扫描与删除任务，每小时触发一次扫描。
	scanExpired := filemaintenance.NewScanExpiredAction(appStorage.DB(), tasks)
	deleteExpired := filemaintenance.NewDeleteExpiredAction(appStorage.DB(), serverfilecontent.NewDeleter(localFiles, fileS3))
	if err := tasks.Registry().RegisterJSON(filemaintenance.ScanExpiredActionName, scanExpired.Execute); err != nil {
		return nil, nil, err
	}
	if err := tasks.Registry().RegisterJSON(filemaintenance.DeleteExpiredActionName, deleteExpired.Execute); err != nil {
		return nil, nil, err
	}
	tasks.RegisterSchedule(servertask.ScheduleDefinition{
		Key: filemaintenance.CleanupScheduleKey, ActionName: filemaintenance.ScanExpiredActionName, Queue: "maintenance",
		Payload: filemaintenance.ScanExpiredInput{}, CronExpression: "@hourly", Timezone: "UTC",
		Enabled: true, MaxAttempts: 5, StartImmediately: true,
	})

	// 注册客服处理周期的自动分配与成员补分配任务。
	serviceAssignment := serviceassignment.NewWorker(appStorage.DB())
	if err := tasks.Registry().RegisterJSON(serviceassignment.AssignActionName, serviceAssignment.Assign); err != nil {
		return nil, nil, err
	}
	if err := tasks.Registry().RegisterJSON(serviceassignment.BackfillActionName, serviceAssignment.Backfill); err != nil {
		return nil, nil, err
	}

	// 注册客服处理周期小结、交接摘要与待补知识起草任务，判断模型标注诉求、分类、是否解决与答复是否可能有误，正文生成复用单次模型调用。
	serviceSummary := servicesummary.NewWorker(appStorage.DB(), tasks, decision.NewClient(), agentRuntime)
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(servicesummary.SummarizeActionName, serviceSummary.Summarize, serviceSummary.FinalizeSummarizeFailure); err != nil {
		return nil, nil, err
	}
	if err := tasks.Registry().RegisterJSON(servicesummary.HandoffSummaryActionName, serviceSummary.HandoffSummary); err != nil {
		return nil, nil, err
	}
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(knowledgegap.DraftActionName, serviceSummary.DraftKnowledgeGap, serviceSummary.FinalizeKnowledgeGapDraftFailure); err != nil {
		return nil, nil, err
	}

	// 注册客服处理周期超时扫描与单条处理任务，每 30 秒扫描一次到期周期；AI 超时跟进经 Agent 调度器追加输入。
	serviceTimeout := servicetimeout.NewWorker(appStorage.DB(), tasks, agentRunScheduler)
	if err := tasks.Registry().RegisterJSON(servicetimeout.ScanActionName, serviceTimeout.Scan); err != nil {
		return nil, nil, err
	}
	if err := tasks.Registry().RegisterJSON(servicetimeout.ProcessActionName, serviceTimeout.Process); err != nil {
		return nil, nil, err
	}
	tasks.RegisterSchedule(servertask.ScheduleDefinition{
		Key: servicetimeout.ScheduleKey, ActionName: servicetimeout.ScanActionName, Queue: "maintenance",
		Payload: struct{}{}, CronExpression: "@every 30s", Timezone: "UTC", Enabled: true, MaxAttempts: 1, StartImmediately: true,
	})

	// 组装企业成员与网站匿名访客各自的业务入口。
	// 客户会话翻译复用单次模型调用。
	translator := translationaction.NewTranslator(appStorage.DB(), agentRuntime)
	directBackend := appservice.NewDirectBackend(appStorage.DB(), config.Deployment.Mode, localFiles, fileS3, tenantResolver, agentRunScheduler, executeAgentRun, tasks, documentConverter, customerReplySuggestions, translator)
	boundService := appservice.New(directBackend)
	websiteVisitorBackend := appservice.NewWebsiteVisitorDirectBackend(appStorage.DB(), agentRunScheduler, tasks, localFiles, fileS3, emailSender)
	websiteVisitorService := appservice.NewWebsiteVisitorService(websiteVisitorBackend)
	// 实时网关复用成员业务调用的身份解析与同步探针，以及访客的渠道身份解析。
	realtimeGateway := gateway.New(directBackend, websiteVisitorBackend, config.NATS.Namespace, gateway.DefaultOptions())

	// 注册客户消息发送与扫描任务，每五秒扫描一次待投递消息。
	telegramAPI := telegramintegration.NewClient(connectiontest.NewHTTPClient())
	deliveryWorker := deliveryaction.NewWorker(appStorage.DB(), telegramAPI, fileReader, tasks)
	if err := tasks.Registry().RegisterJSON(deliveryaction.SendActionName, deliveryWorker.Execute); err != nil {
		return nil, nil, err
	}
	if err := tasks.Registry().RegisterJSON(deliveryaction.ScanActionName, deliveryWorker.Scan); err != nil {
		return nil, nil, err
	}
	tasks.RegisterSchedule(servertask.ScheduleDefinition{
		Key: "customer-delivery-scan", ActionName: deliveryaction.ScanActionName, Queue: "maintenance",
		Payload: struct{}{}, CronExpression: "@every 5s", Timezone: "UTC", Enabled: true, MaxAttempts: 1, StartImmediately: true,
	})

	// 按部署级存储配置导入 Telegram 头像与入站媒体，注册媒体取回任务及最终失败时的附件终态，并接入渠道 Webhook。
	resolveStorageBackend := func(context.Context, string) (domain.FileStorageBackend, error) {
		if fileS3.Enabled {
			return domain.FileStorageBackendS3, nil
		}
		return domain.FileStorageBackendLocal, nil
	}
	fileWriter := serverfilecontent.NewWriter(localFiles, fileS3)
	telegramAvatarFiles := fileaction.NewImportAction(appStorage.DB(), resolveStorageBackend, fileWriter)
	retrieveTelegramMedia := channelaction.NewRetrieveTelegramMediaAction(appStorage.DB(), telegramAPI, fileWriter, agentRunScheduler)
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(channelaction.RetrieveTelegramMediaActionName, retrieveTelegramMedia.Execute, retrieveTelegramMedia.FinalizeFailure); err != nil {
		return nil, nil, err
	}
	telegramWebhook := channelaction.NewReceiveTelegramWebhookAction(appStorage.DB(), agentRunScheduler, telegramAPI, telegramAvatarFiles, resolveStorageBackend, tasks)

	// 将业务入口适配为 HTTP API，并为公开网站渠道提供配置查询。
	httpAPI := api.NewService(
		boundService,
		api.WithDeviceRuns(directBackend),
		api.WithDeviceModelProxy(directBackend),
		api.WithDeviceRunAttachments(directBackend),
		api.WithWebsiteVisitor(websiteVisitorService, config.TLS.Mode != "off", config.Server.VisitorCountryHeader),
		api.WithWebsiteVisitorRealtime(realtimeGateway),
		api.WithTelegramWebhook(telegramWebhook),
	)
	publicLookup := channelaction.NewGetPublicWebsiteChannelQuery(appStorage.DB()).Execute

	// 注册健康检查、业务与文件路由、公开聊天入口及后台服务生命周期。
	services := []application.Service{
		application.NewServiceWithOptions(api.NewLiveness(), application.ServiceOptions{Route: "/healthz"}),
		application.NewServiceWithOptions(api.NewReadiness(appStorage.DB()), application.ServiceOptions{Route: "/readyz"}),
		application.NewService(&realtimeLifecycle{publisher: realtimePublisher, gateway: realtimeGateway}),
		application.NewService(&httpsLifecycle{service: httpsEntry}),
		application.NewServiceWithOptions(boundService, application.ServiceOptions{
			MarshalError: appservice.MarshalError,
		}),
		application.NewServiceWithOptions(httpAPI, application.ServiceOptions{
			Route: "/api",
		}),
		application.NewServiceWithOptions(api.NewLocalObjectService(appStorage.DB(), localFiles, tenantResolver), application.ServiceOptions{
			Route: "/storage/",
		}),
		application.NewService(&serverTaskLifecycle{runtime: tasks}),
		application.NewServiceWithOptions(publicweb.NewEmbedService(publicLookup), application.ServiceOptions{
			Route: "/embed",
		}),
		application.NewServiceWithOptions(publicweb.NewChatService(publicLookup), application.ServiceOptions{
			Route: "/chat/",
		}),
	}
	// 运营接口只在托管部署注册，凭据认证是其唯一访问控制手段。
	if config.Deployment.Mode.Managed() {
		operatorBackend := appservice.NewOperatorDirectBackend(appStorage.DB(), appservice.OperatorConfig{
			Deployment: appservice.OperatorDeployment{
				Mode:                appservice.DeploymentMode(config.Deployment.Mode),
				ManagedDomainSuffix: config.Deployment.ManagedDomainSuffix,
			},
			Credential:             config.Deployment.OperatorCredential,
			OfficialIdentityIssuer: config.Deployment.OfficialIdentityIssuer,
		})
		services = append(services, application.NewServiceWithOptions(api.NewOperatorService(operatorBackend), application.ServiceOptions{
			Route: "/operator/v1",
		}))
	}
	return services, realtimeGateway.Middleware, nil
}

// fileContentS3Config 把部署级对象存储配置转换为文件内容层配置。
func fileContentS3Config(config serverconfig.S3Config) serverfilecontent.S3Config {
	return serverfilecontent.S3Config{
		Enabled: config.Enabled, Endpoint: config.Endpoint, PublicBaseURL: config.PublicBaseURL,
		Region: config.Region, Bucket: config.Bucket, AccessKeyID: config.AccessKeyID,
		SecretAccessKey: config.SecretAccessKey, ForcePathStyle: config.ForcePathStyle,
	}
}

// httpsLifecycle 将 HTTPS 入口接入 Wails 服务生命周期。
type httpsLifecycle struct {
	service *ingress.HTTPSEntry
}

// ServiceStartup 启动 HTTPS 入口。
func (l *httpsLifecycle) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	return l.service.Start(ctx)
}

// ServiceShutdown 关闭 HTTPS 入口。
func (l *httpsLifecycle) ServiceShutdown() error {
	return l.service.Shutdown()
}

// serverTaskLifecycle 将服务端任务运行时接入 Wails 服务生命周期。
type serverTaskLifecycle struct {
	runtime *servertask.Runtime
}

// ServiceStartup 在企业服务端启动后运行异步任务和定时计划。
func (l *serverTaskLifecycle) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	return l.runtime.Start(ctx)
}

// ServiceShutdown 停止服务端异步任务和 NATS 连接。
func (l *serverTaskLifecycle) ServiceShutdown() error {
	return l.runtime.Stop()
}

// realtimeLifecycle 将实时通知发布器与成员实时网关接入 Wails 服务生命周期。
type realtimeLifecycle struct {
	publisher *realtime.Publisher
	gateway   *gateway.Gateway
}

// ServiceStartup 连接 NATS，开始发布已提交通知并接收实时事件流请求。
func (l *realtimeLifecycle) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	if err := l.publisher.Start(); err != nil {
		return err
	}
	l.gateway.Start(l.publisher.Connection())
	// 收到 SIGINT、SIGTERM 时立即结束实时事件流。
	signals, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-signals.Done()
		stop()
		l.gateway.Shutdown()
	}()
	return nil
}

// ServiceShutdown 结束实时事件流后停止实时通知发布器。
func (l *realtimeLifecycle) ServiceShutdown() error {
	l.gateway.Shutdown()
	return l.publisher.Stop()
}
