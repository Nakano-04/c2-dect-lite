package core

import (
	"c2-dect/server/db"
	"crypto/ecdh"
	"sync"
)

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
}

type TaskRequest struct {
	TaskID  int64  `json:"task_id"`
	Command string `json:"command"`
	Args    string `json:"args"`
}

type TaskResult struct {
	TaskID  int64  `json:"task_id"`
	Output  string `json:"output"`
	Error   string `json:"error"`
	LootType string `json:"loot_type,omitempty"`
	LootName string `json:"loot_name,omitempty"`
	LootData []byte `json:"loot_data,omitempty"`
}

type SessionManager struct {
	db          *db.Database
	privateKeys map[string]*ecdh.PrivateKey
	aesKeys     map[string][]byte
	mu          sync.RWMutex
}

func NewSessionManager(database *db.Database) *SessionManager {
	return &SessionManager{
		db:          database,
		privateKeys: make(map[string]*ecdh.PrivateKey),
		aesKeys:     make(map[string][]byte),
	}
}

func (sm *SessionManager) StorePrivateKey(sessionID string, key *ecdh.PrivateKey) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.privateKeys[sessionID] = key
}

func (sm *SessionManager) GetPrivateKey(sessionID string) *ecdh.PrivateKey {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.privateKeys[sessionID]
}

func (sm *SessionManager) StoreAESKey(sessionID string, key []byte) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.aesKeys[sessionID] = key
}

func (sm *SessionManager) GetAESKey(sessionID string) []byte {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.aesKeys[sessionID]
}

func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.privateKeys, sessionID)
	delete(sm.aesKeys, sessionID)
}
