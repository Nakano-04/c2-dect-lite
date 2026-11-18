# C2-DECT — Current Project Status, Licensing and Capabilities

> **Current version:** v3.6.1 · 2026-11-16
> **Nature:** adversary emulation platform with an integrated C2.
> **Intended use only:** authorized security testing, Red Team operations and detection labs with a declared scope.
>
> **About this document:** `c2-dect.md` is the single, up-to-date fact sheet for the project, built with the same
> editorial idea as the **c2-dect-lite** README (what it is / what it is not, components, verified state, license,
> quick start, API, configuration) but applied to the **full product**. It consolidates everything currently spread
> across `README.md` and `CAPACIDADES.md`. A Spanish translation is available as `c2-dect.es.md`.
> `CAPACIDADES.md` is kept as the **historical baseline** of the inventory (probe dated 2026-10-30); its
> «SIMULATED» block no longer describes the current state: **every capability declared there as simulated has
> become real** (see §3.2).

---

## ⚠️ Warning

This tool is **only** for authorized security testing and educational purposes.

- Only operate on systems you own or have **current written authorization** to test.
- Unauthorized access to computer systems is illegal.
- The authors are not responsible for misuse of this software.
- Everything offensive goes through the product's governance: **kill switch, campaign date, scale, time ceiling and operator acknowledgement**.

---

## 1 · What C2-DECT is

### 1.1 · Definition

C2-DECT is an **adversary emulation platform with an integrated C2**: a server that looks like a real C2
(check-ins, tasks, channels, whatever it needs), the **agent** that runs on the target machine, and an
**operator console** that drives it all.

The difference from a "toy" C2 is that **everything the product claims to do, it actually does and leaves
evidence**: the truth contract (`truth.Outcome`) requires every task to declare its status, its evidence and its
confidence, and the server **rejects** an "executed" result without evidence. A module that cannot be proven is
declared simulated or not implemented — never "real".

The goal of the product is **measurement**: the blue team sees the same sequence of techniques, traffic and
artifacts a real attack would produce, and can measure its detection, containment and response time against it.

### 1.2 · What it is and what it is NOT

| **It IS** | **It is NOT** |
|---|---|
| A tool for **labs and authorized operations**, with a declared scope | A product for attacking third parties: everything offensive goes through governance (kill switch, campaign date, scale, time ceiling and operator acknowledgement) |
| A C2 that **queues real tasks** on the agent and returns the real result | A fake "ok" simulation: if a capability does not work, the catalog and the UI say so |
| An agent with a **real PE loader** (resolves imports and relocations), spawnto, process-inject, DNS beacon | A "demo": the injected payload **actually runs** and the thread returns its exit code |
| Two editions (**standard** and **apt**) with governed capabilities | A "hidden mode" without a gate: the edition is restrictive (it only hides panels) and the server holds the impact gate |

### 1.3 · What is inside (chain components)

| Chain stage | Component | What it does |
|---|---|---|
| **Initial access** | `delivery/` | 1-click delivery against a lab target |
| **Reconnaissance** | `cmd/recon-module/` | OSINT, active scanning, CVE correlation |
| **Exploitation** | `exploits/` | Weaponization, fuzzing (AFL), lab validation |
| **Post-exploitation** | `cmd/agent_go/` | Files, processes, capture, persistence, lateral movement |
| **Escalation and lateral** | `modules/lateral-rust/`, `modules/privesc-auto/` | WMI, PsExec, WinRM, DCOM, DCSync, Golden/Silver Ticket |
| **Beaconing / C2** | `server/` + `c2-protocol/` | Gin/HTTP channel: check-in, jitter, kill-date, mTLS, AES-256-GCM + ECDH encryption |
| **Orchestration** | `orchestration/full_chain.py` | Chains the 6 phases with `dry_run=True` by default |
| **Session hijacking** | `relases_news/dect-cookie-hijacking/` | Session replay toolkit (socket-scoped, encrypted vault, CSRF engine, HAR capture) |
| **Operator console** | `gui/C2Dect/` (WPF) | The GUI: built **from the server catalog**, not from per-module forms |

### 1.4 · The three distributed pieces

| Piece | What it is | Where it lives |
|---|---|---|
| **Server** | The C2: REST API + WebSocket, sessions, tasks, what the agent executes | `server/` (Go) + `server/gui/` (WPF) |
| **Console (client)** | The operator console. It is a **REST client** of the server (it does not embed it) | `gui/C2Dect/` (WPF) |
| **Agent** | The implant that runs on the target machine | `cmd/agent_go/` (Go) |

