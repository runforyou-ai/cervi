package connector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// TestDifyAppProbe 验证 Dify 应用密钥通过应用信息接口探测。
func TestDifyAppProbe(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/info" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer app-key" {
			t.Fatalf("unexpected authorization header: %s", authorization)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"name":"客服应用","mode":"chat"}`))
	}))

	probe, err := NewRegistry(server.Client()).NewProbe(Config{
		Type: domain.IntegrationConnectionTypeDify, APIURL: server.URL + "/v1", APIKey: "app-key",
	})
	if err != nil {
		t.Fatalf("create probe: %v", err)
	}
	if err := probe.Run(context.Background()); err != nil {
		t.Fatalf("run probe: %v", err)
	}
}

// TestDifyKnowledgeProbeFallback 验证 Dify 知识库密钥回退到知识库列表接口。
func TestDifyKnowledgeProbeFallback(t *testing.T) {
	paths := make([]string, 0, 2)
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.URL.Path)
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer dataset-key" {
			t.Fatalf("unexpected authorization header: %s", authorization)
		}
		if request.URL.Path == "/v1/info" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if request.URL.Path != "/v1/datasets" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[]}`))
	}))

	probe, err := NewRegistry(server.Client()).NewProbe(Config{
		Type: domain.IntegrationConnectionTypeDify, APIURL: server.URL + "/v1", APIKey: "dataset-key",
	})
	if err != nil {
		t.Fatalf("create probe: %v", err)
	}
	if err := probe.Run(context.Background()); err != nil {
		t.Fatalf("run probe: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/v1/info" || paths[1] != "/v1/datasets" {
		t.Fatalf("unexpected request paths: %v", paths)
	}
}

// TestDifyKnowledgeBaseLister 验证 Dify 知识库列表按页读取并保留文档模式。
func TestDifyKnowledgeBaseLister(t *testing.T) {
	pages := make([]string, 0, 2)
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/datasets" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer dataset-key" {
			t.Fatalf("unexpected authorization header: %s", authorization)
		}
		if limit := request.URL.Query().Get("limit"); limit != "100" {
			t.Fatalf("unexpected limit: %s", limit)
		}
		page := request.URL.Query().Get("page")
		pages = append(pages, page)
		writer.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			_, _ = writer.Write([]byte(`{"data":[{"id":"dataset-1","name":"产品文档","doc_form":"hierarchical_model"}],"has_more":true}`))
		case "2":
			_, _ = writer.Write([]byte(`{"data":[{"id":"dataset-2","name":"常见问题","doc_form":"qa_model"},{"id":"dataset-3","name":"空知识库","doc_form":null}],"has_more":false}`))
		default:
			t.Fatalf("unexpected page: %s", page)
		}
	}))

	items, err := NewDifyKnowledgeBaseLister(server.Client()).List(context.Background(), DifyKnowledgeBaseConfig{
		APIURL: server.URL + "/v1", APIKey: "dataset-key",
	})
	if err != nil {
		t.Fatalf("list knowledge bases: %v", err)
	}
	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Fatalf("unexpected pages: %v", pages)
	}
	if len(items) != 3 || items[0].ID != "dataset-1" || items[0].DocForm != "hierarchical_model" ||
		items[1].ID != "dataset-2" || items[1].DocForm != "qa_model" ||
		items[2].ID != "dataset-3" || items[2].DocForm != "" {
		t.Fatalf("unexpected knowledge bases: %#v", items)
	}
}

