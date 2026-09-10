//go:build server

package appservice

import (
	"context"

	inboxaction "github.com/runforyou-ai/cervi/internal/actions/inbox"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// GetInboxContext 读取锚点当前资格与同一快照内的列表邻域。
func (o *directOperations) GetInboxContext(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input InboxContextInput) (InboxContext, error) {
	result, err := o.loadInbox.ReadContext(ctx, identity, inboxaction.ContextInput{
		Query:    inboxaction.LoadInput{Scope: domain.InboxScope(input.Query.Scope), CustomerView: domain.CustomerInboxView(input.Query.CustomerView), AssigneeIdentityID: input.Query.AssigneeIdentityID},
		AnchorID: input.AnchorID, AnchorCursor: input.AnchorCursor, BeforeLimit: input.BeforeLimit, AfterLimit: input.AfterLimit,
	})
	if err != nil {
		return InboxContext{}, inboxReadError(ctx, meta, identity.Organization.ID, "锚点上下文", err)
	}
	// 锚点与窗口一起解析头像，筛选外但可读的锚点仅放在独立结果中。
	summaries := result.Window.Conversations
	if result.Anchor.Conversation != nil {
		summaries = append(summaries, *result.Anchor.Conversation)
	}
	conversations, err := o.inboxConversationsFromActions(ctx, meta, identity, summaries)
	if err != nil {
		return InboxContext{}, err
	}
	anchor := InboxConversationResult{ID: input.AnchorID, Availability: InboxConversationUnavailable}
	if result.Anchor.Conversation != nil {
		anchor.Conversation = &conversations[len(conversations)-1]
		anchor.Availability = InboxConversationOutsideQuery
		if result.Anchor.MatchesQuery {
			anchor.Availability = InboxConversationMatching
		}
		conversations = conversations[:len(conversations)-1]
	}
	return InboxContext{Anchor: anchor, Window: InboxWindow{
		Conversations: conversations, StartCursor: result.Window.StartCursor, EndCursor: result.Window.EndCursor,
		HasBefore: result.Window.HasBefore, HasAfter: result.Window.HasAfter,
	}}, nil
}

// ReadInboxWindow 按原始边界重读完整连续范围。
func (o *directOperations) ReadInboxWindow(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input InboxWindowInput) (InboxWindow, error) {
	window, err := o.loadInbox.ReadWindow(ctx, identity, inboxaction.ReadWindowInput{
		Query:       inboxaction.LoadInput{Scope: domain.InboxScope(input.Query.Scope), CustomerView: domain.CustomerInboxView(input.Query.CustomerView), AssigneeIdentityID: input.Query.AssigneeIdentityID},
		StartCursor: input.StartCursor, EndCursor: input.EndCursor,
	})
	if err != nil {
		return InboxWindow{}, inboxReadError(ctx, meta, identity.Organization.ID, "列表区间", err)
	}
	conversations, err := o.inboxConversationsFromActions(ctx, meta, identity, window.Conversations)
	if err != nil {
		return InboxWindow{}, err
	}
	return InboxWindow{Conversations: conversations, StartCursor: window.StartCursor, EndCursor: window.EndCursor, HasBefore: window.HasBefore, HasAfter: window.HasAfter}, nil
}
