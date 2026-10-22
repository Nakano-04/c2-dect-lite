package main

// Runtime de scripts del agente Lite (Lethal Script Lite).
//
// El agente puede ejecutar un script Lua firmado. El runtime es una version
// recortada del del modulo comercial: mismo formato de envelope, misma
// verificacion Ed25519, mismo sandbox por denegacion de globales, pero CON
// CAPACIDADES MINIMAS:
//
//   - no hay shell (c2.exec no existe),
//   - no hay escritura, borrado ni movimiento de ficheros,
//   - la API de sistema es de solo lectura y esta acotada,
//   - las ordenes con nivel de impacto necesitan -allow-impact.
//
// Un script no puede hacer nada que el operador no pueda hacer a mano con el
// catalogo de ordenes, y el gate de impacto se sigue aplicando.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
	"c2-dect-lite/scripttrust"
)

const (
	// luaMaxOutputBytes corta la salida acumulada del script.
	luaMaxOutputBytes = 1 << 20
	// luaMaxFSReadBytes corta la lectura de un fichero desde el script.
	luaMaxFSReadBytes = 256 << 10
	// luaMaxSleep es el techo de una llamada c2.sleep.
	luaMaxSleep = 120 * time.Second
	// luaDefaultTimeout es el limite de ejecucion por defecto.
	luaDefaultTimeout = 30 * time.Second
	// luaMaxTimeout es el limite maximo aceptable.
	luaMaxTimeout = 5 * time.Minute
	// luaMaxCryptoRand es el maximo de bytes por c2.crypto.rand.
	luaMaxCryptoRand = 64
	// luaMaxScriptBytes es el tope de fuente admitida.
	luaMaxScriptBytes = scripttrust.MaxScriptBytes
)

// luaDangerousGlobals se anulan antes de ejecutar: es el sandbox real.
// El filtro del servidor es solo una primera capa.
var luaDangerousGlobals = []string{
	"os", "io", "debug", "package",
	"dofile", "loadfile", "load", "loadstring", "require",
	"collectgarbage", "newproxy", "module",
}

// scriptOpenWarnOnce avisa una sola vez por proceso del modo abierto.
var scriptOpenWarnOnce sync.Once

// executeScriptTask ejecuta el script que llega en los args de la tarea.
func executeScriptTask(cfg *Config, cmdArgs string) (string, error) {
	return executeScriptTaskWithTimeout(cfg, cmdArgs, luaDefaultTimeout)
}

// executeScriptTaskWithTimeout ejecuta el script con limite de tiempo.
func executeScriptTaskWithTimeout(cfg *Config, cmdArgs string, timeout time.Duration) (string, error) {
	if timeout <= 0 || timeout > luaMaxTimeout {
		timeout = luaDefaultTimeout
	}

	env, isRaw, err := scripttrust.ParseEnvelope(strings.TrimSpace(cmdArgs))
	if err != nil {
		return "", err
	}
	if isRaw {
		// La edicion Lite exige envelope firmado: un script crudo no viaja.
		return "", errors.New("signed envelope required (C2_SCRIPT_PUBKEY is set)")
	}
	if env.Lang == scripttrust.LangWASM {
		return "", errors.New("wasm scripts are not part of the Lite edition")
	}

	_, enforced, err := scriptPubkey()
	if err != nil {
		return "", err
	}
	if !enforced {
		// Sin clave no hay confianza: el scripting esta desactivado.
		return "", errors.New("scripting disabled: set C2_SCRIPT_PUBKEY with the marketplace public key")
	}

	if _, err := scripttrust.CheckScriptID(env); err != nil {
		return "", err
	}
	if err := verifyScriptSource(env.Source, env.Sig, env.Kid); err != nil {
		return "", err
	}
	skew, err := scriptSkewSec()
	if err != nil {
		return "", err
	}
	if err := scripttrust.CheckWindow(env.Exp, env.Nbf, time.Now().Unix(), skew); err != nil {
		return "", err
	}
	if err := enforceScriptDeny(scripttrust.ScriptIDOf(env.Source)); err != nil {
		return "", err
	}
	if len(env.Source) > scripttrust.MaxScriptBytes {
		return "", fmt.Errorf("script source too large (%d bytes)", len(env.Source))
	}

	args, err := scripttrust.DecodeArgs(env)
	if err != nil {
		return "", err
	}

	return runLuaScript(cfg, env.Source, string(args), timeout)
}

