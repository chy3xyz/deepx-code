package tui

import (
	"deepx/agent"
	"deepx/config"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
)

var providerNames = []string{"deepseek", "xiaomi-mimo", "kimi", "minimax"}

func (m *model) openProviderModal() {
	m.showProviderModal = true
	m.providerStep = 0
	m.providerSelectIdx = 0
	m.providerApiKeyErr = ""
	m.providerApiKeyInput.SetValue("")
	m.providerApiKeyInput.Focus()
	m.input.Blur()
}

func (m *model) closeProviderModal() {
	m.showProviderModal = false
	m.providerApiKeyInput.Blur()
	m.input.Focus()
}

func (m *model) handleProviderKey(msg string) bool {
	if !m.showProviderModal {
		return false
	}
	switch m.providerStep {
	case 0:
		switch msg {
		case "up", "k":
			if m.providerSelectIdx > 0 {
				m.providerSelectIdx--
			}
		case "down", "j":
			if m.providerSelectIdx < len(providerNames)-1 {
				m.providerSelectIdx++
			}
		case "enter":
			m.providerStep = 1
			m.providerApiKeyErr = ""
			m.providerApiKeyInput.SetValue("")
		case "esc", "ctrl+c":
			m.closeProviderModal()
		}
	case 1:
		switch msg {
		case "enter":
			m.submitProviderApiKey()
		case "esc":
			m.providerStep = 0
			m.providerApiKeyErr = ""
		case "ctrl+c":
			m.closeProviderModal()
		}
	}
	return true
}

func (m *model) submitProviderApiKey() {
	key := strings.TrimSpace(m.providerApiKeyInput.Value())
	if key == "" {
		m.providerApiKeyErr = "API key 不能为空"
		return
	}
	name := providerNames[m.providerSelectIdx]

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{Providers: make(map[string]*config.ProviderEntry)}
	}

	// 初始化 provider entry 如果不存在
	if _, ok := cfg.Providers[name]; !ok {
		cfg.Providers[name] = providerDefaults(name)
	}
	entry := cfg.Providers[name]

	// 填 api key 到 flash 和 pro(若存在)
	if entry.Flash != nil {
		entry.Flash.APIKey = key
	}
	if entry.Pro != nil {
		entry.Pro.APIKey = key
	}

	if err := config.Save(cfg); err != nil {
		m.providerApiKeyErr = "保存失败: " + err.Error()
		return
	}

	// 重新加载并更新 ProviderManager
	loaded, err := config.Load()
	if err == nil {
		m.pm = agent.NewProviderManager(loaded)
	}

	path, _ := config.Path()
	m.appendChat("System", fmt.Sprintf("%s API key 已保存到 %s", name, path))
	m.closeProviderModal()
}

func providerDefaults(name string) *config.ProviderEntry {
	home, _ := os.UserHomeDir()
	_ = home
	switch name {
	case "deepseek":
		return &config.ProviderEntry{
			Priority: 1,
			Flash:    &config.ModelEntry{BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash"},
			Pro:      &config.ModelEntry{BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro"},
			CacheTTL: "1h",
		}
	case "xiaomi-mimo":
		return &config.ProviderEntry{
			Priority: 2,
			Flash:    &config.ModelEntry{BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Model: "MiMo-V2.5"},
			Pro:      &config.ModelEntry{BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Model: "MiMo-V2.5-Pro"},
			CacheTTL: "30m",
		}
	case "kimi":
		return &config.ProviderEntry{
			Priority: 3,
			Flash:    &config.ModelEntry{BaseURL: "https://api.kimi.com/coding/v1", Model: "kimi-for-coding"},
			Pro:      &config.ModelEntry{BaseURL: "https://api.kimi.com/coding/v1", Model: "kimi-k2.6"},
		}
	case "minimax":
		return &config.ProviderEntry{
			Priority: 4,
			Flash:    &config.ModelEntry{BaseURL: "https://api.minimaxi.com/v1", Model: "MiniMax-M2.7-highspeed"},
			Pro:      &config.ModelEntry{BaseURL: "https://api.minimaxi.com/v1", Model: "MiniMax-M2.7"},
		}
	}
	return &config.ProviderEntry{Priority: 99}
}

func (m model) providerModalBlock() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(highlightColor).Render("配置 Provider")
	hint := lipgloss.NewStyle().Foreground(subtleColor).Render("选择 provider 并填入 API key")
	dim := lipgloss.NewStyle().Foreground(dimColor)

	parts := []string{title, "", hint, ""}

	if m.providerStep == 0 {
		parts = append(parts, dim.Render("Provider:"))
		for i, name := range providerNames {
			cursor := "  "
			if i == m.providerSelectIdx {
				cursor = "> "
			}
			parts = append(parts, cursor+name)
		}
	} else {
		name := providerNames[m.providerSelectIdx]
		parts = append(parts, dim.Render("Provider: "+name))
		parts = append(parts, dim.Render("API Key:"))
		parts = append(parts, "  "+m.providerApiKeyInput.View())
		if m.providerApiKeyErr != "" {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("✗ "+m.providerApiKeyErr))
		}
	}

	parts = append(parts, "", dim.Render("Enter 确认 · Esc 返回 · Ctrl+C 关闭"))

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	modalWidth := 56
	if maxW := m.width - 4; modalWidth > maxW {
		modalWidth = maxW
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(highlightColor).
		Padding(1, 2).
		Width(modalWidth).
		Render(content)
}
