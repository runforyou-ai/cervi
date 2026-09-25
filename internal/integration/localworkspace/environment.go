package localworkspace

import (
	"os"
	"runtime"
	"slices"
	"strings"
)

// Environment 是命令执行时叠加在基础环境变量上的设置，零值表示不做改动。
type Environment struct {
	// PathPrefix 是依次前置到 PATH 的目录。
	PathPrefix []string
	// Variables 是覆盖同名变量的 KEY=VALUE 项。
	Variables []string
}

// apply 返回叠加后的环境变量，base 为 nil 时以当前进程的环境变量为基础。
func (e Environment) apply(base []string) []string {
	if len(e.PathPrefix) == 0 && len(e.Variables) == 0 {
		return base
	}
	if base == nil {
		base = os.Environ()
	}
	// Windows 的环境变量名不区分大小写。
	sameName := func(a, b string) bool {
		if runtime.GOOS == "windows" {
			return strings.EqualFold(a, b)
		}
		return a == b
	}
	result := make([]string, 0, len(base)+len(e.Variables)+1)
	path := ""
	for _, entry := range base {
		name, value, _ := strings.Cut(entry, "=")
		if sameName(name, "PATH") {
			path = value
			continue
		}
		overridden := slices.ContainsFunc(e.Variables, func(variable string) bool {
			key, _, _ := strings.Cut(variable, "=")
			return sameName(key, name)
		})
		if !overridden {
			result = append(result, entry)
		}
	}
	result = append(result, e.Variables...)
	parts := slices.Clone(e.PathPrefix)
	if path != "" {
		parts = append(parts, path)
	}
	return append(result, "PATH="+strings.Join(parts, string(os.PathListSeparator)))
}
