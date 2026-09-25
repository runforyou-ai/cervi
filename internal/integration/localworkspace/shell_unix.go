//go:build !windows

package localworkspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	// loginEnvironmentTimeout 是读取登录 shell 环境变量的时限。
	loginEnvironmentTimeout = 10 * time.Second
	// loginEnvironmentMarker 标出登录 shell 环境变量输出的起点，之前的内容是 profile 的输出。
	loginEnvironmentMarker = "__CERVI_LOGIN_ENV__"
)

// userShell 返回用户的登录 shell；未设置 SHELL 时 macOS 使用 zsh，其他系统使用 sh。
func userShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/sh"
}

// loginEnvironment 缓存用户登录 shell 的环境变量，读取成功或超时降级后不再重复读取。
var loginEnvironment struct {
	mu          sync.Mutex
	environment []string
}

// commandEnvironment 返回命令的环境变量，即用户登录 shell 的环境变量，首次调用时读取；调用方取消时返回错误且不缓存结果。
func commandEnvironment(ctx context.Context) ([]string, error) {
	loginEnvironment.mu.Lock()
	defer loginEnvironment.mu.Unlock()
	if loginEnvironment.environment != nil {
		return loginEnvironment.environment, nil
	}
	environment, err := readLoginEnvironment(ctx, userShell())
	if err != nil {
		return nil, err
	}
	loginEnvironment.environment = environment
	return environment, nil
}

// readLoginEnvironment 以登录方式启动 shell 读取环境变量，profile 的输出与启动的后台进程不影响结果；读取失败或超时时使用当前进程的环境变量，调用方取消时返回错误。
// 返回的环境变量不含 BASH_ENV 与 ENV，执行命令的 shell 不读取任何启动文件。
func readLoginEnvironment(ctx context.Context, shell string) ([]string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, loginEnvironmentTimeout)
	defer cancel()
	var output bytes.Buffer
	err := runProcessTree(exec.CommandContext(probeCtx, shell, "-l", "-c", "printf '"+loginEnvironmentMarker+"'; env -0"), &output)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	_, listed, found := bytes.Cut(output.Bytes(), []byte(loginEnvironmentMarker))
	environment := os.Environ()
	if err != nil || !found {
		slog.Warn("读取登录 shell 环境变量失败，命令使用当前进程的环境变量", "error", err)
	} else {
		environment = strings.Split(string(listed), "\x00")
	}
	// 只保留键值对，去掉 bash 与 sh 在非交互模式下读取启动文件的变量。
	return slices.DeleteFunc(environment, func(entry string) bool {
		name, _, ok := strings.Cut(entry, "=")
		return !ok || name == "BASH_ENV" || name == "ENV"
	}), nil
}

// shellCommand 用 bash 执行命令，没有 bash 时使用 sh；命令环境不含启动文件变量，不读取任何启动文件。
func shellCommand(ctx context.Context, command string) *exec.Cmd {
	shell := "/bin/sh"
	if path, err := exec.LookPath("bash"); err == nil {
		shell = path
	}
	return exec.CommandContext(ctx, shell, "-c", command)
}

// executableCommand 创建直接运行可执行文件的命令，ctx 结束时终止命令进程。
func executableCommand(ctx context.Context, path string, args []string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, path, args...), nil
}

// processTree 是以命令进程为组长的独立进程组。
type processTree struct {
	cmd *exec.Cmd
}

// newProcessTree 让命令在独立进程组中启动。
func newProcessTree(cmd *exec.Cmd) (*processTree, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	return &processTree{cmd: cmd}, nil
}

// start 启动命令，进程组随命令进程建立。
func (t *processTree) start() error {
	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("无法启动命令：%w", err)
	}
	return nil
}

// kill 终止整个进程组，进程组已不存在时视为成功。
func (t *processTree) kill() error {
	if t.cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-t.cmd.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}

// close 释放进程树资源。
func (t *processTree) close() {}
