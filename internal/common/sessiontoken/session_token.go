// Package sessiontoken 提供登录会话令牌的签发和哈希能力。
package sessiontoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"
)

const defaultDuration = 30 * 24 * time.Hour

// Issued 表示新签发的会话令牌。
type Issued struct {
	Token     string
	TokenHash string
	ExpiresAt time.Time
}

// Issue 签发有效期为三十天的会话令牌。
func Issue() (Issued, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return Issued{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(buffer)
	return Issued{
		Token:     token,
		TokenHash: Hash(token),
		ExpiresAt: time.Now().Add(defaultDuration),
	}, nil
}

// Hash 计算会话令牌的 SHA-256 哈希。
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
