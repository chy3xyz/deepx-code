package main

import (
	"deepx/config"
	"deepx/tui"
	"fmt"
	"os"
)

// 由 goreleaser 在 build 时通过 -ldflags "-X main.version=..." 注入。
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		fmt.Printf("deepx %s (commit %s, built %s)\n", version, commit, date)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "upgrade" {
		if err := tui.RunUpgrade(); err != nil {
			fmt.Fprintln(os.Stderr, "升级失败:", err)
			os.Exit(1)
		}
		return
	}
	cfg, needsSetup, err := loadOrEmptyConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
	if err := tui.Run(cfg, needsSetup, version); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func loadOrEmptyConfig() (*config.Config, bool, error) {
	if !config.Exists() {
		return &config.Config{}, true, nil
	}
	c, err := config.Load()
	if err != nil {
		return nil, false, err
	}
	return c, false, nil
}
