//go:build server

package appservice

import (
	"context"
	"errors"
	"net/http"
	"testing"

	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// TestDirectBackendPreservesCancellation 验证请求取消不会转换为业务错误。
func TestDirectBackendPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := (&DirectBackend{}).contactError(ctx, RequestMeta{}, errors.New("query failed"), cervii18n.ErrorContactReadFailed)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

// TestKnowledgeDocumentReadMapsRemoteNotFound 验证 Dify 文档 404 转换为文档不存在。
func TestKnowledgeDocumentReadMapsRemoteNotFound(t *testing.T) {
	backend := &DirectBackend{}
	err := backend.knowledgeDocumentReadError(
		context.Background(),
		RequestMeta{Locale: LocaleChineseSimplified},
		connectiontest.NewError(connectiontest.StageCapability, connectiontest.FailureNotFound, errors.New("not found")),
		cervii18n.ErrorKnowledgeDocumentReadFailed,
		"organization-1",
		"knowledge-base-1",
		"document-1",
	)
	converted, ok := err.(*Error)
	if !ok || converted.Kind != ErrorKindNotFound || converted.HTTPStatus() != http.StatusNotFound ||
		converted.Message != "没有找到这个知识文档。" {
		t.Fatalf("error = %#v", err)
	}
}

// TestKnowledgeRetrievalKeepsRemoteNotFoundGeneric 验证远端知识库 404 不冒充本地资源不存在。
func TestKnowledgeRetrievalKeepsRemoteNotFoundGeneric(t *testing.T) {
	backend := &DirectBackend{}
	err := backend.knowledgeRetrievalError(
		context.Background(),
		RequestMeta{Locale: LocaleChineseSimplified},
		connectiontest.NewError(connectiontest.StageCapability, connectiontest.FailureNotFound, errors.New("not found")),
		"organization-1",
		"knowledge-base-1",
		0,
		KnowledgeRetrievalInput{},
	)
	converted, ok := err.(*Error)
	if !ok || converted.Kind != ErrorKindFailed || converted.HTTPStatus() != http.StatusInternalServerError ||
		converted.Message != "检索知识库失败。" {
		t.Fatalf("error = %#v", err)
	}
}
