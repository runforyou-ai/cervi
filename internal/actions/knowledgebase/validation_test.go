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
