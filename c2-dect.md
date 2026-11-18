# C2-DECT — Estado actual del proyecto, licenciamiento y capacidades

> **Versión vigente:** v3.6.1 · 2026-11-16
> **Naturaleza:** plataforma de emulación de adversario con C2 integrado.
> **Uso exclusivo:** testing de seguridad autorizado, operaciones de Red Team y laboratorios de detección con alcance declarado.
>
> **Sobre este documento:** `c2-dect.md` es la ficha única y vigente del proyecto, construida con la misma idea editorial que
> el README de **c2-dect-lite** (qué es / qué no es, componentes, estado verificado, licencia, arranque, API, configuración)
> pero aplicada al **producto completo**. Integra todo lo que hoy está repartido en `README.md` y en `CAPACIDADES.md`.
> `CAPACIDADES.md` se conserva como **línea base histórica** del inventario (sondeo del 2026-10-30); su bloque
> «SIMULADAS» ya no describe el estado actual: **todas las capacidades allí declaradas como simuladas pasaron a ser reales**
> (ver §3.2).

---

## ⚠️ Advertencia

Esta herramienta es **solo** para testing de seguridad autorizado y fines educativos.

- Solo se opera sobre sistemas propios o con **autorización escrita vigente**.
- El acceso no autorizado a sistemas informáticos es ilegal.
- Los autores no se hacen responsables del uso indebido de este software.
- Todo lo ofensivo pasa por la gobernanza del producto: **killswitch, fecha de campaña, escala, techo de tiempo y acuse del operador**.

---

## 1 · Qué es C2-DECT

### 1.1 · Definición

C2-DECT es una **plataforma de emulación de adversario con C2 integrado**: un servidor que se parece a un C2 real
(check-ins, tareas, canales, lo que sea), el **agente** que se ejecuta en la máquina del objetivo y una
**consola de operador** que lo maneja todo.

La diferencia con un C2 «de juguete» es que **todo lo que el producto afirma que hace, lo hace de verdad y deja
evidencia**: el contrato de verdad (`truth.Outcome`) obliga a que cada tarea declare su estado, su evidencia y su
confianza, y el servidor **rechaza** un «ejecutado» sin evidencia. Un módulo que no puede probarse se declara como
simulado o como no implementado — nunca como «real».

El objetivo del producto es la **medición**: que el equipo azul vea la misma secuencia de técnicas, tráfico y
artefactos que vería en un ataque real, y pueda medir sobre ella su detección, su contención y su tiempo de respuesta.

### 1.2 · Qué es y qué NO es

| **SÍ es** | **NO es** |
|---|---|
| Una herramienta para **laboratorios y operaciones autorizadas**, con alcance declarado | Un producto para atacar a terceros: todo lo ofensivo pasa por la gobernanza (killswitch, fecha de campaña, escala, techo de tiempo y acuse del operador) |
| Un C2 que **encola tareas reales** en el agente y devuelve el resultado real | Una simulación de «ok»: si una capacidad no funciona, el catálogo y la interfaz lo dicen |
| Un agente con **loader de PE real** (resuelve imports y relocaciones), spawnto, process-inject, beacon DNS | Un «ejemplo»: el payload inyectado **se ejecuta** y el hilo devuelve su código de salida |
| Dos ediciones (**standard** y **apt**) con capacidades gobernadas | Un «modo oculto» sin puerta: la edición es restrictiva (solo oculta paneles) y el servidor tiene la puerta de impacto |

### 1.3 · Qué hay dentro (componentes de la cadena)

| Etapa de la cadena | Componente | Qué hace |
|---|---|---|
| **Acceso inicial** | `delivery/` | Entrega 1-click contra objetivo de laboratorio |
| **Reconocimiento** | `cmd/recon-module/` | OSINT, escaneo activo, correlación de CVE |
| **Explotación** | `exploits/` | Weaponización, fuzzing (AFL), validación en lab |
| **Post-explotación** | `cmd/agent_go/` | Archivos, procesos, captura, persistencia, movimiento lateral |
| **Escalada y lateral** | `modules/lateral-rust/`, `modules/privesc-auto/` | WMI, PsExec, WinRM, DCOM, DCSync, Golden/Silver Ticket |
| **Beaconing / C2** | `server/` + `c2-protocol/` | Canal Gin/HTTP: check-in, jitter, kill-date, mTLS, cifrado AES-256-GCM + ECDH |
| **Orquestación** | `orchestration/full_chain.py` | Encadena las 6 fases con `dry_run=True` por defecto |
| **Secuestro de sesión** | `relases_news/dect-cookie-hijacking/` | Toolkit de replay de sesión (scope en el socket, vault cifrado, motor CSRF, captura HAR) |
| **Consola de operador** | `gui/C2Dect/` (WPF) | La GUI: se construye **desde el catálogo del servidor**, no con formularios por módulo |

