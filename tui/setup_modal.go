package tui

import (
	"deepx/agent"
	"deepx/config"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func overlayCentered(bg, fg string, width, height int) string {
	fgLines := strings.Split(strings.TrimRight(fg, "\n"), "\n")
	fgH := len(fgLines)
	fgW := 0
	for _, ln := range fgLines {
		if w := ansi.StringWidth(ln); w > fgW {
			fgW = w
		}
	}

	startY := (height - fgH) / 2
	startX := (width - fgW) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	bgLines := strings.Split(bg, "\n")
	for i, fgLine := range fgLines {
		y := startY + i
		if y < 0 || y >= len(bgLines) {
			continue
		}
		bgLines[y] = spliceLineCells(bgLines[y], fgLine, startX, fgW)
	}
	return strings.Join(bgLines, "\n")
}

func spliceLineCells(bg, fg string, atCol, fgW int) string {
	pre := ansi.Cut(bg, 0, atCol)
	if preW := ansi.StringWidth(pre); preW < atCol {
		pre += strings.Repeat(" ", atCol-preW)
	}
	post := ""
	if bgW := ansi.StringWidth(bg); atCol+fgW < bgW {
		post = ansi.Cut(bg, atCol+fgW, bgW)
	}
	return pre + fg + post
}

func (m model) setupModalBlock() string {
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(highlightColor).
		Render(T("setup.title"))

	var hint string
	if m.setupRequired {
		hint = T("setup.hint.first_run")
	} else {
		hint = T("setup.hint.reconfig")
	}
	hintBlock := lipgloss.NewStyle().Foreground(subtleColor).Render(hint)

	inputLabel := lipgloss.NewStyle().Foreground(dimColor).Render(T("setup.input_label"))
	inputBlock := inputLabel + "\n  " + m.setupInput.View()

	var errBlock string
	if m.setupErr != "" {
		errBlock = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Render("✗ " + m.setupErr)
	}

	var footer string
	if m.setupRequired {
		footer = lipgloss.NewStyle().Foreground(dimColor).Render(T("setup.footer.first_run"))
	} else {
		footer = lipgloss.NewStyle().Foreground(dimColor).Render(T("setup.footer.reconfig"))
	}

	parts := []string{title, "", hintBlock, "", inputBlock}
	if errBlock != "" {
		parts = append(parts, "", errBlock)
	}
	parts = append(parts, "", footer)

	content := lipgloss.JoinVertical(lipgloss.Left, parts...)

	modalWidth := 62
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

func (m *model) submitSetup() tea.Cmd {
	val := strings.TrimSpace(m.setupInput.Value())
	if val == "" {
		m.setupErr = T("setup.error.empty")
		return nil
	}
	cfg := config.Default(val)
	if err := config.Save(cfg); err != nil {
		m.setupErr = fmt.Sprintf(T("setup.error.save"), err)
		return nil
	}
	loaded, err := config.Load()
	if err != nil {
		m.setupErr = fmt.Sprintf(T("setup.error.reload"), err)
		return nil
	}
	m.pm = agent.NewProviderManager(loaded)
	m.activeModelRole = "flash"
	m.activeModelID = m.flashModelID()
	if m.activeModelID == "" {
		m.activeModelRole = "pro"
		m.activeModelID = m.proModelID()
	}
	m.showSetup = false
	m.setupRequired = false
	m.setupErr = ""
	m.setupInput.Reset()
	m.setupInput.Blur()
	m.input.Focus()

	path, _ := config.Path()
	m.appendChat("System", T("setup.saved_to")+path)
	return nil
}

func (m *model) openSetupModal() {
	m.showSetup = true
	m.setupRequired = false
	m.setupErr = ""
	m.setupInput.SetValue("")
	m.setupInput.Focus()
	m.input.Blur()
}