// TestDifyKnowledgeDocumentLister 验证 Dify 知识文档查询参数和展示字段。
func TestDifyKnowledgeDocumentLister(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/datasets/dataset-1/documents" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer dataset-key" {
			t.Fatalf("unexpected authorization header: %s", authorization)
		}
		if page := request.URL.Query().Get("page"); page != "2" {
			t.Fatalf("unexpected page: %s", page)
		}
		if limit := request.URL.Query().Get("limit"); limit != "20" {
			t.Fatalf("unexpected limit: %s", limit)
		}
		if keyword := request.URL.Query().Get("keyword"); keyword != "产品" {
			t.Fatalf("unexpected keyword: %s", keyword)
		}
		if status := request.URL.Query().Get("status"); status != "available" {
			t.Fatalf("unexpected status: %s", status)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"data":[
				{"id":"document-1","name":"产品手册.pdf","display_status":"available","created_at":1787950000},
				{"id":"document-2","name":"常见问题.txt","display_status":"indexing","created_at":1787952000}
			],
			"page":2,"limit":20,"total":22,"has_more":false
		}`))
	}))

	output, err := NewDifyKnowledgeDocumentLister(server.Client()).List(
		context.Background(),
		DifyKnowledgeBaseConfig{APIURL: server.URL + "/v1", APIKey: "dataset-key"},
		"dataset-1",
		DifyKnowledgeDocumentListInput{Keyword: "产品", Status: "available", Page: 2, PageSize: 20},
	)
	if err != nil {
		t.Fatalf("list knowledge documents: %v", err)
	}
	if output.Total != 22 || len(output.Documents) != 2 {
		t.Fatalf("unexpected output: %#v", output)
	}
	if output.Documents[0].Status != "available" || output.Documents[0].CreatedAt == nil ||
		output.Documents[0].CreatedAt.Unix() != 1787950000 {
		t.Fatalf("unexpected first document: %#v", output.Documents[0])
	}
	if output.Documents[1].CreatedAt == nil || output.Documents[1].CreatedAt.Unix() != 1787952000 {
		t.Fatalf("unexpected second document time: %#v", output.Documents[1].CreatedAt)
	}
}

// TestDifyKnowledgeDocumentDetailAndSegments 验证 Dify 文档详情与分段读取契约。
func TestDifyKnowledgeDocumentDetailAndSegments(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer dataset-key" {
			t.Fatalf("unexpected authorization header: %s", authorization)
		}
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/v1/datasets/dataset-1/documents/document-1":
			if metadata := request.URL.Query().Get("metadata"); metadata != "without" {
				t.Fatalf("unexpected metadata: %s", metadata)
			}
			_, _ = writer.Write([]byte(`{
				"id":"document-1","name":"产品手册.pdf","display_status":"available",
				"word_count":520,"hit_count":12,"created_at":1787950000
			}`))
		case "/v1/datasets/dataset-1/documents/document-1/segments":
			query := request.URL.Query()
			if query.Get("keyword") != "安装" || query.Get("status") != "completed" ||
				query.Get("page") != "2" || query.Get("limit") != "20" {
				t.Fatalf("unexpected segment query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{
				"data":[
					{"id":"segment-1","position":21,"content":"如何安装？","answer":null,"word_count":6,"hit_count":3,"status":"completed","enabled":true,"created_at":1787951000},
					{"id":"segment-2","position":22,"content":"","answer":"请下载安装包。","word_count":8,"hit_count":0,"status":"indexing","enabled":false,"created_at":null}
				],
				"page":2,"limit":20,"total":22,"has_more":false,"doc_form":"qa_model"
			}`))
		default:
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
	}))

	reader := NewDifyKnowledgeDocumentLister(server.Client())
	config := DifyKnowledgeBaseConfig{APIURL: server.URL + "/v1", APIKey: "dataset-key"}
	document, err := reader.Get(context.Background(), config, "dataset-1", "document-1")
	if err != nil {
		t.Fatalf("get knowledge document: %v", err)
	}
	if document.ID != "document-1" || document.Status != "available" || document.WordCount == nil ||
		*document.WordCount != 520 ||
		document.HitCount != 12 || document.CreatedAt == nil || document.CreatedAt.Unix() != 1787950000 {
		t.Fatalf("unexpected document: %#v", document)
	}
	page, err := reader.ListSegments(
		context.Background(),
		config,
		"dataset-1",
		"document-1",
		DifyKnowledgeDocumentSegmentListInput{Keyword: "安装", Status: "completed", Page: 2, PageSize: 20},
	)
	if err != nil {
		t.Fatalf("list knowledge document segments: %v", err)
	}
	if page.Total != 22 || len(page.Segments) != 2 || page.Segments[0].Answer != nil ||
		page.Segments[0].Status != "completed" || page.Segments[0].Position != 21 || page.Segments[1].Answer == nil ||
		*page.Segments[1].Answer != "请下载安装包。" || page.Segments[1].Status != "indexing" {
		t.Fatalf("unexpected segment page: %#v", page)
	}
}

