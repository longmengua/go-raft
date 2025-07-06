package maps

import "sync"

// SafeFloatMap 封裝每個 currency 的 uid->balance map，帶鎖保證寫入安全
type SafeFloatMap struct {
	mu   sync.RWMutex
	data map[string]float64
}

func NewSafeFloatMap() *SafeFloatMap {
	return &SafeFloatMap{
		data: make(map[string]float64),
	}
}

func (s *SafeFloatMap) Get(uid string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[uid]
}

func (s *SafeFloatMap) Add(uid string, amount float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[uid] += amount
}

func (s *SafeFloatMap) Snapshot() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]float64, len(s.data))
	for k, v := range s.data {
		cp[k] = v
	}
	return cp
}

func (s *SafeFloatMap) LoadData(newData map[string]float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = newData
}
