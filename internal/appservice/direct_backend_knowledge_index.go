//go:build server

package appservice

import (
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// knowledgeIndexPresentation 把来源索引阶段映射为展示状态，并本地化失败原因。
func knowledgeIndexPresentation(meta RequestMeta, stage domain.KnowledgeIndexStatus, failureCode string) (KnowledgeIndexStatus, string) {
	message := ""
	if failureCode != "" {
		key := cervii18n.ErrorKnowledgeProcessingFailed
		switch failureCode {
		case "unavailable":
			key = cervii18n.ErrorKnowledgeProcessingUnavailable
		case "connection_timeout":
			key = cervii18n.ErrorKnowledgeConnectionTimeout
		case "request_timeout":
			key = cervii18n.ErrorKnowledgeRequestTimeout
		case "file_read_failed":
			key = cervii18n.ErrorKnowledgeOriginalReadFailed
		case "empty_content":
			key = cervii18n.ErrorKnowledgeContentEmpty
		case "parse_failed", "unsupported_file":
			key = cervii18n.ErrorKnowledgeParseFailed
		case "embedding_model_unavailable":
			key = cervii18n.ErrorKnowledgeEmbeddingUnavailable
		case "embedding_failed":
			key = cervii18n.ErrorKnowledgeEmbeddingFailed
		case "embedding_dimension_mismatch":
			key = cervii18n.ErrorKnowledgeEmbeddingDimension
		}
		message, _ = cervii18n.Localize(string(meta.Locale), key)
	}
	status := KnowledgeIndexRunning
	switch stage {
	case domain.KnowledgeIndexInitial:
		status = KnowledgeIndexInitial
	case domain.KnowledgeIndexQueued:
		status = KnowledgeIndexQueued
	case domain.KnowledgeIndexSucceeded:
		status = KnowledgeIndexSucceeded
	case domain.KnowledgeIndexFailed:
		status = KnowledgeIndexFailed
	case domain.KnowledgeIndexCancelled:
		status = KnowledgeIndexCancelled
	}
	return status, message
}