// TestDifyKnowledgeDocumentDetailAllowsMissingWordCount 验证详情接口未返回字数时保留未知语义。
func TestDifyKnowledgeDocumentDetailAllowsMissingWordCount(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"id":"document-1","name":"产品手册.pdf","display_status":"available",
			"hit_count":12,"created_at":1787950000
		}`))
	}))

	document, err := NewDifyKnowledgeDocumentLister(server.Client()).Get(
		context.Background(),
		DifyKnowledgeBaseConfig{APIURL: server.URL, APIKey: "dataset-key"},
		"dataset-1",
		"document-1",
	)
	if err != nil || document.WordCount != nil {
		t.Fatalf("document = %#v, error = %v", document, err)
	}
}

// TestDifyKnowledgeDocumentSegmentsRejectMissingState 验证分段缺少索引状态时按协议错误处理。
func TestDifyKnowledgeDocumentSegmentsRejectMissingState(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"data":[{"id":"segment-1","position":1,"content":"内容","word_count":2,"hit_count":0}],
			"page":1,"limit":20,"total":1
		}`))
	}))

	_, err := NewDifyKnowledgeDocumentLister(server.Client()).ListSegments(
		context.Background(),
		DifyKnowledgeBaseConfig{APIURL: server.URL, APIKey: "dataset-key"},
		"dataset-1",
		"document-1",
		DifyKnowledgeDocumentSegmentListInput{Page: 1, PageSize: 20},
	)
	_, kind, classified := connectiontest.Details(err)
	if !classified || kind != connectiontest.FailureProtocol {
		t.Fatalf("error = %v, kind = %q", err, kind)
	}
}

// TestDifyKnowledgeRetriever 验证 Dify 检索请求和命中分段映射。
func TestDifyKnowledgeRetriever(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer dataset-key" {
			t.Fatalf("unexpected authorization header: %s", authorization)
		}
		if request.URL.Path == "/v1/datasets/dataset-1" && request.Method == http.MethodGet {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"indexing_technique":"economy","retrieval_model_dict":null}`))
			return
		}
		if request.Method != http.MethodPost || request.URL.Path != "/v1/datasets/dataset-1/retrieve" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		var query string
		if err := json.Unmarshal(body["query"], &query); err != nil || query != "如何安装？" || len(body) != 2 {
			t.Fatalf("unexpected request body: %#v", body)
		}
		var retrievalModel struct {
			SearchMethod          string  `json:"search_method"`
			RerankingEnable       bool    `json:"reranking_enable"`
			TopK                  int     `json:"top_k"`
			ScoreThresholdEnabled bool    `json:"score_threshold_enabled"`
			ScoreThreshold        float64 `json:"score_threshold"`
		}
		if err := json.Unmarshal(body["retrieval_model"], &retrievalModel); err != nil ||
			retrievalModel.SearchMethod != "keyword_search" || !retrievalModel.RerankingEnable ||
			retrievalModel.TopK != 6 || !retrievalModel.ScoreThresholdEnabled || retrievalModel.ScoreThreshold != 0.75 {
			t.Fatalf("unexpected retrieval model: %#v", retrievalModel)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"records":[
				{"segment":{"id":"segment-2","position":2,"document_id":"document-1","content":"第二步","answer":null,"document":{"id":"document-1","name":"安装手册.md"}},"score":0.82},
				{"segment":{"id":"segment-1","position":1,"document_id":"document-2","content":"问题","answer":"答案"},"score":null},
				{"segment":{"id":"segment-3","position":3,"document_id":"document-3","content":"说明","answer":null},"score":0}
			]
		}`))
	}))

	records, err := NewDifyKnowledgeRetriever(server.Client()).Retrieve(
		context.Background(),
		DifyKnowledgeBaseConfig{APIURL: server.URL + "/v1", APIKey: "dataset-key"},
		"dataset-1",
		"如何安装？",
		domain.KnowledgeRetrievalOptions{
			Method: domain.KnowledgeRetrievalMethodKeyword, RerankingEnabled: true,
			TopK: 6, ScoreThresholdEnabled: true, ScoreThreshold: 0.75,
		},
	)
	if err != nil {
		t.Fatalf("retrieve knowledge base: %v", err)
	}
	if len(records) != 3 || records[0].SegmentID != "segment-2" || records[0].DocumentName != "安装手册.md" ||
		records[0].Score == nil || *records[0].Score != 0.82 || records[1].DocumentName != "" ||
		records[1].Score != nil || records[1].Answer == nil || *records[1].Answer != "答案" ||
		records[2].Score != nil {
		t.Fatalf("unexpected records: %#v", records)
	}
}

