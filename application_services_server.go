//go:build server

package main

import (
	"context"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/api"
	"github.com/runforyou-ai/cervi/internal/appservice"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/publicweb"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	"github.com/runforyou-ai/cervi/internal/taskruntime"
	servertask "github.com/runforyou-ai/cervi/internal/taskruntime/server"
	"github.com/uptrace/bun"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// applicationServices 创建服务端应用服务。
func applicationServices(appStorage *serverstorage.Store, config serverconfig.Config, taskConfig servertask.Config) ([]application.Service, error) {
	httpsEntry := api.NewHTTPSEntry(config.HTTPS, config.Server)
	localFiles, err := serverfilecontent.NewLocalStore(config.Storage.LocalDirectory)
	if err != nil {
		return nil, err
	}
	tasks := servertask.New(appStorage.DB(), taskConfig)
	scanExpired := fileaction.NewScanExpiredAction(appStorage.DB(), tasks)
	deleteExpired := fileaction.NewDeleteExpiredAction(appStorage.DB(), newFileContentDeleter(appStorage.DB(), localFiles))
	if err := servertask.RegisterJSON(tasks.Registry(), fileaction.ScanExpiredActionName, scanExpired.Execute); err != nil {
		return nil, err
	}
	if err := servertask.RegisterJSON(tasks.Registry(), fileaction.DeleteExpiredActionName, deleteExpired.Execute); err != nil {
		return nil, err
	}
	tasks.RegisterSchedule(taskruntime.ScheduleDefinition{
		Key: fileaction.CleanupScheduleKey, ActionName: fileaction.ScanExpiredActionName, Queue: "maintenance",
		Payload: fileaction.ScanExpiredInput{}, CronExpression: "@hourly", Timezone: "UTC",
		Enabled: true, MaxAttempts: 5, StartImmediately: true,
	})
	directBackend := appservice.NewDirectBackend(appStorage.DB(), localFiles)
	boundService := appservice.New(directBackend)
	httpAPI := api.NewService(boundService)
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
		application.NewServiceWithOptions(api.NewFileContentService(appStorage.DB(), localFiles), application.ServiceOptions{
			Route: "/files/",
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

// newFileContentDeleter 创建读取企业配置的文件内容删除器。
func newFileContentDeleter(db *bun.DB, local *serverfilecontent.LocalStore) *serverfilecontent.Deleter {
	getS3Setting := settingaction.NewGetS3SettingQuery(db)
	return serverfilecontent.NewDeleter(local, func(ctx context.Context, organizationID string) (serverfilecontent.S3Config, error) {
		setting, err := getS3Setting.ExecuteForOrganization(ctx, organizationID)
		if err != nil {
			return serverfilecontent.S3Config{}, err
		}
		return serverfilecontent.S3Config{
			Endpoint: setting.Endpoint, Region: setting.Region, Bucket: setting.Bucket,
			AccessKeyID: setting.AccessKeyID, SecretAccessKey: setting.SecretAccessKey,
			ForcePathStyle: setting.ForcePathStyle,
		}, nil
	})
}

// httpsLifecycle 将 HTTPS 入口接入 Wails 服务生命周期。
type httpsLifecycle struct {
	service *api.HTTPSEntry
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
