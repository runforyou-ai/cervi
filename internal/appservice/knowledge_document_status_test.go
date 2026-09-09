//go:build server

package appservice

import (
	knowledgeaction "github.com/runforyou-ai/cervi/internal/actions/knowledgebase"
	"github.com/runforyou-ai/cervi/internal/domain"
	"testing"
)

// TestKnowledgeDocumentPresentationStatus 验证文档执行状态到展示状态的映射。
func TestKnowledgeDocumentPresentationStatus(t *testing.T) {
	cases := map[domain.KnowledgeDocumentStatus]KnowledgeDocumentStatus{
		domain.KnowledgeDocumentInitial:     KnowledgeDocumentInitial,
		domain.KnowledgeDocumentQueued:      KnowledgeDocumentQueued,
		domain.KnowledgeDocumentFetching:    KnowledgeDocumentRunning,
		domain.KnowledgeDocumentConverting:  KnowledgeDocumentRunning,
		domain.KnowledgeDocumentExtracting:  KnowledgeDocumentRunning,
		domain.KnowledgeDocumentRecognizing: KnowledgeDocumentRunning,
		domain.KnowledgeDocumentSplitting:   KnowledgeDocumentRunning,
		domain.KnowledgeDocumentEmbedding:   KnowledgeDocumentRunning,
		domain.KnowledgeDocumentIndexing:    KnowledgeDocumentRunning,
		domain.KnowledgeDocumentPublishing:  KnowledgeDocumentRunning,
		domain.KnowledgeDocumentSucceeded:   KnowledgeDocumentSucceeded,
		domain.KnowledgeDocumentFailed:      KnowledgeDocumentFailed,
		domain.KnowledgeDocumentCancelled:   KnowledgeDocumentCancelled,
	}
	for technical, expected := range cases {
		actual := knowledgeDocumentFromAction(RequestMeta{}, knowledgeaction.DocumentRecord{Status: technical})
		if actual.Status != expected || string(actual.ProcessingStatus) != string(technical) {
			t.Fatalf("%s mapped to %+v", technical, actual)
		}
	}
}
