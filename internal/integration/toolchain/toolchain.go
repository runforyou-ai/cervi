// Package toolchain 在本机准备 Agent 命令执行使用的 uv、Node.js 与默认 Python，并给出命令的环境变量。
//
// 工具链根目录下 dist/<名称>/<版本> 只放发行物，升级时整体替换；python、uv-tools、npm-global 与 bin 跨升级保留。
// 运行环境只作用于 Agent 执行的命令，不修改 shell 配置、系统 PATH 与 Windows 注册表。
// 命令优先使用托管解释器，项目已有的 .venv 与 .python-version 按 uv 的规则使用；用户 uv 配置中的离线与禁止下载设置不作用于 Agent 命令。
// 旧版本发行物由持有共享锁的进程保留，没有进程使用时才清理。
package toolchain

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/gofrs/flock"
	"github.com/runforyou-ai/cervi/internal/integration/localworkspace"
	"golang.org/x/mod/semver"
)

const (
	// pythonInstallTimeout 是安装默认 Python 的时限，超时按下载失败处理。
	pythonInstallTimeout = 30 * time.Minute
	// retryBaseInterval 是首次准备失败后的重试间隔，连续失败时逐次翻倍。
	retryBaseInterval = 30 * time.Second
	// retryMaxInterval 是准备失败后的最长重试间隔。
	retryMaxInterval = 30 * time.Minute
	// retryMaxDoublings 是重试间隔翻倍的次数上限，翻倍结果已超过最长间隔。
	retryMaxDoublings = 6
	// staleStagingAge 是清理中断遗留解压目录的最短存在时长。
	staleStagingAge = 24 * time.Hour
	// defaultPythonMarker 是记录已安装默认 Python 版本的文件名。
	defaultPythonMarker = "default-python"
)

// State 是运行环境的准备状态。
type State string

const (
	// StatePreparing 表示运行环境正在准备或尚未开始准备。
	StatePreparing State = "preparing"
	// StateReady 表示已有可用的运行环境。
	StateReady State = "ready"
	// StateFailed 表示最近一次准备失败，等待自动重试。
	StateFailed State = "failed"
)

// Failure 是运行环境准备失败的原因。
type Failure string

const (
	// FailureDownload 表示无法从下载源取得安装文件。
	FailureDownload Failure = "download"
	// FailureVerify 表示下载的安装文件校验失败。
	FailureVerify Failure = "verify"
	// FailureInstall 表示在本机安装失败。
	FailureInstall Failure = "install"
)

// Status 是运行环境的准备状态，失败原因只在失败状态下非空。
type Status struct {
	State   State
	Failure Failure
}

// stepError 是标明失败原因的准备错误。
type stepError struct {
	failure Failure
	err     error
}

// Error 返回底层错误的说明。
func (e *stepError) Error() string { return e.err.Error() }

// Unwrap 返回底层错误。
func (e *stepError) Unwrap() error { return e.err }

// Sources 是企业服务端下发的下载源，空字段使用官方源。
type Sources struct {
	NodeDownloadURL     string
	PythonInstallMirror string
	PyPIIndexURL        string
	NPMRegistry         string
}

// Manager 在后台准备本机运行环境，并给出 Agent 命令使用的环境变量。
type Manager struct {
	root     string
	cache    string
	client   *http.Client
	onChange func()

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu        sync.Mutex
	preparing bool
	// sources 是最近一次开始准备时使用的下载源。
	sources Sources
	// stopPrepare 取消进行中的准备。
	stopPrepare context.CancelFunc
	failures    int
	retryAt     time.Time
	// failure 是最近一次准备失败的原因，准备成功后清空。
	failure Failure
	// inUse 按「名称/版本」保存本进程命令使用过的发行物共享锁，进程结束前不释放。
	inUse map[string]*flock.Flock
}

