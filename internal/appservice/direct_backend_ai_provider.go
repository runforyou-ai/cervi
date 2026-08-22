//go:build server

package appservice

import (
	"context"
	"errors"
	"log/slog"

	aiprovideraction "github.com/runforyou-ai/cervi/internal/actions/aiprovider"
	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// ListAIProviders 返回当前企业的 AI 供应商列表。
func (b *DirectBackend) ListAIProviders(ctx context.Context, meta RequestMeta) (AIProviderList, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return AIProviderList{}, err
	}
	providers, err := b.listAIProviders.Execute(ctx, identity)
	if err != nil {
		return AIProviderList{}, b.aiProviderError(ctx, meta, err, cervii18n.ErrorAIProviderListFailed, identity.Organization.ID)
	}
	output := make([]AIProviderSummary, 0, len(providers))
	for _, provider := range providers {
		modelTypes := make([]AIModelType, 0, len(provider.ModelTypes))
		for _, modelType := range provider.ModelTypes {
			modelTypes = append(modelTypes, AIModelType(modelType))
		}
		output = append(output, AIProviderSummary{
			ID: provider.ID, Brand: AIProviderBrand(provider.Brand), Name: provider.Name, APIURL: provider.APIURL,
			ModelTypes: modelTypes,
		})
	}
	return AIProviderList{Providers: output}, nil
}

// GetAIProvider 返回当前企业中的 AI 供应商详情。
func (b *DirectBackend) GetAIProvider(ctx context.Context, meta RequestMeta, providerID string) (AIProvider, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return AIProvider{}, err
	}
	provider, err := b.getAIProvider.Execute(ctx, identity, providerID)
	if err != nil {
		return AIProvider{}, b.aiProviderError(ctx, meta, err, cervii18n.ErrorAIProviderReadFailed, identity.Organization.ID, "provider_id", providerID)
	}
	return aiProviderFromAction(*provider), nil
}

// ListAvailableAIModels 返回指定品牌的可用模型目录。
func (b *DirectBackend) ListAvailableAIModels(ctx context.Context, meta RequestMeta, brand AIProviderBrand) (AIProviderModelList, error) {
	if _, err := b.authenticate(ctx, meta); err != nil {
		return AIProviderModelList{}, err
	}
	models := aiprovideraction.AvailableModels(domain.AIProviderBrand(brand))
	if len(models) == 0 {
		fields := map[string]cervii18n.Key{"brand": cervii18n.FieldAIProviderBrandInvalid}
		return AIProviderModelList{}, InvalidError(meta, cervii18n.ErrorValidationFailed, fields)
	}
	return AIProviderModelList{Models: aiProviderModelsFromAction(models)}, nil
}

// CreateAIProvider 创建 AI 供应商。
func (b *DirectBackend) CreateAIProvider(ctx context.Context, meta RequestMeta, input AIProviderInput) (AIProvider, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return AIProvider{}, err
	}
	provider, err := b.createAIProvider.Execute(ctx, identity, aiProviderInput(input))
	if err != nil {
		return AIProvider{}, b.aiProviderMutationError(ctx, meta, err, cervii18n.ErrorAIProviderCreateFailed, identity.Organization.ID)
	}
	slog.Info("AI 供应商创建成功", "organization_id", identity.Organization.ID, "provider_id", provider.ID, "brand", provider.Brand, "model_count", len(provider.Models))
	return aiProviderFromAction(*provider), nil
}

// UpdateAIProvider 修改 AI 供应商。
func (b *DirectBackend) UpdateAIProvider(ctx context.Context, meta RequestMeta, providerID string, input AIProviderInput) (AIProvider, error) {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return AIProvider{}, err
	}
	provider, err := b.updateAIProvider.Execute(ctx, identity, providerID, aiProviderInput(input))
	if err != nil {
		return AIProvider{}, b.aiProviderMutationError(ctx, meta, err, cervii18n.ErrorAIProviderUpdateFailed, identity.Organization.ID, "provider_id", providerID)
	}
	slog.Info("AI 供应商保存成功", "organization_id", identity.Organization.ID, "provider_id", provider.ID, "brand", provider.Brand, "model_count", len(provider.Models))
	return aiProviderFromAction(*provider), nil
}

// DeleteAIProvider 删除 AI 供应商。
func (b *DirectBackend) DeleteAIProvider(ctx context.Context, meta RequestMeta, providerID string) error {
	identity, err := b.authenticate(ctx, meta)
	if err != nil {
		return err
	}
	if err := b.deleteAIProvider.Execute(ctx, identity, providerID); err != nil {
		return b.aiProviderError(ctx, meta, err, cervii18n.ErrorAIProviderDeleteFailed, identity.Organization.ID, "provider_id", providerID)
	}
	slog.Info("AI 供应商删除成功", "organization_id", identity.Organization.ID, "provider_id", providerID)
	return nil
}

