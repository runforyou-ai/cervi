package localworkspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

// Command 创建在叠加后的环境变量中运行的命令，可执行文件按叠加后的 PATH 查找，工作目录为 dir。
// 命令的生命周期由调用方管理。
func Command(ctx context.Context, environment Environment, dir, name string, args ...string) (*exec.Cmd, error) {
	base, err := commandEnvironment(ctx)
	if err != nil {
		return nil, err
	}
	env := environment.apply(base)
	if env == nil {
		env = os.Environ()
	}
	path, err := lookPath(name, env)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(path, args...)
	cmd.Env, cmd.Dir = env, dir
	return cmd, nil
}

// lookPath 在环境变量的 PATH 中查找可执行文件，名称含路径分隔符时直接检查该路径；Windows 按 PATHEXT 补全扩展名。
func lookPath(name string, env []string) (string, error) {
	if strings.ContainsAny(name, `/\`) {
		return exec.LookPath(name)
	}
	path := ""
	for _, entry := range env {
		if key, value, _ := strings.Cut(entry, "="); strings.EqualFold(key, "PATH") && (runtime.GOOS == "windows" || key == "PATH") {
			path = value
		}
	}
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			continue
		}
		if found, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
			return found, nil
		}
	}
	return "", fmt.Errorf("找不到命令 %s", name)
}
