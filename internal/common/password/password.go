// Package password 提供密码校验、哈希和比对能力。
package password

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const bcryptMaxBytes = 72

var (
	ErrTooShort = errors.New("password is too short")
	ErrTooLong  = errors.New("password is too long")
)

// Validate 校验密码长度是否满足当前规则。
func Validate(value string) error {
	if utf8.RuneCountInString(value) < 8 {
		return ErrTooShort
	}
	if len([]byte(value)) > bcryptMaxBytes {
		return ErrTooLong
	}
	return nil
}

// Hash 生成 bcrypt 密码哈希。
func Hash(value string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	return string(hash), err
}

// Matches 判断明文密码是否匹配 bcrypt 哈希。
func Matches(hash, value string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(value)) == nil
}
