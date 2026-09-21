//go:build !server && !ios && !android

package native

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// SelectWorkspaceDirectory 使用 Wails 原生对话框选择工作区目录，用户取消时返回空路径。
func SelectWorkspaceDirectory(_ context.Context, meta appservice.RequestMeta) (string, error) {
	app := application.Get()
	if app == nil {
		return "", errors.New("application is not initialized")
	}
	messages := cervii18n.LocalizeMap(string(meta.Locale), map[string]cervii18n.Key{
		"title":  cervii18n.DialogWorkspaceTitle,
		"choose": cervii18n.DialogWorkspaceChoose,
	})
	path, err := app.Dialog.OpenFile().
		CanChooseFiles(false).
		CanChooseDirectories(true).
		CanCreateDirectories(true).
		TreatsFilePackagesAsDirectories(false).
		SetTitle(messages["title"]).
		SetButtonText(messages["choose"]).
		PromptForSingleSelection()
	if err != nil {
		slog.Warn("打开工作区目录选择器失败", "error", err)
		return "", fmt.Errorf("select workspace directory: %w", err)
	}
	return path, nil
}