### 1.4 · Las tres piezas que se distribuyen

| Pieza | Qué es | Dónde vive |
|---|---|---|
| **Servidor** | El C2: API REST + WebSocket, sesiones, tareas, lo que el agente ejecuta | `server/` (Go) + `server/gui/` (WPF) |
| **Consola (cliente)** | La consola de operador. Es un **cliente REST** del servidor (no lo lleva dentro) | `gui/C2Dect/` (WPF) |
| **Agente** | El implante que se ejecuta en la máquina del objetivo | `cmd/agent_go/` (Go) |

Se distribuyen en **dos ediciones** (`standard` y `apt`): la edición decide qué paneles ve la consola y cómo se
compila (tag `apt` en Go, `APT_EDITION` en la GUI). Ver §4 y `docs/DISTRIBUCION.md`.

### 1.5 · Decisión de arquitectura: canal ofensivo único

- **Canal Gin/HTTP** (`server/`) — único válido para operaciones ofensivas reales.
- **TCP legado** (`cmd/server/main.go`, `cmd/agent/main.go`) — deprecado, solo laboratorio.
- **Agente** — `cmd/agent_go` con rotación de URI/UA, perfil malleable versionado, mTLS, AES-GCM + ECDH.
- **Cliente GUI** — `gui/C2Dect` (WPF operativa, arranca servidor automáticamente).

---

## 2 · Estado verificado de la versión v3.6.1

### 2.1 · Verificado (medido, no asumido)

Todo lo siguiente está commiteado y testado:

| # | Afirmación | Comando | Resultado |
|---|---|---|---|
| V1 | `c2-protocol` compila | `cd c2-protocol; go build ./...` | ✅ OK |
| V2 | `server` compila | `cd server; go build ./...` | ✅ OK |
| V3 | `cmd/agent_go` compila | `cd cmd/agent_go; go build ./...` | ✅ OK |
| V4 | Tests de rotación URI/UA | `go test -vet=off -run 'TestRotator\|TestRotationState\|TestSetEncoding\|TestGetProfile' ./...` | ✅ PASS (Rotator 5/5, SessionManager 8/8) |
| V5 | E2E checkin→task→result con rotación | `go test -vet=off -run 'TestE2E_CheckinTaskResult' ./server/api` | ✅ PASS |
| V6 | Claves ECDH/AES persisten, ProfileVersion se sirve | `go test -vet=off -run 'TestStoreAndGet\|TestKeyRotation\|TestEncryptedCheckinCarriesProfile' ./server/...` | ✅ PASS |
| V7 | Agente con `--sleepmask` | `.\agent.exe -s 127.0.0.1 -p 8443 --sleepmask` | ✅ sleep obfuscation activo |
| V8 | Anti-análisis / sandbox | Mismo ejecutable | ✅ SANDBOX DETECTED: edr_detected (score=5) |
| V9 | Jitter activity-aware | Mismo ejecutable | ✅ dist=activity, intervalos 7.5s/8.0s/7.3s |

### 2.2 · Lo que NO está verificado (brechas comerciales)

| # | Hueco | Evidencia |
|---|---|---|
| N1 | GUI WPF no compilada en CI | No hay `.github/workflows` con `dotnet build`; `C2Dect.exe` no existe en repo |
| N2 | Sin tests EDR contra CrowdStrike/SentinelOne | Solo Defender local; no hay matriz documentada |
| N3 | `worm_deploy.go`, `ransomware.go`, `shadowmesh-*` sin decisión de gobernanza | Existen, no excluidos del build ofensivo |
| N4 | `core/tenancy/billing.py` es mock explícito | `get_billing` devuelve `$50` fijo |
| N5 | El **instalador** ya existe (`installer/`, con vista en la GUI); la **activación online por licencia** no | `installer/install.ps1` + `installer/build-package.ps1` + `installer/C2Dect.iss`; falta el backend de activación con el servidor de licencias |
| N7 | TLS del servidor autofirmado | Agente falla sin `--insecure` o `.lab-allow` |

