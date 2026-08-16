//go:build server

package main

import (
	"fmt"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	"github.com/runforyou-ai/cervi/internal/api"
	"github.com/runforyou-ai/cervi/internal/storage"
	"github.com/uptrace/bun"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// serverDatabase 提供服务端 Action 使用的 Bun 数据库连接。
type serverDatabase interface {
	DB() *bun.DB
}

// applicationServices 创建企业服务端 API 服务。
func applicationServices(appStorage storage.Storage) ([]application.Service, error) {
	serverStorage, ok := appStorage.(serverDatabase)
	if !ok {
		return nil, fmt.Errorf("server storage does not provide Bun database")
	}

	db := serverStorage.DB()
	installWorkspace := installationaction.NewInstallWorkspaceAction(db)
	login := authaction.NewLoginAction(db)
	logout := authaction.NewLogoutAction(db)
	resolveSession := authaction.NewResolveSessionQuery(db)
	installation := installationaction.NewStatusQuery(db)
	loadInbox := inboxaction.NewLoadInboxQuery()

	apiService := api.NewService(api.Dependencies{
		InstallWorkspace: installWorkspace.Execute,
		Login:            login.Execute,
		Logout:           logout.Execute,
		ResolveSession:   resolveSession.Execute,
		Installation:     installation.Execute,
		LoadInbox:        loadInbox.Execute,
	})
	return []application.Service{
		application.NewServiceWithOptions(apiService, application.ServiceOptions{
			Route: "/api",
		}),
	}, nil
}
