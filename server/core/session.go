package core

import (
	"crypto/ecdh"
	"sync"
)

// Session state shared with the API layer.
type Beacon struct {
	SessionID  string `json:"session_id"`
	Hostname   string `json:"hostname"`
	Username   string `json:"username"`
	InternalIP string `json:"internal_ip"`
	ExternalIP string `json:"external_ip"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	PID        int    `json:"pid"`
	Process    string `json:"process"`
	SleepSec   int    `json:"sleep_sec"`
	Jitter     int    `json:"jitter"`
	Version    string `json:"agent_version"`
	Token      string `json:"token,omitempty"`
}

// TaskRequest is a unit of work handed to an agent.
type TaskRequest struct {
	ID      int64  `json:"id"`
	Command string `json:"command"`
	Args    string `json:"args"`
}

// TaskResult is the answer of an agent.
type TaskResult struct {
	TaskID     int64  `json:"task_id"`
	Output     string `json:"output"`
	Error      string `json:"error"`
	Success    bool   `json:"success"`
	ChunkIndex int    `json:"chunk_index"`
	ChunkTotal int    `json:"chunk_total"`
	LootType   string `json:"loot_type,omitempty"`
	LootName   string `json:"loot_name,omitempty"`
	LootData   string `json:"loot_data,omitempty"`
}

// sessionKeys holds the transport material of one agent session.
type sessionKeys struct {
	priv      *ecdh.PrivateKey
	aesKey    []byte
	prevKey   []byte
	epoch     int
	checkins  int
	rotatedAt int64
}

// SessionManager keeps the per session transport keys in memory. The keys are
// never written to disk: a restart invalidates every agent session, which is
// the safer default for a lab tool.
type SessionManager struct {
	mu   sync.RWMutex
	keys map[string]*sessionKeys
}

func NewSessionManager() *SessionManager {
	return &SessionManager{keys: make(map[string]*sessionKeys)}
}

// StoreKeyPair installs the freshly negotiated key material for a session.
func (sm *SessionManager) StoreKeyPair(sessionID string, priv *ecdh.PrivateKey, aesKey []byte, epoch int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.keys[sessionID] = &sessionKeys{priv: priv, aesKey: aesKey, epoch: epoch}
}

// GetPrivateKey returns the server side ECDH key of a session.
func (sm *SessionManager) GetPrivateKey(sessionID string) *ecdh.PrivateKey {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if k, ok := sm.keys[sessionID]; ok {
		return k.priv
	}
	return nil
}

// GetAESKey returns the current and the previous session key. The previous key
// stays valid for a short window so an in flight message encrypted with the
// old key is still accepted right after a rotation.
func (sm *SessionManager) GetAESKey(sessionID string) (current, previous []byte, epoch int) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	k, ok := sm.keys[sessionID]
	if !ok {
		return nil, nil, 0
	}
	return k.aesKey, k.prevKey, k.epoch
}

// CountCheckin increments and returns the beacon counter of a session.
func (sm *SessionManager) CountCheckin(sessionID string) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	k, ok := sm.keys[sessionID]
	if !ok {
		return 0
	}
	k.checkins++
	return k.checkins
}

// Rotate replaces the current key and demotes the previous one.
func (sm *SessionManager) Rotate(sessionID string, priv *ecdh.PrivateKey, aesKey []byte) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	k, ok := sm.keys[sessionID]
	if !ok {
		k = &sessionKeys{}
		sm.keys[sessionID] = k
	}
	k.prevKey = k.aesKey
	k.aesKey = aesKey
	k.priv = priv
	k.epoch++
	return k.epoch
}

// Epoch returns the current key epoch of a session.
func (sm *SessionManager) Epoch(sessionID string) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if k, ok := sm.keys[sessionID]; ok {
		return k.epoch
	}
	return 0
}

// RemoveSession drops every trace of a session from memory.
func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.keys, sessionID)
}

// ActiveCount reports how many sessions currently hold key material.
func (sm *SessionManager) ActiveCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.keys)
}
