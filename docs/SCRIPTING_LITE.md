# Lethal Script Lite — referencia del módulo de scripting

> Documentación de la edición **Lite** (open source) del módulo de scripting de
> C2-DECT. Es el mismo módulo que el de la edición comercial, con la
> superficie reducida al mínimo.

## 1. Qué es

Un script es un programa **Lua** que se ejecuta dentro del agente y que sirve
para encadenar operaciones y recoger el resultado en un solo viaje. El módulo
está **abierto a la comunidad**: cualquiera puede escribir un script,
publicarlo y compartirlo.

Lo que un script **no** puede es ejecutar código arbitrario en el host:

- no hay shell (`c2.exec` no existe),
- no hay escritura, borrado ni movimiento de ficheros,
- no hay descarga ni subida de ficheros,
- no hay WASM,
- `os`, `io`, `debug`, `package`, `require`, `loadstring`, `dofile`, `loadfile`,
  `load` y `collectgarbage` están anulados en la VM,
- la API de sistema es de solo lectura y está acotada,
- las órdenes de **impacto** (`shell`, `kill`, `rm`, `mv`) necesitan que el
  agente arranque con `-allow-impact` **y** el consentimiento del operador.

## 2. Anatomía de un script

```lua
-- scope: lab-only
c2.log("INFO", "hola desde " .. c2.session_id())

local info = c2.task("sysinfo")
if info then
    c2.log("INFO", info)
end

return "listo"
```

- La primera línea (o una de las cinco primeras) declara el alcance:
  `-- scope: lab-only`. El servidor lo deduce de la fuente; el campo no viaja
  en el envelope porque un campo sin firmar sería atacable.
- El script se firma **tal cual**: la firma cubre el source exacto, sin
  canonicalizar. Un byte de diferencia y la firma deja de validar.

## 3. Superficie `c2.*`

| Función | Devuelve | Notas |
|---------|----------|-------|
| `c2.log(level, msg)` | `true` | Escribe en la salida del script. `c2.log(msg)` usa `[INFO]`. |
| `c2.sleep(ms)` | `true` | Espera. Máximo 120 s por llamada. |
| `c2.args()` | `string`/`nil` | Argumentos del envelope. |
| `c2.session_id()` | `string` | Id de la sesión. |
| `c2.script_id(src)` | `string` | `sha256` del source. |
| `c2.beacon()` | tabla | `session_id`, `os`, `arch`, `pid`, `process`, `sleep_sec`. |
| `c2.task("orden args")` | `(salida)` / `(nil, error)` | Ejecuta una orden del catálogo del agente. |
| `c2.fs.list([path])` | `string` | Lista un directorio. Sin ruta = el home. |
| `c2.fs.read(path)` | `(texto)` / `(nil, error)` | Lee un fichero. Tope 256 KiB. |
| `c2.fs.stat(path)` | `string` | Metadatos. |
| `c2.proc.list()` | `string` | Procesos en ejecución. |
| `c2.sys.info()` | `string` | Inventario del host. |
| `c2.sys.env()` | `string` | Entorno del agente. |
| `c2.net.info()` | `string` | Interfaces de red. |
| `c2.net.conns()` | `string` | Conexiones de red. |
| `c2.crypto.sha256(s)` | `string` | Hash. |
| `c2.crypto.b64encode(s)` | `string` | Base64. |
| `c2.crypto.b64decode(s)` | `(texto)` / `(nil, error)` | Base64. Tope 1 MiB. |
| `c2.crypto.rand(n)` | `string` | Aleatorio, 1..64 bytes. |

### `c2.task` y los niveles de impacto

`c2.task` usa exactamente el mismo catálogo y el mismo gate que la consola:

| Nivel | Órdenes | Regla |
|-------|----------|-------|
| lectura | `sysinfo`, `hostname`, `whoami`, `uptime`, `ps`, `netinfo`, `services`, `env`, `connections`, `ls`, `cat`, `stat`, `hashfile`, `find` | siempre disponible |
| escritura | `mkdir`, `upload`, `download`, `cp`, `sleep` | siempre disponible |
| impacto | `shell`, `kill`, `rm`, `mv` | **solo** con `-allow-impact` en el agente |

Un `c2.task` con una orden de impacto sin el flag devuelve un error, no ejecuta
nada.

## 4. Envelope y firma

El script viaja al agente dentro de un envelope JSON:

