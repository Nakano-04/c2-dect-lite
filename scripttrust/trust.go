// Package scripttrust concentra la criptografía y validación puras del
// scripting C2-DECT, compartidas por el agente (cmd/agent_go), el teamserver
// (server/api) y el protocolo TCP legacy (pkg/proto).
//
// Fase 12: antes cada lado duplicaba estas funciones "a propósito" (gotcha
// #5 del baseline). Módulo propio con CERO dependencias externas para no
// inflar el agente (un replace hacia server/ arrastraría gin+sqlite al
// binario). Funciones puras, sin I/O ni relojes: `now` y `skew` entran como
// parámetros (determinismo → fuzzing directo).
//
// Lo que NO vive aquí a propósito: storage (caché agente / registry server),
// wiring de env y transporte. Solo matemáticas + parseo.
package scripttrust

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// MaxScriptBytes es el tope de fuente (Lua texto o .wasm binario).
	MaxScriptBytes = 1024 * 1024
	// MaxScriptKeys acota el try-all de verificación (rotación).
	MaxScriptKeys = 8
	// DefaultSkewSec es la tolerancia de reloj cuando el env no la fija.
	DefaultSkewSec = int64(60)
)

// ScriptIDOf es hex(sha256(fuente)); "" si vacía. Usado como script_id en
// envelopes, caché del agente y registry del servidor: un solo cálculo.
func ScriptIDOf(source string) string {
	if source == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

// KidOf identifica una pubkey: hex(sha256(pub))[:16]. El operador lo obtiene
// de GET /api/scripts/pubkey; el envelope lo declara en `kid`.
func KidOf(pub ed25519.PublicKey) string {
	sum := sha256.Sum256([]byte(pub))
	return hex.EncodeToString(sum[:])[:16]
}

// decodeKeyBytes acepta hex o base64 estándar.
func decodeKeyBytes(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if b, err := hex.DecodeString(raw); err == nil {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("not hex nor base64")
}

// ParsePubkey valida hex/base64 de 32B (agente C2_SCRIPT_PUBKEY, claves DB,
// marketplace). Un solo parser para todos los lados.
func ParsePubkey(raw string) (ed25519.PublicKey, error) {
	b, err := decodeKeyBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("pubkey: %v", err)
	}
	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("pubkey: %d bytes (want 32)", len(b))
	}
	return ed25519.PublicKey(b), nil
}

// ParseSignature valida hex/base64 de 64B (campo `sig` del envelope).
func ParseSignature(raw string) ([]byte, error) {
	b, err := decodeKeyBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("bad script sig: %v", err)
	}
	if len(b) != ed25519.SignatureSize {
		return nil, fmt.Errorf("bad script sig: %d bytes (want 64)", len(b))
	}
	return b, nil
}

// ParseSeed valida seed 32B y deriva la privada (servidor SCRIPT_SIGN_KEY).
func ParseSeed(raw string) (ed25519.PrivateKey, error) {
	b, err := decodeKeyBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("seed: %v", err)
	}
	if len(b) != ed25519.SeedSize {
		return nil, fmt.Errorf("seed: %d bytes (want 32)", len(b))
	}
	return ed25519.NewKeyFromSeed(b), nil
}

// ParseKeyList parte coma/espacio/salto y valida 1..max claves (rotación:
// C2_SCRIPT_PUBKEY con varias).
func ParseKeyList(raw string, max int) ([]ed25519.PublicKey, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\r' || r == '\t'
	})
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty key list")
	}
	if len(parts) > max {
		return nil, fmt.Errorf("%d keys (max %d)", len(parts), max)
	}
	keys := make([]ed25519.PublicKey, 0, len(parts))
	for _, p := range parts {
		k, err := ParsePubkey(p)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

// VerifySource verifica `sig` sobre `source` exacto. Con `kid` conocido
// selecciona clave; sin `kid` prueba todas; `kid` desconocido → error sin
// fallback silencioso. pubs vacío = nada que verificar (los lados deciden
// si eso es lab-abierto o denegado).
func VerifySource(pubs []ed25519.PublicKey, source, sig, kid string) error {
	if strings.TrimSpace(sig) == "" {
		return fmt.Errorf("script signature required")
	}
	sigB, err := ParseSignature(sig)
	if err != nil {
		return err
	}
	candidates := pubs
	if strings.TrimSpace(kid) != "" {
		kid = strings.ToLower(strings.TrimSpace(kid))
		candidates = nil
		for _, pub := range pubs {
			if KidOf(pub) == kid {
				candidates = []ed25519.PublicKey{pub}
				break
			}
		}
		if candidates == nil {
			return fmt.Errorf("unknown script kid %q", kid)
		}
	}
	for _, pub := range candidates {
		if ed25519.Verify(pub, []byte(source), sigB) {
			return nil
		}
	}
	return fmt.Errorf("bad script signature")
}

// CheckWindow valida la ventana temporal: exp==0/nbf==0 = extremo abierto.
// skew (segundos, ya validado) tolera relojes: válido si now <= exp+skew y
// now >= nbf-skew. now inyectado (fuzz + tests deterministas).
func CheckWindow(exp, nbf, now, skewSec int64) error {
	if skewSec < 0 {
		skewSec = 0
	}
	if exp < 0 {
		return fmt.Errorf("bad script exp %d", exp)
	}
	if nbf < 0 {
		return fmt.Errorf("bad script nbf %d", nbf)
	}
	if exp != 0 && nbf != 0 && nbf > exp {
		return fmt.Errorf("bad script window: nbf %d after exp %d", nbf, exp)
	}
	if exp != 0 && now > exp+skewSec {
		return fmt.Errorf("script expired (exp=%d)", exp)
	}
	if nbf != 0 && now < nbf-skewSec {
		return fmt.Errorf("script not yet valid (nbf=%d)", nbf)
	}
	return nil
}

// ParseSkew resuelve el env de tolerancia: vacío → DefaultSkewSec;
// entero ≥ 0; otra cosa → error (fail-closed, como la pubkey).
func ParseSkew(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultSkewSec, nil
	}
	var v int64
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("bad skew %q (want seconds >= 0)", raw)
		}
		v = v*10 + int64(c-'0')
		if v > 86400 {
			return 0, fmt.Errorf("bad skew %q (max 86400)", raw)
		}
	}
	return v, nil
}
