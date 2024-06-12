package price

import "sync"

type Tracker struct {
	mu     sync.Mutex
	prices map[string]float64
}

func NewPriceTracker() *Tracker {
	return &Tracker{
		prices: make(map[string]float64),
	}
}

func (pt *Tracker) UpdatePrice(symbol string, newPrice float64) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	oldPrice, exists := pt.prices[symbol]
	if !exists || oldPrice != newPrice {
		pt.prices[symbol] = newPrice
		return true
	}
	return false
}