They ship in **two editions** (`standard` and `apt`): the edition decides which panels the console shows and how
the code is compiled (`apt` tag in Go, `APT_EDITION` in the GUI). See §4 and `docs/DISTRIBUCION.md`.

### 1.5 · Architecture decision: single offensive channel

- **Gin/HTTP channel** (`server/`) — the only one valid for real offensive operations.
- **Legacy TCP** (`cmd/server/main.go`, `cmd/agent/main.go`) — deprecated, lab only.
- **Agent** — `cmd/agent_go` with URI/UA rotation, versioned malleable profile, mTLS, AES-GCM + ECDH.
- **GUI client** — `gui/C2Dect` (working WPF, starts the server automatically).

---

## 2 · Verified state of version v3.6.1

### 2.1 · Verified (measured, not assumed)

Everything below is committed and tested:

| # | Assertion | Command | Result |
|---|---|---|---|
| V1 | `c2-protocol` builds | `cd c2-protocol; go build ./...` | ✅ OK |
| V2 | `server` builds | `cd server; go build ./...` | ✅ OK |
| V3 | `cmd/agent_go` builds | `cd cmd/agent_go; go build ./...` | ✅ OK |
| V4 | URI/UA rotation tests | `go test -vet=off -run 'TestRotator\|TestRotationState\|TestSetEncoding\|TestGetProfile' ./...` | ✅ PASS (Rotator 5/5, SessionManager 8/8) |
| V5 | E2E checkin→task→result with rotation | `go test -vet=off -run 'TestE2E_CheckinTaskResult' ./server/api` | ✅ PASS |
| V6 | ECDH/AES keys persist, ProfileVersion is served | `go test -vet=off -run 'TestStoreAndGet\|TestKeyRotation\|TestEncryptedCheckinCarriesProfile' ./server/...` | ✅ PASS |
| V7 | Agent with `--sleepmask` | `.\agent.exe -s 127.0.0.1 -p 8443 --sleepmask` | ✅ sleep obfuscation active |
| V8 | Anti-analysis / sandbox | Same binary | ✅ SANDBOX DETECTED: edr_detected (score=5) |
| V9 | Activity-aware jitter | Same binary | ✅ dist=activity, intervals 7.5s/8.0s/7.3s |

### 2.2 · What is NOT verified (commercial gaps)

| # | Gap | Evidence |
|---|---|---|
| N1 | WPF GUI not built in CI | No `.github/workflows` with `dotnet build`; `C2Dect.exe` does not exist in the repo |
| N2 | No EDR tests against CrowdStrike/SentinelOne | Only local Defender; no documented matrix |
| N3 | `worm_deploy.go`, `ransomware.go`, `shadowmesh-*` lack a governance decision | They exist and are not excluded from the offensive build |
| N4 | `core/tenancy/billing.py` is an explicit mock | `get_billing` returns a fixed `$50` |
| N5 | The **installer** exists (`installer/`, with a GUI view); **online license activation** does not | `installer/install.ps1` + `installer/build-package.ps1` + `installer/C2Dect.iss`; the activation backend against the license server is missing |
| N7 | Server TLS is self-signed | Agent fails without `--insecure` or `.lab-allow` |

> **Honest reading:** the C2 core (server, protocol, agent and rotation) is measured and green. The gaps are in GUI
> CI, an external EDR matrix, governance of the most aggressive modules, mock billing, online activation and TLS.

---

## 3 · Capabilities (current state)

### 3.1 · Baseline inventory

Source: `CAPACIDADES.md`, generated by `tools/capability_matrix.py` from the measured probe
`docs/capabilities_probe.json` (2026-10-30) and the Go catalog `server/api/module_catalog.go`. Two levels plus a
separate dimension:

- **APT** — verified operational in the installation: the probe actually executed it and the response declares no mock.
- **STANDARD** — the capability is real and its code is correct, but a prerequisite is missing (Postgres, Docker, a
  VM, an existing id, an agent session…). Once given what it needs, it does the job.
- **real** — `true` executes something for real; `false` used to mean a **simulated** capability declared as such by
  its own code.

