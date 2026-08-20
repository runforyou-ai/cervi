//go:build server

package appservice

import "context"

// LoadInbox 返回当前身份所属企业和用户，会话列表暂为空。
func (b *DirectBackend) LoadInbox(ctx context.Context, meta RequestMeta) (Inbox, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return Inbox{}, err
	}
	output := b.loadInbox.Execute(ctx, identity)
	return Inbox{
		Organization:  organizationFromModel(output.Organization),
		User:          userFromModel(output.User),
		Conversations: []Conversation{},
	}, nil
}