// scriptPubkey lee la clave publica de confianza del entorno.
func scriptPubkey() ([]ed25519.PublicKey, bool, error) {
	raw := strings.TrimSpace(os.Getenv("C2_SCRIPT_PUBKEY"))
	if raw == "" {
		scriptOpenWarnOnce.Do(func() {
			fmt.Fprintln(os.Stderr, "[!] scripting disabled: C2_SCRIPT_PUBKEY is not set")
		})
		return nil, false, nil
	}
	keys, err := scripttrust.ParseKeyList(raw, scripttrust.MaxScriptKeys)
	if err != nil {
		return nil, false, fmt.Errorf("bad C2_SCRIPT_PUBKEY: %v", err)
	}
	return keys, true, nil
}

// scriptSkewSec resuelve la tolerancia de reloj.
func scriptSkewSec() (int64, error) {
	skew, err := scripttrust.ParseSkew(strings.TrimSpace(os.Getenv("C2_SCRIPT_SKEW")))
	if err != nil {
		return 0, fmt.Errorf("bad C2_SCRIPT_SKEW: %v", err)
	}
	return skew, nil
}

// verifyScriptSource comprueba la firma Ed25519 del source.
func verifyScriptSource(source, sig, kid string) error {
	pubs, enforced, err := scriptPubkey()
	if err != nil {
		return err
	}
	if !enforced {
		return errors.New("scripting disabled: unsigned scripts rejected by default")
	}
	return scripttrust.VerifySource(pubs, source, sig, kid)
}

// enforceScriptDeny aplica la denylist de script_id.
func enforceScriptDeny(scriptID string) error {
	if scriptID == "" {
		return nil
	}
	deny := scriptDenyList()
	if deny[scriptID] {
		return errors.New("script revoked (denylist)")
	}
	return nil
}

// scriptDenyList lee la denylist del entorno y del fichero.
func scriptDenyList() map[string]bool {
	out := map[string]bool{}
	add := func(raw string) {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\n' || r == '\r' || r == '\t'
		}) {
			part = strings.TrimSpace(part)
			if len(part) != 64 {
				continue
			}
			if _, err := hex.DecodeString(part); err != nil {
				continue
			}
			out[strings.ToLower(part)] = true
		}
	}
	add(os.Getenv("C2_SCRIPT_DENY"))
	if path := strings.TrimSpace(os.Getenv("C2_SCRIPT_DENY_FILE")); path != "" {
		if data, err := os.ReadFile(path); err == nil && len(data) <= 1<<20 {
			add(string(data))
		}
	}
	return out
}

// runLuaScript crea la VM, aplica el sandbox y ejecuta el source.
func runLuaScript(cfg *Config, source, args string, timeout time.Duration) (string, error) {
	L := lua.NewState(lua.Options{
		CallStackSize:       256,
		RegistrySize:        1024,
		MinimizeStackMemory: true,
	})
	defer L.Close()

	for _, name := range luaDangerousGlobals {
		L.SetGlobal(name, lua.LNil)
	}
	hardenLuaStringLib(L)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	L.SetContext(ctx)

	logs := &strings.Builder{}
	registerLuaAPI(cfg, L, logs, args)

	if err := L.DoString(source); err != nil {
		return joinLuaLogs(logs, fmt.Sprintf("lua: %v", err)), err
	}
	return joinLuaLogs(logs, ""), nil
}

// joinLuaLogs une los logs con la salida final.
func joinLuaLogs(logs *strings.Builder, out string) string {
	var sb strings.Builder
	if logs != nil && logs.Len() > 0 {
		sb.WriteString(logs.String())
	}
	if out != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(out)
	}
	result := sb.String()
	if result == "" {
		return "(script completed, no output)"
	}
	return truncateLuaOutput(result)
}

// truncateLuaOutput corta la salida al tope y avisa.
func truncateLuaOutput(s string) string {
	if len(s) <= luaMaxOutputBytes {
		return s
	}
	return s[:luaMaxOutputBytes] + "\n...[truncated]"
}

