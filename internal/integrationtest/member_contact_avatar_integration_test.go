//go:build server

package integrationtest

import (
	"context"
	"testing"
	"time"
	"uuid"

	contactaction "github.com/runforyou-ai/cervi/internal/actions/contact"
	useraction "github.com/runforyou-ai/cervi/internal/actions/user"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// TestMemberAndContactAvatars 验证企业成员读取身份头像，联系人取最近更新且带头像的渠道身份头像。
func TestMemberAndContactAvatars(t *testing.T) {
	f := newCustomerReadFixture(t)
	ctx := context.Background()
	avatarID := uuid.NewV7().String()
	if _, err := f.db.NewUpdate().Model((*servermodels.OrganizationIdentity)(nil)).Set("avatar_file_id = ?", avatarID).Where("id = ?", f.owner.OrganizationIdentity.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	users, err := useraction.NewListUsersQuery(f.db).Execute(ctx, f.member, useraction.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range users.Users {
		want := user.ID == f.owner.User.ID
		if got := user.AvatarFileID != nil && *user.AvatarFileID == avatarID; got != want {
			t.Fatalf("user %s avatar=%v", user.DisplayName, user.AvatarFileID)
		}
	}
	user, err := useraction.NewGetUserQuery(f.db).Execute(ctx, f.member, f.owner.User.ID)
	if err != nil || user.AvatarFileID == nil || *user.AvatarFileID != avatarID {
		t.Fatalf("user=%+v err=%v", user, err)
	}

	// 为同一联系人补一个更早更新、带头像的渠道身份，原渠道身份暂无头像。
	existing := servermodels.ContactChannelIdentity{}
	if err := f.db.NewSelect().Model(&existing).
		Join("JOIN customer_conversations AS cc ON cc.contact_channel_identity_id = cci.id").
		Where("cc.conversation_id = ?", f.conversationID).
		Scan(ctx); err != nil {
		t.Fatal(err)
	}
	olderAvatarID, newerAvatarID := uuid.NewV7().String(), uuid.NewV7().String()
	older := servermodels.ContactChannelIdentity{
		ID: uuid.NewV7().String(), OrganizationID: existing.OrganizationID, ContactID: existing.ContactID, ChannelID: existing.ChannelID,
		ExternalID: uuid.NewV7().String(), AvatarFileID: &olderAvatarID, CreatedAt: time.Now().Add(-time.Hour), UpdatedAt: time.Now().Add(-time.Hour),
	}
	if _, err := f.db.NewInsert().Model(&older).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	assertContactAvatar(t, f.db, f.owner, existing.ContactID, olderAvatarID)
	if _, err := f.db.NewUpdate().Model((*servermodels.ContactChannelIdentity)(nil)).Set("avatar_file_id = ?, updated_at = now()", newerAvatarID).Where("id = ?", existing.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	assertContactAvatar(t, f.db, f.owner, existing.ContactID, newerAvatarID)
}

// assertContactAvatar 检查联系人列表与详情返回同一个头像。
func assertContactAvatar(t *testing.T, db *bun.DB, identity *servermodels.Identity, contactID, avatarID string) {
	t.Helper()
	ctx := context.Background()
	list, err := contactaction.NewListContactsQuery(db).Execute(ctx, identity, contactaction.ListInput{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, contact := range list.Contacts {
		if contact.ID == contactID {
			found = true
			if contact.AvatarFileID == nil || *contact.AvatarFileID != avatarID {
				t.Fatalf("list avatar=%v, want %s", contact.AvatarFileID, avatarID)
			}
		}
	}
	if !found {
		t.Fatal("联系人列表缺少联系人")
	}
	detail, err := contactaction.NewGetContactQuery(db).Execute(ctx, identity, contactID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.AvatarFileID == nil || *detail.AvatarFileID != avatarID {
		t.Fatalf("detail avatar=%v, want %s", detail.AvatarFileID, avatarID)
	}
}
