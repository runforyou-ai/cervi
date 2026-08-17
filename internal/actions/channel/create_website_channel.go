//go:build server

package channel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// CreateWebsiteChannelAction 创建网站消息渠道。
type CreateWebsiteChannelAction struct {
	db *bun.DB
}

// NewCreateWebsiteChannelAction 创建网站渠道操作。
func NewCreateWebsiteChannelAction(db *bun.DB) *CreateWebsiteChannelAction {
	return &CreateWebsiteChannelAction{db: db}
}

// Execute 校验字段并在当前企业中创建网站渠道。
func (a *CreateWebsiteChannelAction) Execute(ctx context.Context, principal *servermodels.Principal, input WebsiteChannelInput) (*servermodels.Channel, error) {
	input, fields := normalizeWebsiteChannelInput(input)
	if len(fields) > 0 {
		return nil, &ValidationError{Fields: fields}
	}
	if principal == nil ||
		!validUUID(principal.Organization.ID) ||
		!validUUID(principal.User.ID) ||
		principal.User.OrganizationID != principal.Organization.ID {
		return nil, ErrPrincipalInvalid
	}

	channel := &servermodels.Channel{
		OrganizationID:  principal.Organization.ID,
		CreatedByUserID: principal.User.ID,
		Type:            TypeWebsite,
		Name:            input.Name,
		DefaultLocale:   input.DefaultLocale,
	}
	if input.Description != "" {
		channel.Description = &input.Description
	}
	err := a.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		organization := &servermodels.Organization{}
		if err := tx.NewSelect().
			Model(organization).
			Column("id").
			Where("o.id = ?", principal.Organization.ID).
			For("KEY SHARE").
			Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPrincipalInvalid
			}
			return err
		}

		user := &servermodels.User{}
		if err := tx.NewSelect().
			Model(user).
			Column("id").
			Where("u.id = ?", principal.User.ID).
			Where("u.organization_id = ?", principal.Organization.ID).
			For("KEY SHARE").
			Scan(ctx); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrPrincipalInvalid
			}
			return err
		}

		_, err := tx.NewInsert().
			Model(channel).
			Column("organization_id", "created_by_user_id", "type", "name", "description", "default_locale").
			Returning("*").
			Exec(ctx)
		return err
	})
	if errors.Is(err, ErrPrincipalInvalid) {
		return nil, ErrPrincipalInvalid
	}
	if err != nil {
		return nil, fmt.Errorf("create website channel: %w", err)
	}
	return channel, nil
}
