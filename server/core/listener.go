package core

import (
	"net"
	"sync"
)

type Listener struct {
	ID       string
	Addr     string
	Protocol string // tcp, http, https
	Profile  string
	Conn     net.Listener
	mu       sync.Mutex
	running  bool
}

type ListenerManager struct {
	listeners map[string]*Listener
	mu        sync.RWMutex
}

func NewListenerManager() *ListenerManager {
	return &ListenerManager{
		listeners: make(map[string]*Listener),
	}
}

func (lm *ListenerManager) AddListener(l *Listener) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.listeners[l.ID] = l
	return nil
}

func (lm *ListenerManager) RemoveListener(id string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if l, ok := lm.listeners[id]; ok {
		l.mu.Lock()
		if l.Conn != nil {
			l.Conn.Close()
		}
		l.running = false
		l.mu.Unlock()
		delete(lm.listeners, id)
	}
}

func (lm *ListenerManager) GetListener(id string) *Listener {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.listeners[id]
}

func (lm *ListenerManager) ListListeners() []*Listener {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	var result []*Listener
	for _, l := range lm.listeners {
		result = append(result, l)
	}
	return result
}
