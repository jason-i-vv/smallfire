package repository

import (
	"sync"
)

// TickerRepoMemory 内存行情数据访问实现（简单实现）
type TickerRepoMemory struct {
	prices     map[int64]float64
	prevPrices map[int64]float64
	mu         sync.RWMutex
}

// NewTickerRepoMemory 创建内存行情数据访问实例
func NewTickerRepoMemory() TickerRepo {
	return &TickerRepoMemory{
		prices:     make(map[int64]float64),
		prevPrices: make(map[int64]float64),
	}
}

func (r *TickerRepoMemory) GetPrice(symbolID int64) float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prices[symbolID]
}

func (r *TickerRepoMemory) GetPrevPrice(symbolID int64) float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prevPrices[symbolID]
}

// UpdatePrice 更新价格
func (r *TickerRepoMemory) UpdatePrice(symbolID int64, price float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 保存当前价格为前一个价格
	if current, ok := r.prices[symbolID]; ok {
		r.prevPrices[symbolID] = current
	}
	r.prices[symbolID] = price
}