// TestDifyKnowledgeRetrieverUsesConfiguredModel 验证检索沿用知识库现有配置。
func TestDifyKnowledgeRetrieverUsesConfiguredModel(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodGet {
			_, _ = writer.Write([]byte(`{
				"indexing_technique":"high_quality",
				"retrieval_model_dict":{
					"search_method":"hybrid_search","reranking_enable":false,"top_k":7,"score_threshold_enabled":false,
					"reranking_model":{"reranking_provider_name":"provider","reranking_model_name":"model"},
					"weights":{"weight_type":"semantic_first"}
				}
			}`))
			return
		}
		var body struct {
			RetrievalModel struct {
				SearchMethod          string         `json:"search_method"`
				RerankingEnable       bool           `json:"reranking_enable"`
				TopK                  int            `json:"top_k"`
				ScoreThresholdEnabled bool           `json:"score_threshold_enabled"`
				ScoreThreshold        float64        `json:"score_threshold"`
				RerankingModel        map[string]any `json:"reranking_model"`
				Weights               map[string]any `json:"weights"`
			} `json:"retrieval_model"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil ||
			body.RetrievalModel.SearchMethod != "semantic_search" || !body.RetrievalModel.RerankingEnable ||
			body.RetrievalModel.TopK != 12 || !body.RetrievalModel.ScoreThresholdEnabled ||
			body.RetrievalModel.ScoreThreshold != 0.4 ||
			body.RetrievalModel.RerankingModel["reranking_model_name"] != "model" ||
			body.RetrievalModel.Weights["weight_type"] != "semantic_first" {
			t.Fatalf("unexpected request body: %#v, error: %v", body, err)
		}
		_, _ = writer.Write([]byte(`{
			"records":[{
				"segment":{"id":"segment-1","position":1,"document_id":"document-1","content":"语义命中"},
				"score":0
			}]
		}`))
	}))

	records, err := NewDifyKnowledgeRetriever(server.Client()).Retrieve(
		context.Background(),
		DifyKnowledgeBaseConfig{APIURL: server.URL, APIKey: "dataset-key"},
		"dataset-1",
		"测试",
		domain.KnowledgeRetrievalOptions{
			Method: domain.KnowledgeRetrievalMethodSemantic, RerankingEnabled: true,
			TopK: 12, ScoreThresholdEnabled: true, ScoreThreshold: 0.4,
		},
	)
	if err != nil || len(records) != 1 || records[0].Score == nil || *records[0].Score != 0 {
		t.Fatalf("records = %#v, error = %v", records, err)
	}
}

// TestDifyKnowledgeRetrieverReportsInvalidField 验证协议错误指出具体记录和字段。
func TestDifyKnowledgeRetrieverReportsInvalidField(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodGet {
			_, _ = writer.Write([]byte(`{"indexing_technique":"high_quality","retrieval_model_dict":null}`))
			return
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		var model struct {
			SearchMethod string `json:"search_method"`
			TopK         int    `json:"top_k"`
		}
		if err := json.Unmarshal(body["retrieval_model"], &model); err != nil ||
			model.SearchMethod != "keyword_search" || model.TopK != 4 {
			t.Fatalf("unexpected retrieval model: %s", body["retrieval_model"])
		}
		_, _ = writer.Write([]byte(`{
			"records":[{"segment":{"id":"segment-1","document_id":"document-1","content":"内容"},"score":0.8}]
		}`))
	}))

	_, err := NewDifyKnowledgeRetriever(server.Client()).Retrieve(
		context.Background(),
		DifyKnowledgeBaseConfig{APIURL: server.URL, APIKey: "dataset-key"},
		"dataset-1",
		"测试",
		domain.KnowledgeRetrievalOptions{
			Method: domain.KnowledgeRetrievalMethodKeyword, TopK: 4, ScoreThreshold: 0.5,
		},
	)
	_, kind, classified := connectiontest.Details(err)
	if !classified || kind != connectiontest.FailureProtocol ||
		!strings.Contains(err.Error(), "record 1 contains invalid segment.position") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestDifyKnowledgeSearchMethod 验证统一检索方式完整映射到 Dify。
func TestDifyKnowledgeSearchMethod(t *testing.T) {
	tests := map[domain.KnowledgeRetrievalMethod]string{
		domain.KnowledgeRetrievalMethodKeyword:  difyKnowledgeSearchMethodKeyword,
		domain.KnowledgeRetrievalMethodSemantic: difyKnowledgeSearchMethodSemantic,
		domain.KnowledgeRetrievalMethodFullText: difyKnowledgeSearchMethodFullText,
		domain.KnowledgeRetrievalMethodHybrid:   difyKnowledgeSearchMethodHybrid,
	}
	for method, expected := range tests {
		actual, valid := difyKnowledgeSearchMethod(method)
		if !valid || actual != expected {
			t.Fatalf("method %q: actual = %q, valid = %v", method, actual, valid)
		}
	}
	if _, valid := difyKnowledgeSearchMethod("future"); valid {
		t.Fatal("unsupported method should fail")
	}
}

// TestDifyKnowledgeRetrieverRejectsEconomySemanticSearch 验证经济模式不会静默改写用户选择。
func TestDifyKnowledgeRetrieverRejectsEconomySemanticSearch(t *testing.T) {
	postCalled := false
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost {
			postCalled = true
		}
		_, _ = writer.Write([]byte(`{"indexing_technique":"economy","retrieval_model_dict":null}`))
	}))

	_, err := NewDifyKnowledgeRetriever(server.Client()).Retrieve(
		context.Background(),
		DifyKnowledgeBaseConfig{APIURL: server.URL, APIKey: "dataset-key"},
		"dataset-1",
		"测试",
		domain.KnowledgeRetrievalOptions{
			Method: domain.KnowledgeRetrievalMethodSemantic, TopK: 4, ScoreThreshold: 0.5,
		},
	)
	_, kind, classified := connectiontest.Details(err)
	if !classified || kind != connectiontest.FailureInvalidConfig || postCalled {
		t.Fatalf("error = %v, kind = %q, post called = %v", err, kind, postCalled)
	}
}

// TestN8NProbe 验证 n8n 探测使用公开工作流接口和 API 密钥请求头。
func TestN8NProbe(t *testing.T) {
	server := httptest.NewTestServer(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/instance/api/v1/workflows" {
			t.Fatalf("unexpected request path: %s", request.URL.Path)
		}
		if limit := request.URL.Query().Get("limit"); limit != "1" {
			t.Fatalf("unexpected limit: %s", limit)
		}
		if apiKey := request.Header.Get("X-N8N-API-KEY"); apiKey != "n8n-key" {
			t.Fatalf("unexpected api key: %s", apiKey)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[]}`))
	}))

	probe, err := NewRegistry(server.Client()).NewProbe(Config{
		Type: domain.IntegrationConnectionTypeN8N, APIURL: server.URL + "/instance", APIKey: "n8n-key",
	})
	if err != nil {
		t.Fatalf("create probe: %v", err)
	}
	if err := probe.Run(context.Background()); err != nil {
		t.Fatalf("run probe: %v", err)
	}
}

// TestProbeDoesNotFollowRedirects 验证探测请求不会把凭据带到重定向地址。
func TestProbeDoesNotFollowRedirects(t *testing.T) {
	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected = true
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusFound)
	}))
	defer origin.Close()

	probe, err := NewRegistry(NewHTTPClient()).NewProbe(Config{
		Type: domain.IntegrationConnectionTypeN8N, APIURL: origin.URL, APIKey: "n8n-key",
	})
	if err != nil {
		t.Fatalf("create probe: %v", err)
	}
	err = probe.Run(context.Background())
	_, kind, classified := connectiontest.Details(err)
	if !classified || kind != connectiontest.FailureProtocol {
		t.Fatalf("unexpected probe error: %v", err)
	}
	if redirected {
		t.Fatal("redirect target received the probe request")
	}
}