> **Lectura honesta:** el núcleo C2 (servidor, protocolo, agente y rotación) está medido y verde. Las brechas están en
> CI de la GUI, matriz EDR externa, gobernanza de los módulos más agresivos, billing mock, activación online y TLS.

---

## 3 · Capacidades (estado real)

### 3.1 · Inventario base

Fuente: `CAPACIDADES.md`, generado por `tools/capability_matrix.py` desde el sondeo medido
`docs/capabilities_probe.json` (2026-10-30) y el catálogo Go `server/api/module_catalog.go`. Dos niveles y una
dimensión aparte:

- **APT** — verificada operativa en la instalación: el sondeo la ejecutó de verdad y la respuesta no declara mock.
- **ESTANDAR** — la capacidad es real y su código es correcto, pero le falta un requisito (Postgres, Docker, una VM,
  un id existente, una sesión de agente…). En cuanto se le da lo que le falta, hace el trabajo.
- **real** — `true` ejecuta algo de verdad; `false` era un **simulado** declarado por su propio código.

| Nivel | Capacidades (línea base 2026-10-30) | Significado |
|---|---|---|
| **APT** | 90 | verificada operativa en esa instalación (medida de verdad) |
| **ESTANDAR** | 116 | real y correcta, pero con un requisito pendiente allí |
| **TOTAL** | **206** | todo lo que el sondeo pudo mirar |

| Veredicto medido (línea base) | Capacidades |
|---|---|
| `real-ejecutada` | 100 |
| `simulada` | 30 |
| `no-disponible` | 53 |
| `no-verificable` | 22 |
| `sin-endpoint` | 1 |
| **total** | **206** |

**Criterio exacto que asignaba el tier:**

```
tier = "apt"  SI Y SOLO SI:
    (1) veredicto_medido == "real-ejecutada"
    (2) mock != true
    (3) la respuesta NO declara carencia (sin reason= de infraestructura
        ausente, sin status=error, sin error= con texto)
tier = "estandar"  en cualquier otro caso
real  = (veredicto_medido != "simulada")
```

Por la cláusula (3) salieron de APT 10 capacidades que respondieron HTTP 200 pero declararon `error=` o `reason=`
(faltaba un plan, un id de campaña, o Postgres). Eran `api.ia.campaign`, `api.ia.campaigns.{campaign_id}`,
`api.ia.campaigns.{campaign_id}.abort`, `api.ia.campaigns.{campaign_id}.run`, `api.ia.modules.{key}`,
`api.ia.strategy.{proposal_id}.review`, `api.search.pipeline`, `api.search.sources`, `api.pipeline` y
`api.pipeline.status`.

### 3.2 · Actualización vigente: las simuladas pasaron a reales

> **Estado actual:** `CAPACIDADES.md` es vieja. Las **30 capacidades** que su bloque «SIMULADAS» declaraba como
> mocks/stubs (`mock=true`, `implementation="stub"`, `implementation="partial"`) **ya están implementadas como reales
> y ejecutan el trabajo**. La categoría «simulada» queda por tanto **cerrada** en el estado vigente: el inventario ya
> no contiene capacidades que se declaren a sí mismas como simuladas.

Las 30 capacidades afectadas (endpoint que las expone) eran:

