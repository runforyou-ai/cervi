//go:build server

// Package setting 实现企业配置领域的应用操作。
package setting

import (
	"net/url"
	"strings"

	"github.com/runforyou-ai/cervi/internal/common"
	"github.com/runforyou-ai/cervi/internal/domain"
)

const s3SettingKey = "object_storage.s3"

var supportedProviders = map[domain.StorageProvider]struct{}{
	domain.StorageProviderGeneric: {},
	domain.StorageProviderAWS:     {},
	domain.StorageProviderR2:      {},
	domain.StorageProviderAliyun:  {},
	domain.StorageProviderTencent: {},
	domain.StorageProviderBaidu:   {},
	domain.StorageProviderQiniu:   {},
	domain.StorageProviderHuawei:  {},
	domain.StorageProviderUCloud:  {},
	domain.StorageProviderMinIO:   {},
	domain.StorageProviderRustFS:  {},
}

// ValidationCode 标识存储配置字段校验结果。
type ValidationCode = common.FieldCode

const (
	ValidationEndpointRequired        ValidationCode = "ENDPOINT_REQUIRED"
	ValidationEndpointInvalid         ValidationCode = "ENDPOINT_INVALID"
	ValidationProviderInvalid         ValidationCode = "PROVIDER_INVALID"
	ValidationRegionRequired          ValidationCode = "REGION_REQUIRED"
	ValidationBucketRequired          ValidationCode = "BUCKET_REQUIRED"
	ValidationAccessKeyIDRequired     ValidationCode = "ACCESS_KEY_ID_REQUIRED"
	ValidationSecretAccessKeyRequired ValidationCode = "SECRET_ACCESS_KEY_REQUIRED"
)

// ValidationError 表示存储配置字段校验失败。
type ValidationError = common.FieldError

// S3Setting 定义企业的 S3 对象存储配置。
type S3Setting struct {
	Enabled         bool                   `json:"enabled"`
	Provider        domain.StorageProvider `json:"provider"`
	Endpoint        string                 `json:"endpoint"`
	Region          string                 `json:"region"`
	Bucket          string                 `json:"bucket"`
	AccessKeyID     string                 `json:"accessKeyId"`
	SecretAccessKey string                 `json:"secretAccessKey"`
	ForcePathStyle  bool                   `json:"forcePathStyle"`
}

// normalizeS3Setting 规范化并校验 S3 配置。
func normalizeS3Setting(input S3Setting) (S3Setting, map[string]ValidationCode) {
	input.Provider = domain.StorageProvider(strings.ToLower(strings.TrimSpace(string(input.Provider))))
	input.Endpoint = strings.TrimSpace(input.Endpoint)
	input.Region = strings.TrimSpace(input.Region)
	input.Bucket = strings.TrimSpace(input.Bucket)
	input.AccessKeyID = strings.TrimSpace(input.AccessKeyID)
	input.SecretAccessKey = strings.TrimSpace(input.SecretAccessKey)

	fields := make(map[string]ValidationCode)
	if _, exists := supportedProviders[input.Provider]; !exists {
		fields["provider"] = ValidationProviderInvalid
	}
	if input.Endpoint == "" {
		fields["endpoint"] = ValidationEndpointRequired
	} else if !validEndpoint(input.Endpoint) {
		fields["endpoint"] = ValidationEndpointInvalid
	}
	if input.Region == "" {
		fields["region"] = ValidationRegionRequired
	}
	if input.Bucket == "" {
		fields["bucket"] = ValidationBucketRequired
	}
	if input.AccessKeyID == "" {
		fields["accessKeyId"] = ValidationAccessKeyIDRequired
	}
	if input.SecretAccessKey == "" {
		fields["secretAccessKey"] = ValidationSecretAccessKeyRequired
	}
	return input, fields
}

// defaultS3Setting 返回尚未配置时使用的初始 S3 配置。
func defaultS3Setting() S3Setting {
	return S3Setting{
		Provider: domain.StorageProviderGeneric,
		Endpoint: "https://s3.us-east-1.amazonaws.com",
		Region:   "us-east-1",
	}
}

// validEndpoint 判断对象存储服务地址是否完整有效。
func validEndpoint(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil &&
		(parsed.Scheme == "http" || parsed.Scheme == "https") &&
		parsed.Host != "" &&
		parsed.User == nil &&
		parsed.RawQuery == "" &&
		parsed.Fragment == ""
}
