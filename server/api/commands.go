package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// Niveles de impacto de una orden del agente. La edicion Lite apunta a una
// letalidad baja-media: por defecto solo se admiten las ordenes de lectura y
// las de escritura acotada. Las de impacto exigen el consentimiento
// explicito del operador.
const (
	RiskRead   = "read"
	RiskWrite  = "write"
	RiskImpact = "impact"
)

// ImpactConfirmToken es el token que el operador tiene que enviar para
// encolar una orden de impacto. Es el mismo nombre que usa la edicion
// comercial: "AGENT-EXEC".
const ImpactConfirmToken = "AGENT-EXEC"

// CommandSpec describe una orden del agente y su nivel de impacto.
type CommandSpec struct {
	Name    string   `json:"name"`
	Risk    string   `json:"risk"`
	Aliases []string `json:"aliases,omitempty"`
	Summary string   `json:"summary"`
	// Techniques son las tecnicas de la matriz que cubre esta orden. Es la
	// documentacion que la comunidad necesita para saber que se esta permitiendo.
	Techniques []string `json:"techniques,omitempty"`
}

// commandCatalog es el conjunto cerrado de ordenes. Tiene que coincidir con
// agent/catalog.go: el servidor decide que se encola, el agente se niega a
// ejecutar lo que no le deja.
//
// Techniques: tecnicas de la matriz ATT&CK que cubre cada orden.
var commandCatalog = map[string]CommandSpec{
	// --- solo lectura ---
	"sysinfo": {Name: "sysinfo", Risk: RiskRead, Summary: "inventory of the host",
		Techniques: []string{"T1082", "T1033"}},
	"hostname": {Name: "hostname", Risk: RiskRead, Summary: "name of the host",
		Techniques: []string{"T1082"}},
	"whoami": {Name: "whoami", Risk: RiskRead, Summary: "user the agent runs as",
		Techniques: []string{"T1033"}},
	"uptime": {Name: "uptime", Risk: RiskRead, Summary: "time since the agent started",
		Techniques: []string{"T1082"}},
	"ps": {Name: "ps", Risk: RiskRead, Aliases: []string{"processes"}, Summary: "running processes",
		Techniques: []string{"T1057"}},
	"netinfo": {Name: "netinfo", Risk: RiskRead, Summary: "network interfaces",
		Techniques: []string{"T1016"}},
	"services": {Name: "services", Risk: RiskRead, Summary: "services of the host",
		Techniques: []string{"T1007"}},
	"env": {Name: "env", Risk: RiskRead, Summary: "environment of the agent",
		Techniques: []string{"T1082"}},
	"connections": {Name: "connections", Risk: RiskRead, Aliases: []string{"netstat"}, Summary: "network connections",
		Techniques: []string{"T1049"}},
	"ls": {Name: "ls", Risk: RiskRead, Aliases: []string{"dir"}, Summary: "content of a directory",
		Techniques: []string{"T1083"}},
	"cat": {Name: "cat", Risk: RiskRead, Aliases: []string{"type"}, Summary: "content of a file",
		Techniques: []string{"T1005"}},
	"stat": {Name: "stat", Risk: RiskRead, Summary: "metadata of a file",
		Techniques: []string{"T1083"}},
	"hashfile": {Name: "hashfile", Risk: RiskRead, Summary: "digest of a file",
		Techniques: []string{"T1083"}},
	"find": {Name: "find", Risk: RiskRead, Aliases: []string{"search"}, Summary: "files matching a pattern",
		Techniques: []string{"T1083"}},
	"exit": {Name: "exit", Risk: RiskRead, Summary: "terminate the agent"},

	// --- escritura (impacto medio) ---
	"mkdir": {Name: "mkdir", Risk: RiskWrite, Summary: "create a directory",
		Techniques: []string{"T1105"}},
	"upload": {Name: "upload", Risk: RiskWrite, Summary: "write a file on the host",
		Techniques: []string{"T1105"}},
	"download": {Name: "download", Risk: RiskWrite, Summary: "read a file from the host",
		Techniques: []string{"T1005"}},
	"cp": {Name: "cp", Risk: RiskWrite, Aliases: []string{"copy"}, Summary: "copy a file",
		Techniques: []string{"T1105"}},
	"sleep": {Name: "sleep", Risk: RiskWrite, Summary: "change the beacon interval"},
	"script": {Name: "script", Risk: RiskWrite, Summary: "run a signed script in the agent runtime",
		Techniques: []string{"T1059.005"}},

	// --- impacto (opt-in explicito) ---
	"shell": {Name: "shell", Risk: RiskImpact, Summary: "run a command through the system shell",
		Techniques: []string{"T1059"}},
	"kill": {Name: "kill", Risk: RiskImpact, Summary: "terminate a process",
		Techniques: []string{"T1489"}},
	"rm": {Name: "rm", Risk: RiskImpact, Aliases: []string{"del"}, Summary: "delete a file",
		Techniques: []string{"T1070.004"}},
	"mv": {Name: "mv", Risk: RiskImpact, Aliases: []string{"move"}, Summary: "move or rename a file",
		Techniques: []string{"T1070.006"}},
}

// lookupCommand resolves a command name or alias.
func lookupCommand(name string) (CommandSpec, bool) {
	name = strings.TrimSpace(strings.ToLower(name))
	if spec, ok := commandCatalog[name]; ok {
		return spec, true
	}
	for _, spec := range commandCatalog {
		for _, alias := range spec.Aliases {
			if alias == name {
				return spec, true
			}
		}
	}
	return CommandSpec{}, false
}

// handleCommands publishes the command catalog with its impact level, so the
// console can show it and decide what to send.
func (s *Server) handleCommands(c *gin.Context) {
	specs := make([]CommandSpec, 0, len(commandCatalog))
	for _, spec := range commandCatalog {
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool {
		if specs[i].Risk != specs[j].Risk {
			return specs[i].Risk < specs[j].Risk
		}
		return specs[i].Name < specs[j].Name
	})
	c.JSON(http.StatusOK, gin.H{
		"commands": specs,
		"confirm_token": ImpactConfirmToken,
		"note": "las ordenes de impacto necesitan el consentimiento explicito del operador " +
			"y un agente arrancado con -allow-impact",
	})
}

