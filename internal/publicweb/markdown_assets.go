//go:build server

package publicweb

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
)

// markdownRepresentation 是正文资源的一种编码表示及其 ETag。
type markdownRepresentation struct {
	etag string
	body []byte
}

// markdownAsset 是预先计算内容版本的正文资源，同时保存原文与 gzip 两种表示。
type markdownAsset struct {
	contentType string
	identity    markdownRepresentation
	gzip        markdownRepresentation
}

// markdownAssetsByName 与 markdownAssetVersion 在启动时由内嵌资源生成，聊天页按版本引用资源地址。
var markdownAssetsByName, markdownAssetVersion = loadMarkdownAssets()

// loadMarkdownAssets 读取内嵌正文资源，计算整体内容版本以及各表示的 ETag 和内容。
func loadMarkdownAssets() (map[string]markdownAsset, string) {
	files := []struct{ name, contentType string }{
		{"markdown.js", "text/javascript; charset=utf-8"},
		{"markdown.css", "text/css; charset=utf-8"},
	}
	assets := make(map[string]markdownAsset, len(files))
	version := sha256.New()
	for _, file := range files {
		raw, err := markdownAssets.ReadFile("dist/" + file.name)
		if err != nil {
			panic("读取内嵌正文资源失败: " + err.Error())
		}
		sum := sha256.Sum256(raw)
		version.Write(sum[:])
		var compressed bytes.Buffer
		encoder, _ := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
		_, _ = encoder.Write(raw)
		_ = encoder.Close()
		digest := hex.EncodeToString(sum[:16])
		assets[file.name] = markdownAsset{
			contentType: file.contentType,
			identity:    markdownRepresentation{etag: `"` + digest + `"`, body: raw},
			gzip:        markdownRepresentation{etag: `"` + digest + `-gzip"`, body: compressed.Bytes()},
		}
	}
	return assets, hex.EncodeToString(version.Sum(nil)[:6])
}

// writeMarkdownAsset 按客户端接受的编码返回正文资源；地址携带当前内容版本时长期缓存，其余地址每次按所选表示的 ETag 校验。
func writeMarkdownAsset(writer http.ResponseWriter, request *http.Request, asset markdownAsset) {
	header := writer.Header()
	header.Set("Content-Type", asset.contentType)
	header.Set("Vary", "Accept-Encoding")
	header.Set("X-Content-Type-Options", "nosniff")
	if request.URL.Query().Get("v") == markdownAssetVersion {
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		header.Set("Cache-Control", "no-cache")
	}
	representation := asset.identity
	if acceptsGzip(request.Header.Get("Accept-Encoding")) {
		representation = asset.gzip
		header.Set("Content-Encoding", "gzip")
	}
	header.Set("ETag", representation.etag)
	// If-None-Match 含所选表示的 ETag 或通配符时返回未修改。
	for _, candidate := range strings.Split(request.Header.Get("If-None-Match"), ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == representation.etag || candidate == "*" {
			header.Del("Content-Encoding")
			writer.WriteHeader(http.StatusNotModified)
			return
		}
	}
	header.Set("Content-Length", strconv.Itoa(len(representation.body)))
	writer.WriteHeader(http.StatusOK)
	if request.Method != http.MethodHead {
		_, _ = writer.Write(representation.body)
	}
}

// acceptsGzip 按 Accept-Encoding 的编码与权重判断客户端是否接受 gzip，q=0 表示拒绝。
func acceptsGzip(acceptEncoding string) bool {
	wildcard := false
	for _, part := range strings.Split(acceptEncoding, ",") {
		coding, params, _ := strings.Cut(part, ";")
		accepted := true
		if weight, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			q, err := strconv.ParseFloat(weight, 64)
			accepted = err == nil && q > 0
		}
		switch strings.ToLower(strings.TrimSpace(coding)) {
		case "gzip", "x-gzip":
			return accepted
		case "*":
			wildcard = accepted
		}
	}
	return wildcard
}
