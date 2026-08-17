//go:build server

package contact

import (
	"strings"
	"testing"
)

// TestNormalizeContactInput 验证联系人字段和联系方式规范化。
func TestNormalizeContactInput(t *testing.T) {
	input, fields := normalizeContactInput(ContactInput{
		DisplayName: "  林晓  ",
		ChannelID:   "00000000-0000-0000-0000-000000000001",
		Stage:       " lead ",
		Notes:       "  采购负责人  ",
		Methods: []MethodInput{
			{Type: MethodEmail, Value: " LIN@Example.com "},
			{Type: MethodPhone, Value: "+86 138-0000-0000"},
		},
	})
	if len(fields) != 0 {
		t.Fatalf("validation fields = %#v, want empty", fields)
	}
	if input.DisplayName != "林晓" || input.Stage != StageLead || input.Notes != "采购负责人" {
		t.Fatalf("unexpected normalized contact: %#v", input)
	}
	if input.Methods[0].Value != "lin@example.com" || !input.Methods[0].IsPrimary {
		t.Fatalf("unexpected email method: %#v", input.Methods[0])
	}
	if input.Methods[1].Value != "+8613800000000" || !input.Methods[1].IsPrimary {
		t.Fatalf("unexpected phone method: %#v", input.Methods[1])
	}
}

// TestNormalizeContactInputRejectsInvalidValues 验证联系人输入边界。
func TestNormalizeContactInputRejectsInvalidValues(t *testing.T) {
	_, fields := normalizeContactInput(ContactInput{})
	if fields["displayName"] != ValidationIdentityRequired {
		t.Fatalf("identity validation = %q, want %q", fields["displayName"], ValidationIdentityRequired)
	}
	if fields["channelId"] != ValidationChannelRequired {
		t.Fatalf("channel validation = %q, want %q", fields["channelId"], ValidationChannelRequired)
	}
	if fields["stage"] != ValidationStageInvalid {
		t.Fatalf("stage validation = %q, want %q", fields["stage"], ValidationStageInvalid)
	}

	_, fields = normalizeContactInput(ContactInput{
		DisplayName: strings.Repeat("鹿", 201),
		ChannelID:   "00000000-0000-0000-0000-000000000001",
		Stage:       "internal",
		Notes:       strings.Repeat("行", 5001),
		Methods: []MethodInput{
			{Type: MethodEmail, Value: "invalid"},
		},
	})
	if fields["displayName"] != ValidationNameTooLong || fields["stage"] != ValidationStageInvalid || fields["notes"] != ValidationNotesTooLong || fields["methods"] != ValidationMethodInvalid {
		t.Fatalf("unexpected validation fields: %#v", fields)
	}

	for _, phone := range []string{"13800000000", "+12", "+1234567890123456"} {
		_, fields = normalizeContactInput(ContactInput{
			DisplayName: "林晓",
			ChannelID:   "00000000-0000-0000-0000-000000000001",
			Stage:       StageVisitor,
			Methods:     []MethodInput{{Type: MethodPhone, Value: phone}},
		})
		if fields["methods"] != ValidationMethodInvalid {
			t.Fatalf("phone %q validation = %q, want %q", phone, fields["methods"], ValidationMethodInvalid)
		}
	}

	tooManyMethods := make([]MethodInput, 21)
	_, fields = normalizeContactInput(ContactInput{
		DisplayName: "林晓",
		ChannelID:   "00000000-0000-0000-0000-000000000001",
		Stage:       StageVisitor,
		Methods:     tooManyMethods,
	})
	if fields["methods"] != ValidationMethodsTooMany {
		t.Fatalf("methods validation = %q, want %q", fields["methods"], ValidationMethodsTooMany)
	}
}

// TestNormalizeContactInputRejectsDuplicateMethods 验证重复联系方式和主要项。
func TestNormalizeContactInputRejectsDuplicateMethods(t *testing.T) {
	_, fields := normalizeContactInput(ContactInput{
		DisplayName: "林晓",
		ChannelID:   "00000000-0000-0000-0000-000000000001",
		Stage:       StageVisitor,
		Methods: []MethodInput{
			{Type: MethodEmail, Value: "lin@example.com", IsPrimary: true},
			{Type: MethodEmail, Value: "LIN@example.com", IsPrimary: true},
		},
	})
	if fields["methods"] != ValidationPrimaryDuplicate {
		t.Fatalf("method validation = %q, want %q", fields["methods"], ValidationPrimaryDuplicate)
	}
}

// TestNormalizeListInput 验证联系人列表查询白名单。
func TestNormalizeListInput(t *testing.T) {
	input, fields := normalizeListInput(ListInput{})
	if len(fields) != 0 || input.Page != 1 || input.PageSize != 50 || input.Sort != "createdAt.desc" {
		t.Fatalf("unexpected list defaults: input=%#v fields=%#v", input, fields)
	}

	_, fields = normalizeListInput(ListInput{Stage: "internal", ChannelID: "invalid", MethodType: "wechat", Sort: "drop table", PageSize: 101})
	if len(fields) != 5 {
		t.Fatalf("validation fields = %#v, want five errors", fields)
	}
}
