package appservice

import "github.com/runforyou-ai/cervi/internal/domain"

// StorageProvider 表示 S3 兼容对象存储提供商。
type StorageProvider string

const (
	StorageProviderGeneric StorageProvider = StorageProvider(domain.StorageProviderGeneric)
	StorageProviderAWS     StorageProvider = StorageProvider(domain.StorageProviderAWS)
	StorageProviderR2      StorageProvider = StorageProvider(domain.StorageProviderR2)
	StorageProviderAliyun  StorageProvider = StorageProvider(domain.StorageProviderAliyun)
	StorageProviderTencent StorageProvider = StorageProvider(domain.StorageProviderTencent)
	StorageProviderBaidu   StorageProvider = StorageProvider(domain.StorageProviderBaidu)
	StorageProviderQiniu   StorageProvider = StorageProvider(domain.StorageProviderQiniu)
	StorageProviderHuawei  StorageProvider = StorageProvider(domain.StorageProviderHuawei)
	StorageProviderUCloud  StorageProvider = StorageProvider(domain.StorageProviderUCloud)
	StorageProviderMinIO   StorageProvider = StorageProvider(domain.StorageProviderMinIO)
	StorageProviderRustFS  StorageProvider = StorageProvider(domain.StorageProviderRustFS)
)

// S3Setting 定义 S3 兼容对象存储配置。
type S3Setting struct {
	Enabled         bool            `json:"enabled"`
	Provider        StorageProvider `json:"provider"`
	Endpoint        string          `json:"endpoint"`
	PublicBaseURL   string          `json:"publicBaseUrl"`
	Region          string          `json:"region"`
	Bucket          string          `json:"bucket"`
	AccessKeyID     string          `json:"accessKeyId"`
	SecretAccessKey string          `json:"secretAccessKey"`
	ForcePathStyle  bool            `json:"forcePathStyle"`
}

// S3SettingInput 定义保存和测试对象存储配置的输入。
type S3SettingInput struct {
	Enabled         bool            `json:"enabled"`
	Provider        StorageProvider `json:"provider"`
	Endpoint        string          `json:"endpoint"`
	PublicBaseURL   string          `json:"publicBaseUrl"`
	Region          string          `json:"region"`
	Bucket          string          `json:"bucket"`
	AccessKeyID     string          `json:"accessKeyId"`
	SecretAccessKey string          `json:"secretAccessKey"`
	ForcePathStyle  bool            `json:"forcePathStyle"`
}
