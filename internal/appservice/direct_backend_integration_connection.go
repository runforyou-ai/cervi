//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	integrationconnectionaction "github.com/runforyou-ai/cervi/internal/actions/integrationconnection"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
)

// ListIntegrationConnections 返回当前企业的连接器列表。
func (b *DirectBackend) ListIntegrationConnections(ctx context.Context, meta RequestMeta) (IntegrationConnectionList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return IntegrationConnectionList{}, err
	}
	connections, err := b.listIntegrationConnections.Execute(ctx, identity)
	if err != nil {
		return IntegrationConnectionList{}, b.integrationConnectionError(ctx, meta, err, cervii18n.ErrorIntegrationConnectionListFailed, identity.Organization.ID)
	}
	output := make([]IntegrationConnectionSummary, 0, len(connections))
	for _, connection := range connections {
		output = append(output, IntegrationConnectionSummary{
			ID: connection.ID, Type: IntegrationConnectionType(connection.Type),
			Name: connection.Name, Description: connection.Description,
			Status: IntegrationConnectionStatus(connection.Status), LastTestedAt: connection.LastTestedAt,
		})
	}
	return IntegrationConnectionList{Connections: output}, nil
}

// GetIntegrationConnection 返回当前企业中的连接器详情。
func (b *DirectBackend) GetIntegrationConnection(ctx context.Context, meta RequestMeta, connectionID string) (IntegrationConnection, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return IntegrationConnection{}, err
	}
	connection, err := b.getIntegrationConnection.Execute(ctx, identity, connectionID)
	if err != nil {
		return IntegrationConnection{}, b.integrationConnectionError(ctx, meta, err, cervii18n.ErrorIntegrationConnectionReadFailed, identity.Organization.ID, "connection_id", connectionID)
	}
	return integrationConnectionFromAction(*connection), nil
}

// TestIntegrationConnection 测试连接器草稿配置。
func (b *DirectBackend) TestIntegrationConnection(ctx context.Context, meta RequestMeta, input IntegrationConnectionTestInput) error {
	if _, err := b.authenticate(ctx, meta); err != nil {
		return err
	}
	if err := b.testIntegrationConnection.Execute(ctx, integrationConnectionTestInput(input)); err != nil {
		return b.integrationConnectionTestError(ctx, meta, err)
	}
	return nil
}

// CreateIntegrationConnection 创建外部系统连接器。
func (b *DirectBackend) CreateIntegrationConnection(ctx context.Context, meta RequestMeta, input IntegrationConnectionInput) (IntegrationConnection, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return IntegrationConnection{}, err
	}
	connection, err := b.createIntegrationConnection.Execute(ctx, identity, integrationConnectionInput(input))
	if err != nil {
		return IntegrationConnection{}, b.integrationConnectionMutationError(ctx, meta, err, cervii18n.ErrorIntegrationConnectionCreateFailed, identity.Organization.ID)
	}
	b.autoTestIntegrationConnection(ctx, identity, connection)
	slog.Info("连接器创建成功", "organization_id", identity.Organization.ID, "connection_id", connection.ID, "connector_type", connection.Type, "connection_status", connection.Status)
	return integrationConnectionFromAction(*connection), nil
}

// UpdateIntegrationConnection 修改外部系统连接器。
func (b *DirectBackend) UpdateIntegrationConnection(ctx context.Context, meta RequestMeta, connectionID string, input IntegrationConnectionInput) (IntegrationConnection, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return IntegrationConnection{}, err
	}
	connection, err := b.updateIntegrationConnection.Execute(ctx, identity, connectionID, integrationConnectionInput(input))
	if err != nil {
		return IntegrationConnection{}, b.integrationConnectionMutationError(ctx, meta, err, cervii18n.ErrorIntegrationConnectionUpdateFailed, identity.Organization.ID, "connection_id", connectionID)
	}
	b.autoTestIntegrationConnection(ctx, identity, connection)
	slog.Info("连接器保存成功", "organization_id", identity.Organization.ID, "connection_id", connection.ID, "connector_type", connection.Type, "connection_status", connection.Status)
	return integrationConnectionFromAction(*connection), nil
}

// DeleteIntegrationConnection 删除外部系统连接器。
func (b *DirectBackend) DeleteIntegrationConnection(ctx context.Context, meta RequestMeta, connectionID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteIntegrationConnection.Execute(ctx, identity, connectionID); err != nil {
		return b.integrationConnectionError(ctx, meta, err, cervii18n.ErrorIntegrationConnectionDeleteFailed, identity.Organization.ID, "connection_id", connectionID)
	}
	slog.Info("连接器删除成功", "organization_id", identity.Organization.ID, "connection_id", connectionID)
	return nil
}

// autoTestIntegrationConnection 测试已保存配置并同步返回记录的连接状态。
func (b *DirectBackend) autoTestIntegrationConnection(ctx context.Context, identity *servermodels.Identity, connection *integrationconnectionaction.Record) {
	result, err := b.testIntegrationConnection.ExecuteAndRecord(ctx, identity, connection.ID, integrationconnectionaction.ConnectionInput{
		Type: connection.Type,
		Configuration: integrationconnectionaction.Configuration{
			APIURL: connection.Configuration.APIURL,
			APIKey: connection.Configuration.APIKey,
		},
	})
	if !result.TestedAt.IsZero() {
		connection.Status = result.Status
		connection.LastTestedAt = &result.TestedAt
	}
	if err == nil || ctx.Err() != nil {
		return
	}
	if _, _, classified := connectiontest.Details(err); !classified {
		slog.Warn("连接器自动测试失败", "organization_id", identity.Organization.ID, "connection_id", connection.ID, "connector_type", connection.Type, "error", err)
	}
}

