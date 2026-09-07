//go:build server

package agentruntime

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// TestTurnPreemptionDoesNotSendEmptyAssistant 验证模型调用期间补入新消息后，下一轮输入有效且思考展示保留。
func TestTurnPreemptionDoesNotSendEmptyAssistant(t *testing.T) {
	for _, kind := range []string{"reasoning-with-skipped-tool", "reasoning-only", "empty"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			feed := &testInputFeed{}
			feed.appendUser("开始计算")
			recorder := newProcessRecorder(RunRequest{})
			execution := &einoExecution{inputs: &turnInputs{feed: feed}, recorder: recorder, maxTurns: 2}
			calls := 0
			chatModel := &processChatModel{generate: func(ctx context.Context, messages []*schema.Message) (*schema.Message, error) {
				calls++
				if calls == 1 {
					feed.appendUser("换一个问题")
					// 等待抢占提交后才返回模型输出，有工具调用时在工具开始前切换轮次。
					if err := execution.inputs.poll(ctx, true); err != nil {
						return nil, err
					}
					output := schema.AssistantMessage("", nil)
					if kind != "empty" {
						output.ReasoningContent = "需要先计算"
					}
					if kind == "reasoning-with-skipped-tool" {
						output.ToolCalls = []schema.ToolCall{{ID: "skipped", Type: "function", Function: schema.FunctionCall{Name: "calculator", Arguments: `{"operation":"add","left":1,"right":2}`}}}
					}
					return output, nil
				}
				var contents []string
				for _, message := range messages {
					if message.Role == schema.Assistant && message.Content == "" && len(message.ToolCalls) == 0 {
						return nil, fmt.Errorf("Invalid assistant message: content or tool_calls must be set")
					}
					if message.Role != schema.System {
						if message.Role != schema.User {
							return nil, fmt.Errorf("unexpected retained role: %s", message.Role)
						}
						contents = append(contents, message.Content)
					}
				}
				if !reflect.DeepEqual(contents, []string{"开始计算", "换一个问题"}) {
					return nil, fmt.Errorf("follow-up input = %v", contents)
				}
				return schema.AssistantMessage("已按新问题回答", nil), nil
			}}
			calculator, err := newCalculatorTool()
			if err != nil {
				t.Fatal(err)
			}
			agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
				Name: "test", Model: chatModel, Handlers: []adk.ChatModelAgentMiddleware{recorder},
				ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: []tool.BaseTool{calculator}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			execution.inputs.loop = adk.NewTurnLoop(adk.TurnLoopConfig[Trigger, *schema.Message]{
				GenInput: execution.genInput,
				PrepareAgent: func(context.Context, *adk.TurnLoop[Trigger, *schema.Message], []Trigger) (adk.Agent, error) {
					return agent, nil
				},
				OnAgentEvents: execution.onAgentEvents,
			})
			if err := execution.inputs.run(ctx); err != nil {
				t.Fatal(err)
			}
			if calls != 2 || execution.inputs.claimedSeq != 2 || execution.result.Content != "已按新问题回答" {
				t.Fatalf("calls = %d, claimed = %d, result = %#v", calls, execution.inputs.claimedSeq, execution.result)
			}
			blocks := recorder.blocks()
			if kind == "empty" && len(blocks) != 0 {
				t.Fatalf("empty output produced process blocks: %#v", blocks)
			}
			if kind != "empty" && (len(blocks) != 1 || blocks[0].Kind != domain.AgentRunBlockThinking || blocks[0].Payload.Text != "需要先计算") {
				t.Fatalf("retained process blocks = %#v", blocks)
			}
		})
	}
}
