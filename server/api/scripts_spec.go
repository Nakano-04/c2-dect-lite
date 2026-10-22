package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"c2-dect-lite/scripttrust"
	"c2-dect/server/db"

	"github.com/gin-gonic/gin"
)

// Lethal Script Lite: guion declarativo para la comunidad.
//
// Un guion NO es codigo: es una lista de pasos, y cada paso es una orden del
// catalogo del agente. El servidor valida cada paso contra el catalogo antes
// de aceptarlo y lo expande en tareas normales al ejecutarlo. El implante no
// sabe que existen los guiones: no recibe nada nuevo, solo las tareas de
// siempre. Esa es la capacidad minima: el guion no puede hacer nada que el
// operador no pueda hacer a mano, y el server es quien aplica el gate de
// impacto a cada paso.
//
// Formato del manifiesto:
//
//	{
//	  "id": "win-enum",
//	  "name": "Windows enumeration",
//	  "version": "1.0",
//	  "author": "community",
//	  "scope": "lab-only",
//	  "description": "...",
//	  "techniques": ["T1082", "T1057"],
//	  "steps": [
//	    {"command": "sysinfo"},
//	    {"command": "cat", "args": "C:\\Windows\\System32\\drivers\\etc\\hosts"}
//	  ]
//	}
const (
	// maxScriptSteps es el numero maximo de pasos de un guion.
	maxScriptSteps = 64
	// scriptSpecVersion es la version del formato que entiende el servidor.
	scriptSpecVersion = "dectm-lite/1"
)

// ScriptStep es un paso de un guion.
type ScriptStep struct {
	Command string `json:"command"`
	Args    string `json:"args,omitempty"`
	Label   string `json:"label,omitempty"`
	// AllowImpact marca el paso como de impacto. El servidor lo rechaza
	// salvo que el guion lo pida explicitamente y el operador consienta.
	AllowImpact bool `json:"allow_impact,omitempty"`
}

// ScriptManifest es el manifiesto de un guion.
type ScriptManifest struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Author      string       `json:"author"`
	Description string       `json:"description"`
	Scope       string       `json:"scope"`
	Techniques  []string     `json:"techniques"`
	Steps       []ScriptStep `json:"steps"`
}

// scriptValidationError describes why a script was refused.
type scriptValidationError struct {
	Reason string
}

func (e *scriptValidationError) Error() string { return e.Reason }

// parseManifest validates a manifest and returns it.
func parseManifest(source string) (*ScriptManifest, error) {
	var manifest ScriptManifest
	if err := json.Unmarshal([]byte(source), &manifest); err != nil {
		return nil, &scriptValidationError{Reason: "the manifest must be a JSON object"}
	}
	if strings.TrimSpace(manifest.ID) == "" {
		return nil, &scriptValidationError{Reason: "the manifest needs an id"}
	}
	if !validPackageID(manifest.ID) {
		return nil, &scriptValidationError{Reason: "invalid script id"}
	}
	if len(manifest.Steps) == 0 {
		return nil, &scriptValidationError{Reason: "the script needs at least one step"}
	}
	if len(manifest.Steps) > maxScriptSteps {
		return nil, &scriptValidationError{Reason: fmt.Sprintf("the script has more than %d steps", maxScriptSteps)}
	}
	if len(manifest.Techniques) > 32 {
		return nil, &scriptValidationError{Reason: "too many techniques"}
	}
	for i, step := range manifest.Steps {
		spec, known := lookupCommand(step.Command)
		if !known {
			return nil, &scriptValidationError{
				Reason: fmt.Sprintf("step %d uses the unknown command %q", i+1, step.Command)}
		}
		if spec.Risk == RiskImpact && !step.AllowImpact {
			return nil, &scriptValidationError{
				Reason: fmt.Sprintf("step %d (%s) has impact level %s: mark it with allow_impact to publish it",
					i+1, spec.Name, spec.Risk)}
		}
	}
	return &manifest, nil
}

