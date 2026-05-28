package tui

import (
	"deepx/agent"
	"deepx/config"
	"deepx/web"
	"os"

	tea "charm.land/bubbletea/v2"
)

// Run 启动 TUI 主循环,直到用户退出或发生错误。
func Run(cfg *config.Config, needsSetup bool, version string) error {
	os.Stderr.Write([]byte("\x1b[3J"))

	pm := agent.NewProviderManager(cfg)

	var hub *web.Hub
	var srv *web.Server
	var webURL string
	webEnabled := cfg.WebEnabled()
	webPort := cfg.WebPort()
	if webEnabled {
		wd, _ := os.Getwd()
		_, flashEntry := pm.Resolve("flash")
		_, proEntry := pm.Resolve("pro")
		hub = web.NewHub(flashEntry.Model, proEntry.Model, wd, string(CurrentLang()))
		srv = web.NewServer(hub)
		if url, err := srv.Listen(webPort); err == nil {
			webURL = url
		} else {
			hub = nil
			srv = nil
		}
	}

	p := tea.NewProgram(initialModel(pm, needsSetup, version, hub, webURL))

	if srv != nil {
		srv.OnInput = func(text string) { p.Send(webInputMsg{text: text}) }
		srv.OnReview = func(approve bool) { p.Send(webReviewMsg{approve: approve}) }
		go func() { _ = srv.Serve() }()
		defer srv.Close()
	}

	_, err := p.Run()
	return err
}
