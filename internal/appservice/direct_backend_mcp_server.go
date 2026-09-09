//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	mcpserveraction "github.com/runforyou-ai/cervi/internal/actions/mcpserver"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// ListMCPServers 返回当前企业配置的 MCP 服务。
func (b *DirectBackend) ListMCPServers(ctx context.Context, meta RequestMeta) (MCPServerList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return MCPServerList{}, err
	}
	records, err := b.listMCPServers.Execute(ctx, identity)
	if err != nil {
		return MCPServerList{}, b.mcpServerError(ctx, meta, err, cervii18n.ErrorMCPServerListFailed, identity.Organization.ID)
	}
	mcpServers := make([]MCPServer, 0, len(records))
	for _, record := range records {
		mcpServers = append(mcpServers, mcpServerFromAction(meta, record))
	}
	return MCPServerList{MCPServers: mcpServers}, nil
}

// GetMCPServer 返回当前企业中的 MCP 服务详情。
func (b *DirectBackend) GetMCPServer(ctx context.Context, meta RequestMeta, mcpServerID string) (MCPServer, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return MCPServer{}, err
	}
	record, err := b.getMCPServer.Execute(ctx, identity, mcpServerID)
	if err != nil {
		return MCPServer{}, b.mcpServerError(
			ctx, meta, err, cervii18n.ErrorMCPServerReadFailed, identity.Organization.ID,
			"mcp_server_id", mcpServerID,
		)
	}
	return mcpServerFromAction(meta, *record), nil
}

// CreateMCPServer 创建 MCP 服务。
func (b *DirectBackend) CreateMCPServer(ctx context.Context, meta RequestMeta, input MCPServerInput) (MCPServer, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return MCPServer{}, err
	}
	record, err := b.createMCPServer.Execute(ctx, identity, mcpServerInput(input))
	if err != nil {
		return MCPServer{}, b.mcpServerMutationError(
			ctx, meta, err, cervii18n.ErrorMCPServerCreateFailed, identity.Organization.ID,
		)
	}
	slog.Info(
		"MCP 服务创建成功",
		"organization_id", identity.Organization.ID,
		"mcp_server_id", record.ID,
		"server_type", record.ServerType,
	)
	return mcpServerFromAction(meta, *record), nil
}

// UpdateMCPServer 修改 MCP 服务。
func (b *DirectBackend) UpdateMCPServer(ctx context.Context, meta RequestMeta, mcpServerID string, input MCPServerInput) (MCPServer, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return MCPServer{}, err
	}
	record, err := b.updateMCPServer.Execute(ctx, identity, mcpServerID, mcpServerInput(input))
	if err != nil {
		return MCPServer{}, b.mcpServerMutationError(
			ctx, meta, err, cervii18n.ErrorMCPServerUpdateFailed, identity.Organization.ID,
			"mcp_server_id", mcpServerID,
		)
	}
	slog.Info(
		"MCP 服务保存成功",
		"organization_id", identity.Organization.ID,
		"mcp_server_id", record.ID,
		"server_type", record.ServerType,
	)
	return mcpServerFromAction(meta, *record), nil
}

// DeleteMCPServer 删除 MCP 服务。
func (b *DirectBackend) DeleteMCPServer(ctx context.Context, meta RequestMeta, mcpServerID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteMCPServer.Execute(ctx, identity, mcpServerID); err != nil {
		return b.mcpServerError(
			ctx, meta, err, cervii18n.ErrorMCPServerDeleteFailed, identity.Organization.ID,
			"mcp_server_id", mcpServerID,
		)
	}
	return nil
}

// mcpServerMutationError 转换 MCP 服务写入错误。
func (b *DirectBackend) mcpServerMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		// 映射 MCP 服务校验错误。
		keys := map[common.FieldCode]cervii18n.Key{
			mcpserveraction.ValidationServerTypeInvalid: cervii18n.FieldMCPServerTypeInvalid,
			mcpserveraction.ValidationNameRequired:      cervii18n.FieldMCPServerNameRequired,
			mcpserveraction.ValidationNameTooLong:       cervii18n.FieldMCPServerNameTooLong,
			mcpserveraction.ValidationNameDuplicate:     cervii18n.FieldMCPServerNameDuplicate,
			mcpserveraction.ValidationURLRequired:       cervii18n.FieldMCPServerURLRequired,
			mcpserveraction.ValidationURLInvalid:        cervii18n.FieldHTTPURLInvalid,
			mcpserveraction.ValidationURLTooLong:        cervii18n.FieldMCPServerURLTooLong,
		}
		return InvalidError(meta, cervii18n.ErrorValidationFailed, translateValidationFields(validationError.Fields, keys))
	}
	return b.mcpServerError(ctx, meta, err, failureKey, organizationID, attributes...)
}

