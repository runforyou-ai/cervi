//go:build !server && windows

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/dock"
)

const windowsTrayIconSize = 32

var windowsBadgeGlyphs = map[rune][5]uint8{
	'0': {0b111, 0b101, 0b101, 0b101, 0b111},
	'1': {0b010, 0b110, 0b010, 0b010, 0b111},
	'2': {0b111, 0b001, 0b111, 0b100, 0b111},
	'3': {0b111, 0b001, 0b111, 0b001, 0b111},
	'4': {0b101, 0b101, 0b111, 0b001, 0b001},
	'5': {0b111, 0b100, 0b111, 0b001, 0b111},
	'6': {0b111, 0b100, 0b111, 0b101, 0b111},
	'7': {0b111, 0b001, 0b010, 0b010, 0b010},
	'8': {0b111, 0b101, 0b111, 0b101, 0b111},
	'9': {0b111, 0b101, 0b111, 0b001, 0b111},
	'+': {0b000, 0b010, 0b111, 0b010, 0b000},
}

// windowsUnreadIndicator 同步更新 Windows 通知区域图标和任务栏角标。
type windowsUnreadIndicator struct {
	tray     *application.SystemTray
	dock     *dock.DockService
	baseIcon []byte

	mu    sync.Mutex
	icons map[string][]byte
}

// newPlatformUnreadIndicator 创建 Windows 未读消息指示器并注册任务栏 Dock 服务。
func newPlatformUnreadIndicator(app *application.App, tray *application.SystemTray, baseIcon []byte) appservice.UnreadIndicator {
	dockService := dock.New()
	app.RegisterService(application.NewService(dockService))
	return &windowsUnreadIndicator{
		tray: tray, dock: dockService, baseIcon: baseIcon,
		icons: make(map[string][]byte),
	}
}

// SetUnreadCount 设置通知区域图标和任务栏角标中的绝对未读消息数。
func (i *windowsUnreadIndicator) SetUnreadCount(count int) error {
	label := appservice.FormatUnreadCount(count)

	i.mu.Lock()
	defer i.mu.Unlock()

	if label == "" {
		i.tray.SetIcon(i.baseIcon)
		if err := i.dock.RemoveBadge(); err != nil {
			return err
		}
	} else {
		icon, ok := i.icons[label]
		if !ok {
			var err error
			icon, err = renderWindowsUnreadTrayIcon(i.baseIcon, label)
			if err != nil {
				return err
			}
			i.icons[label] = icon
		}
		i.tray.SetIcon(icon)
		if err := i.dock.SetBadge(label); err != nil {
			return err
		}
	}

	return nil
}

// renderWindowsUnreadTrayIcon 生成带未读消息数字角标的 Windows 托盘 PNG。
func renderWindowsUnreadTrayIcon(baseIcon []byte, label string) ([]byte, error) {
	source, _, err := image.Decode(bytes.NewReader(baseIcon))
	if err != nil {
		return nil, fmt.Errorf("decode tray icon: %w", err)
	}

	icon := image.NewNRGBA(image.Rect(0, 0, windowsTrayIconSize, windowsTrayIconSize))
	resizeNearest(icon, source)
	drawWindowsUnreadBadge(icon, label)

	var result bytes.Buffer
	if err := png.Encode(&result, icon); err != nil {
		return nil, fmt.Errorf("encode unread tray icon: %w", err)
	}
	return result.Bytes(), nil
}

// resizeNearest 使用最近邻采样将应用图标缩放到 Windows 托盘尺寸。
func resizeNearest(destination *image.NRGBA, source image.Image) {
	sourceBounds := source.Bounds()
	destinationBounds := destination.Bounds()
	for y := destinationBounds.Min.Y; y < destinationBounds.Max.Y; y++ {
		for x := destinationBounds.Min.X; x < destinationBounds.Max.X; x++ {
			sourceX := sourceBounds.Min.X + (x-destinationBounds.Min.X)*sourceBounds.Dx()/destinationBounds.Dx()
			sourceY := sourceBounds.Min.Y + (y-destinationBounds.Min.Y)*sourceBounds.Dy()/destinationBounds.Dy()
			destination.Set(x, y, source.At(sourceX, sourceY))
		}
	}
}

// drawWindowsUnreadBadge 在托盘图标右下角绘制红色数字角标。
func drawWindowsUnreadBadge(icon *image.NRGBA, label string) {
	const centerX, centerY, radius = 22, 22, 10
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	red := color.NRGBA{R: 220, G: 38, B: 38, A: 255}
	for y := centerY - radius; y < windowsTrayIconSize; y++ {
		for x := centerX - radius; x < windowsTrayIconSize; x++ {
			distance := (x-centerX)*(x-centerX) + (y-centerY)*(y-centerY)
			if distance <= (radius+1)*(radius+1) {
				icon.SetNRGBA(x, y, white)
			}
			if distance <= (radius-1)*(radius-1) {
				icon.SetNRGBA(x, y, red)
			}
		}
	}
	drawWindowsBadgeLabel(icon, label, white, centerX, centerY)
}

// drawWindowsBadgeLabel 使用内置像素字形绘制不依赖系统字体的角标文本。
func drawWindowsBadgeLabel(icon *image.NRGBA, label string, textColor color.NRGBA, centerX, centerY int) {
	scale := 2
	if len(label) > 2 {
		scale = 1
	}
	textWidth := (len(label)*3 + len(label) - 1) * scale
	startX := centerX - textWidth/2
	startY := centerY - 5*scale/2

	for index, character := range label {
		glyph, ok := windowsBadgeGlyphs[character]
		if !ok {
			continue
		}
		glyphX := startX + index*4*scale
		for row, bits := range glyph {
			for column := 0; column < 3; column++ {
				if bits&(1<<uint(2-column)) == 0 {
					continue
				}
				for pixelY := 0; pixelY < scale; pixelY++ {
					for pixelX := 0; pixelX < scale; pixelX++ {
						icon.SetNRGBA(glyphX+column*scale+pixelX, startY+row*scale+pixelY, textColor)
					}
				}
			}
		}
	}
}