// aiProviderMutationError 转换 AI 供应商写入校验和操作错误。
func (b *DirectBackend) aiProviderMutationError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	var validationError *common.FieldError
	if errors.As(err, &validationError) {
		return InvalidError(meta, cervii18n.ErrorValidationFailed, aiProviderFieldKeys(validationError.Fields))
	}
	return b.aiProviderError(ctx, meta, err, failureKey, organizationID, attributes...)
}

// aiProviderError 转换 AI 供应商通用操作错误。
func (b *DirectBackend) aiProviderError(ctx context.Context, meta RequestMeta, err error, failureKey cervii18n.Key, organizationID string, attributes ...any) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, common.ErrIdentityInvalid) {
		return SessionError(meta, SessionStateLogin, cervii18n.ErrorAuthenticationRequired)
	}
	if errors.Is(err, aiprovideraction.ErrNotFound) {
		return NotFoundError(meta, cervii18n.ErrorAIProviderNotFound)
	}
	logAttributes := []any{"organization_id", organizationID, "failure", failureKey, "error", err}
	slog.Warn("AI 供应商操作失败", append(logAttributes, attributes...)...)
	return FailedError(meta, failureKey)
}

// aiProviderInput 转换 AI 供应商输入。
func aiProviderInput(input AIProviderInput) aiprovideraction.Input {
	models := make([]aiprovideraction.Model, 0, len(input.Models))
	for _, model := range input.Models {
		models = append(models, aiprovideraction.Model{
			Identifier: model.Identifier, Name: model.Name, Type: domain.AIModelType(model.Type),
			InputModalities: aiModelInputModalitiesToDomain(model.InputModalities),
			ContextWindow:   model.ContextWindow, MaxOutputTokens: model.MaxOutputTokens,
		})
	}
	return aiprovideraction.Input{
		Brand: domain.AIProviderBrand(input.Brand), Name: input.Name, APIKey: input.APIKey, APIURL: input.APIURL, Models: models,
	}
}

// aiProviderFromAction 转换 AI 供应商输出。
func aiProviderFromAction(input aiprovideraction.Record) AIProvider {
	return AIProvider{
		ID: input.ID, Brand: AIProviderBrand(input.Brand), Name: input.Name, APIKey: input.APIKey, APIURL: input.APIURL,
		Models: aiProviderModelsFromAction(input.Models),
	}
}

// aiProviderModelsFromAction 转换 AI 模型目录。
func aiProviderModelsFromAction(input []aiprovideraction.Model) []AIProviderModel {
	models := make([]AIProviderModel, 0, len(input))
	for _, model := range input {
		models = append(models, AIProviderModel{
			Identifier: model.Identifier, Name: model.Name, Type: AIModelType(model.Type),
			InputModalities: aiModelInputModalitiesFromDomain(model.InputModalities),
			ContextWindow:   model.ContextWindow, MaxOutputTokens: model.MaxOutputTokens,
		})
	}
	return models
}

// aiModelInputModalitiesToDomain 转换模型输入模态到领域值。
func aiModelInputModalitiesToDomain(input []AIModelInputModality) []domain.AIModelInputModality {
	output := make([]domain.AIModelInputModality, 0, len(input))
	for _, modality := range input {
		output = append(output, domain.AIModelInputModality(modality))
	}
	return output
}

// aiModelInputModalitiesFromDomain 转换领域模型输入模态到应用契约。
func aiModelInputModalitiesFromDomain(input []domain.AIModelInputModality) []AIModelInputModality {
	output := make([]AIModelInputModality, 0, len(input))
	for _, modality := range input {
		output = append(output, AIModelInputModality(modality))
	}
	return output
}

// aiProviderFieldKeys 把 AI 供应商校验错误码映射为本地化文案键。
func aiProviderFieldKeys(fields map[string]common.FieldCode) map[string]cervii18n.Key {
	keys := map[common.FieldCode]cervii18n.Key{
		aiprovideraction.ValidationBrandInvalid:   cervii18n.FieldAIProviderBrandInvalid,
		aiprovideraction.ValidationNameRequired:   cervii18n.FieldAIProviderNameRequired,
		aiprovideraction.ValidationNameTooLong:    cervii18n.FieldAIProviderNameTooLong,
		aiprovideraction.ValidationNameDuplicate:  cervii18n.FieldAIProviderNameDuplicate,
		aiprovideraction.ValidationAPIKeyRequired: cervii18n.FieldAIProviderAPIKeyRequired,
		aiprovideraction.ValidationAPIKeyTooLong:  cervii18n.FieldAIProviderAPIKeyTooLong,
		aiprovideraction.ValidationAPIURLRequired: cervii18n.FieldAIProviderAPIURLRequired,
		aiprovideraction.ValidationAPIURLInvalid:  cervii18n.FieldAIProviderAPIURLInvalid,
		aiprovideraction.ValidationModelsInvalid:  cervii18n.FieldAIProviderModelsInvalid,
	}
	return translateValidationFields(fields, keys)
}
