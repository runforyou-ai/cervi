//go:build server

// Package server 统一加载并校验企业服务端运行配置。
package server

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

var natsNamespacePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Duration 表示配置文件中的 Go 时长。
type Duration time.Duration

// UnmarshalYAML 解析配置文件中的时长字符串。
func (d *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("时长 %q 无效: %w", value, err)
	}
	*d = Duration(parsed)
	return nil
}

// Value 返回标准库时长值。
func (d Duration) Value() time.Duration {
	return time.Duration(d)
}

// Config 定义服务端运行配置。
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	NATS     NATSConfig     `yaml:"nats"`
	HTTPS    HTTPSConfig    `yaml:"https"`
	Storage  StorageConfig  `yaml:"storage"`
}

// ServerConfig 定义 HTTP 服务监听配置。
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// DatabaseConfig 定义 PostgreSQL 连接、连接池和迁移配置。
type DatabaseConfig struct {
	URL                   string   `yaml:"url"`
	MaxOpenConnections    int      `yaml:"maxOpenConnections"`
	MaxIdleConnections    int      `yaml:"maxIdleConnections"`
	ConnectionMaxLifetime Duration `yaml:"connectionMaxLifetime"`
	ConnectionMaxIdleTime Duration `yaml:"connectionMaxIdleTime"`
	ConnectTimeout        Duration `yaml:"connectTimeout"`
	MigrationTimeout      Duration `yaml:"migrationTimeout"`
}

// NATSConfig 定义 NATS 连接和任务命名空间。
type NATSConfig struct {
	URL       string `yaml:"url"`
	Namespace string `yaml:"namespace"`
}

// HTTPSConfig 定义企业服务端 HTTPS 入口配置。
type HTTPSConfig struct {
	Mode             string `yaml:"mode"`
	TLSDataDirectory string `yaml:"tlsDataDirectory"`
	ACMEEmail        string `yaml:"acmeEmail"`
}

// StorageConfig 定义企业服务端本地文件存储配置。
type StorageConfig struct {
	LocalDirectory string `yaml:"localDirectory"`
}

// Load 从显式配置文件和环境变量加载服务端配置。
func Load(path string) (Config, error) {
	config := defaultConfig()
	if strings.TrimSpace(path) != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("读取服务端配置文件: %w", err)
		}
		if err := yaml.UnmarshalWithOptions(data, &config, yaml.Strict()); err != nil {
			return Config{}, fmt.Errorf("解析服务端配置文件: %w", err)
		}
	}
	if err := applyEnvironment(&config); err != nil {
		return Config{}, err
	}
	config.normalize()
	if err := config.validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

// normalize 统一配置中的枚举和空白字符。
func (config *Config) normalize() {
	config.Server.Host = strings.TrimSpace(config.Server.Host)
	config.Database.URL = strings.TrimSpace(config.Database.URL)
	config.NATS.URL = strings.TrimSpace(config.NATS.URL)
	config.NATS.Namespace = strings.TrimSpace(config.NATS.Namespace)
	config.HTTPS.Mode = strings.ToLower(strings.TrimSpace(config.HTTPS.Mode))
	config.HTTPS.TLSDataDirectory = strings.TrimSpace(config.HTTPS.TLSDataDirectory)
	config.HTTPS.ACMEEmail = strings.TrimSpace(config.HTTPS.ACMEEmail)
	config.Storage.LocalDirectory = strings.TrimSpace(config.Storage.LocalDirectory)
}

// defaultConfig 返回服务端默认配置。
func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Database: DatabaseConfig{
			MaxOpenConnections:    8,
			MaxIdleConnections:    2,
			ConnectionMaxLifetime: Duration(30 * time.Minute),
			ConnectionMaxIdleTime: Duration(5 * time.Minute),
			ConnectTimeout:        Duration(time.Minute),
			MigrationTimeout:      Duration(10 * time.Minute),
		},
		HTTPS: HTTPSConfig{Mode: "off"},
		Storage: StorageConfig{
			LocalDirectory: "data/files",
		},
	}
}

// applyEnvironment 使用已设置的环境变量覆盖文件配置。
func applyEnvironment(config *Config) error {
	applyStringEnvironment("WAILS_SERVER_HOST", &config.Server.Host)
	applyStringEnvironment("TLS_MODE", &config.HTTPS.Mode)
	applyStringEnvironment("TLS_DATA_DIR", &config.HTTPS.TLSDataDirectory)
	applyStringEnvironment("TLS_ACME_EMAIL", &config.HTTPS.ACMEEmail)
	applyStringEnvironment("FILE_STORAGE_PATH", &config.Storage.LocalDirectory)
	applyStringEnvironment("NATS_URL", &config.NATS.URL)
	applyStringEnvironment("NATS_NAMESPACE", &config.NATS.Namespace)

	serverPort, err := intEnvironment("WAILS_SERVER_PORT", config.Server.Port)
	if err != nil {
		return err
	}
	config.Server.Port = serverPort
	if err := applyPostgreSQLEnvironment(&config.Database); err != nil {
		return err
	}
	return nil
}