// DefaultDirs 返回当前操作系统用户的工具链根目录与缓存目录，不随 XDG 变量变化。
func DefaultDirs() (string, string, error) {
	if runtime.GOOS == "windows" {
		// Windows 的用户缓存目录即 %LOCALAPPDATA%。
		local, err := os.UserCacheDir()
		if err != nil {
			return "", "", err
		}
		return filepath.Join(local, "cervi", "toolchains"), filepath.Join(local, "cervi", "cache"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	cache := filepath.Join(home, ".cache", "cervi")
	if runtime.GOOS == "darwin" {
		cache = filepath.Join(home, "Library", "Caches", "cervi")
	}
	return filepath.Join(home, ".local", "share", "cervi", "toolchains"), cache, nil
}

// New 创建运行环境管理器并清理没有进程使用、已被固定版本取代的发行物；onChange 在每次准备开始与结束时调用。
func New(root, cache string, onChange func()) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{root: root, cache: cache, client: &http.Client{}, onChange: onChange, ctx: ctx, cancel: cancel, inUse: map[string]*flock.Flock{}}
	m.removeStale()
	return m
}

// Close 取消进行中的准备，等待其退出后释放本进程持有的发行物共享锁。
func (m *Manager) Close() {
	m.cancel()
	m.wg.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, lock := range m.inUse {
		_ = lock.Unlock()
	}
	clear(m.inUse)
}

// Ensure 在运行环境不是当前固定版本时于后台开始准备，失败后按退避间隔重试；下载源变化时取消进行中的准备并立即按新源重试。
// 返回当前是否已有可用的运行环境。
func (m *Manager) Ensure(sources Sources) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sources != m.sources {
		m.sources, m.failures, m.retryAt = sources, 0, time.Time{}
		if m.stopPrepare != nil {
			m.stopPrepare()
		}
	}
	if !m.preparing && !m.upToDate() && !time.Now().Before(m.retryAt) && m.ctx.Err() == nil {
		ctx, cancel := context.WithCancel(m.ctx)
		m.preparing, m.stopPrepare = true, cancel
		m.wg.Add(1)
		go m.prepare(ctx, sources)
	}
	return m.usable()
}

// Status 返回运行环境的准备状态：已有可用环境时为就绪；已有旧版本时后台升级的进度与失败不在界面展示，命令继续使用旧版本。
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case m.usable():
		return Status{State: StateReady}
	case m.failure != "" && !m.preparing:
		return Status{State: StateFailed, Failure: m.failure}
	default:
		return Status{State: StatePreparing}
	}
}

// Environment 返回 Agent 命令的环境变量：前置当前版本的 uv、Node.js 与稳定命令目录，安装与缓存限定在工具链目录。
// 命令使用的发行物在本进程结束前保持共享锁，其他进程不会将其清理。
func (m *Manager) Environment(sources Sources) localworkspace.Environment {
	return m.environment(sources, m.lockActive("uv", uvVersion), m.lockActive("node", nodeVersion))
}

// lockActive 按优先顺序选出取得共享锁且目录仍在的版本，正被其他进程清理的版本跳过；没有可用版本时返回空串。
func (m *Manager) lockActive(name, pinned string) string {
	for _, version := range m.versions(name, pinned) {
		if m.use(name, version) && dirExists(filepath.Join(m.root, "dist", name, version)) {
			return version
		}
	}
	return ""
}

// use 为本进程使用的发行物版本取得共享锁，已持有时直接返回 true；其他进程正在清理该版本时返回 false。
func (m *Manager) use(name, version string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := name + "/" + version
	if m.inUse[key] != nil {
		return true
	}
	lock := flock.New(filepath.Join(m.root, "dist", name, version+".lock"))
	if locked, err := lock.TryRLock(); err != nil || !locked {
		return false
	}
	m.inUse[key] = lock
	return true
}

// environment 返回使用指定 uv 与 Node.js 版本的命令环境变量，镜像只在环境变量中注入。
func (m *Manager) environment(sources Sources, uv, node string) localworkspace.Environment {
	bin := filepath.Join(m.root, "bin")
	nodeBin := filepath.Join(m.root, "dist", "node", node)
	npmBin := filepath.Join(m.root, "npm-global")
	// Unix 的 Node.js 与 npm 全局命令位于 bin 子目录，Windows 位于目录本身。
	if runtime.GOOS != "windows" {
		nodeBin = filepath.Join(nodeBin, "bin")
		npmBin = filepath.Join(npmBin, "bin")
	}
	variables := []string{
		// 优先使用托管解释器；only-managed 会重建项目中基于系统解释器的 .venv。
		"UV_PYTHON_PREFERENCE=managed",
		// 覆盖用户 uv 配置与登录环境中的离线和禁止下载设置。
		"UV_PYTHON_DOWNLOADS=automatic",
		"UV_OFFLINE=false",
		"UV_PYTHON_INSTALL_DIR=" + filepath.Join(m.root, "python"),
		"UV_PYTHON_BIN_DIR=" + bin,
		"UV_PYTHON_INSTALL_REGISTRY=0",
		"UV_TOOL_DIR=" + filepath.Join(m.root, "uv-tools"),
		"UV_TOOL_BIN_DIR=" + bin,
		"UV_CACHE_DIR=" + filepath.Join(m.cache, "uv"),
		"NPM_CONFIG_PREFIX=" + filepath.Join(m.root, "npm-global"),
		"NPM_CONFIG_CACHE=" + filepath.Join(m.cache, "npm"),
	}
	for _, mirror := range []struct{ name, value string }{
		{"UV_PYTHON_INSTALL_MIRROR", sources.PythonInstallMirror},
		{"UV_DEFAULT_INDEX", sources.PyPIIndexURL},
		{"NPM_CONFIG_REGISTRY", sources.NPMRegistry},
	} {
		if mirror.value != "" {
			variables = append(variables, mirror.name+"="+mirror.value)
		}
	}
	return localworkspace.Environment{
		PathPrefix: []string{filepath.Join(m.root, "dist", "uv", uv), nodeBin, bin, npmBin},
		Variables:  variables,
	}
}

