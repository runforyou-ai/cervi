//go:build server

package knowledgebase

import (
	"strings"
	"unicode/utf8"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const (
	ValidationEmbeddingModelInvalid     common.FieldCode = "KNOWLEDGE_BASE_EMBEDDING_MODEL_INVALID"
	ValidationEmbeddingDimensionInvalid common.FieldCode = "KNOWLEDGE_BASE_EMBEDDING_DIMENSION_INVALID"
	ValidationChunkLengthInvalid        common.FieldCode = "KNOWLEDGE_BASE_CHUNK_LENGTH_INVALID"
	ValidationChunkOverlapInvalid       common.FieldCode = "KNOWLEDGE_BASE_CHUNK_OVERLAP_INVALID"
	ValidationRetrievalCountInvalid     common.FieldCode = "KNOWLEDGE_BASE_RETRIEVAL_COUNT_INVALID"
	ValidationRerankModelInvalid        common.FieldCode = "KNOWLEDGE_BASE_RERANK_MODEL_INVALID"

	ValidationQAQuestionRequired common.FieldCode = "KNOWLEDGE_QA_QUESTION_REQUIRED"
	ValidationQAAnswerRequired   common.FieldCode = "KNOWLEDGE_QA_ANSWER_REQUIRED"
	ValidationQAGroupInvalid     common.FieldCode = "KNOWLEDGE_QA_GROUP_INVALID"
	ValidationQAContentInvalid   common.FieldCode = "KNOWLEDGE_QA_CONTENT_INVALID"
	ValidationNameRequired       common.FieldCode = "KNOWLEDGE_BASE_NAME_REQUIRED"
	ValidationNameTooLong        common.FieldCode = "KNOWLEDGE_BASE_NAME_TOO_LONG"
	ValidationNameDuplicate      common.FieldCode = "KNOWLEDGE_BASE_NAME_DUPLICATE"
	ValidationCategoryInvalid    common.FieldCode = "KNOWLEDGE_BASE_CATEGORY_INVALID"
	ValidationDescriptionTooLong common.FieldCode = "KNOWLEDGE_BASE_DESCRIPTION_TOO_LONG"
	ValidationGroupNameRequired  common.FieldCode = "KNOWLEDGE_GROUP_NAME_REQUIRED"
	ValidationGroupNameTooLong   common.FieldCode = "KNOWLEDGE_GROUP_NAME_TOO_LONG"
	ValidationGroupNameDuplicate common.FieldCode = "KNOWLEDGE_GROUP_NAME_DUPLICATE"
	ValidationGroupParentInvalid common.FieldCode = "KNOWLEDGE_GROUP_PARENT_INVALID"
)

// normalizeInput 规范化并校验知识库字段。
func normalizeInput(input Input) (Input, map[string]common.FieldCode) {
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.EmbeddingProviderID = strings.TrimSpace(input.EmbeddingProviderID)
	input.EmbeddingModelIdentifier = strings.TrimSpace(input.EmbeddingModelIdentifier)
	input.RerankProviderID = strings.TrimSpace(input.RerankProviderID)
	input.RerankModelIdentifier = strings.TrimSpace(input.RerankModelIdentifier)
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
	if !common.ValidUUID(input.EmbeddingProviderID) || input.EmbeddingModelIdentifier == "" {
		fields["embeddingModelIdentifier"] = ValidationEmbeddingModelInvalid
	}
	if input.EmbeddingDimension <= 0 {
		fields["embeddingDimension"] = ValidationEmbeddingDimensionInvalid
	}
	if input.Category == domain.KnowledgeBaseCategoryQA {
		input.ChunkLength, input.ChunkOverlap = nil, nil
	} else {
		if input.ChunkLength == nil || *input.ChunkLength < 256 || *input.ChunkLength > 2048 {
			fields["chunkLength"] = ValidationChunkLengthInvalid
		}
		if input.ChunkOverlap == nil || *input.ChunkOverlap < 0 || *input.ChunkOverlap > 200 {
			fields["chunkOverlap"] = ValidationChunkOverlapInvalid
		}
	}
	if input.RetrievalCount < 1 || input.RetrievalCount > 20 {
		fields["retrievalCount"] = ValidationRetrievalCountInvalid
	}
	if (input.RerankProviderID != "" || input.RerankModelIdentifier != "") &&
		(!common.ValidUUID(input.RerankProviderID) || input.RerankModelIdentifier == "") {
		fields["rerankModelIdentifier"] = ValidationRerankModelInvalid
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
