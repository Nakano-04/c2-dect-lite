# Changelog

## 2.0.0

La edicion Lite pasa de esqueleto a producto de laboratorio.

### Estabilidad (medido)

- **E2E de API**: 26 comprobaciones, **3/3 corridas verdes**.
- **Smoke de la consola** (login real + paneles): **3/3 corridas verdes**.
- `go vet` limpio en los tres modulos Go; `dotnet build` sin errores.
- Bugs encontrados por la medicion y corregidos:
  - `ps` (tasklist) podia **colgar el agente entero** y dejar la sesion sin
    beacon: ahora toda orden externa lleva timeout de 20 s y la tarea tiene un
    watchdog de 90 s;
  - el agente tarda un par de reintentos en registrar si el servidor aun no
    escucha (el backoff es correcto): el arnes ahora espera a que aparezca;
  - la consola se abria dos veces si el login coincidia con el restaurado de
    sesion: hay guardia;
  - la barra lateral no tenia scroll y las secciones de abajo quedaban fuera
    de la ventana.

### Consola

- La consola del operador pasa a ser la de `c2-dect` (misma ventana, mismo tema,
  mismas vistas), portada desde el proyecto completo.
- Modulos **retirados**: Worm, Ransomware Lab, Offensive Modules, Kernel BYOVD,
  Privesc Auto, Evasion, Lateral Movement, Lethal Script, Dynamic VPN, SOCKS5,
  Implant (ransomware), Exploits, 0-Day Lab, Exploit Lab, DECT Phishing, Delivery,
  Weapon Factory & Mutator, Gate & Telemetry y Pipeline 0-Day.
- Modulos **presentes pero inactivos**: Listeners, Payloads, Payload Builder,
  Implant Config, Vulnerabilities, Vuln Scan, Hunting NVD, Recon (Go), Segment Map,
  IA Copilot, Remote Desktop, File Manager, Dashboard, Executive Summary,
  Recon + Vulns, Operations, Recon & Fuzzing y Bounty & Research. Su panel explica
  que la edicion no lo incluye y no ejecuta nada.
- Paneles funcionales: Sessions, Loot, Profiles, **Script Market**, Admin
  (tenants, operadores, tokens de arranque) y Audit.
- Navegacion construida a partir del catalogo del servidor (`GET /api/modules`).
- La vista Script Market (de la consola completa) habla con el marketplace Lite:
  los paquetes del catalogo y los publicados por el operador.
- Los comandos rapidos se ajustan al conjunto que el agente Lite implementa.

### Transporte

- ECDH P-256 para el intercambio de claves, HKDF-SHA256 para derivar la clave
  (`salt = session_id`, `info = "c2-dect-lite v1 aes"`) y AES-256-GCM para el
  cuerpo de cada mensaje.
- Rotacion automatica de la clave de sesion (`C2_KEY_ROTATION_EVERY`).
- Re-negociacion: cuando el servidor reinicia y pierde el material de sesion, el
  agente cae a un hello en claro con su clave publica y el servidor responde con
  la suya.
- El agente guarda su identidad (id de sesion y clave privada) en
  `~/.c2-dect-lite/agent.json`, de modo que sobrevive a un reinicio.
- Resultados troceados: el servidor reensambla los trozos.

### Servidor

- `C2_JWT_SECRET` es obligatorio: el servidor no arranca sin el (antes el secreto
  estaba en el fuente).
- Refresh tokens con rotacion, revocacion por logout y lista de revocacion.
- Tres roles (`admin`, `operator`, `viewer`) con permisos por endpoint.
- Mini multi-tenancy: tabla `tenants`, `tenant_id` en `sessions`, `tasks`, `loot`
  y `audit_log`, y tokens de arranque que ligan un agente a un tenant.
- Trama de auditoria encadenada por hash con verificacion
  (`GET /api/audit/verify`).
- Catalogo de modulos con estado activo/inactivo (`GET /api/modules`).
- Marketplace de paquetes firmados (Ed25519), instalacion por tenant y recetas
  de tareas.
- Rate limiting, cabeceras de seguridad, CORS restringido y bcrypt.
- Correcciones: `CleanupStaleSessions` consultaba `last_checkin` (columna
  inexistente) frente a `last_checkin` real; las tareas ya no heredan el tenant 1
  por defecto; la respuesta de `POST /api/sessions/:id/task` es `{task:{...}}`.

### Agente

- Lista de comandos cerrada: una orden desconocida se rechaza en vez de
  ejecutarse por la shell (asi era en 1.x).
- Letalidad **baja-media por defecto**: las ordenes se clasifican en tres
  niveles (lectura, escritura, impacto) y el servidor solo encola los dos
  primeros.