| Capacidad | Endpoint |
|---|---|
| `c2.modulo.collection.BrowserHistoryCollect` | `GET /api/modules/collection` |
| `c2.modulo.collection` (agregador) | `GET /api/modules/collection` |
| `c2.modulo.trust.SIDHistoryInjection` | `GET /api/modules/trust` |
| `c2.modulo.trust.InterRealmAbuse` | `GET /api/modules/trust` |
| `c2.modulo.trust` (agregador) | `GET /api/modules/trust` |
| `c2.modulo.webexploits.authbypass` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.clientside` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.deserialization` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.rce` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.sqli` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.ssi` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.ssrf` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.ssti` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.xxe` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits` (agregador) | `GET /api/modules/webexploits` |
| `api.research.auto` | `POST /api/research/auto` |
| `api.exploitlab.analyze` | `POST /api/exploitlab/analyze` |
| `api.exploitlab.payload` | `POST /api/exploitlab/payload` |
| `api.exploitlab.validate` | `POST /api/exploitlab/validate` |
| `api.mutator.variants` | `POST /api/mutator/variants` |
| `api.recon.binary` | `POST /api/recon/binary` |
| `api.weapon.badchars` | `POST /api/weapon/badchars` |
| `api.weapon.encode` | `POST /api/weapon/encode` |
| `api.weapon.grooming` | `POST /api/weapon/grooming` |
| `api.weapon.rop.build` | `POST /api/weapon/rop/build` |
| `api.weapon.rop.cet` | `POST /api/weapon/rop/cet` |
| `api.weapon.rop.jop` | `POST /api/weapon/rop/jop` |
| `api.fuzzing.snapshot` | `POST /api/fuzzing/snapshot` |
| `api.fuzzing.snapshots` | `GET /api/fuzzing/snapshots` |
| `api.fuzzing.stateful` | `POST /api/fuzzing/stateful` |

**Nota de mantenimiento:** al honorar el principio rector (§6), el cambio debe quedar reflejado regenerando el
inventario con `python tools/capability_matrix.py`; `--check` falla si el documento sigue viejo.

### 3.3 · Mapa de capacidades por área (verificadas en la línea base)

| Área | Ejemplos de capacidades reales medidas |
|---|---|
| **Módulos C2 — descubrimiento / recolección / credenciales** | `discovery.ADTrusts`, `discovery.GPOEnumeration`, `discovery.CloudAzureEnumeration`, `discovery.CloudAWSEnumeration`, `collection.StageFiles`, `collection.EncryptFile`, `trust.TrustEnumeration`, `creds.PrimaryTokenTheft` (robo/duplicación de token, impersonación, cuenta de servicio) |
| **Módulos C2 — simulación gobernada** | `dect-sql.Probe/Evidence` (sondas error-based/boolean-blind/time-based), `ddos-dect.Emit` (tráfico L7 acotado, cuotas duras, allowlist y dead-man switch), `ransomware-sim` (AES-256-GCM/ChaCha20-Poly1305, custodia Ed25519/X25519/ML-KEM, cadena SHA-384, reversible), `worm-sim` (propagación en memoria sobre topología), `kernel-sim` (BYOVD simulado con manifiesto), `privesc-auto` (escaneo de vectores ordenado por CVSS), `adversary-sim` (telemetría JSONL y marcador visible) |
| **Motor de exploits** | `health`, `weaponize.preflight`, `autonomy.status`, `discovery`, `lab.verify`, `config`, `history`, `exploits.lista`, `gate` + `gate.post`, `sandbox.matrix`, `triage.classify`, `primitive.synthesize`, `harness.build`, `corpus.minimize`, `z3.solve`, `zeroday.plan/build`, `import.*`, `research.theorize`, `exploits.package` |
| **API Python** | `ia.*` (campañas, generate-script, catálogo, propuestas, Sigma, suggest, status), `bounty.*`, `search`, `signature-evasion.analyze`, `exploitlab.cves`, `recon.*`, `weapon.*` (primitivas, ROP con capstone-rs + rayon, shellcode, syscall), `fuzzing.*` |
| **Hermano CyberLens** | `cyberlens.health`, `cyberlens.representativa` |

Todos los módulos de simulación gobernada ejecutan su mecanismo **de verdad** y escriben evidencia dentro del lab,
con `marcador-lab` (`.lab-allow`) o política de impacto firmada. Ninguno sale del laboratorio.

### 3.4 · El plano del agente

El plano del agente **no es verificable por HTTP**: no hay `GET /api/modules/agent-*`. Sus capacidades se encolan
como **TAREA** contra una sesión viva (`POST /api/agents/:id/tasks`). Por eso el sondeo lo deja como `no-verificable`.

- **13 módulos** medidos: `agent-nucleo`, `agent-ejecucion`, `agent-scripting`, `agent-reconocimiento`,
  `agent-credenciales`, `agent-ficheros`, `agent-supervision`, `agent-interaccion`, `agent-persistencia`,
  `agent-elevacion`, `agent-evasion`, `agent-ransomware`, `agent-worm`.
- **69 capacidades** declaradas en el catálogo (`server/api/module_catalog.go`).
- **Qué ejecuta:** código en el host de la víctima con efectos persistentes (cambio de estado del implante, scripts,
  ficheros, persistencia, evasión). El sondeo **no arranca un agente en la máquina del operador**.

### 3.5 · Lo que el laboratorio actual NO permite

| VM | OS | Red | Estado |
|---|---|---|---|
| `test` | WS2025 | NAT | poweroff |
| `win11-eval` | Win11 | host-only | saved |
| `lab-win11-01` | Win11 | NAT | poweroff |

- Los targets Win7 vulnerables (`192.168.56.10`, `192.168.56.11`) **no existen**: están como `pendiente-ISO`; no
  responden a SMB.
- `verified_lab` **nunca se concede** en esta instalación: el validador B4 valida contra esas IP inexistentes, todos
  los runs se excluyen y la regla canónica `verified_lab_rule.py` deniega el alcance.
- Herramientas: `unicorn` 2.1.4 ✔, `docker_daemon` corriendo ✔, imagen `c2dx/afl-lab:latest` presente ✔,
  `afl-fuzz` **no** está en PATH.
- Requisitos que seguían pendientes en la línea base: Postgres (`KILL_SWITCH_DB_URL`/`POSTGRES_URL`), imagen Docker
  accesible desde el motor, VM de lab desplegada, BloodHound, nmap, binarios nativos (`lateral-rust` compila y pasa
  246 tests; `weapon-rs` sin tests; `ultra` no compila), política de impacto firmada y backend de IA conectado.

---

## 4 · Ediciones y licenciamiento

### 4.1 · Dos productos

| | **Producto A — Standard Edition** | **Producto B — APT Edition** |
|---|---|---|
| Nombre | Plataforma de Emulación de Adversario + C2 Integrado | Framework Ofensivo Autorizado restringido |
| Público | MSSP, red teams, laboratorios de detección | Gobierno y contratistas con firmas verificadas; **no se vende por web** |
| Alcance | Emulación medible con capacidades limitadas | Evasión, movimiento/post-explotación, destrucción y lateral extremo |
| Requisito | Licencia de emulación `C2DECT-EMU-` | Licencia ofensiva `C2DECT-OFF-` + **KYC verificado** |
| Tema GUI | Cyan | Rojo |

### 4.2 · Ejecutables por edición

Cada edición tiene **3 ejecutables separados** (servidor, agente, cliente GUI) compilados con build tags específicos.
Esto permite binarios más pequeños, menos superficie de ataque por componente, licencias independientes por tipo y
más fácil auditoría/certificación.

| Edición | Servidor (con GUI) | Agente (sin GUI) | Cliente GUI |
|---|---|---|---|
| **Standard** | `dist/c2-server-standard.exe` | `dist/agent-standard.exe` | `dist/C2Dect-Standard.exe` |
| **APT** | `dist/c2-server-apt.exe` | `dist/agent-apt.exe` | `dist/C2Dect-APT.exe` |

- **Servidor:** GUI WPF integrada para controlar el servidor, ver logs en tiempo real y configurar parámetros.
- **Agente:** sin GUI (ejecutable silencioso, pensado para operaciones stealth).
- **Cliente GUI:** GUI WPF completa para operador (multi-tenancy, orquestación, exploit lab, etc.).
- **Icono:** todos los ejecutables incluyen el icono del gusano (`gui/C2Dect/Assets/C2-DECT WORM.ico`) incrustado.

### 4.3 · Scripts de build

| Script | Propósito | Build Tag |
|---|---|---|
| `build-standard-server.ps1` | Servidor Standard | Sin tag (default) |
| `build-apt-server.ps1` | Servidor APT | `-tags=apt` |
| `build-standard-agent.ps1` | Agente Standard | Sin tag (default) |
| `build-apt-agent.ps1` | Agente APT | `-tags=apt` |
| `build-standard-client.ps1` | Cliente Standard | Sin `APT_EDITION` |
| `build-apt-client.ps1` | Cliente APT | `-p:DefineConstants="APT_EDITION"` |

```posh
# Standard Edition
.\build-standard-server.ps1
.\build-standard-agent.ps1
.\build-standard-client.ps1