| Level | Capabilities (2026-10-30 baseline) | Meaning |
|---|---|---|
| **APT** | 90 | verified operational in that installation (actually measured) |
| **STANDARD** | 116 | real and correct, but with a pending prerequisite there |
| **TOTAL** | **206** | everything the probe could look at |

| Measured verdict (baseline) | Capabilities |
|---|---|
| `real-ejecutada` (real-executed) | 100 |
| `simulada` (simulated) | 30 |
| `no-disponible` (not-available) | 53 |
| `no-verificable` (not-verifiable) | 22 |
| `sin-endpoint` (no-endpoint) | 1 |
| **total** | **206** |

**Exact tier-assignment criterion:**

```
tier = "apt"  IF AND ONLY IF:
    (1) measured_verdict == "real-ejecutada"
    (2) mock != true
    (3) the response declares NO shortfall (no reason= for missing
        infrastructure, no status=error, no error= with text)
tier = "estandar"  in every other case
real  = (measured_verdict != "simulada")
```

Clause (3) kept 10 capabilities out of APT: they answered HTTP 200 but declared `error=` or `reason=` (a plan, a
campaign id, or Postgres was missing). They were `api.ia.campaign`, `api.ia.campaigns.{campaign_id}`,
`api.ia.campaigns.{campaign_id}.abort`, `api.ia.campaigns.{campaign_id}.run`, `api.ia.modules.{key}`,
`api.ia.strategy.{proposal_id}.review`, `api.search.pipeline`, `api.search.sources`, `api.pipeline` and
`api.pipeline.status`.

### 3.2 · Current update: simulated capabilities became real

> **Current state:** `CAPACIDADES.md` is outdated. The **30 capabilities** its «SIMULATED» block declared as
> mocks/stubs (`mock=true`, `implementation="stub"`, `implementation="partial"`) **are now implemented as real and
> do the work**. The "simulated" category is therefore **closed** in the current state: the inventory no longer
> contains capabilities that declare themselves as simulated.

The 30 affected capabilities (and the endpoint exposing them) were:

| Capability | Endpoint |
|---|---|
| `c2.modulo.collection.BrowserHistoryCollect` | `GET /api/modules/collection` |
| `c2.modulo.collection` (aggregator) | `GET /api/modules/collection` |
| `c2.modulo.trust.SIDHistoryInjection` | `GET /api/modules/trust` |
| `c2.modulo.trust.InterRealmAbuse` | `GET /api/modules/trust` |
| `c2.modulo.trust` (aggregator) | `GET /api/modules/trust` |
| `c2.modulo.webexploits.authbypass` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.clientside` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.deserialization` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.rce` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.sqli` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.ssi` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.ssrf` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.ssti` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits.xxe` | `GET /api/modules/webexploits` |
| `c2.modulo.webexploits` (aggregator) | `GET /api/modules/webexploits` |
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

**Maintenance note:** to honor the guiding principle (§6), this change must be reflected by regenerating the
inventory with `python tools/capability_matrix.py`; `--check` fails if the document is still stale.

### 3.3 · Capability map by area (verified in the baseline)

| Area | Examples of real capabilities measured |
|---|---|
| **C2 modules — discovery / collection / credentials** | `discovery.ADTrusts`, `discovery.GPOEnumeration`, `discovery.CloudAzureEnumeration`, `discovery.CloudAWSEnumeration`, `collection.StageFiles`, `collection.EncryptFile`, `trust.TrustEnumeration`, `creds.PrimaryTokenTheft` (token theft/duplication, impersonation, service account) |
| **C2 modules — governed simulation** | `dect-sql.Probe/Evidence` (error-based/boolean-blind/time-based probes), `ddos-dect.Emit` (bounded L7 traffic, hard quotas, allowlist and dead-man switch), `ransomware-sim` (AES-256-GCM/ChaCha20-Poly1305, Ed25519/X25519/ML-KEM custody, SHA-384 chain, reversible), `worm-sim` (in-memory propagation over a topology), `kernel-sim` (simulated BYOVD with manifest), `privesc-auto` (vector scan ordered by CVSS), `adversary-sim` (JSONL telemetry and visible marker) |
| **Exploit engine** | `health`, `weaponize.preflight`, `autonomy.status`, `discovery`, `lab.verify`, `config`, `history`, `exploits.lista`, `gate` + `gate.post`, `sandbox.matrix`, `triage.classify`, `primitive.synthesize`, `harness.build`, `corpus.minimize`, `z3.solve`, `zeroday.plan/build`, `import.*`, `research.theorize`, `exploits.package` |
| **Python API** | `ia.*` (campaigns, generate-script, catalog, proposals, Sigma, suggest, status), `bounty.*`, `search`, `signature-evasion.analyze`, `exploitlab.cves`, `recon.*`, `weapon.*` (primitives, ROP with capstone-rs + rayon, shellcode, syscalls), `fuzzing.*` |
| **Sibling CyberLens** | `cyberlens.health`, `cyberlens.representativa` |

