//go:build server

package agentruntime

import (
	"context"
	"strings"
	"testing"
)

// TestGroupReplyToolValidation 验证结束工具的参数校验与重试提示。
func TestGroupReplyToolValidation(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		arguments string
		reason    string
	}{
		{"参数不是合法 JSON", "{", "合法 JSON"},
		{"未知 outcome", `{"outcome":"unknown","body":"内容"}`, "reply 或 silent"},
		{"回复缺少正文", `{"outcome":"reply","body":"  "}`, "必须填写 body"},
		{"静默携带正文", `{"outcome":"silent","body":"内容"}`, "不能填写 body"},
		{"静默携带点名", `{"outcome":"silent","mentions":["产品经理"]}`, "不能填写 body"},
		{"点名超出范围", `{"outcome":"reply","body":"内容","mentions":["查无此人"]}`, "不在可点名范围"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			finish := newGroupReplyTool(GroupReplyConfig{MentionCandidates: []string{"产品经理"}})
			message, err := finish.InvokableRun(context.Background(), scenario.arguments)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(message, scenario.reason) {
				t.Fatalf("提示 = %q，期望包含 %q", message, scenario.reason)
			}
			if _, submitted := finish.peek(); submitted {
				t.Fatal("校验不通过时不应保存结果")
			}
		})
	}
}

// TestGroupReplyToolInfo 验证工具描述列出可点名成员。
func TestGroupReplyToolInfo(t *testing.T) {
	finish := newGroupReplyTool(GroupReplyConfig{MentionCandidates: []string{"产品经理", "后端工程师"}})
	info, err := finish.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	params, err := info.ParamsOneOf.ToJSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	mentions, ok := params.Properties.Get("mentions")
	if !ok || !strings.Contains(mentions.Description, "产品经理、后端工程师") {
		t.Fatalf("mentions 说明 = %#v", mentions)
	}
}
