package api

import (
	"net/http"
	"strconv"

	"c2-dect/server/db"
	"c2-dect/server/profiles"

	"github.com/gin-gonic/gin"
)

// handleListSessions lists the sessions of the tenant in scope.
func (s *Server) handleListSessions(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	status := c.Query("status")
	sessions, err := s.db.ListSessions(tenantID, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if sessions == nil {
		sessions = []db.Session{}
	}
	c.JSON(http.StatusOK, gin.H{"sessions": sessions, "tenant_id": tenantID})
}

// handleGetSession returns one session.
func (s *Server) handleGetSession(c *gin.Context) {
	session, ok := s.requireSessionAccess(c, c.Param("id"))
	if !ok {
		return
	}
	c.JSON(http.StatusOK, session)
}

// handleTagSession attaches operator tags to a session.
func (s *Server) handleTagSession(c *gin.Context) {
	session, ok := s.requireSessionAccess(c, c.Param("id"))
	if !ok {
		return
	}
	var req struct {
		Tags string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if err := s.db.TagSession(session.ID, req.Tags); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "session", "session_tagged", "info", map[string]any{"session_id": session.ID})
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleKillSession asks an agent to terminate.
func (s *Server) handleKillSession(c *gin.Context) {
	sessionID := c.Param("id")
	if _, ok := s.requireSessionAccess(c, sessionID); !ok {
		return
	}
	s.audit(c, "session", "session_kill_requested", "warning", map[string]any{"session_id": sessionID})
	s.enqueueTask(c, sessionID, "exit", "")
}

// handleSetSleep changes the beacon interval of a session.
func (s *Server) handleSetSleep(c *gin.Context) {
	sessionID := c.Param("id")
	session, ok := s.requireSessionAccess(c, sessionID)
	if !ok {
		return
	}
	var req struct {
		SleepSec int `json:"sleep_sec"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.SleepSec < 1 || req.SleepSec > 86400 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sleep_sec must be between 1 and 86400"})
		return
	}
	if err := s.db.UpdateSessionSleep(session.ID, req.SleepSec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "session", "sleep_changed", "info",
		map[string]any{"session_id": session.ID, "sleep_sec": req.SleepSec})
	s.enqueueTask(c, sessionID, "sleep", strconv.Itoa(req.SleepSec))
}

// handleSubmitTask queues a task for an agent.
//
// Las ordenes fuera del catalogo se rechazan. Las de nivel de impacto
// necesitan el consentimiento explicito del operador: el servidor las deja
// pasar solo con el token de confirmacion, y ademas el agente tiene que
// haberse arrancado con -allow-impact.
func (s *Server) handleSubmitTask(c *gin.Context) {
	sessionID := c.Param("id")
	session, ok := s.requireSessionAccess(c, sessionID)
	if !ok {
		return
	}
	var req struct {
		Command string `json:"command" binding:"required"`
		Args    string `json:"args"`
		Confirm string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	spec, known := lookupCommand(req.Command)
	if !known {
		s.audit(c, "task", "command_rejected", "warning",
			map[string]any{"session_id": session.ID, "command": req.Command})
		c.JSON(http.StatusBadRequest, gin.H{"error": "command not allowed"})
		return
	}
	if spec.Risk == RiskImpact && req.Confirm != ImpactConfirmToken {
		s.audit(c, "task", "impact_denied", "warning",
			map[string]any{"session_id": session.ID, "command": spec.Name})
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "this command has impact level " + spec.Risk + " and needs the explicit consent of the operator",
			"command": spec.Name,
			"risk":    spec.Risk,
			"confirm": ImpactConfirmToken,
		})
		return
	}

	// Un script solo viaja si el envelope esta firmado, dentro de la ventana
	// temporal y con el alcance declarado.
	if spec.Name == "script" {
		if err := s.verifyScriptTask(c, req.Args); err != nil {
			reason := err.Error()
			code := 403
			if gate, ok := err.(*scriptGateError); ok {
				reason = gate.Reason
				code = gate.Code
			}
			s.audit(c, "script", "script_rejected", "warning",
				map[string]any{"session_id": session.ID, "reason": reason})
			c.JSON(code, gin.H{"error": reason, "command": "script"})
			return
		}
	}

	s.enqueueTask(c, sessionID, spec.Name, req.Args)
}

// enqueueTask creates a task for a session and answers with the created row.
func (s *Server) enqueueTask(c *gin.Context, sessionID, command, args string) {
	operatorID := s.callerID(c)
	task := &db.Task{
		SessionID:  sessionID,
		Command:    command,
		Args:       args,
		OperatorID: operatorID,
		TenantID:   s.callerTenant(c),
	}
	if err := s.db.CreateTask(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "task", "task_submitted", "info",
		map[string]any{"session_id": sessionID, "command": command})
	// Envelope {task:{...}}: es el contrato que espera la consola.
	c.JSON(http.StatusCreated, gin.H{"task": task})
}

// handleGetTasks lists the task history of a session.
func (s *Server) handleGetTasks(c *gin.Context) {
	sessionID := c.Param("id")
	if _, ok := s.requireSessionAccess(c, sessionID); !ok {
		return
	}
	tasks, err := s.db.GetTasksBySession(s.callerTenant(c), sessionID, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tasks == nil {
		tasks = []db.Task{}
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

// handleGetTask returns one task.
func (s *Server) handleGetTask(c *gin.Context) {
	sessionID := c.Param("id")
	if _, ok := s.requireSessionAccess(c, sessionID); !ok {
		return
	}
	taskID, err := strconv.ParseInt(c.Param("taskId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task id"})
		return
	}
	task, err := s.db.GetTask(s.callerTenant(c), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

// handleUpload stores a file and asks the agent to write it on disk.
func (s *Server) handleUpload(c *gin.Context) {
	sessionID := c.Param("id")
	session, ok := s.requireSessionAccess(c, sessionID)
	if !ok {
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no file provided"})
		return
	}
	defer file.Close()

	remotePath := c.PostForm("remote_path")
	if remotePath == "" {
		remotePath = header.Filename
	}
	data := make([]byte, header.Size)
	if _, err := file.Read(data); err != nil && len(data) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read the uploaded file"})
		return
	}

	loot := &db.Loot{
		SessionID: session.ID,
		Type:      "upload",
		Name:      header.Filename,
		Data:      data,
		Path:      remotePath,
		TenantID:  session.TenantID,
	}
	if err := s.db.SaveLoot(loot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "loot", "file_uploaded", "info",
		map[string]any{"session_id": session.ID, "name": header.Filename})
	c.JSON(http.StatusOK, gin.H{"status": "upload queued", "remote_path": remotePath, "size": len(data)})
}

// handleDownload asks an agent for a file.
func (s *Server) handleDownload(c *gin.Context) {
	sessionID := c.Param("id")
	if _, ok := s.requireSessionAccess(c, sessionID); !ok {
		return
	}
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file name required"})
		return
	}
	s.enqueueTask(c, sessionID, "download", name)
}

// handleListLoot lists the artifacts of the tenant in scope.
func (s *Server) handleListLoot(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	loots, err := s.db.ListLoot(tenantID, c.Query("session_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if loots == nil {
		loots = []db.Loot{}
	}
	c.JSON(http.StatusOK, gin.H{"loot": loots, "tenant_id": tenantID})
}

// handleStats returns the counters of the tenant in scope.
func (s *Server) handleStats(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total_sessions":   s.db.SessionCount(tenantID),
		"active_sessions":  s.db.ActiveSessionCount(tenantID),
		"tenant_id":        tenantID,
		"key_sessions":     s.sessions.ActiveCount(),
		"version":          Version,
		"timestamp":        nowRFC3339(),
	})
}

// handleDeleteAllSessions removes every session of the tenant in scope.
func (s *Server) handleDeleteAllSessions(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	if err := s.db.DeleteAllSessions(tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "session", "sessions_deleted", "warning", map[string]any{"tenant_id": tenantID})
	c.JSON(http.StatusOK, gin.H{"status": "sessions deleted"})
}

// handleCleanupStaleSessions removes the sessions that stopped checking in.
func (s *Server) handleCleanupStaleSessions(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	var req struct {
		MaxAgeMinutes int `json:"max_age_minutes"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.MaxAgeMinutes <= 0 {
		req.MaxAgeMinutes = 30
	}
	deleted, err := s.db.CleanupStaleSessions(tenantID, req.MaxAgeMinutes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "session", "stale_sessions_cleaned", "info",
		map[string]any{"deleted": deleted, "max_age_minutes": req.MaxAgeMinutes})
	c.JSON(http.StatusOK, gin.H{"deleted": deleted, "max_age_minutes": req.MaxAgeMinutes})
}

// handleListProfiles lists the traffic profiles.
func (s *Server) handleListProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"profiles": s.profiles.List()})
}

// handleCreateProfile stores a traffic profile.
func (s *Server) handleCreateProfile(c *gin.Context) {
	var profile profiles.MalleableProfile
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if profile.Name == "" || !validProfileName(profile.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid profile name"})
		return
	}
	if err := s.profiles.Save(&profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "profile", "profile_created", "info", map[string]any{"name": profile.Name})
	c.JSON(http.StatusCreated, gin.H{"status": "created"})
}

// allowedCommand normalises and validates a command name.
func allowedCommand(command string) bool {
	_, ok := lookupCommand(command)
	return ok
}

func validProfileName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}

