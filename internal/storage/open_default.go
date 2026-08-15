//go:build !server

package storage

import "context"

type noopStorage struct{}

func (noopStorage) Close() error {
	return nil
}

// Open 返回非 server 构建使用的空存储。
func Open(context.Context) (Storage, error) {
	return noopStorage{}, nil
}
