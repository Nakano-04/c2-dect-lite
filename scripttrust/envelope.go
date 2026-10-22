package scripttrust

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Envelope es la forma extendida de Args para Command="script", idéntica en
// agente y servidor (antes dos structs gemelos). Los lados añaden storage
// (caché/registry) y env; aquí solo shape + parseo puro.
type Envelope struct {
	Lang     string `json:"lang"`
	Source   string `json:"source"`
	ScriptID string `json:"script_id"`
	Args     string `json:"args"`
	ArgsB64  string `json:"args_b64"`
	Sig      string `json:"sig"`
	Kid      string `json:"kid"`
	Exp      int64  `json:"exp"`
	Nbf      int64  `json:"nbf"`
}

// ScriptLangs reconocidos (wasm se ejecuta desde Fase 6).
const (
	LangLua  = "lua"
	LangWASM = "wasm"
)

// ParseEnvelope valida shape y lang; lang vacío defaultea a lua.
// Retorna isRaw=true cuando NO es JSON (solo el agente lo acepta, y solo
// como Lua sin firmar en modo lab; el servidor lo trata igual).
func ParseEnvelope(cmdArgs string) (env Envelope, isRaw bool, err error) {
	trimmed := strings.TrimSpace(cmdArgs)
	if trimmed == "" {
		return Envelope{}, false, fmt.Errorf("usage: script <lua source> | {\"lang\":\"lua\",\"source\":\"...\"}")
	}
	if !strings.HasPrefix(trimmed, "{") {
		return Envelope{}, true, nil
	}
	var e Envelope
	if jerr := json.Unmarshal([]byte(trimmed), &e); jerr != nil {
		return Envelope{}, false, fmt.Errorf("bad script envelope: %v", jerr)
	}
	if e.Lang == "" {
		e.Lang = LangLua
	}
	if e.Lang != LangLua && e.Lang != LangWASM {
		return Envelope{}, false, fmt.Errorf("unsupported script lang %q", e.Lang)
	}
	return e, false, nil
}

// DecodeArgs resuelve args/args_b64 (b64 prevalece).
func DecodeArgs(env Envelope) ([]byte, error) {
	args := []byte(env.Args)
	if env.ArgsB64 != "" {
		dec, derr := base64.StdEncoding.DecodeString(env.ArgsB64)
		if derr != nil {
			return nil, fmt.Errorf("bad args_b64: %v", derr)
		}
		args = dec
	}
	return args, nil
}

// CheckScriptID verifica la coherencia del ID declarado ("", ok si ausente).
func CheckScriptID(env Envelope) (computed string, err error) {
	if strings.TrimSpace(env.Source) == "" {
		return "", nil
	}
	computed = ScriptIDOf(env.Source)
	if strings.TrimSpace(env.ScriptID) != "" && strings.TrimSpace(env.ScriptID) != computed {
		return "", fmt.Errorf("script_id mismatch: envelope %q != sha256(source) %q",
			strings.TrimSpace(env.ScriptID), computed)
	}
	return computed, nil
}
