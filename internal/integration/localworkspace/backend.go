// Package localworkspace 以工作区根目录为界提供本机文件的只读访问，模型看到的路径以 / 表示工作区根目录。
package localworkspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
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

var (
	// errReadOnly 表示工作区只读，不接受写入。
	errReadOnly = errors.New("工作区只读，不能写入或修改文件")
	// errWorkspaceUnavailable 表示工作区目录已被移动、删除或替换。
	errWorkspaceUnavailable = errors.New("工作区目录已不可用")
)

// Backend 以工作区根目录为虚拟根读取本机文件，解析符号链接后越出工作区的路径一律拒绝；错误信息只包含虚拟路径。
type Backend struct {
	root     string
	rootInfo os.FileInfo
}

// New 打开本机工作区，目录不存在或不是目录时返回错误。
func New(dir string) (*Backend, error) {
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, errWorkspaceUnavailable
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, errWorkspaceUnavailable
	}
	return &Backend{root: root, rootInfo: info}, nil
}

// target 是一次访问解析得到的虚拟路径与工作区内的真实路径。
type target struct {
	virtual string
	real    string
}

// resolve 把模型给出的路径解析为工作区内的真实路径：先相对虚拟根规范化，再解析符号链接并校验仍位于工作区内。
func (b *Backend) resolve(name string) (target, error) {
	virtual := path.Clean("/" + filepath.ToSlash(name))
	// 工作区根目录被移动、删除或替换为其他目录时拒绝访问。
	if info, err := os.Stat(b.root); err != nil || !os.SameFile(info, b.rootInfo) {
		return target{}, errWorkspaceUnavailable
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(b.root, filepath.FromSlash(virtual)))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return target{}, fmt.Errorf("路径不存在：%s", virtual)
		}
		return target{}, fmt.Errorf("无法访问：%s", virtual)
	}
	if !b.contains(resolved) {
		return target{}, fmt.Errorf("路径不在工作区内：%s", virtual)
	}
	return target{virtual: virtual, real: resolved}, nil
}

// contains 判断真实路径是否位于工作区根目录内。
func (b *Backend) contains(real string) bool {
	rel, err := filepath.Rel(b.root, real)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// virtualPath 返回工作区内真实路径对应的虚拟路径。
func (b *Backend) virtualPath(real string) string {
	rel, err := filepath.Rel(b.root, real)
	if err != nil {
		return "/"
	}
	return path.Clean("/" + filepath.ToSlash(rel))
}

// LsInfo 列出目录的直接子项，目录路径以 / 结尾；路径指向文件时返回该文件。
func (b *Backend) LsInfo(ctx context.Context, req *filesystem.LsInfoRequest) ([]filesystem.FileInfo, error) {
	dir, err := b.resolve(req.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(dir.real)
	if err != nil {
		return nil, fmt.Errorf("无法访问：%s", dir.virtual)
	}
	if !info.IsDir() {
		return []filesystem.FileInfo{fileInfo(dir.virtual, info)}, nil
	}
	entries, err := os.ReadDir(dir.real)
	if err != nil {
		return nil, fmt.Errorf("无法读取目录：%s", dir.virtual)
	}
	infos := make([]filesystem.FileInfo, 0, len(entries))
	for _, entry := range entries {
		// 指向工作区内的符号链接按目标展示，指向工作区外或不存在的目标时只展示条目本身。
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			if resolved, err := filepath.EvalSymlinks(filepath.Join(dir.real, entry.Name())); err == nil && b.contains(resolved) {
				if target, err := os.Stat(resolved); err == nil {
					info = target
				}
			}
		}
		infos = append(infos, fileInfo(path.Join(dir.virtual, entry.Name()), info))
	}
	return infos, nil
}

// fileInfo 转换文件信息，目录路径以 / 结尾。
func fileInfo(virtual string, info os.FileInfo) filesystem.FileInfo {
	result := filesystem.FileInfo{Path: virtual, IsDir: info.IsDir(), Size: info.Size(), ModifiedAt: info.ModTime().UTC().Format(time.RFC3339)}
	if info.IsDir() && virtual != "/" {
		result.Path += "/"
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
		return nil, fmt.Errorf("二进制文件无法按文本读取：%s", file.virtual)
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
func (b *Backend) imageType(file target) (string, error) {
	if info, err := os.Stat(file.real); err != nil || !info.Mode().IsRegular() {
		return "", nil
	}
	handle, err := os.Open(file.real)
	if err != nil {
		return "", fmt.Errorf("无法读取文件：%s", file.virtual)
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

// readFile 读取工作区内的普通文件，目录、管道与设备等非普通文件或超出字节上限时返回错误。
func (b *Backend) readFile(file target, limit int64) ([]byte, error) {
	info, err := os.Stat(file.real)
	if err != nil {
		return nil, fmt.Errorf("无法读取文件：%s", file.virtual)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("路径是目录，请使用 ls 查看：%s", file.virtual)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("不是普通文件，无法读取：%s", file.virtual)
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("文件超过 %d MB，无法读取：%s", limit>>20, file.virtual)
	}
	content, err := os.ReadFile(file.real)
	if err != nil {
		return nil, fmt.Errorf("无法读取文件：%s", file.virtual)
	}
	return content, nil
}

// isBinary 按文件开头是否包含空字节判断二进制文件。
func isBinary(content []byte) bool {
	return bytes.IndexByte(content[:min(len(content), binarySniffBytes)], 0) >= 0
}

// Write 拒绝写入，工作区只读。
func (b *Backend) Write(context.Context, *filesystem.WriteRequest) error {
	return errReadOnly
}

// Edit 拒绝修改，工作区只读。
func (b *Backend) Edit(context.Context, *filesystem.EditRequest) error {
	return errReadOnly
}
