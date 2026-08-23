//go:build !server

package main

import (
	"fmt"

	"github.com/runforyou-ai/cervi/internal/apiproxy"
	"github.com/runforyou-ai/cervi/internal/appservice"
	appservicenative "github.com/runforyou-ai/cervi/internal/appservice/native"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// applicationServices 创建原生端使用的远程应用服务。
func applicationServices(appStorage apiproxy.Store) ([]application.Service, error) {
	backend, err := apiproxy.NewBackend(appStorage)
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
