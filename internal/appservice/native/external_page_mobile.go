//go:build !server && (android || ios)

package native

import "github.com/runforyou-ai/cervi/internal/appservice"

// NewExternalPageOpener 禁用移动端外部页面窗口能力。
func NewExternalPageOpener() appservice.ExternalPageOpener {
	return nil
}
