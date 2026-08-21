//go:build server

// Package filecontent 提供本地文件内容和对象存储直读跳转。
package filecontent

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/filestore"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// Service 处理文件内容的上传、读取和对象存储跳转。
type Service struct {
	resolveIdentity *authaction.ResolveIdentityQuery
	getFile         *fileaction.GetQuery
	getS3Setting    *settingaction.GetS3SettingQuery
	local           *filestore.LocalStore
}

// NewService 创建文件内容服务。
func NewService(db *bun.DB, local *filestore.LocalStore) *Service {
	return &Service{
		resolveIdentity: authaction.NewResolveIdentityQuery(db),
		getFile:         fileaction.NewGetQuery(db),
		getS3Setting:    settingaction.NewGetS3SettingQuery(db),
		local:           local,
	}
}

// ServeHTTP 处理文件内容请求。
func (s *Service) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	allowCrossOrigin(writer)
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusNoContent)
		return
	}
	fileID, ok := contentFileID(request.URL.Path)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	switch request.Method {
	case http.MethodGet, http.MethodHead:
		s.serveFile(writer, request, fileID)
	case http.MethodPut:
		s.uploadLocalFile(writer, request, fileID)
	default:
		writer.Header().Set("Allow", "GET, HEAD, PUT, OPTIONS")
		http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

// uploadLocalFile 将认证后的请求内容保存到本地最终目录。
func (s *Service) uploadLocalFile(writer http.ResponseWriter, request *http.Request, fileID string) {
	identity, err := s.resolveIdentity.Execute(request.Context(), bearerToken(request.Header.Get("Authorization")))
	if err != nil {
		slog.Warn("文件上传认证失败", "error", err)
		http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if identity == nil {
		http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	record, err := s.getFile.Execute(request.Context(), identity, fileID)
	if err != nil {
		writeFileError(writer, err)
		return
	}
	if record.StorageBackend != string(domain.FileStorageBackendLocal) || record.Status != string(domain.FileStatusPending) || record.ExpiresAt == nil || !record.ExpiresAt.After(time.Now().UTC()) {
		http.Error(writer, http.StatusText(http.StatusConflict), http.StatusConflict)
		return
	}
	if request.ContentLength >= 0 && request.ContentLength != record.ByteSize {
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if err := s.local.Save(request.Context(), record.StorageKey, request.Body, record.ByteSize); err != nil {
		slog.Warn("本地文件写入失败", "file_id", record.ID, "error", err)
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// serveFile 输出本地内容或跳转到对象存储预签名地址。
func (s *Service) serveFile(writer http.ResponseWriter, request *http.Request, fileID string) {
	record, err := s.getFile.GetActiveByID(request.Context(), fileID)
	if err != nil {
		writeFileError(writer, err)
		return
	}
	if record.StorageBackend == string(domain.FileStorageBackendS3) {
		s.serveS3File(writer, request, record)
		return
	}
	file, info, err := s.local.Open(record.StorageKey)
	if err != nil {
		slog.Warn("读取本地文件失败", "file_id", record.ID, "error", err)
		http.NotFound(writer, request)
		return
	}
	defer file.Close()
	writer.Header().Set("Content-Type", record.ContentType)
	writer.Header().Set("Content-Disposition", contentDisposition(record.ContentType, record.OriginalName))
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(writer, request, record.OriginalName, info.ModTime(), file)
}

// serveS3File 返回对象元数据或将客户端直接跳转到对象存储。
func (s *Service) serveS3File(writer http.ResponseWriter, request *http.Request, record *servermodels.File) {
	setting, err := s.getS3Setting.ExecuteForOrganization(request.Context(), record.OrganizationID)
	if err != nil {
		slog.Warn("读取文件对象存储配置失败", "file_id", record.ID, "error", err)
		http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	config := filestore.S3Config{
		Endpoint: setting.Endpoint, Region: setting.Region, Bucket: setting.Bucket,
		AccessKeyID: setting.AccessKeyID, SecretAccessKey: setting.SecretAccessKey, ForcePathStyle: setting.ForcePathStyle,
	}
	if request.Method == http.MethodHead {
		info, err := filestore.Stat(request.Context(), config, record.StorageKey)
		if err != nil {
			slog.Warn("读取对象存储文件元数据失败", "file_id", record.ID, "error", err)
			http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			return
		}
		writer.Header().Set("Content-Type", record.ContentType)
		writer.Header().Set("Content-Length", strconv.FormatInt(info.ByteSize, 10))
		writer.Header().Set("Content-Disposition", contentDisposition(record.ContentType, record.OriginalName))
		if info.ETag != "" {
			writer.Header().Set("ETag", info.ETag)
		}
		writer.WriteHeader(http.StatusOK)
		return
	}
	signed, err := filestore.PresignGet(request.Context(), config, record.StorageKey, record.ContentType, record.OriginalName)
	if err != nil {
		slog.Warn("生成对象存储读取地址失败", "file_id", record.ID, "error", err)
		http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	http.Redirect(writer, request, signed.URL, http.StatusTemporaryRedirect)
}

// contentFileID 从内容路由中读取文件编号。
func contentFileID(path string) (string, bool) {
	fileID, suffix, found := strings.Cut(path, "/")
	if !found || fileID == "" || suffix != "content" {
		return "", false
	}
	return fileID, true
}

// bearerToken 读取 Bearer 登录令牌。
func bearerToken(authorization string) string {
	scheme, token, found := strings.Cut(strings.TrimSpace(authorization), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// contentDisposition 返回文件在浏览器中的展示方式。
func contentDisposition(contentType, fileName string) string {
	disposition := "attachment"
	if strings.HasPrefix(contentType, "image/") || contentType == "application/pdf" {
		disposition = "inline"
	}
	return mime.FormatMediaType(disposition, map[string]string{"filename": fileName})
}

// allowCrossOrigin 允许原生端 WebView 直传企业服务器。
func allowCrossOrigin(writer http.ResponseWriter) {
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, OPTIONS")
	writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
}

// writeFileError 输出文件内容路由错误。
func writeFileError(writer http.ResponseWriter, err error) {
	if errors.Is(err, fileaction.ErrFileNotFound) {
		http.Error(writer, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	slog.Warn("读取文件元数据失败", "error", err)
	http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
