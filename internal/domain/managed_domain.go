package domain

import "strings"

// DomainPrefixMaxLength 是企业域名前缀允许的最大字符数，等于 DNS 标签长度上限。
const DomainPrefixMaxLength = 63

// NormalizeDomainPrefix 去除首尾空白并转为小写。
func NormalizeDomainPrefix(prefix string) string {
	return strings.ToLower(strings.TrimSpace(prefix))
}

// DomainPrefixValid 判断规范化后的前缀是否符合 DNS 标签规则：1 到 63 位小写字母、数字或连字符，首尾不是连字符。
func DomainPrefixValid(prefix string) bool {
	if prefix == "" || len(prefix) > DomainPrefixMaxLength || prefix[0] == '-' || prefix[len(prefix)-1] == '-' {
		return false
	}
	for _, char := range prefix {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
			return false
		}
	}
	return true
}

// ManagedAccessHost 由规范化前缀和托管域名后缀组成企业访问地址。
func ManagedAccessHost(prefix, suffix string) string {
	return prefix + "." + suffix
}