// hardenLuaStringLib reimplementa string.rep con el separador de Lua 5.2 y
// un tope de resultado: gopher-lua es 5.1 y lo ignora.
func hardenLuaStringLib(L *lua.LState) {
	st := L.GetGlobal("string")
	if st == lua.LNil {
		return
	}
	tbl, ok := st.(*lua.LTable)
	if !ok {
		return
	}
	orig := tbl.RawGetString("rep")
	if orig == lua.LNil {
		return
	}
	tbl.RawSetString("rep", L.NewFunction(func(L *lua.LState) int {
		s := L.ToString(1)
		n := L.ToInt(2)
		sep := ""
		if L.GetTop() >= 3 {
			sep = L.ToString(3)
		}
		if n <= 0 {
			L.Push(lua.LString(""))
			return 1
		}
		total := len(s)*n + len(sep)*(n-1)
		if total > luaMaxOutputBytes {
			L.RaiseError("string.rep: result too large (cap %d bytes)", luaMaxOutputBytes)
			return 0
		}
		L.Push(lua.LString(strings.Repeat(s, n)))
		return 1
	}))
}

// registerLuaAPI registra la superficie c2.* del script.
func registerLuaAPI(cfg *Config, L *lua.LState, logs *strings.Builder, args string) {
	c2 := L.NewTable()

	log := L.NewFunction(func(L *lua.LState) int {
		level := "INFO"
		msg := ""
		if L.GetTop() == 1 {
			msg = L.ToString(1)
		} else if L.GetTop() >= 2 {
			level = strings.ToUpper(L.ToString(1))
			msg = L.ToString(2)
		}
		logs.WriteString("[" + level + "] " + msg + "\n")
		L.Push(lua.LTrue)
		return 1
	})
	L.SetField(c2, "log", log)

	sleep := L.NewFunction(func(L *lua.LState) int {
		ms := L.ToInt(1)
		if ms < 0 {
			ms = 0
		}
		d := time.Duration(ms) * time.Millisecond
		if d > luaMaxSleep {
			L.RaiseError("sleep: too long (max %s)", luaMaxSleep)
			return 0
		}
		time.Sleep(d)
		L.Push(lua.LTrue)
		return 1
	})
	L.SetField(c2, "sleep", sleep)

	argsFn := L.NewFunction(func(L *lua.LState) int {
		if args == "" {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(lua.LString(args))
		return 1
	})
	L.SetField(c2, "args", argsFn)

	sessionID := L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(cfg.SessionID))
		return 1
	})
	L.SetField(c2, "session_id", sessionID)

	scriptID := L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(scripttrust.ScriptIDOf(L.ToString(1))))
		return 1
	})
	L.SetField(c2, "script_id", scriptID)

	beacon := L.NewFunction(func(L *lua.LState) int {
		t := L.NewTable()
		L.SetField(t, "session_id", lua.LString(cfg.SessionID))
		L.SetField(t, "os", lua.LString(goos()))
		L.SetField(t, "arch", lua.LString(goarch()))
		L.SetField(t, "pid", lua.LNumber(os.Getpid()))
		L.SetField(t, "process", lua.LString(processName()))
		L.SetField(t, "sleep_sec", lua.LNumber(cfg.SleepSec))
		L.Push(t)
		return 1
	})
	L.SetField(c2, "beacon", beacon)

	// --- crypto (puro) ---
	crypto := L.NewTable()
	sha := L.NewFunction(func(L *lua.LState) int {
		sum := sha256.Sum256([]byte(L.ToString(1)))
		L.Push(lua.LString(hex.EncodeToString(sum[:])))
		return 1
	})
	L.SetField(crypto, "sha256", sha)
	b64e := L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(base64.StdEncoding.EncodeToString([]byte(L.ToString(1)))))
		return 1
	})
	L.SetField(crypto, "b64encode", b64e)
	b64d := L.NewFunction(func(L *lua.LState) int {
		data, err := base64.StdEncoding.DecodeString(L.ToString(1))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("b64decode: invalid base64"))
			return 2
		}
		if len(data) > luaMaxScriptBytes {
			L.Push(lua.LNil)
			L.Push(lua.LString("b64decode: payload too large"))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	})
	L.SetField(crypto, "b64decode", b64d)
	rnd := L.NewFunction(func(L *lua.LState) int {
		n := L.ToInt(1)
		if n <= 0 {
			n = 16
		}
		if n > luaMaxCryptoRand {
			L.Push(lua.LNil)
			L.Push(lua.LString(fmt.Sprintf("rand: too many bytes (max %d)", luaMaxCryptoRand)))
			return 2
		}
		buf := make([]byte, n)
		if _, err := rand.Read(buf); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString("rand: entropy unavailable"))
			return 2
		}
		L.Push(lua.LString(hex.EncodeToString(buf)))
		return 1
	})
	L.SetField(crypto, "rand", rnd)
	L.SetField(c2, "crypto", crypto)

	// --- sistema (solo lectura) ---
	sys := L.NewTable()
	info := L.NewFunction(func(L *lua.LState) int {
		out, _, _ := systemInfo()
		L.Push(lua.LString(out))
		return 1
	})
	L.SetField(sys, "info", info)
	envFn := L.NewFunction(func(L *lua.LState) int {
		out, _, _ := environmentVars()
		L.Push(lua.LString(out))
		return 1
	})
	L.SetField(sys, "env", envFn)
	L.SetField(c2, "sys", sys)

	net := L.NewTable()
	netInfo := L.NewFunction(func(L *lua.LState) int {
		out, _, _ := networkInfo()
		L.Push(lua.LString(out))
		return 1
	})
	L.SetField(net, "info", netInfo)
	netConns := L.NewFunction(func(L *lua.LState) int {
		out, _, _ := networkConnections()
		L.Push(lua.LString(out))
		return 1
	})
	L.SetField(net, "conns", netConns)
	L.SetField(c2, "net", net)

	proc := L.NewTable()
	procList := L.NewFunction(func(L *lua.LState) int {
		out, _, _ := processList()
		L.Push(lua.LString(out))
		return 1
	})
	L.SetField(proc, "list", procList)
	L.SetField(c2, "proc", proc)

	// --- ficheros (solo lectura) ---
	fs := L.NewTable()
	fsList := L.NewFunction(func(L *lua.LState) int {
		path := ""
		if L.GetTop() >= 1 {
			path = L.ToString(1)
		}
		out, _, _ := listFiles(path)
		L.Push(lua.LString(out))
		return 1
	})
	L.SetField(fs, "list", fsList)
	fsRead := L.NewFunction(func(L *lua.LState) int {
		data, errMsg, _ := readFileText(L.ToString(1))
		if errMsg != "" {
			L.Push(lua.LNil)
			L.Push(lua.LString(errMsg))
			return 2
		}
		if len(data) > luaMaxFSReadBytes {
			data = data[:luaMaxFSReadBytes] + "\n...[truncated]"
		}
		L.Push(lua.LString(data))
		return 1
	})
	L.SetField(fs, "read", fsRead)
	fsStat := L.NewFunction(func(L *lua.LState) int {
		out, _, _ := fileStat(L.ToString(1))
		L.Push(lua.LString(out))
		return 1
	})
	L.SetField(fs, "stat", fsStat)
	L.SetField(c2, "fs", fs)

	// --- ordenes del catalogo ---
	task := L.NewFunction(func(L *lua.LState) int {
		cmdline := strings.TrimSpace(L.ToString(1))
		if cmdline == "" {
			L.Push(lua.LNil)
			L.Push(lua.LString(`usage: c2.task("<cmd> [args]")`))
			return 2
		}
		out, err := runLuaInternalTask(cfg, cmdline)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(out))
		return 1
	})
	L.SetField(c2, "task", task)

	L.SetGlobal("c2", c2)
}

// runLuaInternalTask ejecuta una orden del catalogo desde el script.
//
// El script usa el mismo catalogo y el mismo gate de impacto que la consola:
// las ordenes de impacto requieren -allow-impact en el agente.
func runLuaInternalTask(cfg *Config, cmdline string) (string, error) {
	name := strings.TrimSpace(strings.ToLower(cmdline))
	args := ""
	if idx := strings.IndexAny(name, " \t"); idx >= 0 {
		args = strings.TrimSpace(name[idx+1:])
		name = name[:idx]
	}
	spec, known := LookupCommand(name)
	if !known {
		return "", fmt.Errorf("task %q is not in the command catalog", name)
	}
	if !cfg.Allowed(spec) {
		return "", fmt.Errorf("task %q has impact level %s and the agent was started without -allow-impact", spec.Name, spec.Risk)
	}
	out, errMsg, _ := execute(cfg, &taskSpec{Command: spec.Name, Args: args})
	if errMsg != "" {
		return out, errors.New(errMsg)
	}
	return out, nil
}
