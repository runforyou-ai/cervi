//go:build server

package knowledgebase

import (
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestNormalizeInput 验证知识库字段会被规范化并校验长度。
func TestNormalizeInput(t *testing.T) {
	input, fields := normalizeInput(Input{Name: "  产品知识  ", Category: domain.KnowledgeBaseCategoryStandard, Description: "  产品资料  "})
	if len(fields) != 0 || input.Name != "产品知识" || input.Description != "产品资料" {
		t.Fatalf("input = %#v, fields = %#v", input, fields)
	}
	_, fields = normalizeInput(Input{
		Name:        strings.Repeat("知", domain.KnowledgeBaseNameMaxLength+1),
		Category:    domain.KnowledgeBaseCategoryStandard,
		Description: strings.Repeat("识", domain.KnowledgeBaseDescriptionMaxLength+1),
	})
	if fields["name"] != ValidationNameTooLong || fields["description"] != ValidationDescriptionTooLong {
		t.Fatalf("fields = %#v", fields)
	}
	_, fields = normalizeInput(Input{Name: "   ", Category: domain.KnowledgeBaseCategoryStandard})
	if fields["name"] != ValidationNameRequired {
		t.Fatalf("fields = %#v", fields)
	}
	_, fields = normalizeInput(Input{Name: "产品知识", Category: "external"})
	if fields["category"] != ValidationCategoryInvalid {
		t.Fatalf("fields = %#v", fields)
	}
	_, fields = normalizeInput(Input{
		Name:                    "外部知识",
		Category:                domain.KnowledgeBaseCategoryStandard,
		IntegrationConnectionID: "invalid",
	})
	if fields["integrationConnectionId"] != ValidationIntegrationConnectionInvalid || fields["externalResourceId"] != ValidationExternalResourceRequired {
		t.Fatalf("fields = %#v", fields)
	}
	_, fields = normalizeInput(Input{
		Name:               "外部知识",
		Category:           domain.KnowledgeBaseCategoryStandard,
		ExternalResourceID: "remote-dataset",
	})
	if fields["integrationConnectionId"] != ValidationIntegrationConnectionInvalid {
		t.Fatalf("fields = %#v", fields)
	}
	input, fields = normalizeInput(Input{
		Name:                    "外部知识",
		Category:                domain.KnowledgeBaseCategoryStandard,
		IntegrationConnectionID: " 019c91a2-7b4e-7e52-a1c9-6f0d8b3a2e14 ",
		ExternalResourceID:      " remote-dataset ",
	})
	if len(fields) != 0 || input.IntegrationConnectionID != "019c91a2-7b4e-7e52-a1c9-6f0d8b3a2e14" || input.ExternalResourceID != "remote-dataset" {
		t.Fatalf("input = %#v, fields = %#v", input, fields)
	}
}

// TestCategoryFromDifyDocForm 验证 Dify 文档模式映射为 Cervi 知识库类型。
func TestCategoryFromDifyDocForm(t *testing.T) {
	tests := map[string]domain.KnowledgeBaseCategory{
		"":                   domain.KnowledgeBaseCategoryStandard,
		"text_model":         domain.KnowledgeBaseCategoryStandard,
		"hierarchical_model": domain.KnowledgeBaseCategoryStandard,
		"qa_model":           domain.KnowledgeBaseCategoryQA,
	}
	for docForm, expected := range tests {
		actual, err := categoryFromDifyDocForm(docForm)
		if err != nil || actual != expected {
			t.Fatalf("doc_form %q: category = %q, error = %v", docForm, actual, err)
		}
	}
	if _, err := categoryFromDifyDocForm("future_model"); err == nil {
		t.Fatal("unsupported doc_form should fail")
	}
}

// TestNormalizeGroupInput 验证分组名称和上级编号校验。
func TestNormalizeGroupInput(t *testing.T) {
	input, fields := normalizeGroupInput(GroupInput{Name: "  产品  "})
	if len(fields) != 0 || input.Name != "产品" {
		t.Fatalf("input = %#v, fields = %#v", input, fields)
	}
	_, fields = normalizeGroupInput(GroupInput{Name: strings.Repeat("组", domain.KnowledgeGroupNameMaxLength+1), ParentID: "invalid"})
	if fields["name"] != ValidationGroupNameTooLong || fields["parentId"] != ValidationGroupParentInvalid {
		t.Fatalf("fields = %#v", fields)
	}
	_, fields = normalizeGroupInput(GroupInput{Name: "   "})
	if fields["name"] != ValidationGroupNameRequired {
		t.Fatalf("fields = %#v", fields)
	}
}

// TestNormalizeDocumentListInput 验证知识文档查询条件规范化和分页范围。
func TestNormalizeDocumentListInput(t *testing.T) {
	input, fields := normalizeDocumentListInput(DocumentListInput{Keyword: "  产品  ", Status: " ready "})
	if len(fields) != 0 || input.Keyword != "产品" || input.Status != domain.KnowledgeDocumentStatusReady ||
		input.Page != 1 || input.PageSize != defaultKnowledgeDocumentPageSize {
		t.Fatalf("input = %#v, fields = %#v", input, fields)
	}
	_, fields = normalizeDocumentListInput(DocumentListInput{Page: 2, PageSize: 101})
	if fields["pageSize"] != ValidationDocumentQueryInvalid {
		t.Fatalf("fields = %#v", fields)
	}
}

// TestNormalizeDocumentSegmentListInput 验证知识文档分段查询条件规范化和分页范围。
func TestNormalizeDocumentSegmentListInput(t *testing.T) {
	input, fields := normalizeDocumentSegmentListInput(DocumentSegmentListInput{Keyword: "  安装  ", Status: " completed "})
	if len(fields) != 0 || input.Keyword != "安装" || input.Status != domain.KnowledgeDocumentSegmentIndexStatusCompleted || input.Page != 1 ||
		input.PageSize != defaultKnowledgeDocumentPageSize {
		t.Fatalf("input = %#v, fields = %#v", input, fields)
	}
	_, fields = normalizeDocumentSegmentListInput(DocumentSegmentListInput{Page: 2, PageSize: 101})
	if fields["pageSize"] != ValidationDocumentQueryInvalid {
		t.Fatalf("fields = %#v", fields)
	}
	_, fields = normalizeDocumentSegmentListInput(DocumentSegmentListInput{Status: "future_status"})
	if fields["status"] != ValidationDocumentQueryInvalid {
		t.Fatalf("fields = %#v", fields)
	}
}

// TestNormalizeRetrievalInput 验证检索内容会被去除首尾空白并限制长度。
func TestNormalizeRetrievalInput(t *testing.T) {
	input, fields := normalizeRetrievalInput(RetrievalInput{
		Query: "  如何安装？  ",
	})
	if len(fields) != 0 || input.Query != "如何安装？" {
		t.Fatalf("input = %#v, fields = %#v", input, fields)
	}
	_, fields = normalizeRetrievalInput(RetrievalInput{
		Query: "   ",
	})
	if fields["query"] != ValidationRetrievalQueryRequired {
		t.Fatalf("fields = %#v", fields)
	}
	_, fields = normalizeRetrievalInput(RetrievalInput{
		Query: strings.Repeat("问", domain.KnowledgeRetrievalQueryMaxLength+1),
	})
	if fields["query"] != ValidationRetrievalQueryTooLong {
		t.Fatalf("fields = %#v", fields)
	}
}

// TestKnowledgeDocumentStatusMapping 验证 Dify 与统一文档状态的双向映射。
func TestKnowledgeDocumentStatusMapping(t *testing.T) {
	tests := map[string]domain.KnowledgeDocumentStatus{
		"queuing":   domain.KnowledgeDocumentStatusQueued,
		"indexing":  domain.KnowledgeDocumentStatusProcessing,
		"available": domain.KnowledgeDocumentStatusReady,
		"paused":    domain.KnowledgeDocumentStatusPaused,
		"error":     domain.KnowledgeDocumentStatusError,
		"disabled":  domain.KnowledgeDocumentStatusDisabled,
		"archived":  domain.KnowledgeDocumentStatusArchived,
	}
	for status, expected := range tests {
		actual, err := knowledgeDocumentStatusFromDify(status)
		if err != nil || actual != expected {
			t.Fatalf("status %q: actual = %q, error = %v", status, actual, err)
		}
	}
	if _, err := knowledgeDocumentStatusFromDify("future_status"); err == nil {
		t.Fatal("unsupported status should fail")
	}
	for difyStatus, expected := range tests {
		actual, ok := knowledgeDocumentStatusToDify(expected)
		if !ok || actual != difyStatus {
			t.Fatalf("status %q: dify status = %q, valid = %v", expected, actual, ok)
		}
	}
	if status, ok := knowledgeDocumentStatusToDify(""); !ok || status != "" {
		t.Fatalf("empty status: dify status = %q, valid = %v", status, ok)
	}
	if _, ok := knowledgeDocumentStatusToDify("future_status"); ok {
		t.Fatal("unsupported filter status should fail")
	}
}

// TestKnowledgeDocumentSegmentIndexStatusMapping 验证 Dify 分段索引状态的完整映射。
func TestKnowledgeDocumentSegmentIndexStatusMapping(t *testing.T) {
	tests := map[string]domain.KnowledgeDocumentSegmentIndexStatus{
		"waiting":    domain.KnowledgeDocumentSegmentIndexStatusWaiting,
		"indexing":   domain.KnowledgeDocumentSegmentIndexStatusIndexing,
		"completed":  domain.KnowledgeDocumentSegmentIndexStatusCompleted,
		"error":      domain.KnowledgeDocumentSegmentIndexStatusError,
		"paused":     domain.KnowledgeDocumentSegmentIndexStatusPaused,
		"re_segment": domain.KnowledgeDocumentSegmentIndexStatusResegment,
	}
	for status, expected := range tests {
		actual, err := knowledgeDocumentSegmentIndexStatusFromDify(status)
		if err != nil || actual != expected {
			t.Fatalf("status %q: actual = %q, error = %v", status, actual, err)
		}
	}
	if _, err := knowledgeDocumentSegmentIndexStatusFromDify("future_status"); err == nil {
		t.Fatal("unsupported status should fail")
	}
}