All governed simulation modules execute their mechanism **for real** and write evidence inside the lab, using a
lab marker (`.lab-allow`) or a signed impact policy. None of them leaves the laboratory.

### 3.4 · The agent plane

The agent plane **is not verifiable over HTTP**: there is no `GET /api/modules/agent-*`. Its capabilities are
queued as a **TASK** against a live session (`POST /api/agents/:id/tasks`). That is why the probe marks it
`no-verificable`.

- **13 modules** measured: `agent-nucleo`, `agent-ejecucion`, `agent-scripting`, `agent-reconocimiento`,
  `agent-credenciales`, `agent-ficheros`, `agent-supervision`, `agent-interaccion`, `agent-persistencia`,
  `agent-elevacion`, `agent-evasion`, `agent-ransomware`, `agent-worm`.
- **69 capabilities** declared in the catalog (`server/api/module_catalog.go`).
- **What it executes:** code on the victim host with persistent effects (implant state change, scripts, files,
  persistence, evasion). The probe **does not start an agent on the operator's machine**.

### 3.5 · What the current lab does NOT allow

| VM | OS | Network | State |
|---|---|---|---|
| `test` | WS2025 | NAT | poweroff |
| `win11-eval` | Win11 | host-only | saved |
| `lab-win11-01` | Win11 | NAT | poweroff |

- The vulnerable Win7 targets (`192.168.56.10`, `192.168.56.11`) **do not exist**: they are listed as
  `pendiente-ISO` (ISO pending); they do not answer SMB.
- `verified_lab` is **never granted** in this installation: validator B4 validates against those non-existent IPs,
  all runs are excluded and the canonical rule `verified_lab_rule.py` denies the scope.
- Tools: `unicorn` 2.1.4 ✔, `docker_daemon` running ✔, image `c2dx/afl-lab:latest` present ✔,
  `afl-fuzz` is **not** in PATH.
- Prerequisites still pending in the baseline: Postgres (`KILL_SWITCH_DB_URL`/`POSTGRES_URL`), Docker image
  reachable from the engine, a deployed lab VM, BloodHound, nmap, native binaries (`lateral-rust` builds and passes
  246 tests; `weapon-rs` has no tests; `ultra` does not build), a signed impact policy and a connected AI backend.

---

## 4 · Editions and licensing

### 4.1 · Two products

| | **Product A — Standard Edition** | **Product B — APT Edition** |
|---|---|---|
| Name | Adversary Emulation Platform + Integrated C2 | Restricted Authorized Offensive Framework |
| Audience | MSSP, red teams, detection labs | Government and contractors with verified signatures; **not sold via web** |
| Scope | Measurable emulation with limited capabilities | Evasion, movement/post-exploitation, destruction and extreme lateral movement |
| Requirement | Emulation license `C2DECT-EMU-` | Offensive license `C2DECT-OFF-` + **verified KYC** |
| GUI theme | Cyan | Red |

### 4.2 · Executables per edition

Each edition has **3 separate executables** (server, agent, GUI client) built with specific build tags. This
enables smaller binaries, less attack surface per component, independent licenses per type and easier auditing /
certification.

| Edition | Server (with GUI) | Agent (no GUI) | GUI client |
|---|---|---|---|
| **Standard** | `dist/c2-server-standard.exe` | `dist/agent-standard.exe` | `dist/C2Dect-Standard.exe` |
| **APT** | `dist/c2-server-apt.exe` | `dist/agent-apt.exe` | `dist/C2Dect-APT.exe` |

- **Server:** integrated WPF GUI to control the server, watch live logs and configure parameters.
- **Agent:** no GUI (silent executable, designed for stealth operations).
- **GUI client:** full operator WPF GUI (multi-tenancy, orchestration, exploit lab, etc.).
- **Icon:** all executables embed the worm icon (`gui/C2Dect/Assets/C2-DECT WORM.ico`).

