package api

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"c2-dect/server/auth"
	"c2-dect/server/core"
	"c2-dect/server/db"
	"c2-dect/server/marketplace"
	"c2-dect/server/profiles"

	"github.com/gin-gonic/gin"
)

// Version is the API version served by this build.
const Version = "2.0.0"

// Server wires the database, the token manager, the session material and the
// marketplace catalog together.
type Server struct {
	db        *db.Database
	jwt       *auth.JWTManager
	sessions  *core.SessionManager
	profiles  *profiles.ProfileManager
	market    *marketplace.Catalog
	startedAt time.Time

	limiter *rateLimiter
}

// NewServer builds the API server.
func NewServer(database *db.Database, jwtMgr *auth.JWTManager, profMgr *profiles.ProfileManager, market *marketplace.Catalog) *Server {
	return &Server{
		db:        database,
		jwt:       jwtMgr,
		sessions:  core.NewSessionManager(),
		profiles:  profMgr,
		market:    market,
		startedAt: time.Now(),
		limiter:   newRateLimiter(),
	}
}

// SetupRouter registers every route of the console.
func (s *Server) SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), s.securityHeaders(), s.corsMiddleware(), s.accessLog())

	r.GET("/api/health", s.handleHealth)

	// --- public auth ---
	r.POST("/api/auth/login", s.limiter.limit(3, 10), s.handleLogin)
	r.POST("/api/auth/refresh", s.handleRefresh)
	r.POST("/api/auth/logout", s.handleLogout)
	r.POST("/api/auth/register", s.limiter.limit(2, 5), s.handleRegister)

	// --- agent plane (authenticated with the session key, not with a JWT) ---
	agent := r.Group("/agent")
	agent.Use(s.limiter.limit(60, 200))
	{
		agent.POST("/checkin", s.handleAgentCheckin)
		agent.GET("/checkin", s.handleAgentCheckin)
		agent.GET("/task/pending", s.handleAgentGetTask)
		agent.POST("/task/pending", s.handleAgentGetTask)
		agent.POST("/task/result", s.handleAgentResult)
		agent.GET("/task/result", s.handleAgentResult)
		agent.POST("/key/exchange", s.handleAgentKeyExchange)
	}

	// --- operator plane ---
	api := r.Group("/api")
	api.Use(s.authMiddleware())
	{
		api.GET("/me", s.handleMe)

		api.GET("/sessions", s.handleListSessions)
		api.GET("/sessions/:id", s.handleGetSession)
		api.PUT("/sessions/:id/tag", s.handleTagSession)
		api.DELETE("/sessions/:id", s.handleKillSession)
		api.PUT("/sessions/:id/sleep", s.handleSetSleep)

		api.POST("/sessions/:id/task", s.requireRole(db.RoleOperator, db.RoleAdmin), s.handleSubmitTask)
		api.GET("/sessions/:id/tasks", s.handleGetTasks)
		api.GET("/sessions/:id/tasks/:taskId", s.handleGetTask)

		api.POST("/sessions/:id/upload", s.requireRole(db.RoleOperator, db.RoleAdmin), s.handleUpload)
		api.GET("/sessions/:id/download/:name", s.handleDownload)

		api.GET("/loot", s.handleListLoot)
		api.GET("/stats", s.handleStats)

		api.GET("/profiles", s.handleListProfiles)
		api.POST("/profiles", s.requireRole(db.RoleOperator, db.RoleAdmin), s.handleCreateProfile)

		api.DELETE("/sessions", s.requireRole(db.RoleAdmin), s.handleDeleteAllSessions)
		api.POST("/sessions/cleanup", s.requireRole(db.RoleAdmin), s.handleCleanupStaleSessions)

		// governance
		api.GET("/audit", s.handleListAudit)
		api.GET("/audit/verify", s.requireRole(db.RoleAdmin), s.handleVerifyAudit)
		api.GET("/modules", s.handleModulesCatalog)
		api.GET("/commands", s.handleCommands)
		api.GET("/operators", s.handleListOperators)
		api.POST("/operators", s.requireRole(db.RoleAdmin), s.handleCreateOperator)

		// multi tenancy (administrators only)
		api.GET("/tenants", s.requireRole(db.RoleAdmin), s.handleListTenants)
		api.POST("/tenants", s.requireRole(db.RoleAdmin), s.handleCreateTenant)
		api.DELETE("/tenants/:id", s.requireRole(db.RoleAdmin), s.handleDeleteTenant)
		api.GET("/tenants/:id/tokens", s.requireRole(db.RoleAdmin), s.handleListAgentTokens)
		api.POST("/tenants/:id/tokens", s.requireRole(db.RoleAdmin), s.handleCreateAgentToken)
		api.DELETE("/tenants/:id/tokens/:tokenId", s.requireRole(db.RoleAdmin), s.handleRevokeAgentToken)

		// marketplace
		api.GET("/marketplace/catalog", s.handleMarketplaceCatalog)
		api.GET("/marketplace/pubkey", s.handleMarketplacePubkey)
		api.GET("/marketplace/installs", s.handleMarketplaceInstalls)
		api.GET("/marketplace/recipes", s.handleMarketplaceRecipes)
		api.POST("/marketplace/packages/:id/install", s.requireRole(db.RoleOperator, db.RoleAdmin), s.handleInstallPackage)
		api.DELETE("/marketplace/packages/:id/install", s.requireRole(db.RoleOperator, db.RoleAdmin), s.handleUninstallPackage)

		// marketplace visto desde la vista "Script Market" de la consola
		api.GET("/scripts/list", s.handleScriptList)
		api.GET("/scripts/get", s.handleScriptGet)
		api.GET("/scripts/pubkey", s.handleScriptPubkey)
		api.GET("/scripts/spec", s.handleScriptSpec)
		api.POST("/scripts/upload", s.requireRole(db.RoleOperator, db.RoleAdmin), s.handleScriptUpload)
		api.POST("/scripts/:id/run", s.requireRole(db.RoleOperator, db.RoleAdmin), s.handleScriptRun)
	}

	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
	return r
}

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":   "ok",
		"version":  Version,
		"uptime_s": int(time.Since(s.startedAt).Seconds()),
	})
}