// integrationConnectionMutationError 转换连接器写入错误。
func (b *DirectBackend) integrationConnectionMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, integrationConnectionFieldKeys(validationError.Fields))
	}
	return b.integrationConnectionError(ctx, meta, err, failureKey, organizationID, attributes...)
}

// integrationConnectionTestError 转换连接器测试错误。
func (b *DirectBackend) integrationConnectionTestError(ctx context.Context, meta RequestMeta, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, integrationConnectionFieldKeys(validationError.Fields))
	}
	if errors.Is(err, integrationconnectionaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorIntegrationConnectionNotFound)
	}
	return integrationConnectionRemoteError(meta, err)
}

// integrationConnectionRemoteError 转换外部连接访问错误。
func integrationConnectionRemoteError(meta RequestMeta, err error) error {
	_, kind, classified := connectiontest.Details(err)
	if !classified {
		slog.Warn("外部连接访问返回未分类错误", "error", err)
		return UnavailableError(meta, cervii18n.ErrorIntegrationConnectionTestFailed, nil)
	}
	switch kind {
	case connectiontest.FailureUnauthorized:
		return UnavailableError(meta, cervii18n.ErrorIntegrationConnectionAuthenticationFailed, nil)
	case connectiontest.FailureForbidden:
		return UnavailableError(meta, cervii18n.ErrorIntegrationConnectionAuthorizationFailed, nil)
	case connectiontest.FailureRateLimited:
		return UnavailableError(meta, cervii18n.ErrorIntegrationConnectionRateLimited, nil)
	default:
		return UnavailableError(meta, cervii18n.ErrorIntegrationConnectionTestFailed, nil)
	}
}

// integrationConnectionError 转换连接器操作错误。
func (b *DirectBackend) integrationConnectionError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, integrationconnectionaction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorIntegrationConnectionNotFound)
	}
	if errors.Is(err, integrationconnectionaction.ErrInUse) {
		return InvalidError(meta, cervii18n.ErrorIntegrationConnectionInUse, nil)
	}
	logAttributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	slog.Warn("连接器操作失败", append(logAttributes, attributes...)...)
	return FailedError(meta, failureKey)
}

// integrationConnectionInput 转换连接器输入。
func integrationConnectionInput(input IntegrationConnectionInput) integrationconnectionaction.Input {
	return integrationconnectionaction.Input{
		Type: domain.IntegrationConnectionType(input.Type), Name: input.Name, Description: input.Description,
		Configuration: integrationconnectionaction.Configuration{
			APIURL: input.Configuration.APIURL, APIKey: input.Configuration.APIKey,
		},
	}
}

// integrationConnectionTestInput 转换连接器测试输入。
func integrationConnectionTestInput(input IntegrationConnectionTestInput) integrationconnectionaction.ConnectionInput {
	return integrationconnectionaction.ConnectionInput{
		Type: domain.IntegrationConnectionType(input.Type),
		Configuration: integrationconnectionaction.Configuration{
			APIURL: input.Configuration.APIURL, APIKey: input.Configuration.APIKey,
		},
	}
}

// integrationConnectionFromAction 转换连接器输出。
func integrationConnectionFromAction(input integrationconnectionaction.Record) IntegrationConnection {
	return IntegrationConnection{
		ID: input.ID, Type: IntegrationConnectionType(input.Type), Name: input.Name, Description: input.Description,
		Configuration: IntegrationConnectionConfiguration{
			APIURL: input.Configuration.APIURL, APIKey: input.Configuration.APIKey,
		},
		Status: IntegrationConnectionStatus(input.Status), LastTestedAt: input.LastTestedAt,
	}
}

// integrationConnectionFieldKeys 映射连接器校验错误。
func integrationConnectionFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		integrationconnectionaction.ValidationTypeInvalid:        cervii18n.FieldIntegrationConnectionTypeInvalid,
		integrationconnectionaction.ValidationNameRequired:       cervii18n.FieldIntegrationConnectionNameRequired,
		integrationconnectionaction.ValidationNameTooLong:        cervii18n.FieldIntegrationConnectionNameTooLong,
		integrationconnectionaction.ValidationNameDuplicate:      cervii18n.FieldIntegrationConnectionNameDuplicate,
		integrationconnectionaction.ValidationDescriptionTooLong: cervii18n.FieldIntegrationConnectionDescriptionTooLong,
		integrationconnectionaction.ValidationAPIKeyRequired:     cervii18n.FieldIntegrationConnectionAPIKeyRequired,
		integrationconnectionaction.ValidationAPIKeyTooLong:      cervii18n.FieldIntegrationConnectionAPIKeyTooLong,
		integrationconnectionaction.ValidationAPIURLRequired:     cervii18n.FieldIntegrationConnectionAPIURLRequired,
		integrationconnectionaction.ValidationAPIURLInvalid:      cervii18n.FieldIntegrationConnectionAPIURLInvalid,
	}
	return translateValidationFields(fields, keys)
}
