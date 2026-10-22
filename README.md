# C2-DECT Lite

> **Open Source Adversary Simulation Platform for Authorized Security Testing.**
> End-to-end red-team simulation, from initial access to exploitation and beaconing.
> The "C2" in the name is the telemetry transport of the simulation, not the product:
> the point is to replay the attack chain in a controlled lab so defenders can measure
> their own detection and containment.

[![License](https://img.shields.io/badge/License-C2--DECT%20Lite%20(non--commercial)-d4a017)](LICENSE)

⚠️ **This tool is for authorized security testing and educational purposes only.**

- Only use it on systems you own or have explicit written permission to test
- Unauthorized access to computer systems is illegal
- The authors are not responsible for misuse of this software

## What it is

C2-DECT Lite is a lightweight **adversary simulation platform**: it replays the
attacker chain inside a controlled lab so a blue team can observe the same technique,
traffic and artifact sequence a real attack would produce, and measure their
detection and containment on it.

| Stage | Component | What it simulates / measures |
|---|---|---|
| **Beaconing** | Go server (`server/`) | Telemetry channel of the simulation: check-in, session registry, JWT auth, SQLite. Used to rebuild the exercise timeline. |
| **Post-exploitation** | Go agent (`agent/`) | Command set tagged by impact level, each one with its real maturity in the source. |
| **Operator console** | WPF GUI (`gui/C2Dect`) | Timeline of the simulated session, tasking, and the evidence a defender would observe. |
| **Channel protection** | ECDH P-256 + HKDF-SHA256 + AES-256-GCM | Keeps the simulation traffic representative of a protected channel. |
| **Scripting** | Lua runtime in the agent | Lets the community write reusable techniques, signed and auditable. |

This is the **Lite** edition: beaconing, agent, console and scripting. The full chain
(initial access, recon, exploit lab, escalation, lateral movement, orchestration)
lives in the full platform.

**What it is not:**
- **Not** a general-purpose C2 or a persistent attack infrastructure: beaconing exists
  to make the simulation observable, measurable and auditable.
- **Not** production-ready: whatever the source marks as `STUB`, `CHECK-ONLY`,
  `SIMULADO` or `dry-run` is described as such, and the modules that this edition
  does not ship are only a skeleton in the menu.
- **Never** run against third-party assets without written authorization.

---

## Version 2.0.0

The 1.0 line was a working skeleton: the transport was plain HTTP, the AES key was
negotiated but never used, there was a single implicit tenant and the console had
two windows. **2.0.0 turns it into something you can put in a lab.**

| Area | 1.0 | 2.0.0 |
|---|---|---|
| Console | 2 windows | the full operator console of C2-DECT, with the strongest modules removed and the rest declared inactive |
| Transport | HTTP plain, key negotiated and unused | ECDH P-256 + HKDF-SHA256 + AES-256-GCM, key rotation, re-negotiation |
| Agent | fixed sleep, no jitter, no reconnect | jitter (uniform/activity), exponential backoff, expiry, persistent identity, chunked results, watchdog, Windows/Linux/macOS |
| Commands | 18, and an unknown command fell through to the shell | closed list with three impact levels and a double gate |
| Multi-tenancy | none | tenants, `tenant_id` on every row, bootstrap tokens, 3 roles, tenant switcher |
| Audit | none | hash-chained trail, verifiable with one call |
| Scripting | none | Lethal Script Lite: signed Lua in the agent, the commercial window, technique docs |
| Marketplace | none | signed packages (Ed25519), per-tenant install, task recipes |
| Secrets | `jwt_secret=change-me` in the source | `C2_JWT_SECRET` is mandatory: the server refuses to start without it |
| Console session | token in memory | refresh token protected with DPAPI, session restored on open |

---

## Console: what is in it, what is not

The console is the **same operator console** as the full platform, cut down:

- **Functional** — Sessions, Loot, Profiles, **Script Market**, **Lethal Script**,
  **Lethal Script Docs**, Admin (tenants, operators, bootstrap tokens) and Audit.
- **Inactive** (skeleton: catalog entry, menu line, and a panel that says so; **no
  code and no description of the capability**) — Listeners, Payloads, Payload
  Builder, Implant Config, Vulnerabilities, Vuln Scan, Hunting NVD, Recon (Go),
  Segment Map, IA Copilot, Remote Desktop, File Manager, Dashboard, Executive
  Summary, Recon + Vulns, Operations, Recon & Fuzzing, Bounty & Research.
- **Removed** (not in the code) — the strongest modules of the commercial edition.

The server catalog (`GET /api/modules`) is the source of truth for that state:
9 active entries and 17 inactive.

---

## The agent: low-medium lethality

Commands are classified in three levels:

| Level | Commands | Rule |
|-------|----------|-------|
| **read** (15) | `sysinfo`, `hostname`, `whoami`, `uptime`, `ps`, `netinfo`, `services`, `env`, `connections`, `ls`, `cat`, `stat`, `hashfile`, `find`, `exit` | always available |
| **write** (6) | `mkdir`, `upload`, `download`, `cp`, `sleep`, `script` | always available |
| **impact** (4) | `shell`, `kill`, `rm`, `mv` | **double gate**: operator consent (the console checkbox) **and** `-allow-impact` on the agent |

The gate works on both sides: the **server** refuses to queue an impact command
without the consent token (`AGENT-EXEC`), and the **agent** refuses to run it
unless it was started with `-allow-impact`.

The command list is **closed and shared**: the server queues nothing that is not
in the catalog, and the agent refuses what it does not know (1.x executed any
unknown command through the shell).

### Transport

ECDH P-256 for the key exchange, HKDF-SHA256 for the derivation
(`salt = session_id`, `info = "c2-dect-lite v1 aes"`) and AES-256-GCM for the
body of every message. The session key is renegotiated every
`C2_KEY_ROTATION_EVERY` beacons (200 by default) and the agent renegotiates on
its own if the server restarted and lost its material.

### Timing

- jitter `uniform`, `activity` (diurnal) or `none`;
- exponential backoff on failure, capped at 5 minutes;
- optional absolute expiry (`-e`);
- persistent session identity (`~/.c2-dect-lite/agent.json`): the operator does
  not lose the session when the agent restarts;
- every external command has a timeout and every task has a 90 s watchdog, so a
  hung command can never stop the beacon.

---

## Lethal Script Lite

The scripting module of the commercial edition, ported. It is **open to the
community**: anyone writes a script, publishes it and shares it.

**What a script is**: a Lua program that runs inside the agent.

**What it cannot do** (minimum capabilities by design): no shell, no
write/delete/rename of files, no upload/download, no WASM, no
`os`/`io`/`debug`/`require`/`loadstring`, and the impact commands remain
subject to the gate above.

**The flow** (identical to the commercial edition):

1. the operator publishes the script → the server validates the source, derives
   the scope from the `-- scope:` header, **signs it** with Ed25519 and returns
   `script_id`, `sig` and `kid`;
2. the console builds the envelope `{lang, source, script_id, sig, kid}` and
   sends it as the `script` task;
3. the server verifies signature, time window and scope again;
4. the agent verifies the signature with `C2_SCRIPT_PUBKEY`, applies the
   denylist and runs it in the sandbox (30 s timeout, 1 MiB output cap).

**The difference with the commercial edition**: there the operator signs with a
local seed (a keystore); here the **server signs**, so the console never stores
signing material on disk.

**Manifests `.dectm`**: the Lite edition transpiles a subset to Lua from the
console itself (no Python): `name`, `version`, `scope`, `mitre`, `reruns` and
`steps[]` with `log`, `task`, `sleep`, `beacon`, `hash` and `crypto.sha256`.

**Documentation**: [docs/SCRIPTING_LITE.md](docs/SCRIPTING_LITE.md) and the
**Lethal Script Docs** window, with the ATT&CK technique each command covers
(T1082, T1057, T1049, T1005, T1105, T1059…).

---

## Multi-tenancy (mini)

```
operator ──► tenant ──► sessions / tasks / loot / audit
                ▲
                └── bootstrap token (shown once, stored hashed)
```

- the administrator creates tenants and issues one bootstrap token per tenant;
- the agent sends the token on its first contact and the server binds the
  session to the tenant;
- every operational row carries `tenant_id` and every query is scoped;
- a session of another tenant answers **404**, never 403: the API never
  confirms that a foreign resource exists;
- the console shows the tenant in the status bar and lets administrators
  change scope.

---

## Marketplace

- packages are signed with Ed25519 (marketplace key) and published with their
  checksum;
- installation is per tenant and recorded;
- a package whose signature does not verify is refused;
- the `lite-recipes` package brings the task recipes (one-click inventory
  sweeps).

---

## Audit

Every entry carries the hash of the previous one:

```
H(n) = SHA256( H(n-1) || ts || tenant || operator || action || severity || details )
```

`GET /api/audit/verify` recomputes the whole chain and reports where it
breaks.

---

## Stability (honest reading)

**This is a lab beta, not a production product.** What is verified and what is
not:

### Verified (measured, not assumed)

| Check | Result |
|---|---|
| `go vet` on the 3 Go modules (`scripttrust`, `server`, `agent`) | clean |
| `dotnet build -c Release` (console) | 0 errors |
| **API E2E** — 26 checks: login, refresh, agent registration, real tasking with result, allow-list, impact gate on both sides, command tiers, module catalog, marketplace (install, sign, publish, refuse inactive), recipes, **a signed Lua script running in the agent sandbox**, an unsigned script refused, tenants, bootstrap tokens, audit chain, viewer role, cross-tenant isolation | **3/3 runs green** |
| **Console smoke** — real login, marketplace, inactive module panel, Lethal Script window, Lethal Script Docs window, audit panel | **3/3 runs green** |
| Cross compilation of the agent | Windows / Linux / macOS (same code, no CGO) |

### Bugs the measurement found (already fixed)

- **`ps` could hang the whole agent.** `tasklist` sticks when the SCM is slow
  and, with no cap, the agent stopped beaconing and the session froze. Now every
  external command has a 20 s timeout and every task has a 90 s watchdog.
- **The console opened twice** if the login coincided with the session
  restore. Now guarded.
- **The sidebar had no scroll**: the lower sections (GOVERNANCE) were off
  screen and unreachable.
- **The session was lost on close**: the refresh token is now stored protected
  with DPAPI and the console logs in on its own.

### Not verified / known gaps

- **No unit tests.** Neither Go nor C# has a suite: the coverage is end to
  end. `scripttrust` is the natural place to start (it is pure cryptography).
- **No CI.** No pipeline builds and tests on every push.
- **The transport has no TLS.** The server listens on plain HTTP. The *content*
  is encrypted (ECDH + AES-256-GCM) and the token travels in a header, but the
  traffic is visible. The agent has `-tls` and the console has the self-signed
  checkbox, but **the Lite server does not serve HTTPS yet**.
- **The console is Windows only** (WPF). The server and the agent are
  cross-platform.
- **The GUI smoke is fragile if another application steals the focus**: the
  harness drives the UI and needs nobody to type. That is a harness problem,
  not a product one, but know it before trusting a red.
- **The marketplace is local**: no remote registry, no distribution, no
  auto-update. Packages live in the server database.
- **One tenant per console** in practice: the tenant switcher only works for
  administrators.
- **The agent delivers one task per beacon** (like the commercial edition): a
  long queue takes a while to drain.

### Verdict

For **labs and authorized testing** it is stable: the whole flow
(server → agent → console → scripting) is verified and repeated, and the
failures the measurement surfaced are fixed. For **exposure to open networks or
adversaries**, no: the server still needs TLS, and the project still needs a
test suite, CI and hardening (per-token rate limit, tenant key rotation, audit
retention).

---

## Quick start

### Prerequisites

- Go 1.21+ (server and agent)
- .NET 8 SDK (console, Windows only)

### 1. Configure

```bash
export C2_JWT_SECRET="$(head -c 32 /dev/urandom | base64)"   # mandatory
export C2_BOOTSTRAP_USER=admin          # only used on the first start
export C2_BOOTSTRAP_PASSWORD='cambia-esto'
```

In Windows (PowerShell):

```powershell
$env:C2_JWT_SECRET = [Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Maximum 256 }))
$env:C2_BOOTSTRAP_USER = "admin"
$env:C2_BOOTSTRAP_PASSWORD = "cambia-esto"
```

The server **does not start** without `C2_JWT_SECRET`. The rest of the
variables are in [.env.example](.env.example).

### 2. Build

```bash
build.bat          # Windows
build.sh           # Linux / macOS (server and agent only)
```

### 3. Start the server

```bash
cd build
./server.exe
```

```
[+] listening on      127.0.0.1:8443
[+] database          c2-dect-lite.db
[+] marketplace key   b3ecd0d4b2275664
[+] script pubkey     6f1c...            <- put it in the agent
```

### 4. Start the agent

```bash
cd build
./agent.exe -s 127.0.0.1 -p 8443 -S 10 -j 30
```

| Flag | What it does |
|------|--------------|
| `-s` | host or address of the server |
| `-p` | port of the server |
| `-S` | beacon interval in seconds |
| `-j` | jitter percentage |
| `-m` | jitter model: `uniform`, `activity`, `none` |
| `-T` | bootstrap token that binds the agent to a tenant |
| `-e` | absolute expiry (unix timestamp) |
| `-f` | state file (session identity) |
| `-allow-impact` | enables the impact commands (shell, kill, rm, mv) |
| `-tls` | use HTTPS |
| `-k` | accept self signed certificates (lab only) |
| `-v` | beacon statistics |

To run signed scripts the agent needs the server key:

```bash
set C2_SCRIPT_PUBKEY=6f1c...      # the one the server prints at startup
build\agent.exe -s 127.0.0.1 -p 8443
```

### 5. Open the console

```bash
cd gui/C2Dect
dotnet run
```

Log in with the bootstrap user. The session is stored protected with DPAPI: the
console **logs in by itself** next time.

---

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/login` | Authenticate an operator |
| POST | `/api/auth/refresh` | Rotate the access token |
| POST | `/api/auth/logout` | Revoke the refresh token |
| GET | `/api/me` | Profile, tenant and permissions |
| GET | `/api/sessions` | Sessions of the tenant in scope |
| POST | `/api/sessions/:id/task` | Queue a command (`{command, args, confirm}`) |
| GET | `/api/sessions/:id/tasks` | Task history |
| GET | `/api/loot` | Artifacts of the tenant |
| GET | `/api/profiles` | Traffic profiles |
| GET | `/api/audit` | Audit trail |
| GET | `/api/audit/verify` | Verify the chain |
| GET | `/api/modules` | Module catalog (active and inactive) |
| GET | `/api/commands` | Command catalog with its impact level |
| GET | `/api/tenants` | Tenants (admin) |
| POST | `/api/tenants/:id/tokens` | Issue a bootstrap token (admin) |
| GET | `/api/marketplace/catalog` | Marketplace packages |
| POST | `/api/marketplace/packages/:id/install` | Install a package |
| GET | `/api/scripts/list` | Marketplace seen from Script Market |
| GET | `/api/scripts/spec` | Format, limits and documented catalog |
| POST | `/api/scripts/upload` | Publish and sign a script |
| POST | `/api/scripts/:id/run` | Run a script or a manifest |
| POST | `/agent/checkin` | Agent beacon |
| POST | `/agent/task/result` | Agent result |

---

## Configuration

| Variable | Default | For what |
|----------|----------|----------|
| `C2_JWT_SECRET` | *(mandatory)* | Token signing key (min. 32 characters) |
| `C2_LISTEN_ADDR` | `127.0.0.1:8443` | Listen address |
| `C2_DB_PATH` | `c2-dect-lite.db` | SQLite database |
| `C2_PROFILE_DIR` | `profiles` | Traffic profile directory |
| `C2_BOOTSTRAP_USER` | - | Administrator created on the first start |
| `C2_BOOTSTRAP_PASSWORD` | - | Its password (min. 12 characters) |
| `C2_ACCESS_MINUTES` | `30` | Access token lifetime |
| `C2_REFRESH_HOURS` | `12` | Refresh token lifetime |
| `C2_KEY_ROTATION_EVERY` | `200` | Beacons between key rotations |
| `C2_MARKET_SIGN_SEED` | *(generated)* | Ed25519 seed of the marketplace (32 bytes hex) |
| `C2_ALLOW_REGISTER` | `false` | Public registration endpoint |
| `C2_CORS_ORIGINS` | *(none)* | Allowed origins |

In the **agent**: `C2_SCRIPT_PUBKEY` (trust key for scripting), `C2_SCRIPT_SKEW`,
`C2_SCRIPT_DENY`, `C2_SCRIPT_DENY_FILE`.

---

## Verification

```bash
# API E2E: 26 checks (starts server and agent)
powershell -File scripts\e2e.ps1

# console smoke: real login and panels
powershell -File scripts\gui_smoke.ps1
```

---

## Project structure

```
c2-dect-lite/
├── server/            # API, auth, db, audit, marketplace, catalog
│   ├── api/           # handlers, RBAC, script gate
│   ├── auth/          # JWT (access + refresh)
│   ├── core/          # session material and key rotation
│   ├── crypto/        # ECDH + HKDF + AES-256-GCM envelope
│   ├── db/            # schema, queries, audit chain
│   ├── marketplace/   # module catalog and signed packages
│   └── profiles/      # traffic profiles
├── agent/             # the implant
├── scripttrust/       # shared cryptography (no dependencies)
├── gui/C2Dect/        # the WPF console
├── scripts/           # e2e.ps1, gui_smoke.ps1
├── docs/              # SCRIPTING_LITE.md
├── build/             # binaries (not versioned)
├── road_map_gui.md    # roadmap
├── FAQ.md             # frequently asked questions
└── CODE_OF_CONDUCT.md
```

---

## License

**C2-DECT Lite Non-Commercial License.** Free for education, personal audits and
internal authorized security testing. Commercial use of the Lite edition
requires written permission. See [LICENSE](LICENSE) for the full terms.

## Acknowledgments

- Built with Go, C# WPF
- Inspired by leading adversary simulation platforms in the security community
- @AngelaLuna14 thank you for your collaboration in the Go development and in the evasion logics
