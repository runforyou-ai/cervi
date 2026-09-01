//go:build server

package main

import (
	"context"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/api"
	"github.com/runforyou-ai/cervi/internal/appservice"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/ingress"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	telegramintegration "github.com/runforyou-ai/cervi/internal/integration/telegram"
	"github.com/runforyou-ai/cervi/internal/publicweb"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// applicationServices 创建企业服务端 HTTPS 入口、绑定服务、HTTP API 和网站渠道入口。
func applicationServices(appStorage *serverstorage.Store, config serverconfig.Config) ([]application.Service, error) {
	tenantResolver := serverstorage.NewTenantResolver(appStorage.DB())
	httpsEntry := ingress.NewHTTPSEntry(config.TLS, config.Server, serverstorage.NewACMECache(appStorage.DB()), tenantResolver)
	localFiles, err := serverfilecontent.NewLocalStore(config.Storage.LocalDirectory)
	if err != nil {
		return nil, err
	}
	resolveFileS3 := newFileContentS3ConfigResolver(appStorage.DB())
	tasks := servertask.New(appStorage.DB(), config.NATS)
	agentRuntime, err := agentruntime.New()
	if err != nil {
		return nil, err
	}
	agentRunScheduler := agentrunaction.NewScheduler(tasks)
	executeAgentRun := agentrunaction.NewExecuteAction(appStorage.DB(), tasks, agentRuntime)
	if err := tasks.Registry().RegisterJSONWithTerminalFailure(agentrunaction.RunActionName, executeAgentRun.Execute, executeAgentRun.FinalizeFailure); err != nil {
		return nil, err
	}
	scanExpired := fileaction.NewScanExpiredAction(appStorage.DB(), tasks)
	deleteExpired := fileaction.NewDeleteExpiredAction(appStorage.DB(), serverfilecontent.NewDeleter(localFiles, resolveFileS3))
	if err := tasks.Registry().RegisterJSON(fileaction.ScanExpiredActionName, scanExpired.Execute); err != nil {
		return nil, err
	}
	if err := tasks.Registry().RegisterJSON(fileaction.DeleteExpiredActionName, deleteExpired.Execute); err != nil {
		return nil, err
	}
	tasks.RegisterSchedule(servertask.ScheduleDefinition{
		Key: fileaction.CleanupScheduleKey, ActionName: fileaction.ScanExpiredActionName, Queue: "maintenance",
		Payload: fileaction.ScanExpiredInput{}, CronExpression: "@hourly", Timezone: "UTC",
		Enabled: true, MaxAttempts: 5, StartImmediately: true,
	})
	directBackend := appservice.NewDirectBackend(appStorage.DB(), localFiles, tenantResolver, agentRunScheduler, executeAgentRun)
	boundService := appservice.New(directBackend)
	websiteVisitorBackend := appservice.NewWebsiteVisitorDirectBackend(appStorage.DB(), agentRunScheduler)
	websiteVisitorService := appservice.NewWebsiteVisitorService(websiteVisitorBackend)
	telegramAPI := telegramintegration.NewClient(connectiontest.NewHTTPClient())
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
	telegramWebhook := channelaction.NewReceiveTelegramWebhookAction(appStorage.DB(), telegramAPI, telegramAvatarFiles)
	httpAPI := api.NewService(
		boundService,
		api.WithWebsiteVisitor(websiteVisitorService, config.TLS.Mode != "off"),
		api.WithTelegramWebhook(telegramWebhook),
	)
	publicLookup := channelaction.NewGetPublicWebsiteChannelQuery(appStorage.DB()).Execute

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
