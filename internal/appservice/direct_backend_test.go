//go:build server

package appservice

import (
	"context"
	"errors"
	"testing"

	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
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