// --- middleware ---

func (s *Server) securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		c.Next()
	}
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	allowed := strings.TrimSpace(envOr("C2_CORS_ORIGINS", ""))
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if allowed == "" {
				// The WPF console talks to the API without a browser origin.
				h := c.Writer.Header()
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Agent-Token")
				h.Set("Access-Control-Max-Age", "600")
			} else {
				for _, o := range strings.Split(allowed, ",") {
					if strings.TrimSpace(o) == origin {
						h := c.Writer.Header()
						h.Set("Access-Control-Allow-Origin", origin)
						h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
						h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Agent-Token")
						h.Set("Access-Control-Allow-Credentials", "true")
						break
					}
				}
			}
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// accessLog writes one structured line per request.
func (s *Server) accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		tenant, _ := c.Get("tenant_id")
		operator, _ := c.Get("username")
		c.Writer.Header().Set("X-Request-Duration-Ms", strconv.FormatInt(time.Since(start).Milliseconds(), 10))
		_ = tenant
		_ = operator
	}
}

// authMiddleware validates the bearer token and publishes the caller context.
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if token == "" || token == header {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
			return
		}
		claims, err := s.jwt.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set("operator_id", claims.OperatorID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("tenant_id", claims.TenantID)
		c.Set("jti", claims.ID)
		c.Next()
	}
}

// requireRole rejects callers whose role is not in the allowed list.
func (s *Server) requireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		for _, r := range roles {
			if role == r {
				c.Next()
				return
			}
		}
		s.audit(c, "authz", "access_denied", "warning",
			map[string]any{"path": c.FullPath(), "role": role})
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
	}
}

// --- helpers ---

func (s *Server) callerTenant(c *gin.Context) int64 {
	v, ok := c.Get("tenant_id")
	if !ok {
		return db.DefaultTenantID
	}
	id, _ := v.(int64)
	if id <= 0 {
		return db.DefaultTenantID
	}
	return id
}

func (s *Server) callerRole(c *gin.Context) string {
	v, _ := c.Get("role")
	role, _ := v.(string)
	return role
}

func (s *Server) callerID(c *gin.Context) int64 {
	v, _ := c.Get("operator_id")
	id, _ := v.(int64)
	return id
}

func (s *Server) callerName(c *gin.Context) string {
	v, _ := c.Get("username")
	name, _ := v.(string)
	return name
}

func (s *Server) isAdmin(c *gin.Context) bool {
	return s.callerRole(c) == db.RoleAdmin
}

// scopeTenant returns the tenant a request is allowed to operate on.
// Administrators may address any existing tenant through ?tenant_id=,
// everybody else is pinned to the tenant carried by their token.
func (s *Server) scopeTenant(c *gin.Context) (int64, bool) {
	caller := s.callerTenant(c)
	requested := int64(0)
	if raw := c.Query("tenant_id"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return 0, false
		}
		requested = parsed
	}
	if requested == 0 {
		return caller, true
	}
	if !s.isAdmin(c) {
		s.audit(c, "tenant", "cross_tenant_access_attempt", "warning",
			map[string]any{"requested": requested, "caller_tenant": caller})
		return 0, false
	}
	if _, err := s.db.GetTenant(requested); err != nil {
		return 0, false
	}
	return requested, true
}

// requireSessionAccess loads a session and enforces the tenant boundary.
// A session of another tenant is reported as "not found" so the API never
// confirms the existence of a foreign resource.
func (s *Server) requireSessionAccess(c *gin.Context, sessionID string) (*db.Session, bool) {
	session, err := s.db.GetSession(sessionID)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return nil, false
	}
	if !s.isAdmin(c) && session.TenantID != s.callerTenant(c) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return nil, false
	}
	return session, true
}

// --- rate limiting ---

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{buckets: make(map[string]*bucket)}
}

// limit returns a middleware allowing rate requests per second with a burst.
func (rl *rateLimiter) limit(rate, burst int) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		rl.mu.Lock()
		b, ok := rl.buckets[key]
		if !ok {
			b = &bucket{tokens: float64(burst), lastSeen: time.Now()}
			rl.buckets[key] = b
		}
		now := time.Now()
		b.tokens += now.Sub(b.lastSeen).Seconds() * float64(rate)
		if b.tokens > float64(burst) {
			b.tokens = float64(burst)
		}
		b.lastSeen = now
		allowed := b.tokens >= 1
		if allowed {
			b.tokens--
		}
		rl.mu.Unlock()

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}