```json
{
  "lang": "lua",
  "source": "-- scope: lab-only\nc2.log(\"hola\")",
  "script_id": "hex64",
  "sig": "hex128",
  "kid": "hex16"
}
```

- `script_id` = `sha256(source)` en hex. Si no coincide, el agente lo rechaza.
- `sig` = firma **Ed25519** sobre el `source` exacto (hex o base64).
- `kid` = `hex(sha256(pubkey))[:16]`, el identificador de la clave.
- `exp` / `nbf` son opcionales: ventana de validez (tolerancia de 60 s).

### Cadena de confianza

1. El servidor tiene una clave Ed25519 (la del marketplace). La publica en
   `GET /api/scripts/pubkey`.
2. La consola publica el script: el servidor valida la fuente, deduce el
   alcance, **firma** y devuelve `script_id`, `sig` y `kid`.
3. La consola envía la tarea `script` con el envelope.
4. El servidor vuelve a verificar la firma, la ventana y el alcance.
5. El agente verifica la firma con `C2_SCRIPT_PUBKEY` y ejecuta.

```bash
# el agente solo acepta scripts firmados por la clave del servidor
set C2_SCRIPT_PUBKEY=<la pubkey que imprime el servidor al arrancar>
```

Sin `C2_SCRIPT_PUBKEY` el scripting queda **desactivado** en el agente: no hay
modo abierto por defecto. La denylist (`C2_SCRIPT_DENY`, `C2_SCRIPT_DENY_FILE`)
revoca por `script_id` sin reiniciar el agente.

## 5. Manifiestos `.dectm`

La edición Lite transpila un subconjunto de manifiestos `.dectm` a Lua desde la
propia consola (sin Python). Campos: `name`, `version`, `scope`, `mitre`,
`reruns` y `steps[]` con las operaciones `log`, `task`, `sleep`, `beacon`,
`hash` y `crypto.sha256`.

```yaml
# scope: lab-only
name: demo-triage
version: 1.0.0
scope: lab-only
mitre: [T1082, T1057]
reruns: 1
steps:
  - {name: aviso, log: "inicio triage"}
  - {name: sys, task: sysinfo}
  - {name: espera, sleep: 2000}
```

El Lua generado solo usa la superficie `c2.*` de arriba, así que un manifiesto
no puede escalonar a capacidades que el agente no expone.

## 6. Técnicas que cubren las órdenes del catálogo

Esta es la referencia de ataque que accompany al módulo: qué puede revelar un
script escrito con la superficie anterior.

| Técnica | Nombre | Órdenes que la cubren |
|---------|--------|-----------------------|
| T1082 | System Information Discovery | `sysinfo`, `hostname`, `env`, `uptime` |
| T1033 | System Owner/User Discovery | `whoami` |
| T1057 | Process Discovery | `ps`, `c2.proc.list` |
| T1016 | System Network Configuration Discovery | `netinfo`, `c2.net.info` |
| T1049 | System Network Connections Discovery | `connections`, `c2.net.conns` |
| T1007 | System Service Discovery | `services` |
| T1083 | File and Directory Discovery | `ls`, `find`, `stat`, `hashfile` |
| T1005 | Data from Local System | `cat`, `download`, `c2.fs.read` |
| T1105 | Ingress Tool Transfer | `upload`, `cp`, `mkdir` |
| T1059 | Command and Scripting Interpreter | `shell`, `script` (opt-in) |
| T1489 | Service Stop | `kill` (opt-in) |
| T1070.004 | File Deletion | `rm` (opt-in) |
| T1070.006 | Timestomp | `mv` (opt-in) |

## 7. Límites

| Límite | Valor |
|--------|-------|
| Tamaño de la fuente | 1 MiB |
| Timeout de ejecución | 30 s (máx 5 min) |
| Salida acumulada | 1 MiB (`...[truncated]`) |
| `c2.fs.read` | 256 KiB |
| `c2.crypto.rand` | 1..64 bytes |
| Pasos de un manifiesto | 64 |
| Tamaño del alcance | 64 caracteres |
| Claves de confianza por agente | 8 (`C2_SCRIPT_PUBKEY`) |

## 8. Uso responsable

El módulo es para **pruebas autorizadas**. La superficie está acotada a propósito:
recolectar y encadenar. Lo que no cabe en esa superficie (shell, escritura,
destrucción) necesita dos consentimientos explícitos y sigue siendo una
decisión del operador, no del script.