// prepare 准备固定版本的运行环境，结束后记录重试时间并通知调用方；被取消的准备不计为失败。
func (m *Manager) prepare(ctx context.Context, sources Sources) {
	defer m.wg.Done()
	m.onChange()
	err := m.install(ctx, sources)
	cancelled := ctx.Err() != nil
	m.mu.Lock()
	m.preparing = false
	m.stopPrepare()
	switch {
	case err == nil:
		m.failures, m.retryAt, m.failure = 0, time.Time{}, ""
	case cancelled:
	default:
		m.failures++
		m.retryAt = time.Now().Add(min(retryBaseInterval<<min(m.failures-1, retryMaxDoublings), retryMaxInterval))
		// 未标明原因的错误发生在本机安装步骤。
		m.failure = FailureInstall
		if step, ok := errors.AsType[*stepError](err); ok {
			m.failure = step.failure
		}
	}
	retryAt := m.retryAt
	m.mu.Unlock()
	switch {
	case err == nil:
		slog.Info("Agent 运行环境已就绪", "uv", uvVersion, "node", nodeVersion, "python", PythonVersion)
		m.removeStale()
	case !cancelled:
		slog.Warn("准备 Agent 运行环境失败", "error", err, "retry_at", retryAt)
	}
	m.onChange()
}

