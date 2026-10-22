package api

import (
	"net/http"
	"strconv"

	"c2-dect/server/db"

	"github.com/gin-gonic/gin"
)

// handleListAudit returns the audit trail of the tenant in scope.
func (s *Server) handleListAudit(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	entries, err := s.db.ListAudit(tenantID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []db.AuditEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"audit": entries, "tenant_id": tenantID})
}

// handleVerifyAudit recomputes the hash chain of the audit log.
func (s *Server) handleVerifyAudit(c *gin.Context) {
	ok, message, err := s.db.VerifyAuditChain()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	status := "ok"
	if !ok {
		status = "broken"
	}
	c.JSON(http.StatusOK, gin.H{"valid": ok, "status": status, "message": message})
}
