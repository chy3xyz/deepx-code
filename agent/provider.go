package agent

import (
	"sync"
	"time"

	"deepx/config"
)

// QuotaTracker 内存态配额追踪,线程安全(agent goroutine + TUI goroutine 各读写)。
type QuotaTracker struct {
	mu    sync.Mutex
	limit int64
	used  int64
	reset time.Time
}

// NewQuotaTracker 从配置构造。reset 为空或解析失败 → 零值,不自动清零。
func NewQuotaTracker(q *config.QuotaConfig) *QuotaTracker {
	qt := &QuotaTracker{}
	if q != nil {
		qt.limit = q.Limit
		qt.used = q.Used
		if q.Reset != "" {
			if t, err := time.Parse("2006-01-02", q.Reset); err == nil {
				qt.reset = t
			}
		}
	}
	return qt
}

// Available 返回剩余 token 配额。limit=0 表示未设配额(unlimited)。
func (qt *QuotaTracker) Available() int64 {
	if qt == nil || qt.limit <= 0 {
		return 1<<63 - 1 // effectively unlimited
	}
	qt.mu.Lock()
	defer qt.mu.Unlock()
	// 过了 reset 日期 → 自动清零
	if !qt.reset.IsZero() && time.Now().After(qt.reset) {
		qt.used = 0
	}
	if qt.used >= qt.limit {
		return 0
	}
	return qt.limit - qt.used
}

// Reserve 预扣 tokens(估计值)。成功返回 true,配额不足返回 false。
func (qt *QuotaTracker) Reserve(tokens int64) bool {
	if qt == nil || qt.limit <= 0 {
		return true
	}
	qt.mu.Lock()
	defer qt.mu.Unlock()
	if !qt.reset.IsZero() && time.Now().After(qt.reset) {
		qt.used = 0
	}
	if qt.used+int64(float64(tokens)*1.2) >= qt.limit { // 20% buffer 防估计偏差
		return false
	}
	return true
}

// Add 实际消耗 tokens(API 返回真实值后调用)。
func (qt *QuotaTracker) Add(tokens int64) {
	if qt == nil {
		return
	}
	qt.mu.Lock()
	defer qt.mu.Unlock()
	qt.used += tokens
}

// Used 返回当前已用 token 数。
func (qt *QuotaTracker) Used() int64 {
	if qt == nil {
		return 0
	}
	qt.mu.Lock()
	defer qt.mu.Unlock()
	return qt.used
}

// Snapshot 返回配额快照,供持久化。
func (qt *QuotaTracker) Snapshot() (used int64, limit int64) {
	if qt == nil {
		return 0, 0
	}
	qt.mu.Lock()
	defer qt.mu.Unlock()
	return qt.used, qt.limit
}

// === ProviderManager ===

// ProviderManager 管理多 provider 的优先级、回退和配额。
type ProviderManager struct {
	mu        sync.RWMutex
	providers []*providerSlot
	active    string // 当前活跃 provider name
	cooling   map[string]time.Time
}

type providerSlot struct {
	Name    string
	Config  *config.ProviderEntry
	Quota   *QuotaTracker
}

// NewProviderManager 从 config 构造,按 priority 排序。
func NewProviderManager(cfg *config.Config) *ProviderManager {
	sorted := cfg.SortedProviders()
	slots := make([]*providerSlot, 0, len(sorted))
	for _, s := range sorted {
		if s.Config.Flash == nil && s.Config.Pro == nil {
			continue // 完全空的 entry 跳过
		}
		slots = append(slots, &providerSlot{
			Name:   s.Name,
			Config: s.Config,
			Quota:  NewQuotaTracker(s.Config.Quota),
		})
	}

	active := ""
	if len(slots) > 0 {
		active = slots[0].Name
	}

	return &ProviderManager{
		providers: slots,
		active:    active,
		cooling:   make(map[string]time.Time),
	}
}