// mcpServerError 转换 MCP 服务操作错误。
func (b *DirectBackend) mcpServerError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, mcpserveraction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorMCPServerNotFound)
	}
	if _, kind, ok := connectiontest.Details(err); ok {
		return UnavailableError(meta, mcpConnectionFailureKey(kind), nil)
	}
	logAttributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	slog.Warn("MCP 服务操作失败", append(logAttributes, attributes...)...)
	return FailedError(meta, failureKey)
}

// mcpServerInput 转换 MCP 服务输入。
func mcpServerInput(input MCPServerInput) mcpserveraction.Input {
	return mcpserveraction.Input{
		Name: input.Name, URL: input.URL, ServerType: domain.MCPServerType(input.ServerType), AuthorizationToken: input.AuthorizationToken,
	}
}

// mcpServerFromAction 转换 MCP 服务输出。
func mcpServerFromAction(meta RequestMeta, input mcpserveraction.Record) MCPServer {
	tools := make([]MCPTool, 0, len(input.Tools))
	for _, item := range input.Tools {
		tools = append(tools, MCPTool{Name: item.Name, Description: item.Description})
	}
	message := ""
	if input.ToolsFailure != "" {
		message, _ = cervii18n.Localize(string(meta.Locale), mcpConnectionFailureKey(connectiontest.FailureKind(input.ToolsFailure)))
	}
	return MCPServer{
		Tools: tools, ToolsUpdatedAt: input.ToolsUpdatedAt, ToolsUpdating: input.ToolsUpdating, ToolsError: message,
		ID: input.ID, Name: input.Name, URL: input.URL, ServerType: MCPServerType(input.ServerType), AuthorizationToken: input.AuthorizationToken,
		CreatedAt: input.CreatedAt, UpdatedAt: input.UpdatedAt,
	}
}

// TestMCPServerConnection 测试未保存的 MCP 连接配置。
func (b *DirectBackend) TestMCPServerConnection(ctx context.Context, meta RequestMeta, input MCPServerConnectionInput) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	err = b.testMCPServerConnection.Execute(ctx, mcpserveraction.ConnectionInput{URL: input.URL, ServerType: domain.MCPServerType(input.ServerType), AuthorizationToken: input.AuthorizationToken})
	if err == nil {
		return nil
	}
	return b.mcpServerMutationError(ctx, meta, err, cervii18n.ErrorMCPConnectionFailed, identity.Organization.ID)
}

// TestSavedMCPServerConnection 测试当前企业中已保存的 MCP 服务。
func (b *DirectBackend) TestSavedMCPServerConnection(ctx context.Context, meta RequestMeta, mcpServerID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	record, err := b.getMCPServer.Execute(ctx, identity, mcpServerID)
	if err == nil {
		err = b.testMCPServerConnection.Execute(ctx, mcpserveraction.ConnectionInput{URL: record.URL, ServerType: record.ServerType, AuthorizationToken: record.AuthorizationToken})
	}
	if err == nil {
		return nil
	}
	return b.mcpServerMutationError(ctx, meta, err, cervii18n.ErrorMCPConnectionFailed, identity.Organization.ID, "mcp_server_id", mcpServerID)
}

// RefreshMCPServerTools 提交当前企业全部 MCP 服务的工具更新任务。
func (b *DirectBackend) RefreshMCPServerTools(ctx context.Context, meta RequestMeta) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.refreshMCPServerTools.Execute(ctx, identity); err != nil {
		return b.mcpServerError(ctx, meta, err, cervii18n.ErrorMCPToolsRefreshFailed, identity.Organization.ID)
	}
	return nil
}

// mcpConnectionFailureKey 将 MCP 连接失败原因映射为用户文案。
func mcpConnectionFailureKey(kind connectiontest.FailureKind) cervii18n.Key {
	switch kind {
	case connectiontest.FailureUnauthorized:
		return cervii18n.ErrorMCPAuthenticationFailed
	case connectiontest.FailureForbidden:
		return cervii18n.ErrorMCPAuthorizationFailed
	case connectiontest.FailureTimeout:
		return cervii18n.ErrorMCPConnectionTimeout
	case connectiontest.FailureProtocol:
		return cervii18n.ErrorMCPProtocolFailed
	default:
		return cervii18n.ErrorMCPConnectionFailed
	}
}
