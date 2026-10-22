package api

import (
	"net/http"
	"strconv"
	"time"

	"c2-dect/server/db"

	"github.com/gin-gonic/gin"
)

// handleListTenants lists the tenants. Only administrators may look at them.
func (s *Server) handleListTenants(c *gin.Context) {
	tenants, err := s.db.ListTenants()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tenants == nil {
		tenants = []db.Tenant{}
	}
	c.JSON(http.StatusOK, gin.H{"tenants": tenants})
}

// handleCreateTenant creates a tenant.
func (s *Server) handleCreateTenant(c *gin.Context) {
	var req struct {
		Name               string `json:"name" binding:"required"`
		Description        string `json:"description"`
		AllowImpactModules bool   `json:"allow_impact_modules"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if len(req.Name) < 2 || len(req.Name) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the tenant name must be between 2 and 64 characters"})
		return
	}
	tenant, err := s.db.CreateTenant(req.Name, req.Description, req.AllowImpactModules)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "the tenant already exists"})
		return
	}
	s.audit(c, "tenant", "tenant_created", "info", map[string]any{"tenant": tenant.Name})
	c.JSON(http.StatusCreated, tenant)
}

// handleDeleteTenant removes a tenant and everything it owns.
func (s *Server) handleDeleteTenant(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id"})
		return
	}
	if err := s.db.DeleteTenant(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "tenant", "tenant_deleted", "warning", map[string]any{"tenant_id": id})
	c.JSON(http.StatusOK, gin.H{"status": "tenant deleted"})
}

// handleListAgentTokens lists the bootstrap tokens of a tenant.
func (s *Server) handleListAgentTokens(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id"})
		return
	}
	tokens, err := s.db.ListAgentTokens(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tokens == nil {
		tokens = []db.AgentToken{}
	}
	c.JSON(http.StatusOK, gin.H{"tokens": tokens})
}

// handleCreateAgentToken issues a bootstrap token that binds new agents to a
// tenant. The token is returned once, only its hash is stored.
func (s *Server) handleCreateAgentToken(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id"})
		return
	}
	if _, err := s.db.GetTenant(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	var req struct {
		Label     string `json:"label"`
		ExpiresIn int    `json:"expires_in_hours"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.ExpiresIn <= 0 {
		req.ExpiresIn = 24
	}
	if req.ExpiresIn > 720 {
		req.ExpiresIn = 720
	}
	token, err := randomToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate the token"})
		return
	}
	expires := time.Now().UTC().Add(time.Duration(req.ExpiresIn) * time.Hour)
	if err := s.db.CreateAgentToken(req.Label, id, hashToken(token), expires, s.callerID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "tenant", "agent_token_created", "info",
		map[string]any{"tenant_id": id, "label": req.Label, "expires_in_hours": req.ExpiresIn})
	c.JSON(http.StatusCreated, gin.H{
		"token":      token,
		"tenant_id":  id,
		"expires_at": expires.Format(time.RFC3339),
		"hint":       "the token is shown once, only its hash is stored",
	})
}

// handleRevokeAgentToken revokes a bootstrap token.
func (s *Server) handleRevokeAgentToken(c *gin.Context) {
	tenantID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant id"})
		return
	}
	tokenID, err := strconv.ParseInt(c.Param("tokenId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token id"})
		return
	}
	if err := s.db.RevokeAgentToken(tokenID, tenantID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "tenant", "agent_token_revoked", "info",
		map[string]any{"tenant_id": tenantID, "token_id": tokenID})
	c.JSON(http.StatusOK, gin.H{"status": "token revoked"})
}
