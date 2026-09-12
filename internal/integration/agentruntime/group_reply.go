//go:build server

package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const groupReplyToolName = "submit_group_reply"

// GroupReplyConfig 声明本次运行使用结构化群聊回复，并限定可点名的成员。
type GroupReplyConfig struct {
	MentionCandidates []string
}

type groupReplySubmission struct {
	Outcome  RunOutcome
	Body     string
	Mentions []string
}

// groupReplyTool 让模型以结构化结果结束群内一次发言。
type groupReplyTool struct {
	candidates []string
	mu         sync.Mutex
	submission *groupReplySubmission
}

// newGroupReplyTool 创建限定点名范围的群聊结束工具。
func newGroupReplyTool(config GroupReplyConfig) *groupReplyTool {
	return &groupReplyTool{candidates: config.MentionCandidates}
}

// Info 描述结束工具的参数与可点名成员。
func (t *groupReplyTool) Info(context.Context) (*schema.ToolInfo, error) {
	mentionDesc := "本次回复要点名的成员名称，留空表示不点名。"
	if len(t.candidates) > 0 {
		mentionDesc += "只能从以下成员中选择：" + strings.Join(t.candidates, "、")
	}
	return &schema.ToolInfo{
		Name: groupReplyToolName,
		Desc: "提交本次群内发言并结束本轮。面向群成员的回复只通过本工具提交；本轮补入新消息时需要重新提交。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"outcome": {
				Desc: "reply 表示在群内发言，silent 表示本次无需发言。", Required: true, Type: schema.String,
				Enum: []string{string(RunOutcomeReply), string(RunOutcomeSilent)},
			},
			"body":     {Desc: "发到群里的正文，outcome 为 reply 时必填。", Type: schema.String},
			"mentions": {Desc: mentionDesc, Type: schema.Array, ElemInfo: &schema.ParameterInfo{Type: schema.String}},
		}),
	}, nil
}

// InvokableRun 校验并保存本次结构化结果，校验不通过时把原因返回给模型重试。
func (t *groupReplyTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var params struct {
		Outcome  string   `json:"outcome"`
		Body     string   `json:"body"`
		Mentions []string `json:"mentions"`
	}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "参数不是合法 JSON，请重新提交。", nil
	}
	submission := groupReplySubmission{Outcome: RunOutcome(params.Outcome), Body: strings.TrimSpace(params.Body)}
	switch submission.Outcome {
	case RunOutcomeReply:
		if submission.Body == "" {
			return "outcome 为 reply 时必须填写 body，请重新提交。", nil
		}
	case RunOutcomeSilent:
		if submission.Body != "" || len(params.Mentions) > 0 {
			return "outcome 为 silent 时不能填写 body 或 mentions，请重新提交。", nil
		}
	default:
		return "outcome 只能是 reply 或 silent，请重新提交。", nil
	}
	for _, mention := range params.Mentions {
		name := strings.TrimSpace(mention)
		if name == "" || !slices.Contains(t.candidates, name) {
			return fmt.Sprintf("成员 %q 不在可点名范围内，请从给定成员中选择或留空。", mention), nil
		}
		if !slices.Contains(submission.Mentions, name) {
			submission.Mentions = append(submission.Mentions, name)
		}
	}
	t.mu.Lock()
	t.submission = &submission
	t.mu.Unlock()
	if err := adk.SendToolGenAction(ctx, groupReplyToolName, adk.NewExitAction()); err != nil {
		return "", err
	}
	return "已提交。", nil
}

// peek 读取本轮已提交的结构化结果。
func (t *groupReplyTool) peek() (groupReplySubmission, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.submission == nil {
		return groupReplySubmission{}, false
	}
	return *t.submission, true
}

// clear 清空已提交的结构化结果，下一轮需要重新提交。
func (t *groupReplyTool) clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.submission = nil
}
