// Package recordid 校验记录标识。
package recordid

import (
	"strings"

	"github.com/google/uuid"
)

// ValidUUID 判断记录标识是否为规范 UUID。
func ValidUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && strings.EqualFold(parsed.String(), value)
}
