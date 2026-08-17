//go:build server

package main

import (
	"fmt"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
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
	listWebsiteChannels := channelaction.NewListWebsiteChannelsQuery(db)
	getWebsiteChannel := channelaction.NewGetWebsiteChannelQuery(db)
	createWebsiteChannel := channelaction.NewCreateWebsiteChannelAction(db)
	updateWebsiteChannel := channelaction.NewUpdateWebsiteChannelAction(db)
	updateWebsiteChannelChatInterface := channelaction.NewUpdateWebsiteChannelChatInterfaceAction(db)
	deleteWebsiteChannel := channelaction.NewDeleteWebsiteChannelAction(db)
	restoreWebsiteChannel := channelaction.NewRestoreWebsiteChannelAction(db)

	apiService := api.NewService(api.Dependencies{
		InstallWorkspace:                  installWorkspace.Execute,
		Login:                             login.Execute,
		Logout:                            logout.Execute,
		ResolveSession:                    resolveSession.Execute,
		Installation:                      installation.Execute,
		LoadInbox:                         loadInbox.Execute,
		ListWebsiteChannels:               listWebsiteChannels.Execute,
		GetWebsiteChannel:                 getWebsiteChannel.Execute,
		CreateWebsiteChannel:              createWebsiteChannel.Execute,
		UpdateWebsiteChannel:              updateWebsiteChannel.Execute,
		UpdateWebsiteChannelChatInterface: updateWebsiteChannelChatInterface.Execute,
		DeleteWebsiteChannel:              deleteWebsiteChannel.Execute,
		RestoreWebsiteChannel:             restoreWebsiteChannel.Execute,
	})
	return []application.Service{
		application.NewServiceWithOptions(apiService, application.ServiceOptions{
			Route: "/api",
		}),
	}, nil
}