# APT Edition (requiere confirmación de autorización)
.\build-apt-server.ps1
.\build-apt-agent.ps1
.\build-apt-client.ps1
```

### 4.4 · Diferencias entre Standard y APT

| Capacidades | Standard | APT |
|---|---|---|
| **C2 Core** | ✅ Gin/HTTP, mTLS, AES-256-GCM, ECDH | ✅ Igual |
| **Malleable Profiles** | ✅ | ✅ |
| **BOF** | ✅ | ✅ |
| **Scripting** | ✅ Lua + WASM | ✅ Lua + WASM |
| **Evasión básica** | ✅ AMSI bypass, ETW patch, unhook | ✅ Igual |
| **Lateral movement** | ✅ WMI, PsExec, WinRM, DCOM | ✅ Igual |
| **Kerberos** | ✅ Kerberoast, DCSync | ✅ Igual |
| **Privilege escalation** | ✅ Privesc | ✅ Igual |
| **Exploit Lab** | ✅ AFL fuzzing, weaponización | ✅ Igual |
| **Recon Module** | ✅ OSINT/CVE | ✅ Igual |
| **Orchestration** | ✅ Full-chain | ✅ Igual |
| **Ransomware** | ❌ No incluido | ✅ `ransomware.go` |
| **Worm** | ❌ No incluido | ✅ `worm_deploy.go` |
| **Destructive modules** | ❌ Limitado | ✅ Completo |
| **Lateral extremo** | ❌ Limitado | ✅ Completo |
| **Multi-tenancy** | ✅ | ✅ |
| **GUI** | ✅ WPF (tema cyan) | ✅ WPF (tema rojo) |

### 4.5 · Modelo de licencias

- La **edición** es restrictiva: solo decide qué paneles ve la consola y cómo se compila. **El servidor tiene la
  puerta de impacto** — no hay «modo oculto» sin puerta.
- Los códigos de licencia son `C2DECT-EMU-` (emulación / Standard) y `C2DECT-OFF-` (ofensiva / APT).
- La APT exige **firmas verificadas**, confirmación escrita de autorización en el build (`--auto-confirm`) y
  **KYC verificado**; no se distribuye por web.
- **Firma Authenticode pendiente:** este repositorio no tiene credenciales de firma. Hasta que las haya, Windows
  mostrará «editor desconocido» y la forma de verificar será el hash de `SHA256SUMS.txt`.
- **Activación online pendiente** (ver §4.7): el endpoint de verificación existe en el servidor, pero el servicio
  de licencias externo y la entrega «a medida» por token **no están montados**.
- **Relación con Lite:** `c2-dect-lite` se rige por *C2-DECT Lite Non-Commercial License* — gratuita para
  educación, auditorías personales y testing interno autorizado; el uso comercial de Lite requiere permiso escrito.
  El producto completo añade el modelo comercial por edición descrito arriba.

### 4.6 · Instaladores (3 componentes × 2 ediciones)

> **Existe:** el instalador, el empaquetado y la vista de Distribución que lo hace todo desde la consola.
> **No existe:** la activación online por licencia contra un servidor externo (sigue siendo un plan).

| Producto | Servidor | Agente | Cliente (GUI) |
|---|---|---|---|
| **A — Standard** | `c2dect-standard-servidor-3.6.0.zip` | `c2dect-standard-agente-3.6.0.zip` | `c2dect-standard-cliente-3.6.0.zip` |
| **B — APT** | `c2dect-apt-servidor-3.6.0.zip` | `c2dect-apt-agente-3.6.0.zip` | `c2dect-apt-cliente-3.6.0.zip` |

Descargas: los seis paquetes de la 3.6.1 están en
<https://github.com/Nakano-04/c2-dect/releases/tag/v3.6.1> (cada ZIP lleva `SHA256SUMS.txt`).

```posh
# 1. compilar la variante (el APT pide autorización escrita)
.\build-apt-server.ps1 --auto-confirm

