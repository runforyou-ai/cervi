//go:build server

package main

import (
	"github.com/runforyou-ai/cervi/internal/api"
	"github.com/runforyou-ai/cervi/internal/appservice"
	serverstorage "github.com/runforyou-ai/cervi/internal/storage/server"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// applicationServices 创建企业服务端的绑定服务和对外 HTTP API。
func applicationServices(appStorage *serverstorage.Store) ([]application.Service, error) {
	directBackend := appservice.NewDirectBackend(appStorage.DB())
	boundService := appservice.New(directBackend)
	httpAPI := api.NewService(boundService)

	return []application.Service{
		application.NewServiceWithOptions(boundService, application.ServiceOptions{
			MarshalError: appservice.MarshalError,
		}),
		application.NewServiceWithOptions(httpAPI, application.ServiceOptions{
			Route: "/api",
		}),
	}, nil
}