// install 下载并校验固定版本的 uv 与 Node.js，再用 uv 安装默认 Python。
func (m *Manager) install(ctx context.Context, sources Sources) error {
	uvItem, uvFound := uvArtifacts[platform()]
	nodeItem, nodeFound := nodeArtifacts[platform()]
	if !uvFound || !nodeFound {
		return fmt.Errorf("unsupported platform %s", platform())
	}
	// uv 取自 PyPI 上的 wheel，与 Python 包使用同一个索引。
	uvURL := func(ctx context.Context) (string, error) {
		return packageFileURL(ctx, m.client, cmp.Or(sources.PyPIIndexURL, defaultPyPIIndexURL), "uv", uvItem.file)
	}
	if err := m.installDist(ctx, "uv", uvVersion, uvURL, uvItem, uvWheelContent); err != nil {
		return err
	}
	nodeURL := func(context.Context) (string, error) {
		return strings.Join([]string{cmp.Or(sources.NodeDownloadURL, defaultNodeDownloadURL), "v" + nodeVersion, nodeItem.file}, "/"), nil
	}
	if err := m.installDist(ctx, "node", nodeVersion, nodeURL, nodeItem, ""); err != nil {
		return err
	}
	marker := filepath.Join(m.root, defaultPythonMarker)
	if installed, _ := os.ReadFile(marker); string(installed) == PythonVersion {
		return nil
	}
	// 默认 Python 经命令执行安装，与 Agent 命令使用同一套环境变量与进程管理，不读取用户的 uv 配置文件。
	if !m.use("uv", uvVersion) || !m.use("node", nodeVersion) {
		return errors.New("toolchain is being removed by another process")
	}
	shell := localworkspace.New(m.root, m.environment(sources, uvVersion, nodeVersion))
	timeout := pythonInstallTimeout
	response, err := shell.Execute(ctx, &filesystem.ExecuteRequest{
		Command: "uv python install " + PythonVersion + " --default --no-registry --no-config --preview-features python-install-default",
		Timeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("install python: %w", err)
	}
	if response.TimedOut || response.ExitCode == nil || *response.ExitCode != 0 {
		err := fmt.Errorf("install python: %s", strings.TrimSpace(response.Output))
		// uv 下载解释器失败时输出 Failed to download，超时通常发生在下载过程中。
		if response.TimedOut || strings.Contains(response.Output, "Failed to download") {
			return &stepError{failure: FailureDownload, err: err}
		}
		return err
	}
	return os.WriteFile(marker, []byte(PythonVersion), 0o644)
}

// installDist 下载并解压一个发行物到 dist/<名称>/<版本>，目录已存在时直接返回；下载地址在需要下载时解析，content 是压缩包内作为版本目录的目录。
func (m *Manager) installDist(ctx context.Context, name, version string, resolveURL func(context.Context) (string, error), item artifact, content string) error {
	target := filepath.Join(m.root, "dist", name, version)
	if dirExists(target) {
		return nil
	}
	cacheDir := filepath.Join(m.cache, "downloads", name, version)
	archivePath := filepath.Join(cacheDir, item.file)
	// 缓存中已有校验一致的压缩包时不再解析下载地址。
	if checksum(archivePath) != item.sha256 {
		url, err := resolveURL(ctx)
		if err != nil {
			return err
		}
		if archivePath, err = download(ctx, m.client, url, cacheDir, item); err != nil {
			return err
		}
	}
	err := install(archivePath, target, content)
	// 解压完成后压缩包不再需要。
	if err == nil {
		_ = os.RemoveAll(filepath.Dir(archivePath))
	}
	// 其他 Cervi 进程已先完成同一版本时视为成功。
	if err != nil && dirExists(target) {
		return nil
	}
	return err
}

// upToDate 判断固定版本的 uv、Node.js 与默认 Python 是否都已就位。
func (m *Manager) upToDate() bool {
	installed, _ := os.ReadFile(filepath.Join(m.root, defaultPythonMarker))
	return dirExists(filepath.Join(m.root, "dist", "uv", uvVersion)) &&
		dirExists(filepath.Join(m.root, "dist", "node", nodeVersion)) &&
		string(installed) == PythonVersion
}

// usable 判断是否已有可用的 uv、Node.js 与默认 Python，版本可以早于固定版本。
func (m *Manager) usable() bool {
	_, err := os.Stat(filepath.Join(m.root, defaultPythonMarker))
	return err == nil && m.activeVersion("uv", uvVersion) != "" && m.activeVersion("node", nodeVersion) != ""
}

// activeVersion 返回命令优先使用的发行物版本，没有已就位版本时返回空串。
func (m *Manager) activeVersion(name, pinned string) string {
	if versions := m.versions(name, pinned); len(versions) > 0 {
		return versions[0]
	}
	return ""
}

// versions 按优先顺序返回已就位的发行物版本：固定版本在前，其余按语义化版本从高到低。
func (m *Manager) versions(name, pinned string) []string {
	entries, _ := os.ReadDir(filepath.Join(m.root, "dist", name))
	var versions []string
	for _, entry := range entries {
		if entry.IsDir() && semver.IsValid("v"+entry.Name()) {
			versions = append(versions, entry.Name())
		}
	}
	slices.SortFunc(versions, func(a, b string) int {
		switch {
		case a == pinned:
			return -1
		case b == pinned:
			return 1
		default:
			return semver.Compare("v"+b, "v"+a)
		}
	})
	return versions
}

// removeStale 删除中断遗留的解压目录，以及已被固定版本取代且没有任何进程持有共享锁的发行物。
func (m *Manager) removeStale() {
	for _, dist := range []struct{ name, pinned string }{{"uv", uvVersion}, {"node", nodeVersion}} {
		directory := filepath.Join(m.root, "dist", dist.name)
		entries, _ := os.ReadDir(directory)
		for _, entry := range entries {
			path := filepath.Join(directory, entry.Name())
			if strings.HasPrefix(entry.Name(), ".staging-") {
				if info, err := entry.Info(); err == nil && time.Since(info.ModTime()) > staleStagingAge {
					_ = os.RemoveAll(path)
				}
				continue
			}
			if !entry.IsDir() || entry.Name() == dist.pinned || !dirExists(filepath.Join(directory, dist.pinned)) {
				continue
			}
			// 取得独占锁说明没有进程在使用该版本。
			lock := flock.New(path + ".lock")
			if locked, err := lock.TryLock(); err != nil || !locked {
				continue
			}
			if err := os.RemoveAll(path); err != nil {
				slog.Warn("清理旧版本运行环境失败", "path", path, "error", err)
			}
			_ = lock.Unlock()
			_ = os.Remove(path + ".lock")
		}
	}
}

// dirExists 判断路径是否为已存在的目录。
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
