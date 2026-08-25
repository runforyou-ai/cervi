//go:build server

package aiprovider

import (
	"net/url"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// ValidationCode 标识模型服务供应商字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationBrandInvalid   ValidationCode = "BRAND_INVALID"
	ValidationNameRequired   ValidationCode = "NAME_REQUIRED"
	ValidationNameTooLong    ValidationCode = "NAME_TOO_LONG"
	ValidationNameDuplicate  ValidationCode = "NAME_DUPLICATE"
	ValidationAPIKeyRequired ValidationCode = "API_KEY_REQUIRED"
	ValidationAPIKeyTooLong  ValidationCode = "API_KEY_TOO_LONG"
	ValidationAPIURLRequired ValidationCode = "API_URL_REQUIRED"
	ValidationAPIURLInvalid  ValidationCode = "API_URL_INVALID"
	ValidationModelsInvalid  ValidationCode = "MODELS_INVALID"
	ValidationModelsInUse    ValidationCode = "MODELS_IN_USE"
)

// ValidationError 表示模型服务供应商字段校验失败。
type ValidationError = common.FieldError

// normalizeInput 规范化模型服务供应商输入并校验模型目录。
func normalizeInput(input Input) (Input, map[string]ValidationCode) {
	fields := make(map[string]ValidationCode)
	input.Name = strings.TrimSpace(input.Name)
	input.APIKey = strings.TrimSpace(input.APIKey)
	input.APIURL = strings.TrimSpace(input.APIURL)

	if input.Brand != domain.AIProviderBrandDeepSeek &&
		input.Brand != domain.AIProviderBrandAlibaba &&
		input.Brand != domain.AIProviderBrandOpenAI {
		fields["brand"] = ValidationBrandInvalid
	}
	if input.Name == "" {
		fields["name"] = ValidationNameRequired
	} else if len([]rune(input.Name)) > 100 {
		fields["name"] = ValidationNameTooLong
	}
	if input.APIKey == "" {
		fields["apiKey"] = ValidationAPIKeyRequired
	} else if len(input.APIKey) > 2048 {
		fields["apiKey"] = ValidationAPIKeyTooLong
	}
	if input.APIURL == "" {
		fields["apiUrl"] = ValidationAPIURLRequired
	} else if !validAPIURL(input.APIURL) {
		fields["apiUrl"] = ValidationAPIURLInvalid
	}

	seen := make(map[string]struct{}, len(input.Models))
	models := make([]Model, 0, len(input.Models))
	for _, model := range input.Models {
		model.Identifier = strings.TrimSpace(model.Identifier)
		model.Name = strings.TrimSpace(model.Name)
		if model.Identifier == "" || len([]rune(model.Identifier)) > 200 ||
			model.Name == "" || len([]rune(model.Name)) > 200 ||
			model.ContextWindow <= 0 || !normalizeModel(&model) {
			fields["models"] = ValidationModelsInvalid
			continue
		}
		if _, exists := seen[model.Identifier]; exists {
			fields["models"] = ValidationModelsInvalid
			continue
		}
		seen[model.Identifier] = struct{}{}
		models = append(models, model)
	}
	if len(models) == 0 {
		fields["models"] = ValidationModelsInvalid
	}
	input.Models = models
	return input, fields
}

// normalizeModel 规范化并校验模型用途与输入模态。
func normalizeModel(model *Model) bool {
	if !validInputModalities(model.InputModalities) {
		return false
	}
	switch model.Type {
	case domain.AIModelTypeChat:
		return model.MaxOutputTokens > 0
	case domain.AIModelTypeEmbedding:
		model.MaxOutputTokens = 0
		return true
	case domain.AIModelTypeRerank:
		model.MaxOutputTokens = 0
		return true
	default:
		return false
	}
}

// validInputModalities 校验模型至少声明一种且不重复的输入模态。
func validInputModalities(modalities []domain.AIModelInputModality) bool {
	if len(modalities) == 0 {
		return false
	}
	seen := make(map[domain.AIModelInputModality]struct{}, len(modalities))
	for _, modality := range modalities {
		switch modality {
		case domain.AIModelInputModalityText,
			domain.AIModelInputModalityImage,
			domain.AIModelInputModalityAudio,
			domain.AIModelInputModalityVideo:
		default:
			return false
		}
		if _, exists := seen[modality]; exists {
			return false
		}
		seen[modality] = struct{}{}
	}
	return true
}

// validAPIURL 校验 API 地址为不含认证信息的完整 HTTP 地址。
func validAPIURL(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.IsAbs() && parsed.Host != "" && parsed.User == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.RawQuery == "" && parsed.Fragment == ""
}
