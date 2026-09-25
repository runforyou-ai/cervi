//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	websearchaction "github.com/runforyou-ai/cervi/internal/actions/websearch"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	"github.com/runforyou-ai/cervi/internal/integration/websearch"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

// webSearchOps 持有联网搜索设置的 Action 和 Query。
type webSearchOps struct {
	getWebSearchSettings    *websearchaction.GetSettingsQuery
	updateWebSearchSettings *websearchaction.UpdateSettingsAction
	testWebSearchService    *websearchaction.TestAction
}

// newWebSearchOps 创建联网搜索设置的业务依赖。
func newWebSearchOps(db *bun.DB, connectionRunner *connectiontest.Runner) webSearchOps {
	return webSearchOps{
		getWebSearchSettings:    websearchaction.NewGetSettingsQuery(db),
		updateWebSearchSettings: websearchaction.NewUpdateSettingsAction(db),
		testWebSearchService:    websearchaction.NewTestAction(connectionRunner, websearch.NewClient()),
	}
}

// GetWebSearchSettings 读取当前企业的联网搜索设置。
func (o *directOperations) GetWebSearchSettings(ctx context.Context, meta RequestMeta, identity *servermodels.Identity) (WebSearchSettings, error) {
	config, err := o.getWebSearchSettings.Execute(ctx, identity)
	if err != nil {
		if ctx.Err() != nil {
			return WebSearchSettings{}, ctx.Err()
		}
		slog.Warn("读取联网搜索设置失败", "organization_id", identity.Organization.ID, "error", err)
		return WebSearchSettings{}, FailedError(meta, cervii18n.ErrorWebSearchSettingsLoadFailed)
	}
	return webSearchSettingsFromConfig(config), nil
}

// UpdateWebSearchSettings 修改当前企业的联网搜索设置。
func (o *directOperations) UpdateWebSearchSettings(ctx context.Context, meta RequestMeta, identity *servermodels.Identity, input WebSearchSettings) (WebSearchSettings, error) {
	var config *websearch.Config
	if input.Service != nil {
		converted := webSearchConfig(*input.Service)
		config = &converted
	}
	saved, err := o.updateWebSearchSettings.Execute(ctx, identity, config)
	if err != nil {
		if ctx.Err() != nil {
			return WebSearchSettings{}, ctx.Err()
		}
		if validationError, ok := errors.AsType[*common.FieldError](err); ok {
			return WebSearchSettings{}, InvalidError(meta, cervii18n.ErrorValidationFailed, webSearchFieldKeys(validationError.Fields))
		}
		if errors.Is(err, common.ErrIdentityInvalid) {
			return WebSearchSettings{}, SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
		}
		slog.Warn("修改联网搜索设置失败", "organization_id", identity.Organization.ID, "error", err)
		return WebSearchSettings{}, FailedError(meta, cervii18n.ErrorWebSearchSettingsUpdateFailed)
	}
	return webSearchSettingsFromConfig(saved), nil
}

// TestWebSearchService 用草稿配置执行一次搜索，验证搜索服务可用。
func (o *directOperations) TestWebSearchService(ctx context.Context, meta RequestMeta, _ *servermodels.Identity, input WebSearchService) error {
	err := o.testWebSearchService.Execute(ctx, webSearchConfig(input))
	if err == nil {
		return nil
	}
	if validationError, ok := errors.AsType[*common.FieldError](err); ok {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, webSearchFieldKeys(validationError.Fields))
	}
	return webSearchError(ctx, meta, err)
}

// webSearchError 按连接失败原因转换调用搜索服务产生的错误。
func webSearchError(ctx context.Context, meta RequestMeta, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_, kind, _ := connectiontest.Details(err)
	switch kind {
	case connectiontest.FailureUnauthorized:
		return UnavailableError(meta, cervii18n.ErrorWebSearchAuthenticationFailed, nil)
	case connectiontest.FailureForbidden:
		return UnavailableError(meta, cervii18n.ErrorWebSearchAuthorizationFailed, nil)
	case connectiontest.FailureRateLimited:
		return UnavailableError(meta, cervii18n.ErrorWebSearchRateLimited, nil)
	default:
		return UnavailableError(meta, cervii18n.ErrorWebSearchTestFailed, nil)
	}
}

// webSearchFieldKeys 转换联网搜索设置的字段校验结果。
func webSearchFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		websearchaction.ValidationProviderInvalid: cervii18n.FieldWebSearchProviderInvalid,
		websearchaction.ValidationAPIKeyRequired:  cervii18n.FieldAPIKeyRequired,
		websearchaction.ValidationBaseURLRequired: cervii18n.FieldWebSearchBaseURLRequired,
		websearchaction.ValidationBaseURLInvalid:  cervii18n.FieldWebSearchBaseURLInvalid,
	}
	return translateValidationFields(fields, keys)
}

// webSearchConfig 转换搜索服务契约。
func webSearchConfig(service WebSearchService) websearch.Config {
	return websearch.Config{Provider: domain.WebSearchProvider(service.Provider), APIKey: service.APIKey, BaseURL: service.BaseURL}
}

// webSearchSettingsFromConfig 转换企业联网搜索设置契约。
func webSearchSettingsFromConfig(config *websearch.Config) WebSearchSettings {
	if config == nil {
		return WebSearchSettings{}
	}
	return WebSearchSettings{Service: &WebSearchService{
		Provider: WebSearchProvider(config.Provider), APIKey: config.APIKey, BaseURL: config.BaseURL,
	}}
}
