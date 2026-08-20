//go:build server

package appservice

import "context"

// LoadInbox 返回当前身份可访问的统一收件箱。
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
