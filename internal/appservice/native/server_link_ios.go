//go:build !server && ios

package native

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#include "server_link_ios.h"
*/
import "C"

import "github.com/wailsapp/wails/v3/pkg/application"

// iosServerLinks 是接收唤起链接的唯一连接链接能力实例。
var iosServerLinks = &openedServerLink{}

// NewServerLinks 创建 iOS 连接链接能力，并开始接收唤起链接；应用被链接唤起时暂存的链接随即转来。
func NewServerLinks() (ServerLinks, []application.Service) {
	C.app_server_link_listen()
	return iosServerLinks, nil
}

// appServerLinkOpened 接收原生层转来的唤起链接。
//
//export appServerLinkOpened
func appServerLinkOpened(link *C.char) {
	iosServerLinks.Open(C.GoString(link))
}
