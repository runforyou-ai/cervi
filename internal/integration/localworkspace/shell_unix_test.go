//go:build !windows

package localworkspace

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk/filesystem"
)

// TestExecute 验证命令在默认文件夹中执行，合并输出并返回退出码。
func TestExecute(t *testing.T) {
	backend, root, _ := newTestWorkspace(t)
	result, err := backend.Execute(context.Background(), &filesystem.ExecuteRequest{Command: "pwd; echo err >&2; exit 3"})
	if err != nil {
		t.Fatal(err)
	}
	resolved, _ := filepath.EvalSymlinks(root)
	inRoot := strings.Contains(result.Output, root) || strings.Contains(result.Output, resolved)
	if result.ExitCode == nil || *result.ExitCode != 3 || !inRoot || !strings.Contains(result.Output, "err") || result.TimedOut {
		t.Fatalf("result=%+v", result)
	}
}

// TestExecuteTerminatesProcessGroup 验证超时、取消与命令结束都终止命令启动的整个进程组。
func TestExecuteTerminatesProcessGroup(t *testing.T) {
	backend, root, _ := newTestWorkspace(t)
	marker := filepath.Join(root, "survivor")
	// 后台子进程在 1 秒后写入标记文件，进程组被终止时不会写入。
	background := "(sleep 1; touch " + marker + ") > /dev/null 2>&1 &"
	timeout := 200 * time.Millisecond
	started := time.Now()
	result, err := backend.Execute(context.Background(), &filesystem.ExecuteRequest{Command: background + " sleep 30", Timeout: &timeout})
	if err != nil || !result.TimedOut || time.Since(started) > 5*time.Second {
		t.Fatalf("timeout result=%+v err=%v elapsed=%v", result, err, time.Since(started))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := backend.Execute(ctx, &filesystem.ExecuteRequest{Command: background + " sleep 30"}); err == nil {
		t.Fatal("cancelled command returned no error")
	}
	if result, err := backend.Execute(context.Background(), &filesystem.ExecuteRequest{Command: background + " echo done"}); err != nil || !strings.Contains(result.Output, "done") {
		t.Fatalf("finished result=%+v err=%v", result, err)
	}
	// 后台进程继承输出管道时，主进程退出后同样立即终止。
	inherited := "(sleep 1; touch " + marker + ") & echo parent_done"
	if result, err := backend.Execute(context.Background(), &filesystem.ExecuteRequest{Command: inherited}); err != nil || !strings.Contains(result.Output, "parent_done") {
		t.Fatalf("inherited result=%+v err=%v", result, err)
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("background process survived: %v", err)
	}
}

// TestOutputBufferKeepsHeadAndTail 验证输出超出上限时保留开头与结尾并标记裁剪。
func TestOutputBufferKeepsHeadAndTail(t *testing.T) {
	output := &outputBuffer{}
	output.Write([]byte("HEAD"))
	output.Write([]byte(strings.Repeat("x", maxOutputBytes)))
	output.Write([]byte("TAIL"))
	text := output.String()
	if !output.truncated() || !strings.HasPrefix(text, "HEAD") || !strings.HasSuffix(text, "TAIL") || len(text) > maxOutputBytes+100 {
		t.Fatalf("truncated=%v len=%d", output.truncated(), len(text))
	}
}

// TestReadLoginEnvironment 验证读取登录环境时忽略 profile 输出、立即终止 profile 启动的后台进程，并去掉启动文件变量。
func TestReadLoginEnvironment(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "survivor")
	shell := filepath.Join(dir, "fake-shell")
	script := "#!/bin/sh\necho profile-noise\n(sleep 1; touch " + marker + ") &\nexport BASH_ENV=/tmp/startup ENV=/tmp/startup CERVI_LOGIN=yes\nexec /bin/sh -c \"$3\"\n"
	if err := os.WriteFile(shell, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	environment, err := readLoginEnvironment(context.Background(), shell)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(environment, "\n")
	if !strings.Contains(joined, "CERVI_LOGIN=yes") || strings.Contains(joined, "profile-noise") || strings.Contains(joined, "BASH_ENV=") || strings.Contains(joined, "\nENV=") {
		t.Fatalf("environment=%v", environment)
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("profile background process survived: %v", err)
	}
}

// TestCancelledQueuedOperations 验证命令执行期间排队的文件改动在运行取消后不再执行。
func TestCancelledQueuedOperations(t *testing.T) {
	backend, root, _ := newTestWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	executed := make(chan struct{})
	go func() {
		defer close(executed)
		_, _ = backend.Execute(ctx, &filesystem.ExecuteRequest{Command: "sleep 30"})
	}()
	time.Sleep(300 * time.Millisecond)
	errs := make(chan error, 3)
	go func() { errs <- backend.Write(ctx, &filesystem.WriteRequest{FilePath: "queued.txt", Content: "x"}) }()
	go func() {
		errs <- backend.Edit(ctx, &filesystem.EditRequest{FilePath: "README.md", OldString: "Cervi", NewString: "changed"})
	}()
	go func() { errs <- backend.Delete(ctx, "web/app.ts") }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-executed
	for range 3 {
		if err := <-errs; err == nil {
			t.Fatal("queued operation ran after cancel")
		}
	}
	if _, err := os.Stat(filepath.Join(root, "queued.txt")); !os.IsNotExist(err) {
		t.Fatalf("queued write applied: %v", err)
	}
	if content, _ := os.ReadFile(filepath.Join(root, "README.md")); strings.Contains(string(content), "changed") {
		t.Fatal("queued edit applied")
	}
	if _, err := os.Stat(filepath.Join(root, "web", "app.ts")); err != nil {
		t.Fatalf("queued delete applied: %v", err)
	}
}

// TestReadLoginEnvironmentCancel 验证读取登录环境响应调用方取消并终止登录 shell。
func TestReadLoginEnvironmentCancel(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "profile-done")
	shell := filepath.Join(dir, "slow-shell")
	if err := os.WriteFile(shell, []byte("#!/bin/sh\nsleep 1\ntouch "+marker+"\nexec /bin/sh -c \"$3\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := readLoginEnvironment(ctx, shell); err == nil {
		t.Fatal("cancelled read returned environment")
	}
	time.Sleep(1500 * time.Millisecond)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("login shell survived cancel: %v", err)
	}
}
