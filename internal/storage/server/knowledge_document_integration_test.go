//go:build server

package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"uuid"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	"github.com/runforyou-ai/cervi/internal/actions/filemaintenance"
	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/api"
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/domain"
	filecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/runforyou-ai/cervi/internal/tenant"
	"github.com/uptrace/bun"
)

// newDocumentFixture 创建独立企业和标准知识库。
func newDocumentFixture(t *testing.T, db *bun.DB) (installationaction.InstallWorkspaceOutput, *knowledgeaction.Record) {
	t.Helper()
	installed, err := installationaction.NewInstallWorkspaceAction(db).Execute(context.Background(), installationaction.InstallWorkspaceInput{
		AccessHost: uuid.NewV7().String() + ".documents.test", OrganizationName: "文档测试", DisplayName: "维护人员", Email: "owner@documents.test", Password: "password123", Locale: domain.LocaleChineseSimplified, TimeZone: "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	base, err := knowledgeaction.NewCreateKnowledgeBaseAction(db).Execute(context.Background(), installed.Identity, newKnowledgeBaseInput(t, db, installed.Identity, "资料", domain.KnowledgeBaseCategoryStandard))
	if err != nil {
		t.Fatal(err)
	}
	return installed, base
}

// uploadedDocumentFile 创建已核验上传的测试原件。
func uploadedDocumentFile(t *testing.T, db *bun.DB, identity *servermodels.Identity, name string) *servermodels.File {
	t.Helper()
	ctx := context.Background()
	record, err := fileaction.NewCreateUploadAction(db).Execute(ctx, identity, domain.FileStorageBackendLocal, fileaction.UploadInput{Purpose: domain.FilePurposeKnowledgeDocument, FileName: name, ContentType: "application/octet-stream", ByteSize: 12})
	if err != nil {
		t.Fatal(err)
	}
	record, err = fileaction.NewMarkUploadedAction(db).Execute(ctx, identity, record.ID, "etag")
	if err != nil {
		t.Fatal(err)
	}
	return record
}

// TestKnowledgeDocumentLifecycle 验证批次幂等、倒序、分组归属及删除原件状态。
func TestKnowledgeDocumentLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	installed, base := newDocumentFixture(t, db)
	identity := installed.Identity
	first := uploadedDocumentFile(t, db, identity, "报表100%.XLSX")
	second := uploadedDocumentFile(t, db, identity, "说明.pdf")
	create := knowledgeaction.NewCreateDocumentsAction(db)
	query := knowledgeaction.NewDocumentQuery(db)
	docs, err := create.Execute(ctx, identity, base.ID, base.Groups[0].ID, []string{first.ID, second.ID})
	if err != nil || len(docs) != 2 {
		t.Fatalf("create=%+v %v", docs, err)
	}
	repeat, err := create.Execute(ctx, identity, base.ID, base.Groups[0].ID, []string{first.ID, second.ID})
	if err != nil || repeat[0].ID != docs[0].ID {
		t.Fatalf("retry=%+v %v", repeat, err)
	}
	page, err := query.List(ctx, identity, base.ID, knowledgeaction.DocumentListInput{GroupID: base.Groups[0].ID, PageSize: 1})
	if err != nil || page.Total != 2 || page.Documents[0].ID != docs[1].ID {
		t.Fatalf("page=%+v %v", page, err)
	}
	for keyword, count := range map[string]int{"100%": 1, "_": 0, "报表": 1} {
		result, err := query.List(ctx, identity, base.ID, knowledgeaction.DocumentListInput{GroupID: base.Groups[0].ID, Keyword: keyword})
		if err != nil || result.Total != count {
			t.Fatalf("search %s=%+v %v", keyword, result, err)
		}
	}
	if docs[0].Status != domain.KnowledgeDocumentInitial {
		t.Fatal("new document not initial")
	}
	grouped, err := knowledgeaction.NewCreateKnowledgeGroupAction(db).Execute(ctx, identity, base.ID, knowledgeaction.GroupInput{Name: "归档"})
	if err != nil {
		t.Fatal(err)
	}
	target := grouped.Groups[1].ID
	if err := knowledgeaction.NewMoveDocumentAction(db).Execute(ctx, identity, base.ID, docs[0].ID, target); err != nil {
		t.Fatal(err)
	}
	moved, err := query.Get(ctx, identity, base.ID, docs[0].ID)
	if err != nil || moved.GroupID != target || !moved.CreatedAt.Equal(docs[0].CreatedAt) {
		t.Fatalf("moved=%+v %v", moved, err)
	}
	repeat, err = create.Execute(ctx, identity, base.ID, base.Groups[0].ID, []string{first.ID})
	if err != nil || repeat[0].GroupID != target {
		t.Fatal("retry reverted group", err)
	}
	if _, err := knowledgeaction.NewDeleteKnowledgeGroupAction(db).Execute(ctx, identity, base.ID, target); !errors.Is(err, knowledgeaction.ErrGroupNotEmpty) {
		t.Fatal("occupied group", err)
	}
	if _, err := knowledgeaction.NewUpdateKnowledgeBaseAction(db).Execute(ctx, identity, base.ID, newKnowledgeBaseInput(t, db, identity, base.Name, domain.KnowledgeBaseCategoryQA)); !errors.Is(err, knowledgeaction.ErrBaseHasContent) {
		t.Fatal("occupied base", err)
	}
	if err := knowledgeaction.NewDeleteDocumentAction(db).Execute(ctx, identity, base.ID, docs[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := query.File(ctx, identity, base.ID, docs[0].ID); !errors.Is(err, knowledgeaction.ErrDocumentNotFound) {
		t.Fatal("deleted preview", err)
	}
	if err := knowledgeaction.NewDeleteKnowledgeBaseAction(db).Execute(ctx, identity, base.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{first.ID, second.ID} {
		record, err := fileaction.NewGetQuery(db).Execute(ctx, identity, id)
		if err != nil || record.Status != string(domain.FileStatusDeleting) || record.ExpiresAt == nil {
			t.Fatalf("released=%+v %v", record, err)
		}
	}
}

// TestKnowledgeDocumentBatchIsolation 验证十个文件限制、跨企业边界、事务回滚和并发重试。
func TestKnowledgeDocumentBatchIsolation(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	owner, base := newDocumentFixture(t, db)
	other, otherBase := newDocumentFixture(t, db)
	create := knowledgeaction.NewCreateDocumentsAction(db)
	first := uploadedDocumentFile(t, db, owner.Identity, "same.txt")
	foreign := uploadedDocumentFile(t, db, other.Identity, "foreign.txt")
	if _, err := create.Execute(ctx, owner.Identity, base.ID, base.Groups[0].ID, []string{first.ID, foreign.ID}); !errors.Is(err, fileaction.ErrFileNotFound) {
		t.Fatal("foreign file", err)
	}
	record, _ := fileaction.NewGetQuery(db).Execute(ctx, owner.Identity, first.ID)
	if record.Status != string(domain.FileStatusUploaded) {
		t.Fatal("batch failed to roll back")
	}
	if _, err := create.Execute(ctx, owner.Identity, base.ID, otherBase.Groups[0].ID, []string{first.ID}); !errors.Is(err, knowledgeaction.ErrGroupNotFound) {
		t.Fatal("foreign group", err)
	}
	for _, ids := range [][]string{nil, {first.ID, first.ID}, make([]string, 11)} {
		if _, err := create.Execute(ctx, owner.Identity, base.ID, base.Groups[0].ID, ids); !errors.Is(err, knowledgeaction.ErrDocumentBatchInvalid) {
			t.Fatal("batch bound", err)
		}
	}
	var wait sync.WaitGroup
	results := make(chan []knowledgeaction.DocumentRecord, 2)
	failures := make(chan error, 2)
	for range 2 {
		wait.Go(func() {
			result, err := create.Execute(ctx, owner.Identity, base.ID, base.Groups[0].ID, []string{first.ID})
			results <- result
			failures <- err
		})
	}
	wait.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	for result := range results {
		if id != "" && result[0].ID != id {
			t.Fatal("duplicate document")
		}
		id = result[0].ID
	}
	query := knowledgeaction.NewDocumentQuery(db)
	if _, err := query.Get(ctx, other.Identity, base.ID, id); !errors.Is(err, knowledgeaction.ErrNotFound) {
		t.Fatal("foreign document", err)
	}
	if _, err := query.File(ctx, other.Identity, base.ID, id); !errors.Is(err, knowledgeaction.ErrDocumentNotFound) {
		t.Fatal("foreign preview", err)
	}
	if err := knowledgeaction.NewMoveDocumentAction(db).Execute(ctx, owner.Identity, base.ID, id, otherBase.Groups[0].ID); !errors.Is(err, knowledgeaction.ErrGroupNotFound) {
		t.Fatal("foreign move", err)
	}
	ids := make([]string, 10)
	for i := range ids {
		ids[i] = uploadedDocumentFile(t, db, owner.Identity, "same.txt").ID
	}
	if result, err := create.Execute(ctx, owner.Identity, base.ID, base.Groups[0].ID, ids); err != nil || len(result) != 10 {
		t.Fatal("ten files", err)
	}
}

// TestKnowledgeDocumentLocalPreview 验证原件只允许当前企业已登录成员读取，删除后失效。
func TestKnowledgeDocumentLocalPreview(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	owner, base := newDocumentFixture(t, db)
	other, _ := newDocumentFixture(t, db)
	file := uploadedDocumentFile(t, db, owner.Identity, "preview.txt")
	docs, err := knowledgeaction.NewCreateDocumentsAction(db).Execute(ctx, owner.Identity, base.ID, base.Groups[0].ID, []string{file.ID})
	if err != nil {
		t.Fatal(err)
	}
	local, err := filecontent.NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content := "preview text"
	if err := local.Save(ctx, file.StorageKey, strings.NewReader(content), int64(len(content))); err != nil {
		t.Fatal(err)
	}
	service := api.NewLocalObjectService(db, local, NewTenantResolver(db))
	for _, test := range []struct {
		token  string
		status int
	}{{"", 401}, {other.Token, 401}, {owner.Token, 200}} {
		request := httptest.NewRequest(http.MethodGet, "/"+file.StorageKey, nil).WithContext(tenant.WithAccessHost(ctx, owner.Identity.Organization.AccessHost))
		request.Header.Set("Authorization", "Bearer "+test.token)
		response := httptest.NewRecorder()
		service.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Fatalf("preview status=%d want=%d", response.Code, test.status)
		}
		if test.status == 200 && (response.Body.String() != content || response.Header().Get("Cache-Control") != "private, no-store") {
			t.Fatal("preview content or cache")
		}
	}
	if err := knowledgeaction.NewDeleteDocumentAction(db).Execute(ctx, owner.Identity, base.ID, docs[0].ID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/"+file.StorageKey, nil).WithContext(tenant.WithAccessHost(ctx, owner.Identity.Organization.AccessHost))
	request.Header.Set("Authorization", "Bearer "+owner.Token)
	response := httptest.NewRecorder()
	service.ServeHTTP(response, request)
	if response.Code != 404 {
		t.Fatal("deleted raw file still readable")
	}
}

// TestKnowledgeDocumentS3Preview 验证停用 S3 后仍签发原件预览，删除阻止签发并交给对象清理。
func TestKnowledgeDocumentS3Preview(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	owner, base := newDocumentFixture(t, db)
	ctx = tenant.WithAccessHost(ctx, owner.Identity.Organization.AccessHost)
	// HTTP 测试端点只记录客户端读取与清理请求，不启动独立对象存储服务。
	var mu sync.Mutex
	objects := map[string]bool{}
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodDelete {
			if r.Header.Get("Authorization") == "" {
				t.Error("unsigned cleanup")
			}
			delete(objects, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if !objects[r.URL.Path] {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("X-Amz-Signature") == "" {
			t.Error("unsigned preview")
		}
		_, _ = io.WriteString(w, "S3 preview")
	}))
	defer endpoint.Close()
	setting := settingaction.S3Setting{Enabled: true, Provider: domain.StorageProviderAWS, Endpoint: endpoint.URL, PublicBaseURL: endpoint.URL + "/cervi", Region: "us-east-1", Bucket: "cervi", AccessKeyID: "test-access", SecretAccessKey: "test-secret", ForcePathStyle: true}
	if _, err := settingaction.NewSaveS3SettingAction(db).Execute(ctx, owner.Identity, setting); err != nil {
		t.Fatal(err)
	}
	backend := appservice.NewDirectBackend(db, nil, NewTenantResolver(db), nil, nil, nil)
	meta := appservice.RequestMeta{Token: owner.Token, Locale: appservice.LocaleChineseSimplified}
	files := make([]*servermodels.File, 2)
	for i := range files {
		record, err := fileaction.NewCreateUploadAction(db).Execute(ctx, owner.Identity, domain.FileStorageBackendS3, fileaction.UploadInput{Purpose: domain.FilePurposeKnowledgeDocument, FileName: "source.txt", ByteSize: 10})
		if err != nil {
			t.Fatal(err)
		}
		files[i], err = fileaction.NewMarkUploadedAction(db).Execute(ctx, owner.Identity, record.ID, "etag")
		if err != nil {
			t.Fatal(err)
		}
		objects["/cervi/"+record.StorageKey] = true
	}
	docs, err := knowledgeaction.NewCreateDocumentsAction(db).Execute(ctx, owner.Identity, base.ID, base.Groups[0].ID, []string{files[0].ID, files[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	setting.Enabled = false
	if _, err := settingaction.NewSaveS3SettingAction(db).Execute(ctx, owner.Identity, setting); err != nil {
		t.Fatal(err)
	}
	config := filecontent.S3Config{Endpoint: setting.Endpoint, Region: setting.Region, Bucket: setting.Bucket, AccessKeyID: setting.AccessKeyID, SecretAccessKey: setting.SecretAccessKey, ForcePathStyle: true}
	cleanup := filemaintenance.NewDeleteExpiredAction(db, filecontent.NewDeleter(nil, func(_ context.Context, orgID string) (filecontent.S3Config, error) {
		if orgID != owner.Identity.Organization.ID {
			t.Error("wrong cleanup organization")
		}
		return config, nil
	}))
	for i, doc := range docs {
		request, err := backend.GetKnowledgeDocumentPreview(ctx, meta, base.ID, doc.ID)
		if err != nil {
			t.Fatal(err)
		}
		signed, err := url.Parse(request.URL)
		if err != nil {
			t.Fatal(err)
		}
		if signed.Path != "/cervi/"+files[i].StorageKey || signed.Query().Get("response-content-disposition") != "inline" || len(request.Headers) != 0 {
			t.Fatal("invalid direct preview request")
		}
		response, err := http.Get(request.URL)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || string(body) != "S3 preview" {
			t.Fatal("preview failed", err)
		}
		if i == 0 {
			err = backend.DeleteKnowledgeDocument(ctx, meta, base.ID, doc.ID)
		} else {
			err = backend.DeleteKnowledgeBase(ctx, meta, base.ID)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err = backend.GetKnowledgeDocumentPreview(ctx, meta, base.ID, doc.ID); err == nil {
			t.Fatal("deleted document still signs preview")
		}
		if err = cleanup.Execute(ctx, filemaintenance.DeleteExpiredInput{FileID: files[i].ID}); err != nil {
			t.Fatal(err)
		}
		response, err = http.Get(request.URL)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 404 {
			t.Fatal("cleaned object still readable")
		}
		if _, err = fileaction.NewGetQuery(db).Execute(ctx, owner.Identity, files[i].ID); !errors.Is(err, fileaction.ErrFileNotFound) {
			t.Fatal("cleaned metadata retained", err)
		}
	}
}
