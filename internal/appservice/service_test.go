package appservice

import (
	"context"
	"errors"
	"testing"
)

type stubBackend struct {
	Backend
}

// TestPlatformMethodsRequireCapability 验证未实现平台能力的 Backend 返回方法不允许。
func TestPlatformMethodsRequireCapability(t *testing.T) {
	service := New(&stubBackend{})
	meta := RequestMeta{Locale: LocaleChineseSimplified}

	_, err := service.InstallWorkspace(context.Background(), meta, InstallWorkspaceInput{})
	assertMethodNotAllowed(t, err)

	_, err = service.ServerURL(context.Background(), meta)
	assertMethodNotAllowed(t, err)

	_, err = service.ProbeServer(context.Background(), meta, "https://cervi.example.com")
	assertMethodNotAllowed(t, err)

	err = service.ConnectServer(context.Background(), meta, "https://cervi.example.com")
	assertMethodNotAllowed(t, err)
}

func assertMethodNotAllowed(t *testing.T, err error) {
	t.Helper()
	var apiError *Error
	if !errors.As(err, &apiError) || apiError.Code != "METHOD_NOT_ALLOWED" {
		t.Fatalf("error = %#v, want METHOD_NOT_ALLOWED", err)
	}
}
