package domain

// StorageProvider 定义 S3 兼容对象存储提供商。
type StorageProvider string

const (
	StorageProviderGeneric StorageProvider = "generic"
	StorageProviderAWS     StorageProvider = "aws"
	StorageProviderR2      StorageProvider = "r2"
	StorageProviderAliyun  StorageProvider = "aliyun"
	StorageProviderTencent StorageProvider = "tencent"
	StorageProviderBaidu   StorageProvider = "baidu"
	StorageProviderQiniu   StorageProvider = "qiniu"
	StorageProviderHuawei  StorageProvider = "huawei"
	StorageProviderUCloud  StorageProvider = "ucloud"
	StorageProviderMinIO   StorageProvider = "minio"
	StorageProviderRustFS  StorageProvider = "rustfs"
)