### 4.3 · Build scripts

| Script | Purpose | Build tag |
|---|---|---|
| `build-standard-server.ps1` | Standard server | No tag (default) |
| `build-apt-server.ps1` | APT server | `-tags=apt` |
| `build-standard-agent.ps1` | Standard agent | No tag (default) |
| `build-apt-agent.ps1` | APT agent | `-tags=apt` |
| `build-standard-client.ps1` | Standard client | Without `APT_EDITION` |
| `build-apt-client.ps1` | APT client | `-p:DefineConstants="APT_EDITION"` |

```posh
# Standard Edition
.\build-standard-server.ps1
.\build-standard-agent.ps1
.\build-standard-client.ps1

# APT Edition (requires authorization confirmation)
.\build-apt-server.ps1
.\build-apt-agent.ps1
.\build-apt-client.ps1
```

### 4.4 · Standard vs APT differences

| Capabilities | Standard | APT |
|---|---|---|
| **C2 Core** | ✅ Gin/HTTP, mTLS, AES-256-GCM, ECDH | ✅ Same |
| **Malleable Profiles** | ✅ | ✅ |
| **BOF** | ✅ | ✅ |
| **Scripting** | ✅ Lua + WASM | ✅ Lua + WASM |
| **Basic evasion** | ✅ AMSI bypass, ETW patch, unhook | ✅ Same |
| **Lateral movement** | ✅ WMI, PsExec, WinRM, DCOM | ✅ Same |
| **Kerberos** | ✅ Kerberoast, DCSync | ✅ Same |
| **Privilege escalation** | ✅ Privesc | ✅ Same |
| **Exploit Lab** | ✅ AFL fuzzing, weaponization | ✅ Same |
| **Recon Module** | ✅ OSINT/CVE | ✅ Same |
| **Orchestration** | ✅ Full-chain | ✅ Same |
| **Ransomware** | ❌ Not included | ✅ `ransomware.go` |
| **Worm** | ❌ Not included | ✅ `worm_deploy.go` |
| **Destructive modules** | ❌ Limited | ✅ Full |
| **Extreme lateral** | ❌ Limited | ✅ Full |
| **Multi-tenancy** | ✅ | ✅ |
| **GUI** | ✅ WPF (cyan theme) | ✅ WPF (red theme) |

### 4.5 · Licensing model

- The **edition** is restrictive: it only decides which panels the console shows and how the code is compiled.
  **The server holds the impact gate** — there is no "hidden mode" without a gate.
- License codes are `C2DECT-EMU-` (emulation / Standard) and `C2DECT-OFF-` (offensive / APT).
- APT requires **verified signatures**, written authorization confirmation in the build (`--auto-confirm`) and
  **verified KYC**; it is not distributed via the web.
- **Authenticode signing pending:** this repository has no signing credentials. Until it does, Windows will show
  "unknown publisher" and verification will rely on the `SHA256SUMS.txt` hash.
- **Online activation pending** (see §4.7): the verification endpoint exists in the server, but the external
  license service and token-based custom delivery **are not in place**.
- **Relationship with Lite:** `c2-dect-lite` is governed by the *C2-DECT Lite Non-Commercial License* — free for
  education, personal audits and authorized internal testing; commercial use of Lite requires written permission.
  The full product adds the per-edition commercial model described above.

### 4.6 · Installers (3 components × 2 editions)

> **Exists:** the installer, the packaging and the Distribution view that does everything from the console.
> **Does not exist:** online license activation against an external server (still a plan).

| Product | Server | Agent | Client (GUI) |
|---|---|---|---|
| **A — Standard** | `c2dect-standard-servidor-3.6.0.zip` | `c2dect-standard-agente-3.6.0.zip` | `c2dect-standard-cliente-3.6.0.zip` |
| **B — APT** | `c2dect-apt-servidor-3.6.0.zip` | `c2dect-apt-agente-3.6.0.zip` | `c2dect-apt-cliente-3.6.0.zip` |

Downloads: the six 3.6.1 packages are in the GitHub release at
<https://github.com/Nakano-04/c2-dect/releases/tag/v3.6.1> (each ZIP includes `SHA256SUMS.txt`).

