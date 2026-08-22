//go:build server

package main

import (
	"context"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/api"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/filecleanup"
	"github.com/runforyou-ai/cervi/internal/filecontent"
	"github.com/runforyou-ai/cervi/internal/filestore"
	"github.com/runforyou-ai/cervi/internal/publicweb"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// applicationServices 创建企业服务端 HTTPS 入口、绑定服务、HTTP API 和网站渠道入口。
func applicationServices(appStorage *serverstorage.Store) ([]application.Service, error) {
	httpsEntry, err := api.NewHTTPSEntry()
	if err != nil {
		return nil, err
	}
	localFiles, err := filestore.NewLocalStoreFromEnv()
	if err != nil {
		return nil, err
	}
	directBackend := appservice.NewDirectBackend(appStorage.DB(), localFiles)
	boundService := appservice.New(directBackend)
	httpAPI := api.NewService(boundService)
	publicLookup := channelaction.NewGetPublicWebsiteChannelQuery(appStorage.DB()).Execute
	cleanupLifecycle := &fileCleanupLifecycle{service: filecleanup.NewService(appStorage.DB(), localFiles)}

	return []application.Service{
		application.NewService(&httpsLifecycle{service: httpsEntry}),
		application.NewServiceWithOptions(boundService, application.ServiceOptions{
			MarshalError: appservice.MarshalError,
		}),
		application.NewServiceWithOptions(httpAPI, application.ServiceOptions{
			Route: "/api",
		}),
		application.NewServiceWithOptions(filecontent.NewService(appStorage.DB(), localFiles), application.ServiceOptions{
			Route: "/files/",
		}),
		application.NewService(cleanupLifecycle),
		application.NewServiceWithOptions(publicweb.NewEmbedService(publicLookup), application.ServiceOptions{
			Route: "/embed",
		}),
		application.NewServiceWithOptions(publicweb.NewChatService(publicLookup), application.ServiceOptions{
			Route: "/chat/",
		}),
	}, nil
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

// fileCleanupLifecycle 将文件清理循环接入 Wails 服务生命周期。
type fileCleanupLifecycle struct {
	service *filecleanup.Service
}

// ServiceStartup 在企业服务端启动后运行文件清理循环。
func (l *fileCleanupLifecycle) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	l.service.Start(ctx)
	return nil
}
