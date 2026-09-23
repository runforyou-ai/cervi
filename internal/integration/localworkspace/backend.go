// Package localworkspace 提供本机文件的只读访问，相对路径以会话默认文件夹为起点，模型看到的是本机绝对路径。
package localworkspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk/filesystem"
)

const (
	// maxReadBytes 是按文本读取单个文件的字节上限。
	maxReadBytes = 10 << 20
	// maxImageBytes 是以图片读取单个文件的字节上限。
	maxImageBytes = 10 << 20
	// binarySniffBytes 是判断二进制文件时检查的文件开头字节数。
	binarySniffBytes = 8000
)

// imageTypes 是可以按图片读取的内容类型。
var imageTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}

// errReadOnly 表示文件只读，不接受写入。
var errReadOnly = errors.New("文件只读，不能写入或修改文件")

// Backend 读取本机文件：绝对路径直接访问，~ 开头按用户主目录展开，相对路径以默认文件夹为起点。
type Backend struct {
	root string
}

// New 以默认文件夹为相对路径起点打开本机文件访问，目录不存在时创建。
func New(dir string) (*Backend, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create default folder: %w", err)
	}
	return &Backend{root: filepath.Clean(dir)}, nil
}

// resolve 把模型给出的路径解析为本机绝对路径。
func (b *Backend) resolve(name string) (string, error) {
	switch {
	case name == "~" || strings.HasPrefix(name, "~/") || strings.HasPrefix(name, `~\`):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("无法确定用户主目录")
		}
		name = filepath.Join(home, name[1:])
	case !filepath.IsAbs(name):
		name = filepath.Join(b.root, name)
	}
	return filepath.Clean(name), nil
}

// LsInfo 列出目录的直接子项，目录路径以 / 结尾；路径指向文件时返回该文件。
func (b *Backend) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	dir, err := b.resolve(req.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("无法访问：%s", dir)
	}
	if !info.IsDir() {
		return []filesystem.FileInfo{fileInfo(dir, info)}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("无法读取目录：%s", dir)
	}
	infos := make([]filesystem.FileInfo, 0, len(entries))
	for _, entry := range entries {
		// 符号链接按目标展示，目标不存在时只展示条目本身。
		name := filepath.Join(dir, entry.Name())
		info, err := os.Stat(name)
		if err != nil {
			if info, err = entry.Info(); err != nil {
				continue
			}
		}
		infos = append(infos, fileInfo(name, info))
	}
	return infos, nil
}

// fileInfo 转换文件信息，目录路径以路径分隔符结尾。
func fileInfo(name string, info os.FileInfo) filesystem.FileInfo {
	result := filesystem.FileInfo{Path: name, IsDir: info.IsDir(), Size: info.Size(), ModifiedAt: info.ModTime().UTC().Format(time.RFC3339)}
	if info.IsDir() && !strings.HasSuffix(name, string(filepath.Separator)) {
		result.Path += string(filepath.Separator)
	}
	return result
}

// Read 按行读取文本文件，Offset 从 1 开始，Limit 为 0 时读到文件末尾；二进制文件与超出上限的文件返回错误。
func (b *Backend) Read(ctx context.Context, req *filesystem.ReadRequest) (*filesystem.FileContent, error) {
	file, err := b.resolve(req.FilePath)
	if err != nil {
		return nil, err
	}
	content, err := b.readFile(file, maxReadBytes)
	if err != nil {
		return nil, err
	}
	if isBinary(content) {
		return nil, fmt.Errorf("二进制文件无法按文本读取：%s", file)
	}
	lines := strings.SplitAfter(string(content), "\n")
	start := max(req.Offset, 1) - 1
	if start >= len(lines) {
		return &filesystem.FileContent{}, nil
	}
	end := len(lines)
	if req.Limit > 0 {
		end = min(end, start+req.Limit)
	}
	return &filesystem.FileContent{Content: strings.TrimSuffix(strings.Join(lines[start:end], ""), "\n")}, nil
}

// MultiModalRead 把图片文件读取为图片内容，其他文件按文本读取。
func (b *Backend) MultiModalRead(ctx context.Context, req *filesystem.MultiModalReadRequest) (*filesystem.MultiFileContent, error) {
	file, err := b.resolve(req.FilePath)
	if err != nil {
		return nil, err
	}
	mimeType, err := b.imageType(file)
	if err != nil {
		return nil, err
	}
	if mimeType == "" {
		content, err := b.Read(ctx, &req.ReadRequest)
		if err != nil {
			return nil, err
		}
		return &filesystem.MultiFileContent{FileContent: content}, nil
	}
	data, err := b.readFile(file, maxImageBytes)
	if err != nil {
		return nil, err
	}
	return &filesystem.MultiFileContent{Parts: []filesystem.FileContentPart{{Type: filesystem.FileContentPartTypeImage, MIMEType: mimeType, Data: data}}}, nil
}

// imageType 按普通文件开头内容识别图片类型，不是图片或不是普通文件时返回空串。
func (b *Backend) imageType(file string) (string, error) {
	if info, err := os.Stat(file); err != nil || !info.Mode().IsRegular() {
		return "", nil
	}
	handle, err := os.Open(file)
	if err != nil {
		return "", fmt.Errorf("无法读取文件：%s", file)
	}
	defer handle.Close()
	head := make([]byte, 512)
	n, _ := handle.Read(head)
	mimeType := http.DetectContentType(head[:n])
	if slices.Contains(imageTypes, mimeType) {
		return mimeType, nil
	}
	return "", nil
}

// readFile 读取普通文件，目录、管道与设备等非普通文件或超出字节上限时返回错误。
func (b *Backend) readFile(file string, limit int64) ([]byte, error) {
	info, err := os.Stat(file)
	if err != nil {
		return nil, fmt.Errorf("无法读取文件：%s", file)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("路径是目录，请使用 ls 查看：%s", file)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("不是普通文件，无法读取：%s", file)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("文件超过 %d MB，无法读取：%s", limit>>20, file)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("无法读取文件：%s", file)
	}
	return content, nil
}

// isBinary 按文件开头是否包含空字节判断二进制文件。
func isBinary(content []byte) bool {
	return bytes.IndexByte(content[:min(len(content), binarySniffBytes)], 0) >= 0
}

// Write 拒绝写入，文件只读。
func (b *Backend) Write(context.Context, *filesystem.WriteRequest) error {
	return errReadOnly
}

// Edit 拒绝修改，文件只读。
func (b *Backend) Edit(context.Context, *filesystem.EditRequest) error {
	return errReadOnly
}
