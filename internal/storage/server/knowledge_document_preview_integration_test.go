//go:build server

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"uuid"

	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	"github.com/runforyou-ai/cervi/internal/integration/connector"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestKnowledgeSegmentLocation 验证 Dify 分段页定位、失效位置和企业隔离。
func TestKnowledgeSegmentLocation(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	identity, base := newQAFixture(t, db)
	var pages []int
	calls := 0
	sparse := false
	remoteStatus := http.StatusOK
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer context-test" {
			t.Error("missing Dify credentials")
		}
		if remoteStatus != http.StatusOK {
			w.WriteHeader(remoteStatus)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/datasets/context-dataset/documents/document":
			_, _ = w.Write([]byte(`{"id":"document","name":"制度问答","display_status":"available"}`))
		case "/v1/datasets/context-dataset/documents/document/segments":
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			pages = append(pages, page)
			if r.URL.Query().Get("limit") != "100" {
				t.Error("unexpected page size")
			}
			data := make([]map[string]any, 0)
			for row := (page-1)*100 + 1; row <= min(page*100, 205); row++ {
				position := row
				if sparse {
					position = row*2 - 1
				}
				data = append(data, map[string]any{"id": fmt.Sprintf("segment-%d", position), "position": position, "content": fmt.Sprintf("问题 %d", position), "answer": "完整答案", "status": "completed"})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "page": page, "limit": 100, "total": 205})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer remote.Close()
	connection := &servermodels.IntegrationConnection{
		OrganizationID: identity.Organization.ID, Type: string(domain.IntegrationConnectionTypeDify), Name: "上下文测试",
		Configuration: servermodels.IntegrationConnectionConfiguration{APIURL: remote.URL + "/v1", APIKey: "context-test"},
	}
	if _, err := db.NewInsert().Model(connection).Column("organization_id", "connector_type", "name", "configuration").Returning("id").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NewUpdate().Model((*servermodels.KnowledgeBase)(nil)).Set("integration_connection_id = ?", connection.ID).Set("external_resource_id = ?", "context-dataset").Where("id = ?", base.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	reader := connector.NewDifyKnowledgeDocumentLister(remote.Client())
	query := knowledgeaction.NewListKnowledgeDocumentSegmentsQuery(db, reader)
	for _, tc := range []struct {
		position int
		want     []int
		pages    []int
	}{
		{1, []int{1, 100}, []int{1}},
		{100, []int{1, 100}, []int{1}},
		{205, []int{201, 205}, []int{3}},
	} {
		t.Run(strconv.Itoa(tc.position), func(t *testing.T) {
			pages = nil
			rows, err := query.Execute(ctx, identity, base.ID, "document", knowledgeaction.DocumentSegmentListInput{PageSize: 100, SegmentID: fmt.Sprintf("segment-%d", tc.position), Position: tc.position})
			if err != nil {
				t.Fatal(err)
			}
			positions := make([]int, 0, len(rows.Segments))
			for _, row := range rows.Segments {
				positions = append(positions, row.Position)
				if row.Answer == nil || *row.Answer != "完整答案" {
					t.Fatalf("invalid context row: %+v", row)
				}
			}
			if !slices.Equal([]int{positions[0], positions[len(positions)-1]}, tc.want) || !slices.Equal(pages, tc.pages) {
				t.Fatalf("positions=%v pages=%v", positions, pages)
			}
		})
	}
	// 分段删除后序号存在空缺时，仍能定位正确页。
	sparse = true
	pages = nil
	found, err := query.Execute(ctx, identity, base.ID, "document", knowledgeaction.DocumentSegmentListInput{PageSize: 100, SegmentID: "segment-205", Position: 205})
	if err != nil || found.Page != 2 || !slices.Equal(pages, []int{3, 1, 2}) {
		t.Fatalf("sparse location page=%d requests=%v err=%v", found.Page, pages, err)
	}
	sparse = false
	input := knowledgeaction.DocumentSegmentListInput{PageSize: 100, SegmentID: "segment-100", Position: 100}
	for _, stale := range []knowledgeaction.DocumentSegmentListInput{
		{PageSize: 100, SegmentID: "deleted", Position: 100},
		{PageSize: 100, SegmentID: "segment-100", Position: 101},
	} {
		_, err := query.Execute(ctx, identity, base.ID, "document", stale)
		if _, kind, _ := connectiontest.Details(err); kind != connectiontest.FailureNotFound {
			t.Fatalf("stale context error=%v", err)
		}
	}
	remoteStatus = http.StatusServiceUnavailable
	if _, err := query.Execute(ctx, identity, base.ID, "document", input); err == nil {
		t.Fatal("remote failure accepted")
	}
	remoteStatus = http.StatusOK
	// 企业或知识库范围不匹配时，不向远端发送读取请求。
	before := calls
	foreign := *identity
	foreign.Organization.ID = uuid.NewV7().String()
	if _, err := query.Execute(ctx, &foreign, base.ID, "document", input); !errors.Is(err, knowledgeaction.ErrNotFound) {
		t.Fatalf("foreign context error=%v", err)
	}
	otherID := uuid.NewV7().String()
	if _, err := query.Execute(ctx, identity, otherID, "document", input); !errors.Is(err, knowledgeaction.ErrNotFound) {
		t.Fatalf("unknown base error=%v", err)
	}
	if calls != before {
		t.Fatal("out-of-scope request reached Dify")
	}
}
