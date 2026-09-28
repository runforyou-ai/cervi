//go:build server

package api

import (
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	authaction "github.com/runforyou-ai/cervi/internal/actions/auth"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/common/customeridentity"
	"github.com/runforyou-ai/cervi/internal/domain"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// LocalObjectService 通过稳定对象键处理本地文件的上传和静态读取。
type LocalObjectService struct {
	resolveIdentity *authaction.ResolveIdentityQuery
	getFile         *fileaction.GetQuery
	verifyCustomer  *conversationaction.VerifyWebsiteCustomerQuery
	local           *serverfilecontent.LocalStore
	objects         http.Handler
}

// NewLocalObjectService 创建本地对象服务，对象所属工作区取自对象键。
func NewLocalObjectService(db *bun.DB, local *serverfilecontent.LocalStore) *LocalObjectService {
	return &LocalObjectService{
		resolveIdentity: authaction.NewResolveIdentityQuery(db),
		getFile:         fileaction.NewGetQuery(db), verifyCustomer: conversationaction.NewVerifyWebsiteCustomerQuery(db), local: local, objects: http.FileServerFS(local.ObjectsFS()),
	}
}

// ServeHTTP 处理本地对象请求。
func (s *LocalObjectService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// 允许原生端 WebView 直传和读取企业服务器对象。
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, PUT, OPTIONS")
	writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, "+websiteVisitorHeader+", "+websiteCustomerHeader)
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
		if strings.Split(storageKey, "/")[2] == "knowledge-documents" {
			s.previewKnowledgeObject(writer, request, storageKey)
			return
		}
		// 按文件元数据设置内嵌图片的响应内容类型。
		if request.URL.Query().Get("inline") == "1" {
			contentType, err := s.getFile.ContentTypeByStorageKey(request.Context(), storageKeyOrganizationID(storageKey), storageKey)
			if err != nil {
				http.NotFound(writer, request)
				return
			}
			writer.Header().Set("Content-Type", contentType)
		}
		// 通过最终对象目录的静态文件服务输出不可变文件。
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		if name := request.URL.Query().Get("download"); name != "" {
			writer.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
		}
		s.objects.ServeHTTP(&localObjectResponseWriter{ResponseWriter: writer}, request)
	case http.MethodPut:
		s.uploadLocalObject(writer, request, storageKey)
	default:
		writer.Header().Set("Allow", "GET, HEAD, PUT, OPTIONS")
		http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

// uploadLocalObject 将认证后的请求内容保存到本地最终对象目录，网站登录用户按签名身份、渠道访客按访客令牌校验文件归属。
func (s *LocalObjectService) uploadLocalObject(writer http.ResponseWriter, request *http.Request, storageKey string) {
	var record *servermodels.File
	var err error
	if customerToken := strings.TrimSpace(request.Header.Get(websiteCustomerHeader)); customerToken != "" {
		// 按对象键所属工作区验签，文件须属于该工作区下该登录用户的渠道身份。
		organizationID := storageKeyOrganizationID(storageKey)
		verified, verifyErr := s.verifyCustomer.ExecuteForOrganization(request.Context(), organizationID, customerToken)
		if verifyErr != nil {
			http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		record, err = s.getFile.VisitorPendingByStorageKey(request.Context(), customeridentity.CustomerExternalID(verified.Customer.UserID), storageKey)
		if err == nil && record.OrganizationID != organizationID {
			err = fileaction.ErrFileNotFound
		}
	} else if visitorToken := strings.TrimSpace(request.Header.Get(websiteVisitorHeader)); visitorToken != "" {
		if !validWebsiteVisitorToken(visitorToken) {
			http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		record, err = s.getFile.VisitorPendingByStorageKey(request.Context(), customeridentity.AnonymousExternalID(visitorToken), storageKey)
	} else {
		identity, identityErr := s.resolveIdentity.Execute(request.Context(), storageKeyOrganizationID(storageKey), bearerToken(request.Header.Get("Authorization")))
		if errors.Is(identityErr, authaction.ErrIdentityNotFound) || errors.Is(identityErr, authaction.ErrMembershipNotFound) {
			http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if identityErr != nil {
			slog.Warn("文件上传认证失败", "error", identityErr)
			http.Error(writer, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		record, err = s.getFile.ExecuteByStorageKey(request.Context(), identity, storageKey)
		if err == nil && record.CreatedByUserID != identity.User.ID {
			err = fileaction.ErrFileNotFound
		}
	}
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
	expectedSize := record.ByteSize
	partNumber := int64(0)
	if record.PartSize > 0 {
		partNumber, err = strconv.ParseInt(request.URL.Query().Get("partNumber"), 10, 32)
		if err == nil {
			expectedSize, err = fileaction.UploadPartSize(record, int32(partNumber))
		}
		if err != nil {
			http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	}
	if request.ContentLength >= 0 && request.ContentLength != expectedSize {
		http.Error(writer, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if record.PartSize > 0 {
		err = s.local.SavePart(request.Context(), storageKey, int32(partNumber), request.Body, expectedSize)
	} else {
		err = s.local.Save(request.Context(), storageKey, request.Body, expectedSize)
	}
	if err != nil {
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

// storageKeyOrganizationID 返回规范对象键中的工作区编号，对象键格式为 organizations/<工作区编号>/<类别>/<文件名>。
func storageKeyOrganizationID(storageKey string) string {
	return strings.Split(storageKey, "/")[1]
}

// localObjectStorageKey 从公开路径中读取规范对象键。
func localObjectStorageKey(requestPath string) (string, bool) {
	storageKey := strings.TrimPrefix(requestPath, "/")
	parts := strings.Split(storageKey, "/")
	if len(parts) != 4 || parts[0] != "organizations" || (parts[2] != "files" && parts[2] != "knowledge-documents") || !common.ValidUUID(parts[1]) {
		return "", false
	}
	extension := path.Ext(parts[3])
	if extension == "" || !common.ValidUUID(strings.TrimSuffix(parts[3], extension)) {
		return "", false
	}
	return storageKey, true
}

// previewKnowledgeObject 认证后读取仍在使用的知识文档原件，禁止共享缓存。
func (s *LocalObjectService) previewKnowledgeObject(writer http.ResponseWriter, request *http.Request, storageKey string) {
	identity, err := s.resolveIdentity.Execute(request.Context(), storageKeyOrganizationID(storageKey), bearerToken(request.Header.Get("Authorization")))
	if err != nil {
		http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	record, err := s.getFile.ExecuteByStorageKey(request.Context(), identity, storageKey)
	if err != nil || record.Status != string(domain.FileStatusActive) || record.Purpose != string(domain.FilePurposeKnowledgeDocument) {
		http.NotFound(writer, request)
		return
	}
	file, info, err := s.local.Open(request.Context(), storageKey)
	if err != nil {
		http.NotFound(writer, request)
		return
	}
	defer file.Close()
	writer.Header().Set("Cache-Control", "private, no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(writer, request, record.OriginalName, info.ModTime(), file)
}
