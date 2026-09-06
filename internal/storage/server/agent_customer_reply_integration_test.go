//go:build server

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	channelaction "github.com/runforyou-ai/cervi/internal/actions/channel"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
	"github.com/uptrace/bun"
)

// testAgentCustomerReplies 验证转交 AI 后仍能理解窗口外的客户引用，删除的原文不进入模型。
func testAgentCustomerReplies(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string) {
	ctx := context.Background()
	created, err := agentaction.NewCreateAgentAction(db).Execute(ctx, identity, agentaction.CreateInput{
		DisplayName: "客服引用助手", RoleID: roleID,
		Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{ProviderID: providerID, ModelIdentifier: modelID, SystemInstruction: "结合引用回答"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	tasks := servertask.New(db, serverconfig.NATSConfig{})
	if err := tasks.Registry().RegisterJSON(agentrunaction.RunActionName, func(context.Context, agentrunaction.RunInput) error { return nil }); err != nil {
		t.Fatal(err)
	}
	scheduler := agentrunaction.NewScheduler(tasks)
	for _, deleted := range []bool{false, true} {
		t.Run(fmt.Sprintf("deleted=%t", deleted), func(t *testing.T) {
			channel, err := channelaction.NewCreateMessageChannelAction(db).Execute(ctx, identity, channelaction.CreateMessageChannelInput{
				Type: domain.ChannelTypeWebsite, Name: "引用上下文", DefaultLocale: domain.LocaleChineseSimplified,
				NewConversationTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue}, FallbackTarget: channelaction.RoutingTarget{Type: domain.ChannelRoutingTargetTypePublicQueue},
			})
			if err != nil {
				t.Fatal(err)
			}
			receive := conversationaction.NewReceiveWebsiteCustomerTextMessageAction(db, scheduler)
			input := conversationaction.WebsiteCustomerTextMessageInput{ChannelID: channel.ID, ExternalID: "web-session:0123456789abcdef0123456789abcdef", ClientMessageID: uuid.NewV7().String(), Body: "最早的客户问题"}
			original, err := receive.Execute(ctx, input)
			if err != nil {
				t.Fatal(err)
			}
			input.ConversationID = &original.Conversation.ID
			for i := range 105 {
				input.ClientMessageID, input.Body = uuid.NewV7().String(), fmt.Sprintf("中间问题 %d", i)
				if _, err := receive.Execute(ctx, input); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := conversationaction.NewSendCustomerTextMessageAction(db).Execute(ctx, identity, conversationaction.CustomerTextMessageInput{ConversationID: original.Conversation.ID, ClientMessageID: uuid.NewV7().String(), Body: "针对早期问题的回答", ReplyToMessageID: original.Message.ID}); err != nil {
				t.Fatal(err)
			}
			if deleted {
				if _, err := db.NewUpdate().Model((*servermodels.Message)(nil)).Set("deleted_at = now()").Where("id = ?", original.Message.ID).Exec(ctx); err != nil {
					t.Fatal(err)
				}
			}
			coordinator := agentrunaction.NewExecuteAction(db, tasks, nil, nil)
			if _, err := conversationaction.NewTransferServiceSessionAction(db, coordinator, scheduler).Execute(ctx, identity, conversationaction.TransferServiceSessionInput{ConversationID: original.Conversation.ID, AssigneeIdentityID: created.IdentityID}); err != nil {
				t.Fatal(err)
			}
			input.ClientMessageID, input.Body = uuid.NewV7().String(), "请接着解释"
			if _, err := receive.Execute(ctx, input); err != nil {
				t.Fatal(err)
			}
			calls := 0
			runtime := &testAgentRuntime{run: func(ctx context.Context, _ agentruntime.RunRequest, feed agentruntime.InputFeed) (agentruntime.RunResult, error) {
				calls++
				triggers, err := feed.Peek(ctx, 0)
				if err != nil {
					return agentruntime.RunResult{}, err
				}
				claimed, err := feed.Claim(ctx, triggers[len(triggers)-1].Seq)
				if err != nil {
					return agentruntime.RunResult{}, err
				}
				if len(claimed.Messages) != 100 {
					t.Fatalf("history length=%d", len(claimed.Messages))
				}
				last, quoted := claimed.Messages[99], claimed.Messages[98]
				if last.Role != agentruntime.MessageRoleUser || last.Content != input.Body || quoted.Role != agentruntime.MessageRoleAssistant {
					t.Fatalf("roles/plain body changed: %+v %+v", last, quoted)
				}
				var content struct {
					Body    string `json:"body"`
					ReplyTo struct {
						MessageID      string `json:"messageId"`
						Deleted        bool   `json:"deleted"`
						SenderKind     string `json:"senderKind"`
						SenderSourceID string `json:"senderSourceId"`
						SenderName     string `json:"senderName"`
						Body           string `json:"body"`
					} `json:"replyTo"`
				}
				if err := json.Unmarshal([]byte(quoted.Content), &content); err != nil {
					t.Fatal(err)
				}
				reference := content.ReplyTo
				if content.Body != "针对早期问题的回答" || reference.MessageID != original.Message.ID || reference.Deleted != deleted {
					t.Fatalf("reference=%+v", content)
				}
				if deleted {
					if reference.Body != "" || reference.SenderSourceID != "" || reference.SenderKind != "" || reference.SenderName != "" || strings.Contains(quoted.Content, original.Message.Body) {
						t.Fatalf("deleted reference leaked: %s", quoted.Content)
					}
				} else if reference.Body != original.Message.Body || reference.SenderKind != string(domain.ChatSubjectKindContact) || reference.SenderSourceID == "" {
					t.Fatalf("customer reference=%+v", reference)
				}
				for _, message := range claimed.Messages[:98] {
					if message.Content == original.Message.Body {
						t.Fatal("reference target should be outside history window")
					}
				}
				return agentruntime.RunResult{Content: "AI 后续回答", EndSeq: claimed.EndSeq}, nil
			}}
			run := &servermodels.AgentRun{}
			if err := db.NewSelect().Model(run).Where("agr.conversation_id = ? AND agr.status = ?", original.Conversation.ID, domain.AgentRunStatusQueued).Scan(ctx); err != nil {
				t.Fatal(err)
			}
			if err := agentrunaction.NewExecuteAction(db, tasks, runtime, nil).Execute(ctx, agentrunaction.RunInput{RunID: run.ID}); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("runtime calls=%d", calls)
			}
		})
	}
}
