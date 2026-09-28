//go:build server

package direct

import (
	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// knowledgeIndexPresentation 把来源索引阶段映射为展示状态，并本地化失败原因。
func knowledgeIndexPresentation(meta appservice.RequestMeta, stage domain.KnowledgeIndexStatus, failureCode string) (appservice.KnowledgeIndexStatus, string) {
	message := ""
	if failureCode != "" {
		key := cervii18n.ErrorKnowledgeProcessingFailed
		switch failureCode {
		case "file_read_failed":
			key = cervii18n.ErrorKnowledgeOriginalReadFailed
		case "empty_content":
			key = cervii18n.ErrorKnowledgeContentEmpty
		case "parse_failed", "unsupported_file":
			key = cervii18n.ErrorKnowledgeParseFailed
		case "url_unreachable", "url_invalid":
			key = cervii18n.ErrorKnowledgePageUnreachable
		case "url_content_unsupported":
			key = cervii18n.ErrorKnowledgePageUnsupported
		case "url_content_too_large":
			key = cervii18n.ErrorKnowledgePageTooLarge
		case "embedding_model_unavailable":
			key = cervii18n.ErrorKnowledgeEmbeddingUnavailable
		case "embedding_failed":
			key = cervii18n.ErrorKnowledgeEmbeddingFailed
		case "embedding_dimension_mismatch":
			key = cervii18n.ErrorKnowledgeEmbeddingDimension
		}
		message, _ = cervii18n.Localize(string(meta.Locale), key)
	}
	status := appservice.KnowledgeIndexRunning
	switch stage {
	case domain.KnowledgeIndexInitial:
		status = appservice.KnowledgeIndexInitial
	case domain.KnowledgeIndexQueued:
		status = appservice.KnowledgeIndexQueued
	case domain.KnowledgeIndexSucceeded:
		status = appservice.KnowledgeIndexSucceeded
	case domain.KnowledgeIndexFailed:
		status = appservice.KnowledgeIndexFailed
	case domain.KnowledgeIndexCancelled:
		status = appservice.KnowledgeIndexCancelled
	}
	return status, message
}