- Las ordenes de impacto (`shell`, `kill`, `rm`, `mv`) estan implementadas pero
  bloqueadas por los dos lados: el servidor exige el token de consentimiento
  (`AGENT-EXEC`, la casilla del consentimiento de la consola) y el agente solo
  las ejecuta si arranco con `-allow-impact`.
- Catalogo de ordenes publicado en `GET /api/commands` con su nivel de impacto.
- New commands: `hostname`, `whoami`, `uptime`, `stat`, `hashfile`.
- Compatible con Windows, Linux y macOS (en 1.x solo habia llamadas a `tasklist`,
  `ipconfig`, `net` y `netstat`).
- Modelo de jitter: `uniform`, `activity` (diurno) y `none`.
- Backoff exponencial ante fallos, con techo de 5 minutos.
- Fecha de expiracion opcional (`-e`).
- Cifrado del transporte, troceado de resultados y re-negociacion de clave.

### Lethal Script Lite (abierto a la comunidad)

- Se reabre el modulo de guiones, pero **en version Lite y con capacidades
  minimas**: un guion es una lista de pasos y cada paso es una orden del
  catalogo del agente. No hay motor de scripting dentro del implante.
- El servidor valida cada paso antes de publicar (una orden desconocida se
  rechaza con el motivo exacto) y **expande el guion en tareas normales** al
  ejecutarlo: el implante no recibe nada nuevo.
- Los pasos de impacto necesitan `allow_impact` en el manifiesto, el
  consentimiento del operador y un agente con `-allow-impact`.
- Publicacion firmada con Ed25519 y checksum, con la clave en
  `GET /api/scripts/pubkey`.
- Limites: 64 pasos, 1 MiB y alcance obligatorio.
- Consola: vista **Lethal Script** (editor, plantilla, publicar, cargar y
  ejecutar contra la sesion seleccionada) y vista **Lethal Script Docs**.
- Documentacion de ataque: cada orden del catalogo lleva las tecnicas de la
  matriz ATT&CK que cubre (`T1082`, `T1057`, `T1049`, `T1105`, ...), publicadas
  en `GET /api/scripts/spec` y en el panel de documentacion.

### Lethal Script Lite (el del modulo comercial, portado)

- Se porta el modulo de scripting **tal cual**: la misma ventana (editor,
  snippets, validacion estatica, modo .dectm, firma, envio y debug iterativo),
  el mismo runtime Lua con gopher-lua en el agente, el mismo envelope
  `{lang, source, script_id, sig, kid}` y el mismo modulo compartido
  `scripttrust` (criptografia pura, sin dependencias) usado por agente y servidor.
- Cambios deliberados respecto al modulo comercial:
  - la **firma la pone el servidor** (clave del marketplace): la consola no
    guarda seeds en disco;
  - la **superficie `c2.*` es la minima**: sin `c2.exec`, sin escritura/borrado
    de ficheros, sin `c2.download/upload`, sin `c2.kv` y sin WASM;
  - las ordenes de impacto siguen el gate de la consola y del agente
    (`-allow-impact` + consentimiento del operador);
  - el agente exige `C2_SCRIPT_PUBKEY`: sin clave el scripting queda
    desactivado y un script sin firma nunca se ejecuta.
- El servidor verifica el envelope antes de encolar (firma, ventana
  temporal y alcance) y el agente lo vuelve a verificar.
- Transpilador `.dectm` a Lua en la propia consola (subset Lite, sin Python):
  `name`, `version`, `scope`, `mitre`, `reruns` y `steps[]` con `log`, `task`,
  `sleep`, `beacon`, `hash` y `crypto.sha256`.
- Documentacion: `docs/SCRIPTING_LITE.md` y la ventana **Lethal Script Docs**.

### Consola

- La ventana Lethal Script del modulo comercial, portada al Lite.
- La barra lateral ahora tiene scroll: sin ella las secciones de abajo
  (GOVERNANCE) quedaban fuera de la ventana.
- Los botones del menu resuelven el comando contra el ViewModel y no contra la
  entrada de navegacion (con el patron RelativeSource del modulo comercial).

### Modulos no Lite

- Los modulos que la edicion Lite no implementa quedan como **esqueleto**:
  entrada de catalogo con nombre y estado, linea en el menu y panel que avisa.
  Sin codigo, sin configuracion y sin descripcion de lo que harian en la
  edicion comercial.
- El catalogo del servidor replica la navegacion de la consola (24 entradas:
  7 activas, 17 inactivas).

### Otros

- El registro de la consola deja de escribir en una ruta absoluta compilada y va
  a `%LOCALAPPDATA%\C2Dect\logs`.
- `.env.example` con toda la configuracion del servidor.

## 1.0.0

- Servidor REST con JWT y SQLite, agente Go con 18 comandos, consola WPF de dos
  ventanas.
- ECDH negociado pero sin cifrado de cuerpo, un solo tenant implicit y sin
  auditoria.

