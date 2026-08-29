//go:build android

package appservice

import "encoding/base64"

const androidErrorMarker = "\n__CERVI_API_ERROR_V1__:"

// Error 在 Android 错误文本中附加结构化载荷，兼容 wailsapp/wails#6053。
func (e *Error) Error() string {
	message := e.displayMessage()
	payload := MarshalError(e)
	if payload == nil {
		return message
	}
	return message + androidErrorMarker + base64.StdEncoding.EncodeToString(payload)
}
