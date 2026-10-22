package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"c2-dect/server/db"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	errUnsignedPackage = errors.New("the package is not signed")
	errBadSignature    = errors.New("the package signature does not verify")
)

// nowRFC3339 returns the current UTC time in RFC3339.
func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// uuidNew returns a random identifier.
func uuidNew() string { return uuid.New().String() }

// hashToken returns the hex encoded SHA-256 of a bearer style token. Only the
// hash is stored, so a database dump never reveals a usable token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// randomToken returns a URL safe random token.
func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// audit writes one entry in the tamper evident audit chain.
func (s *Server) audit(c *gin.Context, category, action, severity string, details map[string]any) {
	payload := ""
	if len(details) > 0 {
		if raw, err := json.Marshal(details); err == nil {
			payload = string(raw)
		}
	}
	s.auditEntry(c, s.callerTenant(c), s.callerID(c), s.callerName(c), category, action, severity, payload)
}

// auditEntry writes a fully specified audit entry.
func (s *Server) auditEntry(c *gin.Context, tenantID, operatorID int64, username, category, action, severity, details string) {
	_ = c
	entry := &db.AuditEntry{
		TenantID:   tenantID,
		OperatorID: operatorID,
		Username:   username,
		Category:   category,
		Action:     action,
		Severity:   severity,
		Details:    details,
	}
	_ = s.db.AppendAudit(entry)
}

// auditAnonymous writes an audit entry for an unauthenticated event.
func (s *Server) auditAnonymous(c *gin.Context, category, action, severity, subject string) {
	_ = c
	entry := &db.AuditEntry{
		TenantID: 0,
		Category: category,
		Action:   action,
		Severity: severity,
		Details:  subject,
	}
	_ = s.db.AppendAudit(entry)
}
