//go:build server

package integrationtest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"uuid"

	agentaction "github.com/runforyou-ai/cervi/internal/actions/agent"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// testGroupAgentMembership 验证活跃 Agent 建群、添加、移除和群主边界。
func testGroupAgentMembership(t *testing.T, db *bun.DB, identity *servermodels.Identity, roleID, providerID, modelID string) {
	ctx := context.Background()
	agents := make([]*agentaction.Agent, 0, 2)
	for i := range 2 {
		agent, err := agentaction.NewCreateAgentAction(db).Execute(ctx, identity, agentaction.CreateInput{
			DisplayName: fmt.Sprintf("群成员助手 %d", i), RoleID: roleID,
			Execution: agentaction.ExecutionInput{Mode: domain.AgentExecutionModeManaged, Managed: &agentaction.ManagedExecutionInput{
				ProviderID: providerID, ModelIdentifier: modelID, SystemInstruction: "协助企业成员",
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		agents = append(agents, agent)
	}
	group, err := conversationaction.NewCreateGroupConversationAction(db).Execute(ctx, identity, conversationaction.GroupConversationInput{
		Title: "AI 成员群", MemberIdentityIDs: []string{agents[0].IdentityID},
	})
	if err != nil {
		t.Fatal(err)
	}
	add := conversationaction.NewAddGroupConversationMembersAction(db)
	detail, err := add.Execute(ctx, identity, conversationaction.GroupConversationMembersInput{
		ConversationID: group.ID, MemberIdentityIDs: []string{agents[1].IdentityID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Participants) != 3 {
		t.Fatalf("participants=%+v", detail.Participants)
	}
	var agentSubjectID string
	for _, member := range detail.Participants {
		if member.IdentityID == identity.OrganizationIdentity.ID {
			if member.IdentityType != domain.OrganizationIdentityTypeUser || member.Role != domain.ConversationParticipantRoleOwner {
				t.Fatalf("owner=%+v", member)
			}
		} else {
			if member.IdentityType != domain.OrganizationIdentityTypeAgent || member.Role != domain.ConversationParticipantRoleMember {
				t.Fatalf("agent member=%+v", member)
			}
			if member.IdentityID == agents[0].IdentityID {
				agentSubjectID = member.ChatSubjectID
			}
		}
	}
	testGroupAgentMessages(t, db, identity, group.ID, agentSubjectID)
	testGroupAgentEligibility(t, db, identity, group.ID, agents[1])
	_, err = conversationaction.NewTransferGroupConversationOwnerAction(db).Execute(ctx, identity, conversationaction.GroupConversationOwnerInput{
		ConversationID: group.ID, OwnerIdentityID: agents[0].IdentityID,
	})
	var conflict *conversationaction.ConflictError
	if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonGroupMemberNotActive {
		t.Fatalf("agent owner error=%v", err)
	}
	// 最后一位真人可直接解散含有 Agent 的群聊。
	if _, err := conversationaction.NewDissolveGroupConversationAction(db).Execute(ctx, identity, group.ID); err != nil {
		t.Fatal(err)
	}
	var stored servermodels.Conversation
	if err := db.NewSelect().Model(&stored).Where("cv.id = ?", group.ID).Scan(ctx); err != nil {
		t.Fatal(err)
	}
	if stored.Status != string(domain.ConversationStatusArchived) {
		t.Fatalf("status=%s", stored.Status)
	}
}

// testGroupAgentMessages 验证群内消息不触发 Agent，结构化提醒仅接受真人。
func testGroupAgentMessages(t *testing.T, db *bun.DB, identity *servermodels.Identity, groupID, agentSubjectID string) {
	ctx := context.Background()
	send := conversationaction.NewSendGroupTextMessageAction(db)
	_, err := send.Execute(ctx, identity, conversationaction.GroupTextMessageInput{
		ConversationID: groupID, ClientMessageID: uuid.NewV7().String(), Body: "提醒 Agent", MentionSubjectIDs: []string{agentSubjectID},
	})
	var conflict *conversationaction.ConflictError
	if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonGroupMentionTargetInvalid {
		t.Fatalf("agent mention error=%v", err)
	}
	for _, all := range []bool{false, true} {
		if _, err := send.Execute(ctx, identity, conversationaction.GroupTextMessageInput{
			ConversationID: groupID, ClientMessageID: uuid.NewV7().String(), Body: "群内消息", MentionAll: all,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"agent_runs", "conversation_agent_triggers", "conversation_agent_states"} {
		count, err := db.NewSelect().Table(table).Where("conversation_id = ?", groupID).Count(ctx)
		if err != nil || count != 0 {
			t.Fatalf("%s count=%d err=%v", table, count, err)
		}
	}
}

// testGroupAgentEligibility 验证重新添加、停用状态和企业隔离。
func testGroupAgentEligibility(t *testing.T, db *bun.DB, identity *servermodels.Identity, groupID string, agent *agentaction.Agent) {
	ctx := context.Background()
	remove := conversationaction.NewRemoveGroupConversationMemberAction(db)
	add := conversationaction.NewAddGroupConversationMembersAction(db)
	for _, active := range []bool{true, false} {
		if _, err := remove.Execute(ctx, identity, conversationaction.GroupConversationMemberInput{ConversationID: groupID, MemberIdentityID: agent.IdentityID}); err != nil {
			t.Fatal(err)
		}
		if !active {
			if _, err := agentaction.NewUpdateStatusAction(db).Execute(ctx, identity, agent.ID, domain.UserStatusInactive); err != nil {
				t.Fatal(err)
			}
		}
		_, err := add.Execute(ctx, identity, conversationaction.GroupConversationMembersInput{ConversationID: groupID, MemberIdentityIDs: []string{agent.IdentityID}})
		if active && err != nil {
			t.Fatal(err)
		}
		if !active && !errors.Is(err, conversationaction.ErrGroupMemberNotFound) {
			t.Fatalf("inactive add error=%v", err)
		}
	}
	_, err := conversationaction.NewCreateGroupConversationAction(db).Execute(ctx, identity, conversationaction.GroupConversationInput{
		Title: "停用 Agent", MemberIdentityIDs: []string{agent.IdentityID},
	})
	if !errors.Is(err, conversationaction.ErrGroupMemberNotFound) {
		t.Fatalf("inactive create error=%v", err)
	}
	if _, err := agentaction.NewUpdateStatusAction(db).Execute(ctx, identity, agent.ID, domain.UserStatusActive); err != nil {
		t.Fatal(err)
	}
	foreign := newNavigationFixture(t)
	_, err = conversationaction.NewCreateGroupConversationAction(db).Execute(ctx, foreign.owner, conversationaction.GroupConversationInput{
		Title: "跨企业 Agent", MemberIdentityIDs: []string{agent.IdentityID},
	})
	if !errors.Is(err, conversationaction.ErrGroupMemberNotFound) {
		t.Fatalf("foreign create error=%v", err)
	}
	_, err = add.Execute(ctx, foreign.owner, conversationaction.GroupConversationMembersInput{ConversationID: foreign.groupID, MemberIdentityIDs: []string{agent.IdentityID}})
	if !errors.Is(err, conversationaction.ErrGroupMemberNotFound) {
		t.Fatalf("foreign add error=%v", err)
	}
}
