package api

import (
	"crypto/sha256"
	"fmt"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"c2-dect/server/db"
	"c2-dect/server/marketplace"

	"github.com/gin-gonic/gin"
)

// maxPackageBytes is the maximum size of a published package.
const maxPackageBytes = 1 << 20

// handleScriptList publishes the marketplace as a script catalog.
//
// The console Lite reuses the "script market" view of the full console, so the
// marketplace answers on the /api/scripts surface. The entries are the signed
// packages of the catalog plus the packages the operator published.
func (s *Server) handleScriptList(c *gin.Context) {
	tenantID, ok := s.scopeTenant(c)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown tenant"})
		return
	}

	names := []string{}
	for _, p := range s.market.Packages() {
		if p.Available {
			names = append(names, p.ID)
		}
	}
	custom, err := s.db.ListCustomPackages(tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, p := range custom {
		names = append(names, p.PackageID)
	}
	c.JSON(http.StatusOK, gin.H{"scripts": names, "count": len(names)})
}

// handleScriptGet returns the manifest of one package.
func (s *Server) handleScriptGet(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}

	// published by the operator
	if row, err := s.db.GetCustomPackage(name); err == nil {
		c.JSON(http.StatusOK, gin.H{
			"name":      row.PackageID,
			"source":    row.Source,
			"scope":     row.Scope,
			"sig":       row.Signature,
			"kid":       row.Kid,
			"script_id": row.ScriptID,
		})
		return
	}

	pkg, found := s.market.Get(name)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "package not found"})
		return
	}
	manifest, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sum := sha256.Sum256(manifest)
	c.JSON(http.StatusOK, gin.H{
		"name":      pkg.ID,
		"source":    string(manifest),
		"scope":     pkg.Tier,
		"sig":       pkg.Signature,
		"kid":       pkg.Kid,
		"script_id": hex.EncodeToString(sum[:]),
	})
}

// handleScriptPubkey publishes the signing key of the marketplace.
func (s *Server) handleScriptPubkey(c *gin.Context) {
	signer := s.market.Signer()
	if signer == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no signing key configured"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"pubkey":     signer.PubHex(),
		"kid":        signer.Kid(),
		"tenant_id":  s.callerTenant(c),
		"tenant_keys": []gin.H{},
		"prev":       []gin.H{},
	})
}

// handleScriptUpload publica un script.
//
// Acepta dos formas:
//   - un script Lua (el formato del modulo comercial): se filtra con el
//     validador de sandbox y tiene que declarar su alcance en la cabecera;
//   - un manifiesto declarativo con pasos: cada paso se valida contra el
//     catalogo de ordenes.
//
// En los dos casos el servidor firma el source con la clave del marketplace
// y lo guarda con su checksum.
func (s *Server) handleScriptUpload(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		Source string `json:"source"`
		Scope  string `json:"scope"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	if len(req.Source) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source required"})
		return
	}
	if len(req.Source) > maxPackageBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the package is too large"})
		return
	}
	if !validPackageID(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid package name"})
		return
	}

	scope := strings.TrimSpace(req.Scope)
	steps := 0
	isManifest := false

	// 1) manifiesto declarativo con pasos
	if manifest, err := parseManifest(req.Source); err == nil {
		isManifest = true
		steps = len(manifest.Steps)
		if scope == "" {
			scope = manifest.Scope
		}
		if scope == "" {
			scope = "lab-only"
		}
	} else {
		// 2) script Lua: sandbox + cabecera de alcance
		if err := validateLuaSource(req.Source); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if scope == "" {
			scope = declaredScope(req.Source)
		}
		if scope == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "scope declaration required (add '-- scope: lab-only' to the header)"})
			return
		}
	}
	if len(scope) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the scope is too long"})
		return
	}

	signer := s.market.Signer()
	if signer == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "no signing key configured"})
		return
	}
	signature := signer.Sign([]byte(req.Source))
	sum := sha256.Sum256([]byte(req.Source))
	scriptID := hex.EncodeToString(sum[:])

	row := &db.MarketplacePackageRow{
		PackageID: name,
		Name:      name,
		Version:   "1.0",
		Source:    req.Source,
		Scope:     scope,
		Signature: signature,
		Kid:       signer.Kid(),
		ScriptID:  scriptID,
		Checksum:  scriptID,
		TenantID:  s.callerTenant(c),
	}
	if err := s.db.SaveCustomPackage(row); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.audit(c, "marketplace", "package_published", "info",
		map[string]any{"package": name, "scope": scope, "script_id": scriptID})
	c.JSON(http.StatusCreated, gin.H{
		"name":      name,
		"status":    "published",
		"script_id": scriptID,
		"scope":     scope,
		"steps":     steps,
		"kind":      scriptKind(isManifest),
		"sig":       signature,
		"kid":       signer.Kid(),
	})
}

// scriptKind dice que forma tiene el paquete publicado.
func scriptKind(isManifest bool) string {
	if isManifest {
		return "manifest"
	}
	return "lua"
}

// scriptBlockedPatterns son los patrones que el sandbox del servidor rechaza
// antes de publicar. El sandbox real es la denegacion de globales dentro del
// agente: esto es solo la primera capa.
var scriptBlockedPatterns = []string{
	"os.execute", "loadfile", "dofile", "io.popen", "os.remove", "os.rename",
}

// validateLuaSource aplica el filtro de sandbox del servidor.
func validateLuaSource(source string) error {
	low := strings.ToLower(source)
	for _, blocked := range scriptBlockedPatterns {
		if strings.Contains(low, blocked) {
			return fmt.Errorf("sandbox: '%s' is blocked", blocked)
		}
	}
	return nil
}

// validPackageID accepts names that are safe to use as a file name.
func validPackageID(name string) bool {
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

// declaredScope deduce el alcance de las primeras lineas del manifiesto:
// la consola antepone la cabecera "-- scope: <etiqueta>".
func declaredScope(source string) string {
	lines := strings.Split(source, "\n")
	if len(lines) > 5 {
		lines = lines[:5]
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "--") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "--"))
		if strings.HasPrefix(strings.ToLower(rest), "scope:") {
			label := strings.TrimSpace(rest[len("scope:"):])
			if len(label) > 64 {
				label = label[:64]
			}
			return label
		}
	}
	return ""
}

// stringField reads a string field of a manifest.
func stringField(manifest map[string]any, key string) string {
	if v, ok := manifest[key].(string); ok {
		return v
	}
	return ""
}

// verifyCustomPackage checks the signature of a published package.
func (s *Server) verifyCustomPackage(row *db.MarketplacePackageRow) error {
	signer := s.market.Signer()
	if signer == nil {
		return errUnsignedPackage
	}
	if !marketplace.Verify(signer.PubHex(), []byte(row.Source), row.Signature) {
		return errBadSignature
	}
	return nil
}
