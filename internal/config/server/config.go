//go:build server

// Package serverconfig 统一加载并校验企业服务端运行配置。
package serverconfig

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
)

const defaultDevelopmentDSN = "postgres://cervi:cervi_local_dev@localhost:5432/cervi?sslmode=disable"

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

// Config 定义企业服务端全部基础设施配置。
type Config struct {
	Environment string         `yaml:"environment"`
	Server      ServerConfig   `yaml:"server"`
	Database    DatabaseConfig `yaml:"database"`
	HTTPS       HTTPSConfig    `yaml:"https"`
	Storage     StorageConfig  `yaml:"storage"`
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
	if err := config.validate(productionBuild); err != nil {
		return Config{}, err
	}
	return config, nil
}

// normalize 统一配置中的枚举和空白字符。
func (config *Config) normalize() {
	config.Environment = strings.ToLower(strings.TrimSpace(config.Environment))
	config.Server.Host = strings.TrimSpace(config.Server.Host)
	config.Database.URL = strings.TrimSpace(config.Database.URL)
	config.HTTPS.Mode = strings.ToLower(strings.TrimSpace(config.HTTPS.Mode))
	config.HTTPS.TLSDataDirectory = strings.TrimSpace(config.HTTPS.TLSDataDirectory)
	config.HTTPS.ACMEEmail = strings.TrimSpace(config.HTTPS.ACMEEmail)
	config.Storage.LocalDirectory = strings.TrimSpace(config.Storage.LocalDirectory)
}

// defaultConfig 返回当前构建模式的服务端默认配置。
func defaultConfig() Config {
	config := Config{
		Environment: "development",
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
	if productionBuild {
		config.Environment = "production"
		config.HTTPS.Mode = "external"
		config.Storage.LocalDirectory = ""
		return config
	}
	config.Database.URL = defaultDevelopmentDSN
	return config
}

// applyEnvironment 使用已设置的环境变量覆盖文件配置。
func applyEnvironment(config *Config) error {
	applyStringEnvironment("CERVI_ENV", &config.Environment)
	applyStringEnvironment("WAILS_SERVER_HOST", &config.Server.Host)
	applyStringEnvironment("DATABASE_URL", &config.Database.URL)
	applyStringEnvironment("CERVI_HTTPS_MODE", &config.HTTPS.Mode)
	applyStringEnvironment("CERVI_TLS_DATA_DIR", &config.HTTPS.TLSDataDirectory)
	applyStringEnvironment("CERVI_ACME_EMAIL", &config.HTTPS.ACMEEmail)
	applyStringEnvironment("FILE_STORAGE_PATH", &config.Storage.LocalDirectory)

	var err error
	if config.Server.Port, err = intEnvironment("WAILS_SERVER_PORT", config.Server.Port); err != nil {
		return err
	}
	if config.Database.MigrationTimeout, err = durationEnvironment("POSTGRES_MIGRATION_TIMEOUT", config.Database.MigrationTimeout); err != nil {
		return err
	}
	return nil
}

// validate 校验配置并阻止生产构建使用开发默认值。
func (config Config) validate(strict bool) error {
	if config.Environment != "development" && config.Environment != "production" {
		return fmt.Errorf("CERVI_ENV 必须是 development 或 production")
	}
	if strict && config.Environment != "production" {
		return fmt.Errorf("生产构建必须使用 production 环境")
	}
	production := strict || config.Environment == "production"
	if !validServerHost(config.Server.Host) {
		return fmt.Errorf("服务监听地址无效")
	}
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		return fmt.Errorf("服务监听端口必须在 1 到 65535 之间")
	}
	if config.Database.URL == "" {
		return fmt.Errorf("必须配置 DATABASE_URL")
	}
	if production && config.Database.URL == defaultDevelopmentDSN {
		return fmt.Errorf("生产环境必须配置 DATABASE_URL")
	}
	databaseURL, err := url.Parse(config.Database.URL)
	if err != nil || databaseURL.Host == "" || databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql" {
		return fmt.Errorf("DATABASE_URL 必须是有效的 PostgreSQL 连接地址")
	}
	if production && strings.Trim(databaseURL.Path, "/") == "" {
		return fmt.Errorf("生产环境 DATABASE_URL 必须显式指定数据库名称")
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
	mode := config.HTTPS.Mode
	if mode != "auto" && mode != "external" && mode != "off" {
		return fmt.Errorf("HTTPS 模式必须是 auto、external 或 off")
	}
	if config.Storage.LocalDirectory == "" {
		return fmt.Errorf("必须配置本地文件存储目录")
	}
	if production && !filepath.IsAbs(config.Storage.LocalDirectory) {
		return fmt.Errorf("生产环境本地文件存储目录必须是绝对路径")
	}
	if mode == "auto" {
		if config.HTTPS.TLSDataDirectory == "" {
			return fmt.Errorf("自动 HTTPS 模式必须配置证书数据目录")
		}
		if production && !filepath.IsAbs(config.HTTPS.TLSDataDirectory) {
			return fmt.Errorf("生产环境证书数据目录必须是绝对路径")
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

// intEnvironment 读取非负整数环境变量。
func intEnvironment(name string, fallback int) (int, error) {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s 必须是非负整数", name)
	}
	return parsed, nil
}

// durationEnvironment 读取正数时长环境变量。
func durationEnvironment(name string, fallback Duration) (Duration, error) {
	value, ok := os.LookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s 必须是正数 Go 时长，例如 30s 或 5m", name)
	}
	return Duration(parsed), nil
}
