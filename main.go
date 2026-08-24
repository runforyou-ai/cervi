package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

//go:embed all:frontend/dist
var assets embed.FS

// main 启动应用并记录无法恢复的运行错误。
func main() {
	if err := validateFrontendPlatform(); err != nil {
		slog.Error("Cervi 前端平台校验失败", "error", err)
		os.Exit(1)
	}
	if err := run(os.Args[1:]); err != nil {
		slog.Error("Cervi 运行失败", "error", err)
		os.Exit(1)
	}
}

type frontendPlatformManifest struct {
	Platform string `json:"platform"`
}

// validateFrontendPlatform 校验嵌入的前端产物与当前 Go 构建目标一致。
func validateFrontendPlatform() error {
	content, err := assets.ReadFile("frontend/dist/cervi-platform.json")
	if err != nil {
		return fmt.Errorf("读取前端平台清单: %w", err)
	}
	var manifest frontendPlatformManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		return fmt.Errorf("解析前端平台清单: %w", err)
	}
	if manifest.Platform != expectedFrontendPlatform {
		return fmt.Errorf("前端平台为 %q，Go 构建目标需要 %q", manifest.Platform, expectedFrontendPlatform)
	}
	return nil
}
