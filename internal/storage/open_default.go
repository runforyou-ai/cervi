//go:build !server

package storage

import "context"

type noopStorage struct{}

func (noopStorage) Close() error {
	return nil
}

// Open leaves desktop and mobile storage unconfigured until their SQLite
// implementation is introduced.
func Open(context.Context) (Storage, error) {
	return noopStorage{}, nil
}
