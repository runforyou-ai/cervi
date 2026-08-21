//go:build server

package models

import (
	"time"

	"github.com/uptrace/bun"
)

// File 表示企业上传文件的元数据。
type File struct {
	bun.BaseModel `bun:"table:files,alias:f"`

	ID              string     `bun:"id,pk"`
	OrganizationID  string     `bun:"organization_id"`
	CreatedByUserID string     `bun:"created_by_user_id"`
	Purpose         string     `bun:"purpose"`
	StorageBackend  string     `bun:"storage_backend"`
	StorageKey      string     `bun:"storage_key"`
	OriginalName    string     `bun:"original_name"`
	ContentType     string     `bun:"content_type"`
	ByteSize        int64      `bun:"byte_size"`
	Status          string     `bun:"status"`
	ETag            *string    `bun:"etag"`
	UploadedAt      *time.Time `bun:"uploaded_at"`
	ExpiresAt       *time.Time `bun:"expires_at"`
	CreatedAt       time.Time  `bun:"created_at"`
	UpdatedAt       time.Time  `bun:"updated_at"`
}
