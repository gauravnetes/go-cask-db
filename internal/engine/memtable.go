package engine 

import (
	"sync"
)

type MemTable struct {
	data map[string]Entry 
	mu sync.RWMutex 
	size int 
}

func NewMemTable() *MemTable {
	return &MemTable {
		data: make(map[string]Entry), 
	}
}

func (m *MemTable) Put(entry Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[entry.Key] = entry
}

func (m *MemTable) Get(key string) (Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.data[key]
	return entry, exists
}