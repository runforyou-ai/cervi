//go:build server

package main

import (
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	"github.com/runforyou-ai/cervi/internal/api"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/publicweb"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// applicationServices 创建企业服务端绑定服务、HTTP API 和网站渠道公开入口。
func applicationServices(appStorage *serverstorage.Store) ([]application.Service, error) {
	directBackend := appservice.NewDirectBackend(appStorage.DB())
	boundService := appservice.New(directBackend)
	httpAPI := api.NewService(boundService)
	publicLookup := channelaction.NewGetPublicWebsiteChannelQuery(appStorage.DB()).Execute

	return []application.Service{
		application.NewServiceWithOptions(boundService, application.ServiceOptions{
			MarshalError: appservice.MarshalError,
		}),
		application.NewServiceWithOptions(httpAPI, application.ServiceOptions{
			Route: "/api",
		}),
		application.NewServiceWithOptions(publicweb.NewEmbedService(publicLookup), application.ServiceOptions{
			Route: "/embed",
		}),
		application.NewServiceWithOptions(publicweb.NewChatService(publicLookup), application.ServiceOptions{
			Route: "/chat/",
		}),
	}, nil
}
