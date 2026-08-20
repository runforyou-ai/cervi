// Package token 提供登录令牌的签发和哈希。
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

const defaultDuration = 30 * 24 * time.Hour

// Issued 表示新签发的登录令牌。
type Issued struct {
	Token     string
	TokenHash string
	ExpiresAt time.Time
}

// Issue 签发有效期为三十天的登录令牌。
func Issue() (Issued, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return Issued{}, err
	}
	value := base64.RawURLEncoding.EncodeToString(buffer)
	return Issued{
		Token:     value,
		TokenHash: Hash(value),
		ExpiresAt: time.Now().Add(defaultDuration),
	}, nil
}

// Hash 计算登录令牌的 SHA-256 哈希。
func Hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
