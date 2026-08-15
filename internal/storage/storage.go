// Package storage selects and owns the storage implementation for the current
// deployment target.
package storage

// Storage is the lifecycle shared by target-specific storage implementations.
type Storage interface {
	Close() error
}
