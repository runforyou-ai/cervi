//go:build !server && !android && !ios

package systemtray

import (
	"runtime"

	"github.com/runforyou-ai/cervi/internal/appservice"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// localizedApplicationMenu 保存 macOS 原生应用菜单及其可本地化菜单项。
type localizedApplicationMenu struct {
	menu      *application.Menu
	roleItems map[application.Role]*application.MenuItem
	learnMore *application.MenuItem
}

var applicationMenuMessageKeys = map[application.Role]cervii18n.Key{
	application.AppMenu:            cervii18n.AppProductName,
	application.FileMenu:           cervii18n.AppMenuFile,
	application.EditMenu:           cervii18n.AppMenuEdit,
	application.ViewMenu:           cervii18n.AppMenuView,
	application.WindowMenu:         cervii18n.AppMenuWindow,
	application.HelpMenu:           cervii18n.AppMenuHelp,
	application.About:              cervii18n.AppMenuAbout,
	application.ServicesMenu:       cervii18n.AppMenuServices,
	application.Hide:               cervii18n.AppMenuHide,
	application.HideOthers:         cervii18n.AppMenuHideOthers,
	application.UnHide:             cervii18n.AppMenuShowAll,
	application.Quit:               cervii18n.AppMenuQuit,
	application.CloseWindow:        cervii18n.AppMenuClose,
	application.Undo:               cervii18n.AppMenuUndo,
	application.Redo:               cervii18n.AppMenuRedo,
	application.Cut:                cervii18n.AppMenuCut,
	application.Copy:               cervii18n.AppMenuCopy,
	application.Paste:              cervii18n.AppMenuPaste,
	application.PasteAndMatchStyle: cervii18n.AppMenuPasteAndMatchStyle,
	application.Delete:             cervii18n.AppMenuDelete,
	application.SelectAll:          cervii18n.AppMenuSelectAll,
	application.SpeechMenu:         cervii18n.AppMenuSpeech,
	application.StartSpeaking:      cervii18n.AppMenuStartSpeaking,
	application.StopSpeaking:       cervii18n.AppMenuStopSpeaking,
	application.Reload:             cervii18n.AppMenuReload,
	application.ForceReload:        cervii18n.AppMenuForceReload,
	application.OpenDevTools:       cervii18n.AppMenuOpenDevTools,
	application.ResetZoom:          cervii18n.AppMenuActualSize,
	application.ZoomIn:             cervii18n.AppMenuZoomIn,
	application.ZoomOut:            cervii18n.AppMenuZoomOut,
	application.ToggleFullscreen:   cervii18n.AppMenuToggleFullscreen,
	application.Minimise:           cervii18n.AppMenuMinimize,
	application.Zoom:               cervii18n.AppMenuZoom,
	application.Front:              cervii18n.AppMenuBringAllToFront,
}

// newLocalizedApplicationMenu 按指定语言创建保留原生角色和快捷键的 macOS 应用菜单。
func newLocalizedApplicationMenu(app *application.App, locale appservice.Locale) *localizedApplicationMenu {
	if runtime.GOOS != "darwin" {
		return nil
	}

	menu := application.DefaultApplicationMenu()
	controller := &localizedApplicationMenu{
		menu:      menu,
		roleItems: make(map[application.Role]*application.MenuItem, len(applicationMenuMessageKeys)),
		learnMore: menu.FindByLabel("Learn More"),
	}
	for role := range applicationMenuMessageKeys {
		controller.roleItems[role] = menu.FindByRole(role)
	}
	controller.applyLocale(locale)
	app.Menu.Set(menu)
	return controller
}

// setLocale 更新 macOS 原生应用菜单并立即刷新屏幕顶部菜单栏。
func (m *localizedApplicationMenu) setLocale(locale appservice.Locale) {
	m.applyLocale(locale)
	m.menu.Update()
}

// applyLocale 更新应用菜单模型中的全部本地化标签。
func (m *localizedApplicationMenu) applyLocale(locale appservice.Locale) {
	labels := cervii18n.LocalizeMap(string(locale), applicationMenuMessageKeys)
	for role, item := range m.roleItems {
		item.SetLabel(labels[role])
	}
	learnMore, _ := cervii18n.Localize(string(locale), cervii18n.AppMenuLearnMore)
	m.learnMore.SetLabel(learnMore)
}
