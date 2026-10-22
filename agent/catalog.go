package main

// Catalogo de comandos del agente con su nivel de impacto.
//
// La edicion Lite apunta a una letalidad BAJA-MEDIA: por defecto el implante
// solo lee el estado del host y mueve ficheros. Las ordenes de IMPACTO
// (ejecucion arbitraria, terminacion de procesos, borrado o renombrado de
// ficheros) estanImplementadas pero bloqueadas: el servidor las rechaza
// salvo que el operador marque el consentimiento explicito, y el agente
// necesita arrancar con -allow-impact.
//
// Este catalogo tiene que coincidir con server/api/commands.go: el servidor
// es quien decide, el agente es quien se niega.

type Risk string

const (
	// RiskRead: solo lee. No cambia el estado del host.
	RiskRead Risk = "read"
	// RiskWrite: escribe o transfiere. Cambia el estado del host de forma
	// acotada y reversible.
	RiskWrite Risk = "write"
	// RiskImpact: puede romper la operacion del host. Opt-in explicito.
	RiskImpact Risk = "impact"
)

// CommandSpec describe un comando del agente.
type CommandSpec struct {
	Name    string
	Risk    Risk
	Aliases []string
}

// CommandCatalog es el conjunto cerrado de ordenes que el agente Lite acepta.
var CommandCatalog = map[string]CommandSpec{
	// --- solo lectura ---
	"sysinfo":    {Name: "sysinfo", Risk: RiskRead},
	"hostname":   {Name: "hostname", Risk: RiskRead},
	"whoami":     {Name: "whoami", Risk: RiskRead},
	"uptime":     {Name: "uptime", Risk: RiskRead},
	"ps":         {Name: "ps", Risk: RiskRead, Aliases: []string{"processes"}},
	"netinfo":    {Name: "netinfo", Risk: RiskRead},
	"services":   {Name: "services", Risk: RiskRead},
	"env":        {Name: "env", Risk: RiskRead},
	"connections": {Name: "connections", Risk: RiskRead, Aliases: []string{"netstat"}},
	"ls":         {Name: "ls", Risk: RiskRead, Aliases: []string{"dir"}},
	"cat":        {Name: "cat", Risk: RiskRead, Aliases: []string{"type"}},
	"stat":       {Name: "stat", Risk: RiskRead},
	"hashfile":   {Name: "hashfile", Risk: RiskRead},
	"find":       {Name: "find", Risk: RiskRead, Aliases: []string{"search"}},
	"exit":       {Name: "exit", Risk: RiskRead},

	// --- escritura (impacto medio) ---
	"mkdir":   {Name: "mkdir", Risk: RiskWrite},
	"upload":  {Name: "upload", Risk: RiskWrite},
	"download": {Name: "download", Risk: RiskWrite},
	"cp":      {Name: "cp", Risk: RiskWrite, Aliases: []string{"copy"}},
	"sleep":   {Name: "sleep", Risk: RiskWrite},

	// --- impacto (opt-in explicito) ---
	"shell": {Name: "shell", Risk: RiskImpact},
	"kill":  {Name: "kill", Risk: RiskImpact},
	"rm":    {Name: "rm", Risk: RiskImpact, Aliases: []string{"del"}},
	"mv":    {Name: "mv", Risk: RiskImpact, Aliases: []string{"move"}},
}

// LookupCommand resolves a command name or alias and returns its spec.
func LookupCommand(name string) (CommandSpec, bool) {
	name = normalizeCommand(name)
	if spec, ok := CommandCatalog[name]; ok {
		return spec, true
	}
	for _, spec := range CommandCatalog {
		for _, alias := range spec.Aliases {
			if alias == name {
				return spec, true
			}
		}
	}
	return CommandSpec{}, false
}

// normalizeCommand lowercases and trims a command name.
func normalizeCommand(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		out = append(out, c)
	}
	return trimSpace(string(out))
}

// trimSpace removes the surrounding blanks of a string.
func trimSpace(s string) string {
	start := 0
	for start < len(s) && isSpace(s[start]) {
		start++
	}
	end := len(s)
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

// isSpace reports whether the byte is a blank.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// Allowed reports whether the agent may run the command with the given
// configuration. Las ordenes de impacto necesitan -allow-impact.
func (c *Config) Allowed(spec CommandSpec) bool {
	if spec.Risk == RiskImpact {
		return c.AllowImpact
	}
	return true
}
