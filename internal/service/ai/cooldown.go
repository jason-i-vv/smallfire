package ai

import (
	"fmt"
	"sync"
	"time"
)

// CooldownTracker 冷却期和每日限额追踪器
type CooldownTracker struct {
	mu          sync.RWMutex
	lastCall    map[int]time.Time // symbol_id -> 上次调用时间
	dailyCount  int               // 当日已调用次数
	dailyReset  time.Time         // 当日计数器重置时间
	maxDaily    int               // 每日最大调用次数
	cooldownMin int               // 冷却时间（分钟）
}

// NewCooldownTracker 创建冷却期追踪器
func NewCooldownTracker(maxDaily int, cooldownMinutes int) *CooldownTracker {
	now := time.Now()
	return &CooldownTracker{
		lastCall:    make(map[int]time.Time),
		dailyReset:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()),
		maxDaily:    maxDaily,
		cooldownMin: cooldownMinutes,
	}
}

// CanAnalyze 检查是否可以分析
func (t *CooldownTracker) CanAnalyze(symbolID int) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	// 检查是否需要重置每日计数器
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if today.After(t.dailyReset) {
		t.dailyCount = 0
		t.dailyReset = today
	}

	// 检查每日限额
	if t.dailyCount >= t.maxDaily {
		return false, fmt.Sprintf("已达到每日最大调用次数 (%d/%d)", t.dailyCount, t.maxDaily)
	}

	// 检查冷却期
	if lastTime, exists := t.lastCall[symbolID]; exists {
		elapsed := now.Sub(lastTime)
		cooldown := time.Duration(t.cooldownMin) * time.Minute
		if elapsed < cooldown {
			remaining := cooldown - elapsed
			return false, fmt.Sprintf("该标的冷却中，还需等待 %.0f 分钟", remaining.Minutes())
		}
	}

	return true, ""
}

// Record 记录一次调用
func (t *CooldownTracker) Record(symbolID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastCall[symbolID] = time.Now()
	t.dailyCount++
}
