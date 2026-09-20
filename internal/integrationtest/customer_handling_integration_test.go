//go:build server

package integrationtest

import (
	"context"
	"errors"
	"testing"
	"uuid"

	agentrunaction "github.com/runforyou-ai/cervi/internal/actions/agentrun"
	conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	serverconfig "github.com/runforyou-ai/cervi/internal/config/server"
	"github.com/runforyou-ai/cervi/internal/domain"
	servertask "github.com/runforyou-ai/cervi/internal/task/server"
)

// TestCustomerHandlingAuthorization 验证只有开启接待的成员可以领取、对客回复、关闭与重开客服周期，未开启的成员仍可写内部备注，也不能作为转交目标。
func TestCustomerHandlingAuthorization(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	coordinator := newGroupAgentCoordinator(f.db)
	// 群主恢复管理员角色，修改成员需要企业保留有效管理员。
	if _, err := f.db.NewUpdate().Table("organization_identities").
		Set("role_id = (SELECT id FROM roles WHERE organization_id = ? AND kind = ?)", f.owner.Organization.ID, domain.RoleKindAdmin).
		Where("id = ?", f.owner.OrganizationIdentity.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	setHandlesCustomers := func(handlesCustomers bool) {
		t.Helper()
		if _, err := useraction.NewUpdateUserAction(f.db, testServiceSessionReturner(f.db)).Execute(ctx, f.owner, f.member.User.ID, useraction.UpdateInput{
			DisplayName: f.member.OrganizationIdentity.DisplayName, Email: f.member.User.Email,
			RoleID: f.member.OrganizationIdentity.RoleID, HandlesCustomers: handlesCustomers,
		}); err != nil {
			t.Fatal(err)
		}
	}
	expectHandlingRequired := func(step string, err error) {
		t.Helper()
		var conflict *conversationaction.ConflictError
		if !errors.As(err, &conflict) || conflict.Reason != conversationaction.ConflictReasonCustomerHandlingRequired {
			t.Fatalf("%s = %v", step, err)
		}
	}

	setHandlesCustomers(false)
	claim := conversationaction.NewClaimServiceSessionAction(f.db, coordinator)
	_, err := claim.Execute(ctx, f.member, f.conversationID)
	expectHandlingRequired("未开启接待领取", err)
	send := conversationaction.NewSendCustomerTextMessageAction(f.db, nil)
	_, err = send.Execute(ctx, f.member, conversationaction.CustomerTextMessageInput{
		ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(), Body: "未开启接待的回复",
	})
	expectHandlingRequired("未开启接待对客回复", err)

	// 未开启接待的成员仍可写内部备注。
	if _, err := send.Execute(ctx, f.member, conversationaction.CustomerTextMessageInput{
		ConversationID: f.conversationID, ClientMessageID: uuid.NewV7().String(),
		Body: "未开启接待的内部备注", Visibility: domain.MessageVisibilityInternalOnly,
	}); err != nil {
		t.Fatalf("未开启接待写内部备注 = %v", err)
	}

	// 未开启接待的成员不能作为转交目标。
	if _, err := claim.Execute(ctx, f.owner, f.conversationID); err != nil {
		t.Fatal(err)
	}
	transfer := conversationaction.NewTransferServiceSessionAction(f.db, coordinator, agentrunaction.NewScheduler(servertask.New(f.db, serverconfig.NATSConfig{})))
	_, err = transfer.Execute(ctx, f.owner, conversationaction.TransferServiceSessionInput{
		ConversationID: f.conversationID, TargetKind: domain.ServiceSessionTargetMember, IdentityID: f.member.OrganizationIdentity.ID,
	})
	var validation *conversationaction.ValidationError
	if !errors.As(err, &validation) || validation.Fields["identityId"] != conversationaction.ValidationTargetIdentityIDInvalid {
		t.Fatalf("转交给未开启接待的成员 = %v", err)
	}

	// 开启接待后领取、关闭与重开都可用。
	setHandlesCustomers(true)
	if _, err := transfer.Execute(ctx, f.owner, conversationaction.TransferServiceSessionInput{
		ConversationID: f.conversationID, TargetKind: domain.ServiceSessionTargetMember, IdentityID: f.member.OrganizationIdentity.ID,
	}); err != nil {
		t.Fatalf("转交给开启接待的成员 = %v", err)
	}
	closeSession := conversationaction.NewCloseServiceSessionAction(f.db, coordinator)
	if _, err := closeSession.Execute(ctx, f.member, f.conversationID); err != nil {
		t.Fatalf("开启接待关闭周期 = %v", err)
	}

	// 关闭开关后连重开也被拒绝。
	setHandlesCustomers(false)
	reopen := conversationaction.NewReopenServiceSessionAction(f.db)
	_, err = reopen.Execute(ctx, f.member, f.conversationID)
	expectHandlingRequired("未开启接待重开", err)
	_, err = closeSession.Execute(ctx, f.member, f.conversationID)
	expectHandlingRequired("未开启接待关闭", err)
}
