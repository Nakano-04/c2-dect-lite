# Roadmap GUI — c2-dect-lite 2.0.0

> Estado a 2026-10-21 · Consola: `gui/C2Dect` (WPF, .NET 8) · Servidor: `server/` (Go, gin + SQLite)
> Frontera: **lite = pública/open-source. c2-dect = privada, equipo/comercial.**
> La consola Lite es un recorte de la consola completa: se retiran los módulos
> más ofensivos y los que quedan se declaran inactivos.

## 0. Regla público vs privado

- En el repo Lite **no** entra código, configuración ni payload de los módulos
  retirados (worm, ransomware, exploit engine, hollowing, phishing, delivery,
  weapon factory, lateral movement, credential access, AMSI/ETW, SOCKS, etc.).
- Los módulos que la edición Lite **no implementa** siguen en el menú de la
  consola, pero su panel dice exactamente eso y no ejecuta nada.
- El texto de esos paneles **no describe** la capacidad: la edición open source
  publica la estructura, no la implementación.

## 1. Qué hay en 2.0.0 (hecho)

### Consola

- [x] Consola del operador = la de `c2-dect` (misma ventana, tema, vistas, rail lateral, terminal).
- [x] Módulos retirados: Worm, Ransomware Lab, Offensive Modules, Kernel BYOVD,
      Privesc Auto, Evasion, Lateral Movement, Lethal Script, Dynamic VPN, SOCKS5,
      Implant (ransomware), Exploits, 0-Day Lab, Exploit Lab, DECT Phishing, Delivery,
      Weapon Factory & Mutator, Gate & Telemetry, Pipeline 0-Day.
- [x] Módulos inactivos: Listeners, Payloads, Payload Builder, Implant Config,
      Vulnerabilities, Vuln Scan, Hunting NVD, Recon (Go), Segment Map, IA Copilot,
      Remote Desktop, File Manager, Dashboard, Executive Summary, Recon + Vulns,
      Operations, Recon & Fuzzing, Bounty & Research.
- [x] Paneles activos: Sessions, Loot, Profiles, Script Market, Admin, Audit.
- [x] Panel genérico de módulo inactivo (`Views/ModuleInactiveView.xaml`).
- [x] Vista Script Market de la consola completa speaks con el marketplace Lite
      (`/api/scripts/list|get|pubkey|upload`).
- [x] Panel Admin reducido a lo que el servidor Lite tiene: tenants, operadores
      y tokens de arranque (fuera las tablas de categorías/permisos/roles).
- [x] Panel Audit nuevo con la verificación de la cadena.
- [x] Quick commands ajustados al conjunto del agente Lite.
- [x] Fuera el bloque REMOTE CONTROL (captura, teclado, webcam) y el de campañas.
- [x] `Logger` a `%LOCALAPPDATA%\C2Dect\logs` (antes escribía en una ruta
      absoluta compilada).
- [x] `App.xaml.cs` Lite: solo arranca el servidor Lite, ningún componente ofensivo.

### Servidor

- [x] `C2_JWT_SECRET` obligatorio (fail-closed), refresh tokens con rotación.
- [x] RBAC de tres roles con permisos por endpoint.
- [x] Mini multi-tenancy: `tenants`, `tenant_id` en toda fila operativa, tokens
      de arranque, aislamiento cross-tenant (404, no 403).
- [x] Auditoría encadenada por hash con verificación.
- [x] Catálogo de módulos que replica la navegación de la consola.
- [x] Marketplace de paquetes firmados (Ed25519) + instalación por tenant + recetas.
- [x] Rate limiting, cabeceras de seguridad, CORS restringido, bcrypt.
- [x] Correcciones: `last_checkin` vs `last_checkin`, `tenant_id` de las tareas,
      `sent_at`/`completed_at` NULL, contrato `{task:{...}}`.

### Agente

- [x] ECDH P-256 + HKDF-SHA256 + AES-256-GCM en todo el plano del agente.
- [x] Rotación de clave y re-negociación cuando el servidor reinicia.
- [x] Identidad persistente de sesión.
- [x] Jitter `uniform` / `activity` / `none`, backoff exponencial, expiración.
- [x] Resultados troceados, lista de comandos cerrada, Windows/Linux/macOS.

## 2. Qué queda (3.0)

### F0 — Pulido de la consola (1 sem) [P1]

- [ ] Panel de Audit con detalle de la entrada (hash anterior/siguiente, diff).
- [ ] Selector de tenant con búsqueda y creación rápida desde la propia vista.
- [ ] Exportar la traza de auditoría a JSON con la cadena completa.
- [ ] Atajo `Ctrl+Shift+A` para *Launch Agent* (ya está el botón, falta el atajo).
- [ ] Historial de la terminal con navegacion ↑/↓ ya existe; persistirlo entre sesiones.

### F1 — Tiempo real (1 sem) [P1]

- [ ] WebSocket del servidor (`/api/ws`) con ticket de un solo uso, nunca `?token=`.
- [ ] `ClientWebSocket` en la consola: eventos de sesión, tarea y auditoría en vivo.
- [ ] Reconexión con backoff `1s → 30s` y `ping` cada 20 s.

### F2 — Recetas operativas (1 sem) [P2]

- [ ] Editor de recetas en la consola: la vista Script Market publica manifiestos
      firmados, pero no los componentes.
- [ ] Recetas parametrizables (el destino del comando se elige en el formulario).
- [ ] Exportar una receta como guion para el README del proyecto.

### F3 — Endurecer (2 sem) [P2]

- [ ] `audit_log` con retention y rotación (el chain se verifica solo hasta el
      primer segmento).
- [ ] Rate limit por token, no solo por IP.
- [ ] Cabecera `Permissions-Policy` y `Cross-Origin-Opener-Policy`.
- [ ] Empaquetado del servidor (systemd / Windows service) y SBOM.

### F4 — E2E continuo (1 sem) [P2]

- [ ] `scripts/e2e.ps1` en el repo: login → checkin → task → resultado → audit →
      marketplace → aislamiento cross-tenant.
- [ ] Test de la consola con `GuiSmoke` (headless): abrir, entrar y comprobar
      que los paneles activos cargan.

## 3. No-hacer (bloqueante)

1. No reintroducir en Lite ningún modulo retirado, ni siquiera "desactivado por
   defecto": la ausencia es lo que hace publicable el proyecto.
2. No escribir en los paneles inactivos una descripcion de lo que haria el
   modulo en la edicion comercial.
3. No subir `build/*.exe`, `*.db`, `*.zip`, `server.key/crt`, `tokens.json` ni
   `.env` al repo: revisar cada commit con `git diff --name-only`.
4. No reintroducir secretos en el fuente: `C2_JWT_SECRET` se genera fuera.
5. No anadir endpoints que el servidor Lite no implemente: la consola y el
   servidor se despliegan juntos.
