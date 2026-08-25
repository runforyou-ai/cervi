//go:build server

package agent

import (
	"errors"
	"strings"
	"testing"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestNormalizeExecutionInputTrimsValues 验证平台托管执行配置文本字段会被规范化。
func TestNormalizeExecutionInputTrimsValues(t *testing.T) {
	input, err := normalizeExecutionInput(ExecutionInput{
		Mode: domain.AgentExecutionModeManaged,
		Managed: &ManagedExecutionInput{
			ProviderID:        " 019c7f37-8c0b-7ef0-8eca-cb672194d28d ",
			ModelIdentifier:   " chat-model ",
			SystemInstruction: " 负责回答产品问题。 ",
		},
	})
	if err != nil {
		t.Fatalf("normalizeExecutionInput() error = %v", err)
	}
	if input.Managed.ProviderID != "019c7f37-8c0b-7ef0-8eca-cb672194d28d" || input.Managed.ModelIdentifier != "chat-model" || input.Managed.SystemInstruction != "负责回答产品问题。" {
		t.Fatalf("normalizeExecutionInput() = %#v", input)
	}
}

// TestNormalizeExecutionInputRejectsInvalidEnvelope 验证无效执行模式和缺失配置会被拒绝。
func TestNormalizeExecutionInputRejectsInvalidEnvelope(t *testing.T) {
	fields := executionValidationFields(t, ExecutionInput{})
	if fields["execution"] != ValidationExecutionInvalid {
		t.Fatalf("normalizeExecutionInput() fields = %#v", fields)
	}
}

// TestNormalizeExecutionInputRejectsRequiredFields 验证无效模型和空工作指令会被拒绝。
func TestNormalizeExecutionInputRejectsRequiredFields(t *testing.T) {
	fields := executionValidationFields(t, ExecutionInput{
		Mode:    domain.AgentExecutionModeManaged,
		Managed: &ManagedExecutionInput{ProviderID: "invalid"},
	})
	if fields["providerId"] != ValidationModelInvalid || fields["modelIdentifier"] != ValidationModelInvalid || fields["systemInstruction"] != ValidationSystemInstructionRequired {
		t.Fatalf("normalizeExecutionInput() fields = %#v", fields)
	}
}

// TestNormalizeExecutionInputRejectsLongInstruction 验证工作指令使用字符数限制。
func TestNormalizeExecutionInputRejectsLongInstruction(t *testing.T) {
	fields := executionValidationFields(t, ExecutionInput{
		Mode: domain.AgentExecutionModeManaged,
		Managed: &ManagedExecutionInput{
			ProviderID:        "019c7f37-8c0b-7ef0-8eca-cb672194d28d",
			ModelIdentifier:   "chat-model",
			SystemInstruction: strings.Repeat("鹿", maxSystemInstructionLength+1),
		},
	})
	if fields["systemInstruction"] != ValidationSystemInstructionTooLong {
		t.Fatalf("normalizeExecutionInput() fields = %#v", fields)
	}
}

// TestDecodeRevisionExecutionReadsManagedV1 验证平台托管第一版配置快照可以完整解码。
func TestDecodeRevisionExecutionReadsManagedV1(t *testing.T) {
	execution, err := decodeRevisionExecution(servermodels.AgentRevision{
		ID:            "revision-1",
		ExecutionMode: string(domain.AgentExecutionModeManaged),
		SchemaVersion: managedExecutionSchemaVersion,
		Configuration: []byte(`{"model":{"providerId":"019c7f37-8c0b-7ef0-8eca-cb672194d28d","providerName":"企业模型","identifier":"chat-model","name":"对话模型"},"systemInstruction":"回答产品问题。"}`),
	})
	if err != nil {
		t.Fatalf("decodeRevisionExecution() error = %v", err)
	}
	if execution.RevisionID != "revision-1" || execution.Mode != domain.AgentExecutionModeManaged || execution.Managed == nil || execution.Managed.ProviderName != "企业模型" || execution.Managed.ModelIdentifier != "chat-model" || execution.Managed.SystemInstruction != "回答产品问题。" {
		t.Fatalf("decodeRevisionExecution() = %#v", execution)
	}
}

// TestDecodeRevisionExecutionRejectsUnknownMode 验证未知执行模式会明确失败。
func TestDecodeRevisionExecutionRejectsUnknownMode(t *testing.T) {
	_, err := decodeRevisionExecution(servermodels.AgentRevision{
		ExecutionMode: "connected",
		SchemaVersion: managedExecutionSchemaVersion,
		Configuration: []byte(`{}`),
	})
	if err == nil {
		t.Fatal("decodeRevisionExecution() error = nil")
	}
}

// TestDecodeRevisionExecutionRejectsUnknownSchema 验证未知配置结构版本会明确失败。
func TestDecodeRevisionExecutionRejectsUnknownSchema(t *testing.T) {
	_, err := decodeRevisionExecution(servermodels.AgentRevision{
		ExecutionMode: string(domain.AgentExecutionModeManaged),
		SchemaVersion: managedExecutionSchemaVersion + 1,
		Configuration: []byte(`{}`),
	})
	if err == nil {
		t.Fatal("decodeRevisionExecution() error = nil")
	}
}

// TestDecodeRevisionExecutionRejectsUnknownFields 验证配置快照包含未知字段时会明确失败。
func TestDecodeRevisionExecutionRejectsUnknownFields(t *testing.T) {
	_, err := decodeRevisionExecution(servermodels.AgentRevision{
		ExecutionMode: string(domain.AgentExecutionModeManaged),
		SchemaVersion: managedExecutionSchemaVersion,
		Configuration: []byte(`{"model":{"providerId":"019c7f37-8c0b-7ef0-8eca-cb672194d28d","providerName":"企业模型","identifier":"chat-model","name":"对话模型"},"systemInstruction":"回答产品问题。","unknown":true}`),
	})
	if err == nil {
		t.Fatal("decodeRevisionExecution() error = nil")
	}
}

// executionValidationFields 返回执行配置字段校验结果。
func executionValidationFields(t *testing.T, input ExecutionInput) map[string]common.FieldCode {
	t.Helper()
	_, err := normalizeExecutionInput(input)
	var fieldError *common.FieldError
	if !errors.As(err, &fieldError) {
		t.Fatalf("normalizeExecutionInput() error = %v", err)
	}
	return fieldError.Fields
}