// applyPostgreSQLEnvironment 使用 POSTGRES_* 环境变量覆盖连接地址。
func applyPostgreSQLEnvironment(config *DatabaseConfig) error {
	names := []string{
		"POSTGRES_HOST",
		"POSTGRES_PORT",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_DB",
		"POSTGRES_SSLMODE",
	}
	override := false
	for _, name := range names {
		if _, ok := os.LookupEnv(name); ok {
			override = true
			break
		}
	}
	if !override {
		return nil
	}

	host, err := requiredEnvironment("POSTGRES_HOST")
	if err != nil {
		return err
	}
	portValue, err := requiredEnvironment("POSTGRES_PORT")
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("POSTGRES_PORT 必须是 1 到 65535 之间的整数")
	}
	user, err := requiredEnvironment("POSTGRES_USER")
	if err != nil {
		return err
	}
	password, err := requiredEnvironment("POSTGRES_PASSWORD")
	if err != nil {
		return err
	}
	databaseName, err := requiredEnvironment("POSTGRES_DB")
	if err != nil {
		return err
	}
	sslMode, err := requiredEnvironment("POSTGRES_SSLMODE")
	if err != nil {
		return err
	}

	databaseURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, portValue),
		Path:   databaseName,
	}
	query := databaseURL.Query()
	query.Set("sslmode", sslMode)
	databaseURL.RawQuery = query.Encode()
	config.URL = databaseURL.String()
	return nil
}

// requiredEnvironment 读取必填环境变量。
func requiredEnvironment(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("必须配置 %s", name)
	}
	return value, nil
}

// validate 校验服务端配置。
func (config Config) validate() error {
	if !validServerHost(config.Server.Host) {
		return fmt.Errorf("服务监听地址无效")
	}
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return fmt.Errorf("服务监听端口必须在 1 到 65535 之间")
	}
	if config.Database.URL == "" {
		return fmt.Errorf("必须通过 database.url 或 POSTGRES_* 配置 PostgreSQL")
	}
	databaseURL, err := url.Parse(config.Database.URL)
	if err != nil || databaseURL.Host == "" || databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql" {
		return fmt.Errorf("PostgreSQL 连接地址无效")
	}
	if strings.Trim(databaseURL.Path, "/") == "" {
		return fmt.Errorf("必须显式指定 PostgreSQL 数据库名称")
	}
	if config.Database.MaxOpenConnections < 0 || config.Database.MaxIdleConnections < 0 {
		return fmt.Errorf("PostgreSQL 连接数不能为负数")
	}
	if config.Database.MaxOpenConnections > 0 && config.Database.MaxIdleConnections > config.Database.MaxOpenConnections {
		return fmt.Errorf("PostgreSQL 最大空闲连接数不能超过最大连接数")
	}
	if config.Database.ConnectionMaxLifetime.Value() <= 0 || config.Database.ConnectionMaxIdleTime.Value() <= 0 || config.Database.ConnectTimeout.Value() <= 0 || config.Database.MigrationTimeout.Value() <= 0 {
		return fmt.Errorf("PostgreSQL 时长配置必须为正数")
	}
	if config.NATS.URL == "" {
		return fmt.Errorf("必须配置 NATS 地址")
	}
	if !natsNamespacePattern.MatchString(config.NATS.Namespace) {
		return fmt.Errorf("NATS 命名空间必须匹配 %s", natsNamespacePattern.String())
	}
	mode := config.HTTPS.Mode
	if mode != "auto" && mode != "external" && mode != "off" {
		return fmt.Errorf("HTTPS 模式必须是 auto、external 或 off")
	}
	if mode == "auto" && (config.Server.Port == 80 || config.Server.Port == 443) {
		return fmt.Errorf("自动 HTTPS 模式下服务监听端口不能是 80 或 443")
	}
	if config.Storage.LocalDirectory == "" {
		return fmt.Errorf("必须配置本地文件存储目录")
	}
	if mode == "auto" {
		if config.HTTPS.TLSDataDirectory == "" {
			return fmt.Errorf("自动 HTTPS 模式必须配置证书数据目录")
		}
	}
	return nil
}

// validServerHost 校验监听主机名、IPv4 地址和带方括号的 IPv6 地址。
func validServerHost(host string) bool {
	if host == "" || strings.ContainsAny(host, "/\\ \t\r\n") {
		return false
	}
	if !strings.Contains(host, ":") {
		return true
	}
	if len(host) < 3 || host[0] != '[' || host[len(host)-1] != ']' {
		return false
	}
	return net.ParseIP(host[1:len(host)-1]) != nil
}

// applyStringEnvironment 覆盖非空字符串环境变量。
func applyStringEnvironment(name string, target *string) {
	value, ok := os.LookupEnv(name)
	if ok && strings.TrimSpace(value) != "" {
		*target = strings.TrimSpace(value)
	}
}

// intEnvironment 读取整数环境变量。
func intEnvironment(name string, fallback int) (int, error) {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s 必须是整数", name)
	}
	return parsed, nil
}
