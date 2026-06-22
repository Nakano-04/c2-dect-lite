# C2-DECT Lite

> **Open Source C2 Framework for Authorized Security Testing**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## ⚠️ Disclaimer

**This tool is for authorized security testing and educational purposes only.**

- Only use on systems you own or have explicit written permission to test
- Unauthorized access to computer systems is illegal
- The authors are not responsible for misuse of this software

## Features

### Lite Version (Free, Open Source)

- **Server**: REST API with JWT authentication, SQLite database
- **Agent**: 18 commands for basic system interaction
- **GUI**: WPF-based operator console
- **Encryption**: ECDH key exchange + AES-256-GCM

### Supported Commands

| Category | Commands |
|----------|----------|
| **System** | `sysinfo`, `netinfo`, `ps`, `kill`, `services`, `env`, `connections` |
| **Files** | `ls`, `cat`, `upload`, `download`, `cp`, `mv`, `rm`, `mkdir`, `find` |
| **Shell** | `shell <command>` |
| **Agent** | `sleep`, `exit` |

## Quick Start

### Prerequisites

- Go 1.21+
- .NET 8 SDK (for GUI)

### 1. Start Server

```bash
cd server
go build -o ../build/server.exe ./cmd/main.go
cd ../build
./server.exe
```

**Default credentials:** `admin` / `c2-dect`

### 2. Start Agent

```bash
cd agent
go build -o ../build/agent.exe .
cd ../build
./agent.exe -s 127.0.0.1 -p 8443
```

### 3. Open GUI

```bash
cd gui/C2Dect
dotnet build
dotnet run
```

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/login` | Authenticate operator |
| GET | `/api/sessions` | List all sessions |
| POST | `/api/sessions/:id/task` | Submit command to agent |
| GET | `/api/sessions/:id/tasks` | Get task history |
| POST | `/agent/checkin` | Agent heartbeat |

## Project Structure

```
c2-dect-lite/
├── server/           # Go server (REST API, JWT, SQLite)
├── agent/            # Go agent (18 commands)
├── gui/              # C# WPF GUI
├── build/            # Compiled binaries
├── LICENSE           # MIT License
└── README.md         # This file
```

## Want More Features?

**C2-DECT Pro** includes advanced capabilities:

- 🌐 **P2P Mesh Network** - Distributed agent communication
- 🎭 **Process Hollowing** - Advanced injection techniques
- 🔀 **Polymorphic Engine** - Traffic and binary obfuscation
- 🤖 **AI Assistant** - Ollama/Llama integration for tactical advice
- 💥 **Exploit Engine** - EternalBlue, Log4Shell, BlueKeep, and more
- 🔄 **Lateral Movement** - PsExec, WMI, WinRM, DCOM, SMB Relay
- 🔑 **Credential Access** - LSASS dump, Kerberos attacks, DPAPI
- 🛡️ **Advanced Evasion** - AMSI/ETW bypass, VM detection, PatchGuard bypass
- 📡 **Multiple Protocols** - DNS, WebSocket, SMB, SOCKS5 listeners
- 🎯 **Kill Switch** - 4-level agent termination control

### Contact for Pro Version

For inquiries about C2-DECT Pro with all advanced features:

- **Email:** nakano.04.2025@gmail.com
- **Instagram:** @ruiso_nakano

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Built with Go, C# WPF
- Inspired by leading C2 frameworks in the security community
- Create for @Nakano_04 and @AngelaLuna14
