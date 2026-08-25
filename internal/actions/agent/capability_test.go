//go:build server

package agent

import (
	"errors"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/common"
)

// TestNormalizeCapabilityInputTrimsValues 验证能力配置文本字段会被规范化。
func TestNormalizeCapabilityInputTrimsValues(t *testing.T) {
	input, err := normalizeCapabilityInput(CapabilityInput{
		ProviderID:        " 019c7f37-8c0b-7ef0-8eca-cb672194d28d ",
		ModelIdentifier:   " chat-model ",
		SystemInstruction: " 负责回答产品问题。 ",
	})
	if err != nil {
		t.Fatalf("normalizeCapabilityInput() error = %v", err)
	}
	if input.ProviderID != "019c7f37-8c0b-7ef0-8eca-cb672194d28d" || input.ModelIdentifier != "chat-model" || input.SystemInstruction != "负责回答产品问题。" {
		t.Fatalf("normalizeCapabilityInput() = %#v", input)
	}
}

// TestNormalizeCapabilityInputRejectsRequiredFields 验证无效模型和空工作指令会被拒绝。
func TestNormalizeCapabilityInputRejectsRequiredFields(t *testing.T) {
	fields := capabilityValidationFields(t, CapabilityInput{ProviderID: "invalid"})
	if fields["providerId"] != ValidationModelInvalid || fields["modelIdentifier"] != ValidationModelInvalid || fields["systemInstruction"] != ValidationSystemInstructionRequired {
		t.Fatalf("normalizeCapabilityInput() fields = %#v", fields)
	}
}

// TestNormalizeCapabilityInputRejectsLongInstruction 验证工作指令使用字符数限制。
func TestNormalizeCapabilityInputRejectsLongInstruction(t *testing.T) {
	fields := capabilityValidationFields(t, CapabilityInput{
		ProviderID:        "019c7f37-8c0b-7ef0-8eca-cb672194d28d",
		ModelIdentifier:   "chat-model",
		SystemInstruction: strings.Repeat("鹿", maxSystemInstructionLength+1),
	})
	if fields["systemInstruction"] != ValidationSystemInstructionTooLong {
		t.Fatalf("normalizeCapabilityInput() fields = %#v", fields)
	}
}

// capabilityValidationFields 返回能力配置字段校验结果。
func capabilityValidationFields(t *testing.T, input CapabilityInput) map[string]common.FieldCode {
	t.Helper()
	_, err := normalizeCapabilityInput(input)
	var fieldError *common.FieldError
	if !errors.As(err, &fieldError) {
		t.Fatalf("normalizeCapabilityInput() error = %v", err)
	}
	return fieldError.Fields
}
