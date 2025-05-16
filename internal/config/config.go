// internal/config/config.go
package config

import (
	"fmt"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Bilibili BilibiliConfig `mapstructure:"bilibili"`
	Download DownloadConfig `mapstructure:"download"`
	Schedule ScheduleConfig `mapstructure:"schedule"`
	Proxy    ProxyConfig    `mapstructure:"proxy"`
	Log      LogConfig      `mapstructure:"log"`
	Advanced AdvancedConfig `mapstructure:"advanced"`
}

type AppConfig struct {
	Name            string        `mapstructure:"name"`
	Env             string        `mapstructure:"env"`
	Port            int           `mapstructure:"port"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type BilibiliConfig struct {
	Cookies struct {
		SESSDATA   string `mapstructure:"SESSDATA"`
		BiliJCT    string `mapstructure:"bili_jct"`
		DedeUserID string `mapstructure:"DedeUserID"`
	} `mapstructure:"cookies"`
	UserAgent string `mapstructure:"user_agent"`
}

type DownloadConfig struct {
	BaseDir       string        `mapstructure:"base_dir"`
	Concurrent    int           `mapstructure:"concurrent"`
	Retry         RetryConfig   `mapstructure:"retry"`
	Timeout       time.Duration `mapstructure:"timeout"`
	NamingPattern string        `mapstructure:"naming_pattern"`
	Quality       string        `mapstructure:"quality"`
	Format        string        `mapstructure:"format"`
}

type RetryConfig struct {
	MaxAttempts int           `mapstructure:"max_attempts"`
	Backoff     time.Duration `mapstructure:"backoff"`
}

type ScheduleConfig struct {
	SyncInterval time.Duration `mapstructure:"sync_interval"`
	MaxHistory   int           `mapstructure:"max_history"`
	Cleanup      CleanupConfig `mapstructure:"cleanup"`
}

type CleanupConfig struct {
	Enabled  bool `mapstructure:"enabled"`
	KeepDays int  `mapstructure:"keep_days"`
}

type ProxyConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	HTTP    string   `mapstructure:"http"`
	HTTPS   string   `mapstructure:"https"`
	Bypass  []string `mapstructure:"bypass"`
}

type LogConfig struct {
	Level    string `mapstructure:"level"`
	Path     string `mapstructure:"path"`
	MaxSize  int    `mapstructure:"max_size"`
	MaxAge   int    `mapstructure:"max_age"`
	Compress bool   `mapstructure:"compress"`
	Stdout   bool   `mapstructure:"stdout"`
}

type AdvancedConfig struct {
	DebugMode   bool          `mapstructure:"debug_mode"`
	EnablePprof bool          `mapstructure:"enable_pprof"`
	CacheTTL    time.Duration `mapstructure:"cache_ttl"`
	RateLimit   int           `mapstructure:"rate_limit"`
}

func Load(path string) (*Config, error) {
	v := viper.New()

	// 基础配置
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	v.AutomaticEnv()
	v.SetEnvPrefix("BILI") // 环境变量前缀 BILI_APP_PORT=8080

	// 设置默认值
	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 监听配置变化
	v.WatchConfig()
	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("检测到配置文件变更:", e.Name)
	})

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "bilibili-collector")
	v.SetDefault("app.env", "development")
	v.SetDefault("app.port", 8080)
	v.SetDefault("app.shutdown_timeout", "30s")

	v.SetDefault("download.base_dir", "./downloads")
	v.SetDefault("download.concurrent", 3)
	v.SetDefault("download.retry.max_attempts", 3)
	v.SetDefault("download.retry.backoff", "2s")
	v.SetDefault("download.timeout", "30s")
}

func (c *Config) Validate() error {
	if c.Bilibili.Cookies.SESSDATA == "" {
		return fmt.Errorf("必须配置 SESSDATA")
	}

	if c.Download.Concurrent <= 0 {
		return fmt.Errorf("并发下载数必须大于 0")
	}

	// if c.Schedule.SyncInterval < time.Minute {
	// 	return fmt.Errorf("同步间隔不能小于 1 分钟")
	// }

	return nil
}

// 安全打印配置（隐藏敏感信息）
func (c *Config) String() string {
	return fmt.Sprintf(`App:
  Name: %s
  Env: %s
  Port: %d
Bilibili:
  UserAgent: %s
Download:
  BaseDir: %s
  Concurrent: %d
Schedule:
  SyncInterval: %v`,
		c.App.Name,
		c.App.Env,
		c.App.Port,
		c.Bilibili.UserAgent,
		c.Download.BaseDir,
		c.Download.Concurrent,
		c.Schedule.SyncInterval,
	)
}
