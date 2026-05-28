// Package config 负责 ~/.deepx/model.yaml 的读写。
//
// YAML 结构 — 多 provider 格式(新):
//
//	providers:
//	  deepseek:
//	    priority: 1
//	    flash:
//	      base_url: https://api.deepseek.com
//	      model: deepseek-v4-flash
//	      api_key: sk-...
//	    pro:
//	      base_url: https://api.deepseek.com
//	      model: deepseek-v4-pro
//	      api_key: sk-...
//
// 向后兼容旧格式(单 provider flash/pro):
//
//	flash:
//	  base_url: https://api.deepseek.com
//	  model: deepseek-v4-flash
//	  api_key: sk-...
//	pro:
//	  base_url: https://api.deepseek.com
//	  model: deepseek-v4-pro
//	  api_key: sk-...
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ModelEntry 单个 role(flash / pro)的完整配置。
type ModelEntry struct {
	BaseURL       string `yaml:"base_url"`
	Model         string `yaml:"model"`
	APIKey        string `yaml:"api_key"`
	ContextWindow int    `yaml:"context_window"` // 上下文窗口大小(tokens)
}

// QuotaConfig 某个 provider 的 token 配额。
type QuotaConfig struct {
	Limit int64  `yaml:"limit"`  // token 总额
	Used  int64  `yaml:"used"`   // 已用 token 数
	Reset string `yaml:"reset"`  // 配额重置日期 "2006-01-02"
}

// PricingConfig 某个 provider 的 token 单价(每百万 token,美元)。
type PricingConfig struct {
	InputPer1M    float64 `yaml:"input_per_1m"`
	OutputPer1M   float64 `yaml:"output_per_1m"`
	CacheHitPer1M float64 `yaml:"cache_hit_per_1m"`
}

// ProviderEntry 单个 AI provider 的完整配置。
type ProviderEntry struct {
	Priority int            `yaml:"priority"`
	Flash    *ModelEntry    `yaml:"flash"`
	Pro      *ModelEntry    `yaml:"pro"`
	Quota    *QuotaConfig   `yaml:"quota"`
	Pricing  *PricingConfig `yaml:"pricing"`
	CacheTTL string         `yaml:"cache_ttl"` // "1h", "30m", "5m"
}

// CacheTTLDuration 解析 cache_ttl 字段为 time.Duration,失败返回 0。
func (p *ProviderEntry) CacheTTLDuration() time.Duration {
	if p == nil || p.CacheTTL == "" {
		return 0
	}
	d, err := time.ParseDuration(p.CacheTTL)
	if err != nil {
		return 0
	}
	return d
}

// WebConfig 本地 web dashboard 配置。
type WebConfig struct {
	Enabled *bool `yaml:"enabled"`
	Port    int   `yaml:"port"`
}

// Config 整份 model.yaml 的反序列化目标。
// 优先读 Providers map,为空则退回旧格式 Flash/Pro 顶层字段。
type Config struct {
	// 新格式:多 provider
	Providers map[string]*ProviderEntry `yaml:"providers"`

	// 旧格式:单一 provider (向后兼容)
	Flash ModelEntry `yaml:"flash"`
	Pro   ModelEntry `yaml:"pro"`
	Web   WebConfig  `yaml:"web"`
}

// SortedProviders 返回按 priority 排序的 provider 列表。
// 旧格式 (flash/pro) 时,构造一个名为 "default" 的 provider。
func (c *Config) SortedProviders() []struct {
	Name   string
	Config *ProviderEntry
} {
	if len(c.Providers) > 0 {
		names := make([]string, 0, len(c.Providers))
		for n := range c.Providers {
			names = append(names, n)
		}
		sort.Slice(names, func(i, j int) bool {
			pi := c.Providers[names[i]].Priority
			pj := c.Providers[names[j]].Priority
			if pi != pj {
				return pi < pj
			}
			return names[i] < names[j]
		})
		out := make([]struct {
			Name   string
			Config *ProviderEntry
		}, len(names))
		for i, n := range names {
			out[i].Name = n
			out[i].Config = c.Providers[n]
		}
		return out
	}

	// 旧格式兼容:把 flash/pro 包成一个 provider
	flash := c.Flash
	if flash.BaseURL == "" {
		flash.BaseURL = defaultBaseURL
		flash.Model = defaultFlashModel
	}
	if flash.ContextWindow <= 0 {
		flash.ContextWindow = defaultContextWindow(flash.Model)
	}
	pro := c.Pro
	if pro.BaseURL == "" {
		pro.BaseURL = defaultBaseURL
		pro.Model = defaultProModel
	}
	if pro.ContextWindow <= 0 {
		pro.ContextWindow = defaultContextWindow(pro.Model)
	}
	return []struct {
		Name   string
		Config *ProviderEntry
	}{
		{Name: "default", Config: &ProviderEntry{
			Priority: 1,
			Flash:    &flash,
			Pro:      &pro,
		}},
	}
}