// handleScriptRun expands a script into tasks for a session.
//
// Cada paso se encola como una tarea normal y cada uno pasa su propio gate:
// los pasos de impacto necesitan el consentimiento del operador y un agente
// arrancado con -allow-impact.
func (s *Server) handleScriptRun(c *gin.Context) {
	name := strings.TrimSpace(c.Param("id"))
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name required"})
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
		Confirm   string `json:"confirm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.SessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id required"})
		return
	}
	session, ok := s.requireSessionAccess(c, req.SessionID)
	if !ok {
		return
	}

	row, err := s.db.GetCustomPackage(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}
	if err := s.verifyCustomPackage(row); err != nil {
		s.audit(c, "script", "script_rejected", "warning", map[string]any{"script": name})
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	manifest, manifestErr := parseManifest(row.Source)
	if manifestErr != nil {
		// No es un manifiesto: es un script Lua. Viaja como la tarea
		// "script" con el envelope firmado dentro, y el agente lo ejecuta
		// en su sandbox.
		envelope := map[string]any{
			"lang":      scripttrust.LangLua,
			"source":    row.Source,
			"script_id": row.ScriptID,
			"sig":       row.Signature,
			"kid":       row.Kid,
		}
		payload, err := json.Marshal(envelope)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		task := &db.Task{
			SessionID:  session.ID,
			Command:    "script",
			Args:       string(payload),
			OperatorID: s.callerID(c),
			TenantID:   s.callerTenant(c),
		}
		if err := s.db.CreateTask(task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		s.audit(c, "script", "script_executed", "info",
			map[string]any{"script": name, "kind": "lua", "session_id": session.ID})
		c.JSON(http.StatusOK, gin.H{
			"status":    "queued",
			"script":    name,
			"kind":      "lua",
			"steps":     1,
			"session":   session.ID,
			"script_id": row.ScriptID,
		})
		return
	}

	// El gate de impacto se aplica paso a paso, no al guion entero.
	queued := 0
	for _, step := range manifest.Steps {
		spec, _ := lookupCommand(step.Command)
		if spec.Risk == RiskImpact && req.Confirm != ImpactConfirmToken {
			s.audit(c, "script", "script_step_denied", "warning",
				map[string]any{"script": name, "step": spec.Name})
			c.JSON(http.StatusForbidden, gin.H{
				"error":   fmt.Sprintf("the step %q has impact level %s and needs the explicit consent of the operator", spec.Name, spec.Risk),
				"command": spec.Name,
				"risk":    spec.Risk,
				"confirm": ImpactConfirmToken,
			})
			return
		}
		task := &db.Task{
			SessionID:  session.ID,
			Command:    spec.Name,
			Args:       step.Args,
			OperatorID: s.callerID(c),
			TenantID:   s.callerTenant(c),
		}
		if err := s.db.CreateTask(task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		queued++
	}

	s.audit(c, "script", "script_executed", "info",
		map[string]any{"script": name, "steps": queued, "session_id": session.ID})
	c.JSON(http.StatusOK, gin.H{
		"status":    "queued",
		"script":    name,
		"steps":     queued,
		"session":   session.ID,
		"script_id": row.ScriptID,
	})
}

// handleScriptSpec publishes the format of the scripts and its limits. The
// console shows it verbatim in the documentation panel.
func (s *Server) handleScriptSpec(c *gin.Context) {
	limits := map[string]any{
		"max_steps":      maxScriptSteps,
		"max_bytes":      maxPackageBytes,
		"spec":           scriptSpecVersion,
		"signature":      "Ed25519, key published at /api/scripts/pubkey",
		"risk_levels":    []string{RiskRead, RiskWrite, RiskImpact},
		"impact_gate":    "a step with impact level needs allow_impact in the manifest, the operator consent and an agent started with -allow-impact",
		"execution":      "the server expands the steps into normal tasks: the agent receives nothing new",
		"allowed":        "only the commands of /api/commands",
	}
	example := `{
  "id": "windows-enum",
  "name": "Windows enumeration",
  "version": "1.0",
  "author": "community",
  "scope": "lab-only",
  "description": "Inventario basico de un host Windows",
  "techniques": ["T1082", "T1057", "T1016"],
  "steps": [
    { "command": "sysinfo" },
    { "command": "ps" },
    { "command": "netinfo" },
    { "command": "cat", "args": "C:\\Windows\\System32\\drivers\\etc\\hosts" }
  ]
}`
	c.JSON(http.StatusOK, gin.H{
		"spec":     scriptSpecVersion,
		"limits":   limits,
		"example":  example,
		"commands": commandCatalog,
		"scope":    "lab-only",
	})
}

// scriptChecksum returns the digest of a manifest.
func scriptChecksum(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}
