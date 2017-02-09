package health

import "sync"

// atomic map from string to *ServerLoc

type AtomicServerLocMap struct {
	U   map[string]*ServerLoc `json:"U"`
	tex sync.RWMutex
}

func NewAtomicServerLocMap() *AtomicServerLocMap {
	return &AtomicServerLocMap{
		U: make(map[string]*ServerLoc),
	}
}

func (m *AtomicServerLocMap) Len() int {
	m.tex.RLock()
	n := len(m.U)
	m.tex.RUnlock()
	return n
}

func (m *AtomicServerLocMap) Get(key string) *ServerLoc {
	m.tex.RLock()
	s := m.U[key]
	m.tex.RUnlock()
	return s
}

func (m *AtomicServerLocMap) Get2(key string) (*ServerLoc, bool) {
	m.tex.RLock()
	v, ok := m.U[key]
	m.tex.RUnlock()
	return v, ok
}

func (m *AtomicServerLocMap) Set(key string, val *ServerLoc) {
	m.tex.Lock()
	m.U[key] = val
	m.tex.Unlock()
}

func (m *AtomicServerLocMap) Del(key string) {
	m.tex.Lock()
	delete(m.U, key)
	m.tex.Unlock()
}
