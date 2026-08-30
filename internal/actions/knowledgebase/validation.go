//go:build server

package knowledgebase

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const (
	ValidationNameRequired                   common.FieldCode = "KNOWLEDGE_BASE_NAME_REQUIRED"
	ValidationNameTooLong                    common.FieldCode = "KNOWLEDGE_BASE_NAME_TOO_LONG"
	ValidationNameDuplicate                  common.FieldCode = "KNOWLEDGE_BASE_NAME_DUPLICATE"
	ValidationCategoryInvalid                common.FieldCode = "KNOWLEDGE_BASE_CATEGORY_INVALID"
	ValidationDescriptionTooLong             common.FieldCode = "KNOWLEDGE_BASE_DESCRIPTION_TOO_LONG"
	ValidationIntegrationConnectionInvalid   common.FieldCode = "KNOWLEDGE_BASE_INTEGRATION_CONNECTION_INVALID"
	ValidationExternalResourceRequired       common.FieldCode = "KNOWLEDGE_BASE_EXTERNAL_RESOURCE_REQUIRED"
	ValidationExternalResourceTooLong        common.FieldCode = "KNOWLEDGE_BASE_EXTERNAL_RESOURCE_TOO_LONG"
	ValidationExternalResourceDuplicate      common.FieldCode = "KNOWLEDGE_BASE_EXTERNAL_RESOURCE_DUPLICATE"
	ValidationGroupNameRequired              common.FieldCode = "KNOWLEDGE_GROUP_NAME_REQUIRED"
	ValidationGroupNameTooLong               common.FieldCode = "KNOWLEDGE_GROUP_NAME_TOO_LONG"
	ValidationGroupNameDuplicate             common.FieldCode = "KNOWLEDGE_GROUP_NAME_DUPLICATE"
	ValidationGroupParentInvalid             common.FieldCode = "KNOWLEDGE_GROUP_PARENT_INVALID"
	ValidationDocumentQueryInvalid           common.FieldCode = "KNOWLEDGE_DOCUMENT_QUERY_INVALID"
	ValidationRetrievalQueryRequired         common.FieldCode = "KNOWLEDGE_RETRIEVAL_QUERY_REQUIRED"
	ValidationRetrievalQueryTooLong          common.FieldCode = "KNOWLEDGE_RETRIEVAL_QUERY_TOO_LONG"
	ValidationRetrievalMethodInvalid         common.FieldCode = "KNOWLEDGE_RETRIEVAL_METHOD_INVALID"
	ValidationRetrievalTopKInvalid           common.FieldCode = "KNOWLEDGE_RETRIEVAL_TOP_K_INVALID"
	ValidationRetrievalScoreThresholdInvalid common.FieldCode = "KNOWLEDGE_RETRIEVAL_SCORE_THRESHOLD_INVALID"
)

// normalizeInput 规范化并校验知识库字段。
func normalizeInput(input Input) (Input, map[string]common.FieldCode) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.IntegrationConnectionID = strings.TrimSpace(input.IntegrationConnectionID)
	input.ExternalResourceID = strings.TrimSpace(input.ExternalResourceID)
	fields := make(map[string]common.FieldCode)
	if input.Name == "" {
		fields["name"] = ValidationNameRequired
	} else if utf8.RuneCountInString(input.Name) > domain.KnowledgeBaseNameMaxLength {
		fields["name"] = ValidationNameTooLong
	}
	if input.Category != domain.KnowledgeBaseCategoryStandard && input.Category != domain.KnowledgeBaseCategoryQA {
		fields["category"] = ValidationCategoryInvalid
	}
	if utf8.RuneCountInString(input.Description) > domain.KnowledgeBaseDescriptionMaxLength {
		fields["description"] = ValidationDescriptionTooLong
	}
	hasIntegration := input.IntegrationConnectionID != ""
	hasExternalResource := input.ExternalResourceID != ""
	if hasIntegration && !common.ValidUUID(input.IntegrationConnectionID) {
		fields["integrationConnectionId"] = ValidationIntegrationConnectionInvalid
	} else if !hasIntegration && hasExternalResource {
		fields["integrationConnectionId"] = ValidationIntegrationConnectionInvalid
	}
	if hasIntegration && !hasExternalResource {
		fields["externalResourceId"] = ValidationExternalResourceRequired
	} else if utf8.RuneCountInString(input.ExternalResourceID) > domain.KnowledgeBaseExternalResourceIDMaxLength {
		fields["externalResourceId"] = ValidationExternalResourceTooLong
	}
	return input, fields
}

// normalizeGroupInput 规范化并校验知识库分组字段。
func normalizeGroupInput(input GroupInput) (GroupInput, map[string]common.FieldCode) {
	input.Name = strings.TrimSpace(input.Name)
	input.ParentID = strings.TrimSpace(input.ParentID)
	fields := make(map[string]common.FieldCode)
	if input.Name == "" {
		fields["name"] = ValidationGroupNameRequired
	} else if utf8.RuneCountInString(input.Name) > domain.KnowledgeGroupNameMaxLength {
		fields["name"] = ValidationGroupNameTooLong
	}
	if input.ParentID != "" && !common.ValidUUID(input.ParentID) {
		fields["parentId"] = ValidationGroupParentInvalid
	}
	return input, fields
}

// normalizeRetrievalInput 规范化并校验知识库检索条件。
func normalizeRetrievalInput(input RetrievalInput) (RetrievalInput, map[string]common.FieldCode) {
	input.Query = strings.TrimSpace(input.Query)
	fields := make(map[string]common.FieldCode)
	if input.Query == "" {
		fields["query"] = ValidationRetrievalQueryRequired
	} else if utf8.RuneCountInString(input.Query) > domain.KnowledgeRetrievalQueryMaxLength {
		fields["query"] = ValidationRetrievalQueryTooLong
	}
	switch input.Method {
	case domain.KnowledgeRetrievalMethodKeyword,
		domain.KnowledgeRetrievalMethodSemantic,
		domain.KnowledgeRetrievalMethodFullText,
		domain.KnowledgeRetrievalMethodHybrid:
	default:
		fields["method"] = ValidationRetrievalMethodInvalid
	}
	if input.TopK < 1 || input.TopK > domain.KnowledgeRetrievalTopKMax {
		fields["topK"] = ValidationRetrievalTopKInvalid
	}
	if input.ScoreThresholdEnabled &&
		(math.IsNaN(input.ScoreThreshold) || math.IsInf(input.ScoreThreshold, 0) ||
			input.ScoreThreshold < 0 || input.ScoreThreshold > 1) {
		fields["scoreThreshold"] = ValidationRetrievalScoreThresholdInvalid
	}
	return input, fields
}
