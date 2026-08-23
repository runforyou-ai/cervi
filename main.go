package main

import (
	"embed"
	"log/slog"
	"os"
)

//go:embed all:frontend/dist
var assets embed.FS

var version = "dev"

// main 启动应用并记录无法恢复的运行错误。
func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("Cervi 运行失败", "error", err)
		os.Exit(1)
	}
}
