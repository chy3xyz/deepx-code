package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"deepx/agent"
	"deepx/mcp"
	"deepx/tools"

	tea "charm.land/bubbletea/v2"
)

// defaultCacheWarmWindow:重启压缩只在距上次请求这么久之内才有意义 —— 超过此窗口缓存大概率已失效。
// 具体 provider 的 TTL 由 ProviderEntry.CacheTTL 覆盖,未设回退到此默认值。
const defaultCacheWarmWindow = 1 * time.Hour

// cacheWarmWindow 返回当前 provider 的缓存存活窗口;未设则用默认 1h。
func (m *model) cacheWarmWindow() time.Duration {
	if ttl := m.pm.CacheTTL(); ttl > 0 {
		return ttl
	}
	return defaultCacheWarmWindow
}

// restartCompactKeepFactor:启动检测到前缀变化时,仅当"历史 token"(estimateHistoryTokens)≥
// 保留目标(ctxWin×20%)的这个倍数才压缩。
const restartCompactKeepFactor = 2

// prefixSignature 计算前缀变化检测用的签名: hash(核心系统提示词文本 + 内置工具 catalog JSON + 排序后的 mcp.json 配置)
func (m *model) prefixSignature() string {
	core := agent.BuildSystemPrompt(m.workspace, m.skillCatalog, "")

	specs := make([]tools.OpenAIToolSpec, 0, len(tools.Tools))
	for _, t := range tools.Tools {
		specs = append(specs, t.ToOpenAISpec())
	}
	toolsJSON := agent.MarshalToolSpecs(specs)

	servers, _ := mcp.LoadConfig()
	sort.Slice(servers, func(i, j int) bool { return servers[i].Name < servers[j].Name })
	mcpJSON, _ := json.Marshal(servers)

	h := sha256.New()
	h.Write([]byte(core))
	h.Write([]byte{0})
	h.Write([]byte(toolsJSON))
	h.Write([]byte{0})
	h.Write(mcpJSON)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// onPrefixSnapshot 持久化本轮"实际发送"的前缀(供压缩复刻热缓存)+ 当前稳定签名(供重启检测)。
func (m *model) onPrefixSnapshot(msg agent.PrefixSnapshotMsg) {
	if m.session == nil {
		return
	}
	sig := m.prefixSignature()
	m.session.SavePrefixSnapshot(sig, m.activeProvider, msg.Model, msg.SystemPrompt, msg.ToolSpecsJSON)
}

// lastPromptTokens 返回"下一次请求 prompt 大约多大"的 token 数,用于压缩触发判断。
func (m *model) lastPromptTokens() int {
	if m.lastUsage != nil && m.lastUsage.PromptTokens > 0 {
		return m.lastUsage.PromptTokens
	}
	return m.estimatePromptTokens()
}

func (m *model) estimateHistoryTokens() int {
	return agent.EstimateHistoryTokens(m.history)
}

func (m *model) estimatePromptTokens() int {
	return agent.EstimatePromptTokens(m.workspace, m.skillCatalog, m.summary, m.history)
}

// entryForModel 按 provider + model ID 取对应的 ModelEntry。
func (m *model) entryForModel(providerName, id string) agent.ModelEntry {
	return m.pm.EntryForModel(providerName, id)
}

// detectRestartCompaction 在启动加载历史后调用:若签名相对上次会话变了、且历史够大,
// 暂存上次的前缀快照(oldSys/oldTools)并返回 true,表示需要在首请求前跑一次缓存友好压缩。
func (m *model) detectRestartCompaction() bool {
	if m.session == nil {
		return false
	}
	_, proEntry := m.pm.Resolve("pro")
	if proEntry.Model == "" {
		return false
	}
	persistedSig, oldProvider, oldModel, oldSys, oldTools := m.session.LoadPrefixSnapshot()
	if persistedSig == "" || oldSys == "" {
		return false
	}
	if m.prefixSignature() == persistedSig {
		return false
	}
	if t, ok := m.session.PrefixSnapshotTime(); !ok || time.Since(t) > m.cacheWarmWindow() {
		return false
	}
	ctxWin := proEntry.ContextWindow
	if ctxWin <= 0 {
		ctxWin = 65536
	}
	keepTarget := ctxWin * 20 / 100
	if m.estimateHistoryTokens() < keepTarget*restartCompactKeepFactor {
		return false
	}
	m.pendingCompactModel = oldModel
	m.pendingCompactProvider = oldProvider
	m.pendingCompactSys = oldSys
	m.pendingCompactTools = oldTools
	m.compacting = true
	return true
}

// restartCompactionCmd 返回一个在首请求前执行的缓存友好压缩 Cmd(复刻旧前缀命中热缓存)。
func (m *model) restartCompactionCmd() tea.Cmd {
	snapshot := append([]agent.ChatMessage(nil), m.history...)
	oldSys, oldTools := m.pendingCompactSys, m.pendingCompactTools
	entry := m.entryForModel(m.pendingCompactProvider, m.pendingCompactModel)
	_, proEntry := m.pm.Resolve("pro")
	ctxWin := proEntry.ContextWindow
	if ctxWin <= 0 {
		ctxWin = 65536
	}
	return func() tea.Msg {
		summary, cutIdx, compressedTurns, err := agent.RunCompression(oldSys, oldTools, snapshot, entry, ctxWin)
		return compressionResultMsg{
			summary:         summary,
			cutIdx:          cutIdx,
			compressedTurns: compressedTurns,
			err:             err,
		}
	}
}
