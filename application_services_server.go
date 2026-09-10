//go:build server

package main

import (
	"context"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	deliveryaction "github.com/runforyou-ai/cervi/internal/actions/customerdelivery"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/actions/filemaintenance"
	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	mcpserveraction "github.com/runforyou-ai/cervi/internal/actions/mcpserver"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/api"
	"github.com/runforyou-ai/cervi/internal/appservice"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/ingress"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeprocessing"
	mcpintegration "github.com/runforyou-ai/cervi/internal/integration/mcp"
	telegramintegration "github.com/runforyou-ai/cervi/internal/integration/telegram"
	"github.com/runforyou-ai/cervi/internal/publicweb"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// applicationServices 组装企业服务端入口、业务服务和后台任务。
func applicationServices(appStorage *serverstorage.Store, config serverconfig.Config) ([]application.Service, error) {
	// 按请求域名解析企业，并为 HTTPS 入口提供证书缓存。
	tenantResolver := serverstorage.NewTenantResolver(appStorage.DB())
	httpsEntry := ingress.NewHTTPSEntry(config.TLS, config.Server, serverstorage.NewACMECache(appStorage.DB()), tenantResolver)

	// 初始化本地文件存储和按企业读取的对象存储配置。
	localFiles, err := serverfilecontent.NewLocalStore(config.Storage.LocalDirectory)
	if err != nil {
		return nil, err
	}
	resolveFileS3 := newFileContentS3ConfigResolver(appStorage.DB())

	// 创建各业务共用的可靠任务运行时，由服务生命周期统一启停。
	tasks := servertask.New(appStorage.DB(), config.NATS)

	// 注册文档处理任务及最终失败时的状态处理。
	knowledgeClient := knowledgeprocessing.NewClient(config.HaystackURL)
	processDocument := knowledgeaction.NewProcessDocumentAction(appStorage.DB(), knowledgeClient, serverfilecontent.NewReader(localFiles, resolveFileS3))
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(knowledgeaction.ProcessDocumentActionName, processDocument.Execute, processDocument.FinalizeFailure); err != nil {
		return nil, err
	}

	// 注册 MCP 工具目录更新任务及最终失败时的状态处理。
	updateMCPTools := mcpserveraction.NewUpdateToolsAction(appStorage.DB(), mcpintegration.NewClient())
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(mcpserveraction.RefreshToolsActionName, updateMCPTools.Execute, updateMCPTools.FinalizeFailure); err != nil {
		return nil, err
	}

	// 初始化智能体运行环境，注册执行任务及最终失败处理。
	agentRuntime, err := agentruntime.New()
	if err != nil {
		return nil, err
	}
	agentRunScheduler := agentrunaction.NewScheduler(tasks)
	executeAgentRun := agentrunaction.NewExecuteAction(appStorage.DB(), tasks, agentRuntime)
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(agentrunaction.RunActionName, executeAgentRun.Execute, executeAgentRun.FinalizeFailure); err != nil {
		return nil, err
	}

	// 注册过期文件扫描与删除任务，每小时触发一次扫描。
	scanExpired := filemaintenance.NewScanExpiredAction(appStorage.DB(), tasks)
	deleteExpired := filemaintenance.NewDeleteExpiredAction(appStorage.DB(), serverfilecontent.NewDeleter(localFiles, resolveFileS3))
	if err := tasks.Registry().RegisterJSON(filemaintenance.ScanExpiredActionName, scanExpired.Execute); err != nil {
		return nil, err
	}
	if err := tasks.Registry().RegisterJSON(filemaintenance.DeleteExpiredActionName, deleteExpired.Execute); err != nil {
		return nil, err
	}
	tasks.RegisterSchedule(servertask.ScheduleDefinition{
		Key: filemaintenance.CleanupScheduleKey, ActionName: filemaintenance.ScanExpiredActionName, Queue: "maintenance",
		Payload: filemaintenance.ScanExpiredInput{}, CronExpression: "@hourly", Timezone: "UTC",
		Enabled: true, MaxAttempts: 5, StartImmediately: true,
	})

	// 组装企业成员与网站匿名访客各自的业务入口。
	directBackend := appservice.NewDirectBackend(appStorage.DB(), localFiles, tenantResolver, agentRunScheduler, executeAgentRun, tasks, knowledgeClient)
	boundService := appservice.New(directBackend)
	websiteVisitorBackend := appservice.NewWebsiteVisitorDirectBackend(appStorage.DB(), agentRunScheduler)
	websiteVisitorService := appservice.NewWebsiteVisitorService(websiteVisitorBackend)

	// 注册客户消息发送与扫描任务，每五秒扫描一次待投递消息。
	telegramAPI := telegramintegration.NewClient(connectiontest.NewHTTPClient())
	deliveryWorker := deliveryaction.NewWorker(appStorage.DB(), telegramAPI, tasks)
	if err := tasks.Registry().RegisterJSON(deliveryaction.SendActionName, deliveryWorker.Execute); err != nil {
		return nil, err
	}
	if err := tasks.Registry().RegisterJSON(deliveryaction.ScanActionName, deliveryWorker.Scan); err != nil {
		return nil, err
	}
	tasks.RegisterSchedule(servertask.ScheduleDefinition{
		Key: "customer-delivery-scan", ActionName: deliveryaction.ScanActionName, Queue: "maintenance",
		Payload: struct{}{}, CronExpression: "@every 5s", Timezone: "UTC", Enabled: true, MaxAttempts: 1, StartImmediately: true,
	})

	// 按企业存储设置导入 Telegram 头像，并接入渠道 Webhook。
	getS3Setting := settingaction.NewGetS3SettingQuery(appStorage.DB())
	telegramAvatarFiles := fileaction.NewImportAction(appStorage.DB(), func(ctx context.Context, organizationID string) (domain.FileStorageBackend, error) {
		setting, err := getS3Setting.ExecuteForOrganization(ctx, organizationID)
		if err != nil {
			return "", err
		}
		if setting.Enabled {
			return domain.FileStorageBackendS3, nil
		}
		return domain.FileStorageBackendLocal, nil
	}, serverfilecontent.NewWriter(localFiles, resolveFileS3))
	telegramWebhook := channelaction.NewReceiveTelegramWebhookAction(appStorage.DB(), agentRunScheduler, telegramAPI, telegramAvatarFiles)

	// 将业务入口适配为 HTTP API，并为公开网站渠道提供配置查询。
	httpAPI := api.NewService(
		boundService,
		api.WithWebsiteVisitor(websiteVisitorService, config.TLS.Mode != "off"),
		api.WithTelegramWebhook(telegramWebhook),
	)
	publicLookup := channelaction.NewGetPublicWebsiteChannelQuery(appStorage.DB()).Execute

	// 注册健康检查、业务与文件路由、公开聊天入口及后台服务生命周期。
	return []application.Service{
		application.NewServiceWithOptions(api.NewLiveness(), application.ServiceOptions{Route: "/healthz"}),
		application.NewServiceWithOptions(api.NewReadiness(appStorage.DB()), application.ServiceOptions{Route: "/readyz"}),
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
	}, nil
}

// newFileContentS3ConfigResolver 创建读取企业对象存储配置的解析器。
func newFileContentS3ConfigResolver(db *bun.DB) serverfilecontent.S3ConfigResolver {
	getS3Setting := settingaction.NewGetS3SettingQuery(db)
	return func(ctx context.Context, organizationID string) (serverfilecontent.S3Config, error) {
		setting, err := getS3Setting.ExecuteForOrganization(ctx, organizationID)
		if err != nil {
			return serverfilecontent.S3Config{}, err
		}
		return serverfilecontent.S3Config{
			Endpoint: setting.Endpoint, Region: setting.Region, Bucket: setting.Bucket,
			AccessKeyID: setting.AccessKeyID, SecretAccessKey: setting.SecretAccessKey,
			ForcePathStyle: setting.ForcePathStyle,
		}, nil
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
