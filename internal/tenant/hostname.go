package tenant

import (
	"net"
	"strconv"
	"strings"
)

// NormalizeHostname 去除端口、末尾点和大小写差异，得到 TLS 使用的主机名。
func NormalizeHostname(value string) string {
	hostname := strings.TrimSpace(value)
	if parsed, _, err := net.SplitHostPort(hostname); err == nil {
		hostname = parsed
	}
	hostname = strings.Trim(hostname, "[]")
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
}

// NormalizeAccessHost 规范化租户访问地址，保留非默认端口。
func NormalizeAccessHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	hostname := NormalizeHostname(value)
	if hostname == "" {
		return ""
	}
	_, port, err := net.SplitHostPort(value)
	if err != nil {
		if strings.Contains(value, ":") && net.ParseIP(strings.Trim(value, "[]")) == nil {
			return ""
		}
		return hostname
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return ""
	}
	if portNumber == 80 || portNumber == 443 {
		return hostname
	}
	return net.JoinHostPort(hostname, strconv.Itoa(portNumber))
}