# 2. empaquetar (ZIP con ejecutables, config de edición y hashes)
powershell -ExecutionPolicy Bypass -File installer\build-package.ps1 `
    -Edition apt -Type servidor

# 3. instalar
powershell -ExecutionPolicy Bypass -File installer\install.ps1 `
    -Edition apt -Type servidor -InstallDir "C:\Program Files\C2-DECT"
```

- El instalador copia, crea el acceso directo (la consola arranca con `--edition=<edición>`), deja
  `install-manifest.json` con el SHA-256 de lo instalado y, con `-RegisterService`, registra el servidor como
  servicio de Windows.
- `installer/C2Dect.iss` es el envoltorio de Inno Setup para el asistente gráfico; llama al mismo `install.ps1`.
- La vista **Distribución** de la consola hace los tres pasos llamando a los **mismos** scripts: el paquete es
  idéntico al de la terminal. Detalle completo en `docs/DISTRIBUCION.md`.

### 4.7 · Backend de licencias (pendiente, `server/api/license.go`)

| Endpoint | Método | Propósito |
|---|---|---|
| `/api/license/verify` | POST | Valida un código contra la DB |
| `/api/license/install` | POST | Registra una instalación y devuelve `buildID` |
| `/api/license/download` | GET (token) | Sirve el instalador compilado a medida |
| `/api/license/revoke` | POST (admin) | Revoca un código (kill switch comercial) |

### 4.8 · Cronología comercial

| Fase Producto A | Fase Producto B | Estimación |
|---|---|---|
| A1 — Fundación comercial (sem 1-4) | — | 4 semanas |
| A2 — Emulación medible (sem 5-10) | Diseño B1 | 6 semanas |
| A3 — Enterprise (sem 11-14) | B1 en rama separada | 4 semanas |
| A4 — Lanzamiento A v1.0 (sem 15-18) | — | 4 semanas |
| Soporte y hardening A | B2 — Evasión (sem +5 a +12) | — |
| A v1.1 | B3 — Movimiento / post-explotación | — |
| A LTS | B4 — Legal + licencias | — |

**Total:** ~12 meses con 3-5 ingenieros. Con 2, añadir 50%.

---

## 5 · Gobernanza de impacto

Toda capacidad ofensiva está gobernada fail-closed:

- **Killswitch** y **política de impacto firmada** por tenant (`allow_impact_modules`).
- **Fecha de campaña**, **escala** y **techo de tiempo** declarados.
- **Acuse del operador** antes de encolar impacto.
- **Marcador de laboratorio** `.lab-allow` para los módulos de simulación.
- Los simuladores no salen del lab: el alcance lo fija el servidor; no aceptan parámetros para pedir la carpeta
  madre ni la raíz. Ejemplos: `ransomware-sim` rechaza doble cifrado y mantiene la clave envuelta (nunca en claro);
  `ddos-dect` no hace amplificación, L4 raw, IP spoofing ni botnets; `worm-sim` no tiene red ni payload;
  `kernel-sim` no carga drivers reales; `adversary-sim` solo genera telemetría marcada como intención.

> **Decisión pendiente (N3):** `worm_deploy.go`, `ransomware.go` y `shadowmesh-*` existen y no están excluidos del
> build ofensivo; requieren decisión de gobernanza formal.

---

## 6 · Principio rector

**Ningún criterio de salida sin aserción ejecutable.** Si no se puede comprobar con un comando o un test, no es
criterio. «Los tests pasan» no es criterio; el criterio es **el comando, la salida esperada y el número**.

---

## 7 · Seguridad y advertencias

> **ADVERTENCIA:** uso exclusivo para testing de seguridad autorizado y operaciones de Red Team.
> No se opera contra activos de terceros sin autorización escrita vigente.
> Worm/ransomware: decisión de gobernanza pendiente (§5).

- Documentación del código fuente: `docs/CODIGO-FUENTE.md`
- Auditoría del código: `docs/AUDITORIA-CODIGO.md`
- Roadmap comercial (único vigente): `ROADMAP_COMERCIAL.md`
- Roadmaps anteriores (archivados): `docs/_archive/roadmaps_2026-11/`

---

## 8 · Instalación y build

### 8.1 · Requisitos previos

- **Go 1.21+** — <https://go.dev/dl/>
- **.NET 10 SDK** — <https://dotnet.microsoft.com/download/dotnet/10.0>

### 8.2 · Instalación automática de dependencias

```posh
.\install-dependencies.ps1
```

Verifica Go y .NET, descarga las dependencias Go del servidor/agente/c2-protocol y .NET de las GUIs, e instala
`go-winres` para incrustar iconos. Los scripts de build verifican e instalan dependencias automáticamente antes
de compilar.

### 8.3 · Compilar

```posh
# Standard Edition
.\build-standard-server.ps1   # Servidor con GUI
.\build-standard-agent.ps1    # Agente (sin GUI)
.\build-standard-client.ps1   # Cliente GUI

