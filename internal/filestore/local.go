//go:build server

// Package filestore 实现本地目录和 S3 兼容对象存储的文件内容访问。
package filestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const defaultLocalDirectory = "data/files"

// LocalStore 管理服务器本地文件目录。
type LocalStore struct {
	root string
}

// NewLocalStoreFromEnv 创建环境变量配置的本地文件存储。
func NewLocalStoreFromEnv() (*LocalStore, error) {
	root := strings.TrimSpace(os.Getenv("FILE_STORAGE_PATH"))
	if root == "" {
		root = defaultLocalDirectory
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve local file storage path: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, fmt.Errorf("create local file storage directory: %w", err)
	}
	slog.Info("本地文件存储已初始化", "path", absolute)
	return &LocalStore{root: absolute}, nil
}

// Save 将内容流式写入本地文件并原子替换目标。
func (s *LocalStore) Save(ctx context.Context, key string, source io.Reader, expectedSize int64) error {
	target, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("create local file directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return fmt.Errorf("create local upload file: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryName)
	}()

	written, err := io.Copy(temporary, io.LimitReader(source, expectedSize+1))
	if err != nil {
		return fmt.Errorf("write local upload file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if written != expectedSize {
		return fmt.Errorf("local upload size = %d, want %d", written, expectedSize)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync local upload file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close local upload file: %w", err)
	}
	if err := os.Rename(temporaryName, target); err != nil {
		return fmt.Errorf("commit local upload file: %w", err)
	}
	return nil
}

// Open 打开本地文件及其状态。
func (s *LocalStore) Open(key string) (*os.File, os.FileInfo, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return file, info, nil
}

// Stat 返回本地文件状态。
func (s *LocalStore) Stat(key string) (os.FileInfo, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Stat(path)
}

// Delete 删除本地文件，文件不存在时直接成功。
func (s *LocalStore) Delete(key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete local file: %w", err)
	}
	return nil
}

// path 将受控存储键转换为本地绝对路径。
func (s *LocalStore) path(key string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(strings.TrimSpace(key)))
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", errors.New("invalid local file storage key")
	}
	return filepath.Join(s.root, cleaned), nil
}
