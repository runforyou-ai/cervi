package tenant

import (
	"net"
	"strings"
)

// NormalizeHostname 去除端口、末尾点和大小写差异，得到租户解析与 TLS 共用的域名键。
func NormalizeHostname(value string) string {
	hostname := strings.TrimSpace(value)
	if parsed, _, err := net.SplitHostPort(hostname); err == nil {
		hostname = parsed
	}
	hostname = strings.Trim(hostname, "[]")
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
}
