// Package storage 管理应用存储的生命周期。
package storage

// Storage 定义存储的生命周期接口。
type Storage interface {
	// Close 释放存储连接及相关资源。
	Close() error
}
