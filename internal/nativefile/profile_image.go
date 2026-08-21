//go:build !server

// Package nativefile 提供原生端文件选择和读取能力。
package nativefile

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const maxProfileImageByteSize int64 = 5 * 1024 * 1024

var profileImageContentTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

// ProfileImageSelector 使用 Wails 原生对话框选择头像文件。
type ProfileImageSelector struct{}

// NewProfileImageSelector 创建原生头像文件选择器。
func NewProfileImageSelector() *ProfileImageSelector {
	return &ProfileImageSelector{}
}

// SelectProfileImage 选择并读取用户头像图片。
func (s *ProfileImageSelector) SelectProfileImage(ctx context.Context, meta appservice.RequestMeta) (appservice.ProfileImageFile, error) {
	if err := ctx.Err(); err != nil {
		return appservice.ProfileImageFile{}, err
	}
	app := application.Get()
	if app == nil {
		return appservice.ProfileImageFile{}, errors.New("application is not initialized")
	}

	title, buttonText := "选择头像图片", "选择"
	if meta.Locale == appservice.LocaleEnglishUnitedStates {
		title, buttonText = "Choose profile image", "Choose"
	}
	path, err := app.Dialog.OpenFile().
		CanChooseFiles(true).
		CanChooseDirectories(false).
		AllowsOtherFileTypes(false).
		TreatsFilePackagesAsDirectories(false).
		AddFilter(title, "*.jpg;*.jpeg;*.png;*.webp").
		SetTitle(title).
		SetButtonText(buttonText).
		PromptForSingleSelection()
	if err != nil {
		slog.Warn("打开头像文件选择器失败", "error", err)
		return appservice.ProfileImageFile{}, fmt.Errorf("select profile image: %w", err)
	}
	if path == "" {
		return appservice.ProfileImageFile{}, nil
	}

	selected, err := readProfileImageFile(path)
	if err != nil {
		slog.Warn("读取头像文件失败", "file_name", filepath.Base(path), "error", err)
		return appservice.ProfileImageFile{}, err
	}
	slog.Info("已选择头像文件", "file_name", selected.Name, "byte_size", selected.ByteSize)
	return selected, nil
}

// readProfileImageFile 校验并读取头像图片。
func readProfileImageFile(path string) (appservice.ProfileImageFile, error) {
	contentType, allowed := profileImageContentTypes[strings.ToLower(filepath.Ext(path))]
	if !allowed {
		return appservice.ProfileImageFile{}, errors.New("unsupported profile image type")
	}
	info, err := os.Stat(path)
	if err != nil {
		return appservice.ProfileImageFile{}, fmt.Errorf("stat profile image: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxProfileImageByteSize {
		return appservice.ProfileImageFile{}, errors.New("invalid profile image size")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return appservice.ProfileImageFile{}, fmt.Errorf("read profile image: %w", err)
	}
	if http.DetectContentType(content) != contentType {
		return appservice.ProfileImageFile{}, errors.New("profile image content does not match its extension")
	}
	return appservice.ProfileImageFile{
		Name:        filepath.Base(path),
		ContentType: contentType,
		ByteSize:    info.Size(),
		DataBase64:  base64.StdEncoding.EncodeToString(content),
	}, nil
}
