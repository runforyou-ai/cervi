//go:build !server

package main

import (
	"context"
	"fmt"

	"github.com/runforyou-ai/cervi/internal/apiproxy"
	"github.com/runforyou-ai/cervi/internal/appservice"
	appservicenative "github.com/runforyou-ai/cervi/internal/appservice/native"
	"github.com/runforyou-ai/cervi/internal/clientsession"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// nativeStorage 组合原生端连接和登录凭据存储能力。
type nativeStorage interface {
	apiproxy.Store
	clientsession.Store
}

// applicationServices 创建原生端使用的远程应用服务。
func applicationServices(appStorage nativeStorage) ([]application.Service, error) {
	sessions, err := clientsession.NewManager(context.Background(), appStorage)
	if err != nil {
		return nil, fmt.Errorf("initialize client session: %w", err)
	}
	backend, err := apiproxy.NewBackend(appStorage, sessions)
	if err != nil {
		return nil, fmt.Errorf("initialize remote application backend: %w", err)
	}
	service := appservice.New(backend, appservice.WithProfileImageSelector(appservicenative.NewProfileImageSelector()))
	return []application.Service{
		application.NewServiceWithOptions(service, application.ServiceOptions{
			MarshalError: appservice.MarshalError,
		}),
	}, nil
}
