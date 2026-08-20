package appservice

import (
	"errors"
	"testing"
)

// TestMarshalErrorPreservesStructuredCause 验证 Wails 错误序列化保留业务字段。
func TestMarshalErrorPreservesStructuredCause(t *testing.T) {
	payload := MarshalError(&Error{
		Kind:    ErrorKindInvalid,
		Message: "输入有误。",
		Fields:  map[string]string{"name": "请输入名称。"},
	})
	if string(payload) != `{"kind":"invalid","message":"输入有误。","fields":{"name":"请输入名称。"}}` {
		t.Fatalf("payload = %s", payload)
	}
}

// TestMarshalErrorFallsBackForUnknownErrors 验证未知错误交给 Wails 默认序列化。
func TestMarshalErrorFallsBackForUnknownErrors(t *testing.T) {
	if payload := MarshalError(errors.New("unexpected")); payload != nil {
		t.Fatalf("payload = %s, want nil", payload)
	}
}
