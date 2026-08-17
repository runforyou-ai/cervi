//go:build server

// Package setting 实现企业配置领域的应用操作。
package setting

import (
	"net/url"
	"strings"
)

const s3SettingKey = "object_storage.s3"

const (
	ProviderGeneric = "generic"
	ProviderAWS     = "aws"
	ProviderR2      = "r2"
	ProviderAliyun  = "aliyun"
	ProviderTencent = "tencent"
	ProviderBaidu   = "baidu"
	ProviderQiniu   = "qiniu"
	ProviderHuawei  = "huawei"
	ProviderUCloud  = "ucloud"
	ProviderMinIO   = "minio"
	ProviderRustFS  = "rustfs"
)

var supportedProviders = map[string]struct{}{
	ProviderGeneric: {},
	ProviderAWS:     {},
	ProviderR2:      {},
	ProviderAliyun:  {},
	ProviderTencent: {},
	ProviderBaidu:   {},
	ProviderQiniu:   {},
	ProviderHuawei:  {},
	ProviderUCloud:  {},
	ProviderMinIO:   {},
	ProviderRustFS:  {},
}

// ValidationCode 标识存储配置字段校验结果。
type ValidationCode string

const (
	ValidationEndpointRequired        ValidationCode = "ENDPOINT_REQUIRED"
	ValidationEndpointInvalid         ValidationCode = "ENDPOINT_INVALID"
	ValidationProviderInvalid         ValidationCode = "PROVIDER_INVALID"
	ValidationRegionRequired          ValidationCode = "REGION_REQUIRED"
	ValidationBucketRequired          ValidationCode = "BUCKET_REQUIRED"
	ValidationAccessKeyIDRequired     ValidationCode = "ACCESS_KEY_ID_REQUIRED"
	ValidationSecretAccessKeyRequired ValidationCode = "SECRET_ACCESS_KEY_REQUIRED"
)

// ValidationError 返回存储配置字段校验结果。
type ValidationError struct {
	Fields map[string]ValidationCode
}

// Error 返回存储配置输入校验错误。
func (e *ValidationError) Error() string {
	return "storage setting validation failed"
}

// S3Setting 定义企业的 S3 对象存储配置。
type S3Setting struct {
	Enabled         bool   `json:"enabled"`
	Provider        string `json:"provider"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	ForcePathStyle  bool   `json:"forcePathStyle"`
}

// normalizeS3Setting 规范化并校验 S3 配置。
func normalizeS3Setting(input S3Setting) (S3Setting, map[string]ValidationCode) {
	input.Provider = strings.ToLower(strings.TrimSpace(input.Provider))
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
		Provider: ProviderGeneric,
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
