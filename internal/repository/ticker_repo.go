package repository

import (
	"sync"
)

// TickerRepo 行情价格缓存
type TickerRepo interface {
	// SetPrice 设置当前价格
	SetPrice(symbolID int64, price float64)
	// GetPrice 获取当前价格
	GetPrice(symbolID int64) float64
	// SetPrevPrice 设置前一个价格
	SetPrevPrice(symbolID int64, price float64)
	// GetPrevPrice 获取前一个价格
	GetPrevPrice(symbolID int64) float64
}

// MemoryTickerRepo 内存行情缓存
type MemoryTickerRepo struct {
	mu        sync.RWMutex
	prices    map[int64]float64
	prevPrice map[int64]float64
}

func NewMemoryTickerRepo() *MemoryTickerRepo {
	return &MemoryTickerRepo{
		prices:    make(map[int64]float64),
		prevPrice: make(map[int64]float64),
	}
}

func (r *MemoryTickerRepo) SetPrice(symbolID int64, price float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prevPrice[symbolID] = r.prices[symbolID]
	r.prices[symbolID] = price
}

func (r *MemoryTickerRepo) GetPrice(symbolID int64) float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prices[symbolID]
}

func (r *MemoryTickerRepo) SetPrevPrice(symbolID int64, price float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prevPrice[symbolID] = price
}

func (r *MemoryTickerRepo) GetPrevPrice(symbolID int64) float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prevPrice[symbolID]
}
