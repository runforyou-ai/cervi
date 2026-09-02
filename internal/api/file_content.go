//go:build server

package api

import (
	"errors"
	"log/slog"
	"net/http"
	"path"
	"strings"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

// LocalObjectService 通过稳定对象键处理本地文件的上传和静态读取。
type LocalObjectService struct {
	resolveTenant   tenant.Resolver
	resolveIdentity *authaction.ResolveIdentityQuery
	getFile         *fileaction.GetQuery
	local           *serverfilecontent.LocalStore
	objects         http.Handler
}

// NewLocalObjectService 创建本地对象服务。
func NewLocalObjectService(db *bun.DB, local *serverfilecontent.LocalStore, tenantResolver tenant.Resolver) *LocalObjectService {
	return &LocalObjectService{
		resolveTenant: tenantResolver, resolveIdentity: authaction.NewResolveIdentityQuery(db),
		getFile: fileaction.NewGetQuery(db), local: local, objects: http.FileServerFS(local.ObjectsFS()),
	}
}

// ServeHTTP 处理本地对象请求。
func (s *LocalObjectService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// 允许原生端 WebView 直传和读取企业服务器对象。
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, OPTIONS")
	writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	storageKey, ok := localObjectStorageKey(request.URL.Path)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	switch request.Method {
	case http.MethodGet, http.MethodHead:
		// 通过最终对象目录的静态文件服务输出不可变文件。
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		s.objects.ServeHTTP(&localObjectResponseWriter{ResponseWriter: writer}, request)
	case http.MethodPut:
		s.uploadLocalObject(writer, request, storageKey)
	default:
		writer.Header().Set("Allow", "GET, HEAD, PUT, OPTIONS")
		http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

// uploadLocalObject 将认证后的请求内容保存到本地最终对象目录。
func (s *LocalObjectService) uploadLocalObject(writer http.ResponseWriter, request *http.Request, storageKey string) {
	scope, err := s.resolveTenant.Resolve(request.Context(), tenant.AccessHost(request.Context()))
	if errors.Is(err, tenant.ErrNotFound) {
		http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if err != nil {
		slog.Warn("文件上传企业解析失败", "error", err)
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	identity, err := s.resolveIdentity.Execute(request.Context(), scope.OrganizationID, bearerToken(request.Header.Get("Authorization")))
	if errors.Is(err, authaction.ErrIdentityNotFound) {
		http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if err != nil {
		slog.Warn("文件上传认证失败", "error", err)
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	record, err := s.getFile.ExecuteByStorageKey(request.Context(), identity, storageKey)
	if err != nil {
		// 输出本地对象元数据错误。
		if errors.Is(err, fileaction.ErrFileNotFound) {
			http.Error(writer, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		slog.Warn("读取文件元数据失败", "error", err)
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if record.StorageBackend != string(domain.FileStorageBackendLocal) || record.Status != string(domain.FileStatusPending) || record.Expired {
		http.Error(writer, http.StatusText(http.StatusConflict), http.StatusConflict)
		return
	}
	if request.ContentLength >= 0 && request.ContentLength != record.ByteSize {
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if err := s.local.Save(request.Context(), storageKey, request.Body, record.ByteSize); err != nil {
		slog.Warn("本地文件写入失败", "file_id", record.ID, "error", err)
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// localObjectResponseWriter 只为已命中的静态对象添加不可变缓存策略。
type localObjectResponseWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

// WriteHeader 根据静态文件服务的最终状态写入缓存策略。
func (w *localObjectResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if (status >= http.StatusOK && status < http.StatusMultipleChoices) || status == http.StatusNotModified {
		w.Header().Set("Cache-Control", serverfilecontent.ImmutableCacheControl)
	} else {
		w.Header().Del("Cache-Control")
	}
	w.ResponseWriter.WriteHeader(status)
}

// Write 确保隐式成功响应同样带上不可变缓存策略。
func (w *localObjectResponseWriter) Write(content []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(content)
}

// localObjectStorageKey 从公开路径中读取规范对象键。
func localObjectStorageKey(requestPath string) (string, bool) {
	storageKey := strings.TrimPrefix(requestPath, "/")
	parts := strings.Split(storageKey, "/")
	if len(parts) != 4 || parts[0] != "organizations" || parts[2] != "files" || !common.ValidUUID(parts[1]) {
		return "", false
	}
	extension := path.Ext(parts[3])
	if extension == "" || !common.ValidUUID(strings.TrimSuffix(parts[3], extension)) {
		return "", false
	}
	return storageKey, true
}
