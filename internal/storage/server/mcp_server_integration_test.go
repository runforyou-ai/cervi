//go:build server

package server

import (
	"context"
	"errors"
	"testing"
	"uuid"

	installationaction "github.com/runforyou-ai/cervi/internal/actions/installation"
	mcpserveraction "github.com/runforyou-ai/cervi/internal/actions/mcpserver"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// TestMCPServerLifecycle 验证 MCP 配置的增删改查、名称唯一性和企业隔离。
func TestMCPServerLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, testDatabaseConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	db := store.DB()
	identities := make([]*servermodels.Identity, 0, 2)
	for range 2 {
		installed, err := installationaction.NewInstallWorkspaceAction(db).Execute(ctx, installationaction.InstallWorkspaceInput{
			AccessHost: uuid.NewV7().String() + ".mcp.test", OrganizationName: "MCP 测试", DisplayName: "维护人员", Email: "owner@mcp.test", Password: "password123", Locale: domain.LocaleChineseSimplified, TimeZone: "UTC",
		})
		if err != nil {
			t.Fatal(err)
		}
		identities = append(identities, installed.Identity)
	}
	owner, other := identities[0], identities[1]
	create := mcpserveraction.NewCreateMCPServerAction(db)
	get := mcpserveraction.NewGetMCPServerQuery(db)
	list := mcpserveraction.NewListMCPServersQuery(db)
	update := mcpserveraction.NewUpdateMCPServerAction(db)
	remove := mcpserveraction.NewDeleteMCPServerAction(db)
	input := mcpserveraction.Input{Name: " Docs ", URL: " https://example.com/mcp ", ServerType: domain.MCPServerTypeStreamableHTTP, AuthorizationToken: "test-token"}
	created, err := create.Execute(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "Docs" || created.URL != "https://example.com/mcp" {
		t.Fatalf("unexpected normalization: %+v", created)
	}
	read, err := get.Execute(ctx, owner, created.ID)
	if err != nil || read.AuthorizationToken != input.AuthorizationToken || read.ServerType != input.ServerType {
		t.Fatalf("read = %+v, error = %v", read, err)
	}
	input.Name = "docs"
	_, err = create.Execute(ctx, owner, input)
	var fields *common.FieldError
	if !errors.As(err, &fields) || fields.Fields["name"] != mcpserveraction.ValidationNameDuplicate {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := create.Execute(ctx, other, input); err != nil {
		t.Fatalf("same name in other organization: %v", err)
	}
	if _, err := get.Execute(ctx, other, created.ID); !errors.Is(err, mcpserveraction.ErrNotFound) {
		t.Fatalf("cross-organization read: %v", err)
	}
	if _, err := update.Execute(ctx, other, created.ID, input); !errors.Is(err, mcpserveraction.ErrNotFound) {
		t.Fatalf("cross-organization update: %v", err)
	}
	if err := remove.Execute(ctx, other, created.ID); !errors.Is(err, mcpserveraction.ErrNotFound) {
		t.Fatalf("cross-organization delete: %v", err)
	}
	records, err := list.Execute(ctx, owner)
	if err != nil || len(records) != 1 || records[0].ID != created.ID {
		t.Fatalf("list = %+v, error = %v", records, err)
	}
	input.Name, input.URL, input.ServerType, input.AuthorizationToken = "Updated", "http://localhost:8080/sse", domain.MCPServerTypeSSE, ""
	saved, err := update.Execute(ctx, owner, created.ID, input)
	if err != nil || saved.Name != input.Name || saved.URL != input.URL || saved.ServerType != input.ServerType || saved.AuthorizationToken != "" {
		t.Fatalf("update = %+v, error = %v", saved, err)
	}
	input.Name = "Another"
	second, err := create.Execute(ctx, owner, input)
	if err != nil {
		t.Fatal(err)
	}
	input.Name = "UPDATED"
	if _, err = update.Execute(ctx, owner, second.ID, input); !errors.As(err, &fields) || fields.Fields["name"] != mcpserveraction.ValidationNameDuplicate {
		t.Fatalf("duplicate update: %v", err)
	}
	if err := remove.Execute(ctx, owner, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := get.Execute(ctx, owner, created.ID); !errors.Is(err, mcpserveraction.ErrNotFound) {
		t.Fatalf("read deleted: %v", err)
	}
	if err := remove.Execute(ctx, owner, second.ID); err != nil {
		t.Fatal(err)
	}
	records, err = list.Execute(ctx, owner)
	if err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty list = %+v, error = %v", records, err)
	}
}