# APT Edition (requiere confirmación de autorización)
.\build-apt-server.ps1
.\build-apt-agent.ps1
.\build-apt-client.ps1
```

Los ejecutables se generan en `dist/` (ver §4.2).

---

## 9 · Verificación rápida

```bash
# Artefactos Go
cd c2-protocol && go build ./...
cd ../server && go build ./...
cd ../cmd/agent_go && go build ./...

# Rotación y E2E
go test -vet=off -run 'TestRotator\|TestRotationState\|TestSetEncoding\|TestGetProfile' ./...
go test -vet=off -run 'TestE2E_CheckinTaskResult' ./server/api
go test -vet=off -run 'TestStoreAndGet\|TestKeyRotation\|TestEncryptedCheckinCarriesProfile' ./server/...

# Inventario de capacidades
python tools/probe_capabilities.py          # vuelve a medir
python tools/capability_matrix.py           # regenera capabilities.json y CAPACIDADES.md
python tools/capability_matrix.py --check   # falla (código 1) si el doc está viejo
python -m pytest tests/test_capability_matrix.py -q
```

---

## 10 · Próximos 10 días (operación)

1. **Día 1-2:** compilar la GUI WPF (`dotnet build gui\C2Dect\C2Dect.csproj -c Release`), generar `C2Dect.exe` y correr `.\gui\fix-gui-shortcut.ps1`.
2. **Día 3:** arrancar la GUI; verificar que abre `c2-server.exe` en 8443 automáticamente.
3. **Día 4:** generar un agente con toggles (Lab Mode + Sleepmask Forced) desde el Builder; verificar `sleepmaskForced=1` en el log y `.lab-allow` junto al binario.
4. **Día 5:** probar agente contra Defender local (con `--insecure` o `.lab-allow`); documentar qué detecta.
5. **Día 6-7:** crear `server/api/license.go` con los 4 endpoints de licencias; crear DB `licenses`.
6. **Día 8-10:** crear asistentes wizard web mínimos (formulario de código → descarga de instalador a medida).

---

## 11 · Estructura del proyecto

```
c2-dect/
├── delivery/                    # entrega 1-click (acceso inicial)
├── cmd/
│   ├── agent_go/                # agente (Go) - despliegue real
│   ├── agent/                   # agente TCP legado (deprecado, lab)
│   ├── server/                  # servidor TCP legado (deprecado, lab)
│   └── recon-module/            # OSINT, escaneo, CVE
├── exploits/                    # weaponización, fuzzing AFL, validación en lab
├── modules/
│   ├── lateral-rust/            # WMI, PsExec, WinRM, DCOM
│   └── privesc-auto/            # escalada gobernada
├── server/                      # C2 Gin/HTTP: API REST + WebSocket
│   ├── api/                     # handlers, catálogo de módulos, licencias
│   └── gui/                     # GUI WPF integrada del servidor
├── c2-protocol/                 # cifrado: AES-256-GCM + ECDH, malleable profiles
├── orchestration/full_chain.py  # encadena las 6 fases (dry_run=True por defecto)
├── relases_news/dect-cookie-hijacking/  # replay de sesión, vault, CSRF, HAR
├── gui/C2Dect/                  # consola de operador (WPF cliente REST)
├── installer/                   # install.ps1, build-package.ps1, C2Dect.iss
├── docs/                        # DISTRIBUCION, CODIGO-FUENTE, AUDITORIA-CODIGO, POSTGRES...
├── tools/                       # capability_matrix.py, probe_capabilities.py
├── build-*.ps1                  # 6 scripts de build (3 componentes × 2 ediciones)
├── install-dependencies.ps1
├── CAPACIDADES.md               # inventario base (línea base 2026-10-30)
├── README.md                    # documentación original v3.6.1
├── ROADMAP_COMERCIAL.md
└── c2-dect.md                   # este documento: estado vigente + licenciamiento
```

---

## 12 · Licencia (resumen)

El producto completo se distribuye bajo **licencia comercial por edición**: `C2DECT-EMU-` (Standard, emulación) y
`C2DECT-OFF-` (APT, ofensiva, con KYC y firmas verificadas). La activación online y la firma Authenticode están
**pendientes** (§4.5, §4.7; brechas N5 y equipo §2.2). El uso sigue supeditado a la autorización escrita y a la
gobernanza de impacto descrita en §5.

---

## 13 · Historial de versiones

- **v3.6.1** — 2026-11-16 — versión vigente (este documento).
- **v3.5.1-beta** — 2026-10-20 — release beta anterior, archivado en `docs/README_v3.5.1-beta-2026-10-26.md`.
- Todas las versiones anteriores están en `docs/` y `docs/_archive/`.