// PrimaryProvider 返回排序后的第一个可用 provider。
func (c *Config) PrimaryProvider() (string, *ProviderEntry) {
	sorted := c.SortedProviders()
	if len(sorted) == 0 {
		return "", nil
	}
	return sorted[0].Name, sorted[0].Config
}

// WebEnabled 解析 web dashboard 是否开启。
func (c *Config) WebEnabled() bool {
	if v, ok := os.LookupEnv("DEEPX_WEB"); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "off", "0", "false", "no":
			return false
		case "on", "1", "true", "yes":
			return true
		}
	}
	if c != nil && c.Web.Enabled != nil {
		return *c.Web.Enabled
	}
	return true
}

// WebPort 解析 web dashboard 端口。
func (c *Config) WebPort() int {
	if v, ok := os.LookupEnv("DEEPX_WEB_PORT"); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n
		}
	}
	if c != nil {
		return c.Web.Port
	}
	return 0
}

const (
	dirName  = ".deepx"
	fileName = "model.yaml"

	defaultBaseURL       = "https://api.deepseek.com"
	defaultFlashModel    = "deepseek-v4-flash"
	defaultProModel      = "deepseek-v4-pro"
	xiaomiMimoBaseURL    = "https://token-plan-cn.xiaomimimo.com/v1"
	xiaomiMimoFlashModel = "MiMo-V2.5"
	xiaomiMimoProModel   = "MiMo-V2.5-Pro"
	minimaxBaseURL       = "https://api.minimaxi.com/v1"
	minimaxFlashModel    = "MiniMax-M2.7-highspeed"
	minimaxProModel      = "MiniMax-M2.7"
	kimiBaseURL          = "https://api.kimi.com/coding/v1"
	kimiFlashModel       = "kimi-for-coding"
	kimiProModel         = "kimi-k2.6"
)

// Path 返回 ~/.deepx/model.yaml 绝对路径。
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法获取用户目录: %w", err)
	}
	return filepath.Join(home, dirName, fileName), nil
}

// Exists 配置文件是否已存在。
func Exists() bool {
	p, err := Path()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Default 用单一 apiKey 构造初始配置,默认 provider=deepseek。
func Default(apiKey string) *Config {
	return &Config{
		Providers: map[string]*ProviderEntry{
			"deepseek": {
				Priority: 1,
				Flash: &ModelEntry{
					BaseURL:       defaultBaseURL,
					Model:         defaultFlashModel,
					APIKey:        apiKey,
					ContextWindow: defaultContextWindow(defaultFlashModel),
				},
				Pro: &ModelEntry{
					BaseURL:       defaultBaseURL,
					Model:         defaultProModel,
					APIKey:        apiKey,
					ContextWindow: defaultContextWindow(defaultProModel),
				},
				CacheTTL: "1h",
			},
		},
	}
}

// defaultContextWindow 根据模型名推断上下文窗口。
func defaultContextWindow(model string) int {
	lower := strings.ToLower(model)
	if strings.Contains(lower, "deepseek") {
		return 1_048_576
	}
	if strings.Contains(lower, "mi-mo") || strings.Contains(lower, "mimo") {
		if strings.Contains(lower, "pro") {
			return 1_048_576
		}
		return 131_072
	}
	if strings.Contains(lower, "minimax") {
		return 204_800
	}
	if strings.Contains(lower, "kimi") {
		return 262_144
	}
	return 65_536
}

// Load 从 ~/.deepx/model.yaml 读配置。
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("解析 %s: %w", p, err)
	}

	// 填补缺失的 context_window
	for _, entry := range c.Providers {
		if entry.Flash != nil && entry.Flash.ContextWindow <= 0 {
			entry.Flash.ContextWindow = defaultContextWindow(entry.Flash.Model)
		}
		if entry.Pro != nil && entry.Pro.ContextWindow <= 0 {
			entry.Pro.ContextWindow = defaultContextWindow(entry.Pro.Model)
		}
	}
	// 旧格式兼容
	if c.Flash.ContextWindow <= 0 {
		c.Flash.ContextWindow = defaultContextWindow(c.Flash.Model)
	}
	if c.Pro.ContextWindow <= 0 {
		c.Pro.ContextWindow = defaultContextWindow(c.Pro.Model)
	}
	return &c, nil
}

// Save 写配置到 ~/.deepx/model.yaml。
func Save(c *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