// ActiveName 返回当前活跃 provider 名。
func (pm *ProviderManager) ActiveName() string {
	if pm == nil {
		return "default"
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.active
}

// Resolve 返回当前活跃 provider 的指定 role entry。
// role = "flash" / "pro"。当前 provider 缺该 role 时,回退到 flash(若有),
// flash 也无则沿 priority 链找下一个。
func (pm *ProviderManager) Resolve(role string) (providerName string, entry ModelEntry) {
	if pm == nil {
		return "default", ModelEntry{}
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	slot := pm.findSlot(pm.active)
	if slot == nil {
		return pm.active, ModelEntry{}
	}
	e, ok := pm.entryForRole(slot, role)
	if ok {
		return slot.Name, e
	}
	// 当前 provider 缺该 role,沿链找
	for _, s := range pm.providers {
		if s.Name == pm.active {
			continue
		}
		if e, ok := pm.entryForRole(s, role); ok {
			return s.Name, e
		}
	}
	// 绝对兜底:返回任意非空 entry
	for _, s := range pm.providers {
		if s.Config.Flash != nil {
			return s.Name, toModelEntry(s.Config.Flash)
		}
		if s.Config.Pro != nil {
			return s.Name, toModelEntry(s.Config.Pro)
		}
	}
	return pm.active, ModelEntry{}
}

func (pm *ProviderManager) entryForRole(s *providerSlot, role string) (ModelEntry, bool) {
	switch role {
	case "flash":
		if s.Config.Flash != nil && s.Config.Flash.Model != "" {
			return toModelEntry(s.Config.Flash), true
		}
		if s.Config.Pro != nil && s.Config.Pro.Model != "" {
			return toModelEntry(s.Config.Pro), true
		}
	case "pro":
		if s.Config.Pro != nil && s.Config.Pro.Model != "" {
			return toModelEntry(s.Config.Pro), true
		}
		if s.Config.Flash != nil && s.Config.Flash.Model != "" {
			return toModelEntry(s.Config.Flash), true
		}
	}
	return ModelEntry{}, false
}

// Fallback 当前 provider 不可用时(配额耗尽/API 错误),沿 priority 链找下一个。
// 跳过冷却中的 provider(30s 内刚失败过)。返回 false 表示全部 provider 不可用。
func (pm *ProviderManager) Fallback(role string) (providerName string, entry ModelEntry, ok bool) {
	if pm == nil {
		return "default", ModelEntry{}, false
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	for _, s := range pm.providers {
		if s.Name == pm.active {
			continue
		}
		// 冷却检查:刚失败过的不立即重试
		if coolUntil, cooling := pm.cooling[s.Name]; cooling && now.Before(coolUntil) {
			continue
		}
		// 配额检查
		if s.Quota.Available() <= 0 {
			continue
		}
		e, eok := pm.entryForRole(s, role)
		if !eok {
			continue
		}
		pm.active = s.Name
		return s.Name, e, true
	}
	return pm.active, ModelEntry{}, false
}

// MarkFailure 标记某 provider 当前不可用,冷却 30s。
func (pm *ProviderManager) MarkFailure(name string) {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.cooling[name] = time.Now().Add(30 * time.Second)
}

// Reserve 预估 token 配额。成功返回 true。
func (pm *ProviderManager) Reserve(tokens int64) bool {
	if pm == nil {
		return true
	}
	slot := pm.findSlot(pm.active)
	if slot == nil {
		return false
	}
	return slot.Quota.Reserve(tokens)
}

// RecordUsage 记录实际 token 消耗。
func (pm *ProviderManager) RecordUsage(providerName string, prompt, completion, cacheHit, cacheMiss int) {
	if pm == nil {
		return
	}
	pm.mu.RLock()
	slot := pm.findSlot(providerName)
	pm.mu.RUnlock()
	if slot != nil {
		slot.Quota.Add(int64(prompt + completion))
	}
}

// CacheTTL 返回当前活跃 provider 的缓存 TTL。未设返回 0(调用方用默认值)。
func (pm *ProviderManager) CacheTTL() time.Duration {
	if pm == nil {
		return 0
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	slot := pm.findSlot(pm.active)
	if slot == nil {
		return 0
	}
	return slot.Config.CacheTTLDuration()
}

// Pricing 返回当前活跃 provider 的 Pricing 配置。
func (pm *ProviderManager) Pricing() *config.PricingConfig {
	if pm == nil {
		return nil
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	slot := pm.findSlot(pm.active)
	if slot == nil {
		return nil
	}
	return slot.Config.Pricing
}

// CompactionThreshold 返回当前 provider 的压缩触发百分比(0-100)。
// MiMo 用 60(更积极),默认 70。
func (pm *ProviderManager) CompactionThreshold() int {
	if pm == nil {
		return 70
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	active := pm.active
	if stringsContains(active, "mimo") || stringsContains(active, "xiaomi") {
		return 60
	}
	return 70
}

// QuotaSnapshot 返回所有 provider 的配额快照,供持久化到 state.json。
func (pm *ProviderManager) QuotaSnapshot() map[string]struct {
	Used  int64
	Limit int64
} {
	if pm == nil {
		return nil
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	out := make(map[string]struct {
		Used  int64
		Limit int64
	}, len(pm.providers))
	for _, s := range pm.providers {
		u, l := s.Quota.Snapshot()
		out[s.Name] = struct {
			Used  int64
			Limit int64
		}{Used: u, Limit: l}
	}
	return out
}

// RestoreQuota 从持久化快照恢复已用配额。
func (pm *ProviderManager) RestoreQuota(snapshot map[string]struct {
	Used  int64
	Limit int64
}) {
	if pm == nil {
		return
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, s := range pm.providers {
		if snap, ok := snapshot[s.Name]; ok {
			s.Quota.mu.Lock()
			s.Quota.used = snap.Used
			if snap.Limit > 0 {
				s.Quota.limit = snap.Limit
			}
			s.Quota.mu.Unlock()
		}
	}
}

// EntryForModel 按 provider + model ID 取对应的 ModelEntry。
// 缓存按模型分,压缩必须用缓存那段历史的同一模型才命中。
func (pm *ProviderManager) EntryForModel(providerName, modelID string) ModelEntry {
	if pm == nil {
		return ModelEntry{}
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	slot := pm.findSlot(providerName)
	if slot == nil {
		return ModelEntry{}
	}
	if slot.Config.Flash != nil && slot.Config.Flash.Model == modelID {
		return toModelEntry(slot.Config.Flash)
	}
	if slot.Config.Pro != nil && slot.Config.Pro.Model == modelID {
		return toModelEntry(slot.Config.Pro)
	}
	// 退:返回任意非空
	if slot.Config.Flash != nil {
		return toModelEntry(slot.Config.Flash)
	}
	if slot.Config.Pro != nil {
		return toModelEntry(slot.Config.Pro)
	}
	return ModelEntry{}
}

func (pm *ProviderManager) findSlot(name string) *providerSlot {
	for _, s := range pm.providers {
		if s.Name == name {
			return s
		}
	}
	if len(pm.providers) > 0 {
		return pm.providers[0]
	}
	return nil
}

// toModelEntry 将 config.ModelEntry 转成 agent.ModelEntry(同结构,不同包)。
func toModelEntry(e *config.ModelEntry) ModelEntry {
	if e == nil {
		return ModelEntry{}
	}
	return ModelEntry{
		BaseURL:       e.BaseURL,
		Model:         e.Model,
		APIKey:        e.APIKey,
		ContextWindow: e.ContextWindow,
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ProviderList 返回所有 provider 名(供 TUI 仪表盘显示)。
func (pm *ProviderManager) ProviderList() []string {
	if pm == nil {
		return nil
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	names := make([]string, len(pm.providers))
	for i, s := range pm.providers {
		names[i] = s.Name
	}
	return names
}
