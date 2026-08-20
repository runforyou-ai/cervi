//go:build server

package publicweb

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
)

var themeColorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

const (
	paintLight = "#FFFFFF"
	paintDark  = "#1C1917"
)

// Theme 是访客页使用的主题色。
type Theme struct {
	Color          string
	OnColor        string
	Focus          string
	LauncherShadow string
}

// DefaultTheme 返回默认蓝主题。
func DefaultTheme() Theme {
	return ParseTheme(channelaction.DefaultWebsiteChannelThemeColor)
}

// ParseTheme 把十六进制主题色算成访客页颜色，非法值回退默认蓝。
func ParseTheme(value string) Theme {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if !themeColorPattern.MatchString(normalized) {
		normalized = channelaction.DefaultWebsiteChannelThemeColor
	}
	r, g, b := hexRGB(normalized)
	themeL := relativeLuminance(r, g, b)
	whiteC := contrastRatio(themeL, relativeLuminance(255, 255, 255))
	darkC := contrastRatio(themeL, relativeLuminance(0x1c, 0x19, 0x17))
	onColor := paintLight
	if darkC > whiteC {
		onColor = paintDark
	}
	focus := fmt.Sprintf("rgba(%d, %d, %d, 0.40)", r, g, b)
	shadow := fmt.Sprintf("0 10px 28px rgba(%d, %d, %d, 0.42)", r, g, b)
	if whiteC < 3 {
		focus = "rgba(28, 25, 23, 0.35)"
		shadow = "0 8px 24px rgba(28, 25, 23, 0.12)"
	}
	return Theme{
		Color:          normalized,
		OnColor:        onColor,
		Focus:          focus,
		LauncherShadow: shadow,
	}
}

// RootCSS 返回页面 :root 变量。
func (t Theme) RootCSS() string {
	page := fmt.Sprintf(
		"linear-gradient(160deg, color-mix(in srgb, %s 20%%, #ffffff), color-mix(in srgb, %s 7%%, #ffffff) 45%%, #ffffff 100%%)",
		t.Color,
		t.Color,
	)
	return fmt.Sprintf(
		":root{--cv-theme:%s;--cv-on-theme:%s;--cv-focus:%s;--cv-assistant-bubble:#ffffff;--cv-assistant-text:#111827;--cv-page:%s;color-scheme:light}html,body{background:var(--cv-page)}",
		t.Color,
		t.OnColor,
		t.Focus,
		page,
	)
}

// HostCSS 返回挂件 Shadow DOM 的 :host 变量。
func (t Theme) HostCSS() string {
	return fmt.Sprintf(
		":host{--cv-theme:%s;--cv-on-theme:%s;--cv-focus:%s;--cv-launcher-shadow:%s}",
		t.Color,
		t.OnColor,
		t.Focus,
		t.LauncherShadow,
	)
}

// hexRGB 解析已校验的六位十六进制颜色。
func hexRGB(hex string) (int, int, int) {
	value, _ := strconv.ParseUint(hex[1:], 16, 32)
	color := int(value)
	return (color >> 16) & 0xFF, (color >> 8) & 0xFF, color & 0xFF
}

// relativeLuminance 计算 sRGB 相对亮度。
func relativeLuminance(r, g, b int) float64 {
	return 0.2126*linearChannel(r) + 0.7152*linearChannel(g) + 0.0722*linearChannel(b)
}

// linearChannel 把 sRGB 通道转成线性光。
func linearChannel(value int) float64 {
	channel := float64(value) / 255
	if channel <= 0.04045 {
		return channel / 12.92
	}
	return math.Pow((channel+0.055)/1.055, 2.4)
}

// contrastRatio 计算两色对比度。
func contrastRatio(l1, l2 float64) float64 {
	lighter := math.Max(l1, l2)
	darker := math.Min(l1, l2)
	return (lighter + 0.05) / (darker + 0.05)
}