```posh
# 1. build the variant (APT asks for written authorization)
.\build-apt-server.ps1 --auto-confirm

# 2. package (ZIP with executables, edition config and hashes)
powershell -ExecutionPolicy Bypass -File installer\build-package.ps1 `
    -Edition apt -Type servidor

# 3. install
powershell -ExecutionPolicy Bypass -File installer\install.ps1 `
    -Edition apt -Type servidor -InstallDir "C:\Program Files\C2-DECT"
```

- The installer copies the files, creates the shortcut (the console starts with `--edition=<edition>`), leaves
  `install-manifest.json` with the SHA-256 of what was installed and, with `-RegisterService`, registers the
  server as a Windows service.
- `installer/C2Dect.iss` is the Inno Setup wrapper for the graphical wizard; it calls the same `install.ps1`.
- The console's **Distribution** view performs the three steps by calling the **same** scripts, so the package is
  identical to the terminal one. Full detail in `docs/DISTRIBUCION.md`.

### 4.7 · License backend (pending, `server/api/license.go`)

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/license/verify` | POST | Validate a code against the DB |
| `/api/license/install` | POST | Register an installation and return a `buildID` |
| `/api/license/download` | GET (token) | Serve the custom-built installer |
| `/api/license/revoke` | POST (admin) | Revoke a code (commercial kill switch) |

### 4.8 · Commercial timeline

| Product A phase | Product B phase | Estimate |
|---|---|---|
| A1 — Commercial foundation (weeks 1-4) | — | 4 weeks |
| A2 — Measurable emulation (weeks 5-10) | B1 design | 6 weeks |
| A3 — Enterprise (weeks 11-14) | B1 on a separate branch | 4 weeks |
| A4 — A v1.0 launch (weeks 15-18) | — | 4 weeks |
| Support and hardening A | B2 — Evasion (weeks +5 to +12) | — |
| A v1.1 | B3 — Movement / post-exploitation | — |
| A LTS | B4 — Legal + licenses | — |

**Total:** ~12 months with 3-5 engineers. With 2, add 50%.

---

## 5 · Impact governance

Every offensive capability is governed fail-closed:

- **Kill switch** and **signed impact policy** per tenant (`allow_impact_modules`).
- Declared **campaign date**, **scale** and **time ceiling**.
- **Operator acknowledgement** before impact is queued.
- **Lab marker** `.lab-allow` for simulation modules.
- Simulators never leave the lab: the scope is fixed by the server; they accept no parameter to request the parent
  folder or the root. Examples: `ransomware-sim` rejects double encryption and keeps the key wrapped (never in
  cleartext); `ddos-dect` does no amplification, raw L4, IP spoofing or botnets; `worm-sim` has no network or
  payload; `kernel-sim` loads no real drivers; `adversary-sim` only generates telemetry marked as intent.

> **Pending decision (N3):** `worm_deploy.go`, `ransomware.go` and `shadowmesh-*` exist and are not excluded from
> the offensive build; they require a formal governance decision.

---

## 6 · Guiding principle

**No exit criterion without an executable assertion.** If it cannot be checked with a command or a test, it is not a
criterion. "The tests pass" is not a criterion; the criterion is **the command, the expected output and the number**.

---

## 7 · Security and warnings

> **WARNING:** for authorized security testing and Red Team operations only.
> Do not operate against third-party assets without current written authorization.
> Worm/ransomware: governance decision pending (§5).

- Source code documentation: `docs/CODIGO-FUENTE.md`
- Code audit: `docs/AUDITORIA-CODIGO.md`
- Commercial roadmap (the only current one): `ROADMAP_COMERCIAL.md`
- Previous roadmaps (archived): `docs/_archive/roadmaps_2026-11/`

---

## 8 · Installation and build

### 8.1 · Prerequisites

- **Go 1.21+** — <https://go.dev/dl/>
- **.NET 10 SDK** — <https://dotnet.microsoft.com/download/dotnet/10.0>

### 8.2 · Automatic dependency installation

```posh
.\install-dependencies.ps1
```

It checks Go and .NET, downloads the Go dependencies of the server/agent/c2-protocol and the .NET dependencies of
the GUIs, and installs `go-winres` for embedding icons. The build scripts verify and install dependencies
automatically before compiling.

### 8.3 · Build

```posh
# Standard Edition
.\build-standard-server.ps1   # Server with GUI
.\build-standard-agent.ps1    # Agent (no GUI)
.\build-standard-client.ps1   # GUI client

