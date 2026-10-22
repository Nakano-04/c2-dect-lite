package api

import (
	"crypto/ed25519"
	"fmt"
	"strings"
	"time"

	"c2-dect-lite/scripttrust"

	"github.com/gin-gonic/gin"
)

// Gate de scripting del servidor (Lethal Script Lite).
//
// El servidor es quien decide que un script viaja al agente:
//   - solo se acepta un envelope firmado (nada de Lua crudo),
//   - la firma se verifica con la clave del marketplace, que es la misma que
//     el agente tiene en C2_SCRIPT_PUBKEY,
//   - la ventana temporal (exp/nbf) se respeta con la tolerancia de reloj,
//   - el alcance tiene que estar declarado en la cabecera de la fuente.
//
// El server no ejecuta nada del script: solo lo encola como la tarea
// "script" con el envelope dentro.

const (
	// scriptSkewSec es la tolerancia de reloj por defecto.
	scriptSkewSec = int64(60)
	// scriptMaxBytes es el tope de fuente.
	scriptMaxBytes = scripttrust.MaxScriptBytes
)

// scriptGateError describe por que se rechaza un script.
type scriptGateError struct {
	Reason string
	Code   int
}

func (e *scriptGateError) Error() string { return e.Reason }

// verifyScriptTask comprueba el envelope de un script antes de encolarlo.
func (s *Server) verifyScriptTask(c *gin.Context, args string) error {
	env, isRaw, err := scripttrust.ParseEnvelope(strings.TrimSpace(args))
	if err != nil {
		return &scriptGateError{Reason: err.Error(), Code: 400}
	}
	if isRaw {
		return &scriptGateError{
			Reason: "signed envelope required: the Lite edition only accepts signed scripts",
			Code:   403,
		}
	}
	if env.Lang != scripttrust.LangLua {
		return &scriptGateError{
			Reason: fmt.Sprintf("unsupported script lang %q in the Lite edition", env.Lang),
			Code:   403,
		}
	}
	if len(env.Source) == 0 {
		return &scriptGateError{Reason: "the script envelope has no source", Code: 400}
	}
	if len(env.Source) > scriptMaxBytes {
		return &scriptGateError{Reason: "the script source is too large", Code: 400}
	}
	if _, err := scripttrust.CheckScriptID(env); err != nil {
		return &scriptGateError{Reason: err.Error(), Code: 400}
	}

	signer := s.market.Signer()
	if signer == nil {
		return &scriptGateError{Reason: "scripting is disabled: no signing key configured", Code: 500}
	}
	if err := scripttrust.VerifySource(
		[]ed25519.PublicKey{signer.PublicKey()}, env.Source, env.Sig, env.Kid,
	); err != nil {
		return &scriptGateError{Reason: err.Error(), Code: 403}
	}

	skew := scriptSkewSec
	if err := scripttrust.CheckWindow(env.Exp, env.Nbf, time.Now().Unix(), skew); err != nil {
		return &scriptGateError{Reason: err.Error(), Code: 403}
	}

	// El alcance se deduce de la fuente: no viaja en el envelope (un campo
	// sin firmar seria atacable).
	scope := declaredScope(env.Source)
	if scope == "" {
		return &scriptGateError{
			Reason: "scope declaration required (add '-- scope: lab-only' to the header)",
			Code:   403,
		}
	}
	return nil
}
