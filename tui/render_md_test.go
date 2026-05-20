package tui

import (
	"deepx/agent"
	"deepx/session"
	"strings"
	"testing"
)

// TestRenderMarkdownTable 验证 GFM table 渲染:边框、对齐、cell 内 inline markdown。
func TestRenderMarkdownTable(t *testing.T) {
	sample := `**🐋 deepx**: 看下表

| 语法     | 渲染     | 备注      |
|:---------|:--------:|----------:|
| **bold** | 粗体     | 行首加粗  |
| ` + "`code`" + ` | 黄色 | inline    |
| *em*     | 斜体     | 单星号    |

over.`
	m := &model{}
	out := m.renderMarkdown(sample, 80)
	visible := strings.ReplaceAll(out, "\x1b", "ESC")
	t.Log("\n" + visible)
	if !strings.Contains(out, "┌") || !strings.Contains(out, "└") || !strings.Contains(out, "│") {
		t.Fatal("table borders missing")
	}
	if strings.Contains(out, "**bold**") {
		t.Fatal("bold marker still literal inside table cell")
	}
}

// TestRenderMarkdownGobRestore 用真实 history.gob 跑全量渲染,验证 fence 不平衡时
// 后续 message 不再被卡在 code block 里(bold/italic/code 仍能正常渲染)。
func TestRenderMarkdownGobRestore(t *testing.T) {
	sess, err := session.New("/Users/solly/data/develop/github/deepx")
	if err != nil {
		t.Skipf("no session: %v", err)
	}
	var hist []agent.ChatMessage
	if err := sess.LoadGob("history.gob", &hist); err != nil || len(hist) == 0 {
		t.Skipf("no gob: %v", err)
	}
	raw := rebuildChatFromHistory(hist)

	m := &model{}
	rendered := m.renderMarkdown(raw, 170)

	if !strings.Contains(rendered, "\x1b[1m") {
		t.Fatalf("no bold ANSI in render output")
	}
	// 表格行不能整行被 dim — 至少要有一条 `| ` 起头的行,bold 标记被处理掉
	if strings.Contains(rendered, "| **") {
		t.Errorf("found literal '| **' in output — table row stuck in code block mode (fence reset not working)")
	}
}
