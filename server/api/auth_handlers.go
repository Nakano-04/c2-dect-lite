package api

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"c2-dect/server/auth"
	"c2-dect/server/db"

	"github.com/gin-gonic/gin"
)

const (
	loginMaxAttempts = 5
	loginLockWindow  = 15 * time.Minute
)

// handleLogin authenticates an operator and returns an access plus a refresh token.
func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if locked := s.db.LockedUntil(req.Username); locked != "" {
		if ts, err := time.Parse(time.RFC3339, locked); err == nil && time.Now().UTC().Before(ts) {
			s.auditAnonymous(c, "auth", "login_locked", "warning", req.Username)
			c.JSON(http.StatusLocked, gin.H{"error": "account temporarily locked"})
			return
		}
	}

	op, err := s.db.AuthenticateOperator(req.Username, req.Password)
	if err != nil {
		s.db.RecordLoginFailure(req.Username, loginMaxAttempts, loginLockWindow)
		s.auditAnonymous(c, "auth", "login_failed", "warning", req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	s.db.ClearLoginFailures(req.Username)

	access, err := s.jwt.GenerateToken(op.ID, op.Username, op.Role, op.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}
	refreshJTI := auth.NewRefreshID()
	if err := s.db.StoreRefreshToken(refreshJTI, op.ID, op.TenantID, time.Now().Add(s.jwt.RefreshTTL())); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	tenant, _ := s.db.GetTenant(op.TenantID)
	s.auditEntry(c, db.DefaultTenantID, op.ID, op.Username, "auth", "login", "info", "")

	c.JSON(http.StatusOK, gin.H{
		"token":         access,
		"access_token":  access,
		"refresh_token": refreshJTI,
		"expires_in":    int(s.jwt.AccessTTL().Seconds()),
		"operator":      op,
		"tenant":        tenant,
	})
}

// handleRefresh exchanges a refresh token for a new access token.
func (s *Server) handleRefresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	rt, err := s.db.GetRefreshToken(req.RefreshToken)
	if err != nil || rt.Revoked || time.Now().UTC().After(rt.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	op, err := s.db.GetOperatorByID(rt.OperatorID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid refresh token"})
		return
	}
	access, err := s.jwt.GenerateToken(op.ID, op.Username, op.Role, op.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}
	// rotate the refresh token: the presented one is burned
	_ = s.db.RevokeRefreshToken(rt.JTI)
	newJTI := auth.NewRefreshID()
	if err := s.db.StoreRefreshToken(newJTI, op.ID, op.TenantID, time.Now().Add(s.jwt.RefreshTTL())); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"refresh_token": newJTI,
		"expires_in":    int(s.jwt.AccessTTL().Seconds()),
	})
}

// handleLogout revokes the refresh token of the caller.
func (s *Server) handleLogout(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.RefreshToken != "" {
		_ = s.db.RevokeRefreshToken(req.RefreshToken)
	}
	_ = s.db.RevokeOperatorRefresh(s.callerID(c))
	s.auditEntry(c, s.callerTenant(c), s.callerID(c), s.callerName(c), "auth", "logout", "info", "")
	c.JSON(http.StatusOK, gin.H{"status": "logged out"})
}

// handleRegister creates an operator. Registration is disabled by default and
// can only be re-enabled explicitly through the environment.
func (s *Server) handleRegister(c *gin.Context) {
	if !strings.EqualFold(envOr("C2_ALLOW_REGISTER", "false"), "true") {
		c.JSON(http.StatusForbidden, gin.H{"error": "registration is disabled"})
		return
	}
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role"`
		TenantID int64  `json:"tenant_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if len(req.Password) < 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 12 characters"})
		return
	}
	switch req.Role {
	case db.RoleAdmin, db.RoleOperator, db.RoleViewer:
	case "":
		req.Role = db.RoleOperator
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown role"})
		return
	}
	if req.TenantID <= 0 {
		req.TenantID = db.DefaultTenantID
	}
	if _, err := s.db.GetTenant(req.TenantID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown tenant"})
		return
	}
	if err := s.db.CreateOperator(req.Username, req.Password, req.Role, req.TenantID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "operator already exists"})
		return
	}
	s.auditEntry(c, req.TenantID, 0, req.Username, "user", "operator_created", "info", req.Username)
	c.JSON(http.StatusCreated, gin.H{"status": "operator created"})
}

// handleMe returns the profile of the caller.
func (s *Server) handleMe(c *gin.Context) {
	op, err := s.db.GetOperatorByID(s.callerID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "operator not found"})
		return
	}
	tenant, _ := s.db.GetTenant(op.TenantID)
	c.JSON(http.StatusOK, gin.H{
		"operator":    op,
		"tenant":      tenant,
		"permissions": permissionsFor(op.Role),
		"version":     Version,
	})
}

// handleCreateOperator creates an operator inside a tenant. Administrators only.
func (s *Server) handleCreateOperator(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role"`
		TenantID int64  `json:"tenant_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if len(req.Password) < 12 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 12 characters"})
		return
	}
	switch req.Role {
	case db.RoleAdmin, db.RoleOperator, db.RoleViewer:
	case "":
		req.Role = db.RoleOperator
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown role"})
		return
	}
	tenantID := req.TenantID
	if tenantID <= 0 {
		tenantID = s.callerTenant(c)
	}
	if _, err := s.db.GetTenant(tenantID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown tenant"})
		return
	}
	if err := s.db.CreateOperator(req.Username, req.Password, req.Role, tenantID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "the operator already exists"})
		return
	}
	s.audit(c, "user", "operator_created", "info",
		map[string]any{"username": req.Username, "role": req.Role, "tenant_id": tenantID})
	c.JSON(http.StatusCreated, gin.H{"status": "operator created"})
}

// handleListOperators lists the operators of the caller tenant.
func (s *Server) handleListOperators(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	ops, err := s.db.ListOperators(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"operators": ops})
}

// permissionsFor maps a role to the permission list shown by the console.
func permissionsFor(role string) []string {
	switch role {
	case db.RoleAdmin:
		return []string{
			"session:read", "session:task", "session:manage", "loot:read",
			"profile:read", "profile:write", "marketplace:read", "marketplace:install",
			"audit:read", "tenant:read", "tenant:write", "operator:read",
		}
	case db.RoleOperator:
		return []string{
			"session:read", "session:task", "loot:read", "profile:read",
			"marketplace:read", "marketplace:install", "audit:read",
		}
	case db.RoleViewer:
		return []string{"session:read", "loot:read", "profile:read", "marketplace:read"}
	default:
		return []string{}
	}
}

// helpers ---

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func newUUID() string {
	return uuidNew()
}

var errNotFound = errors.New("not found")