# APT Edition (requires authorization confirmation)
.\build-apt-server.ps1
.\build-apt-agent.ps1
.\build-apt-client.ps1
```

The executables are generated in `dist/` (see §4.2).

---

## 9 · Quick verification

```bash
# Go artifacts
cd c2-protocol && go build ./...
cd ../server && go build ./...
cd ../cmd/agent_go && go build ./...

# Rotation and E2E
go test -vet=off -run 'TestRotator\|TestRotationState\|TestSetEncoding\|TestGetProfile' ./...
go test -vet=off -run 'TestE2E_CheckinTaskResult' ./server/api
go test -vet=off -run 'TestStoreAndGet\|TestKeyRotation\|TestEncryptedCheckinCarriesProfile' ./server/...

# Capability inventory
python tools/probe_capabilities.py          # re-measure
python tools/capability_matrix.py           # regenerate capabilities.json and CAPACIDADES.md
python tools/capability_matrix.py --check   # exits 1 if the doc is stale
python -m pytest tests/test_capability_matrix.py -q
```

---

## 10 · Next 10 days (operations)

1. **Days 1-2:** build the WPF GUI (`dotnet build gui\C2Dect\C2Dect.csproj -c Release`), generate `C2Dect.exe` and run `.\gui\fix-gui-shortcut.ps1`.
2. **Day 3:** start the GUI; verify it opens `c2-server.exe` on 8443 automatically.
3. **Day 4:** generate an agent with toggles (Lab Mode + Sleepmask Forced) from the Builder; verify `sleepmaskForced=1` in the log and `.lab-allow` next to the binary.
4. **Day 5:** test the agent against local Defender (with `--insecure` or `.lab-allow`); document what it detects.
5. **Days 6-7:** create `server/api/license.go` with the 4 license endpoints; create the `licenses` DB.
6. **Days 8-10:** create minimal web wizard assistants (code form → custom installer download).

---

## 11 · Project structure

```
c2-dect/
├── delivery/                    # 1-click delivery (initial access)
├── cmd/
│   ├── agent_go/                # agent (Go) - real deployment
│   ├── agent/                   # legacy TCP agent (deprecated, lab)
│   ├── server/                  # legacy TCP server (deprecated, lab)
│   └── recon-module/            # OSINT, scanning, CVE
├── exploits/                    # weaponization, AFL fuzzing, lab validation
├── modules/
│   ├── lateral-rust/            # WMI, PsExec, WinRM, DCOM
│   └── privesc-auto/            # governed escalation
├── server/                      # Gin/HTTP C2: REST API + WebSocket
│   ├── api/                     # handlers, module catalog, licensing
│   └── gui/                     # integrated server WPF GUI
├── c2-protocol/                 # encryption: AES-256-GCM + ECDH, malleable profiles
├── orchestration/full_chain.py  # chains the 6 phases (dry_run=True by default)
├── relases_news/dect-cookie-hijacking/  # session replay, vault, CSRF, HAR
├── gui/C2Dect/                  # operator console (WPF REST client)
├── installer/                   # install.ps1, build-package.ps1, C2Dect.iss
├── docs/                        # DISTRIBUCION, CODIGO-FUENTE, AUDITORIA-CODIGO, POSTGRES...
├── tools/                       # capability_matrix.py, probe_capabilities.py
├── build-*.ps1                  # 6 build scripts (3 components × 2 editions)
├── install-dependencies.ps1
├── CAPACIDADES.md               # baseline inventory (2026-10-30)
├── README.md                    # original v3.6.1 documentation
├── ROADMAP_COMERCIAL.md
├── c2-dect.md                   # this document (English): current state + licensing
└── c2-dect.es.md                # Spanish translation
```

---

## 12 · License (summary)

The full product is distributed under a **commercial per-edition license**: `C2DECT-EMU-` (Standard, emulation) and
`C2DECT-OFF-` (APT, offensive, with KYC and verified signatures). Online activation and Authenticode signing are
**pending** (§4.5, §4.7; gap N5 and section §2.2). Use remains subject to written authorization and to the impact
governance described in §5.

---

## 13 · Version history

- **v3.6.1** — 2026-11-16 — current version (this document).
- **v3.5.1-beta** — 2026-10-20 — previous beta release, archived in `docs/README_v3.5.1-beta-2026-10-26.md`.
- All earlier versions are in `docs/` and `docs/_archive/`.
