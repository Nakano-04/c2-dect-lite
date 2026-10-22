# Frequently Asked Questions (FAQ)

## General

**What is c2-dect-lite?**
It is a lightweight Command & Control (C2) framework designed for educational purposes, security research, and penetration testing in controlled environments. It allows security professionals to manage remote agents via a WPF GUI and a REST API.

**Is it completely free?**
The **Lite** version is free to use **exclusively for educational purposes, personal audits, and internal security testing**, strictly subject to the project's custom license. Commercial use of the Lite version without explicit permission is prohibited.

**What is the difference between the Lite and Pro versions?**
The **Pro** version is a commercial, paid edition aimed at professional teams and enterprise use. It includes advanced features not available in Lite, such as:

- **SOCKS5 Tunneling**: Route traffic through agents (pivoting).
- **Interactive File Browser**: Real-time remote filesystem exploration.
- **Advanced Persistence Modules**: Scheduled Tasks, WMI, and Registry persistence.
- **AV/EDR Evasion**: Dynamic obfuscation and polymorphic agent generation.
- **Multiple Output Formats**: Generate agents as PowerShell, HTA, VBS, and MSI.
- **Priority Support**: Direct assistance and deployment consulting.

## Installation & Requirements

**What are the prerequisites to run the server?**
- **Go** (version 1.21 or higher) to compile the server and the agent.
- **.NET SDK** (version 8.0 or higher) to compile the console.
- **Supported OS**: Windows, Linux, or macOS for the server and the agent. The
  console is Windows-only (WPF).

**The server refuses to start. What am I missing?**
`C2_JWT_SECRET` is mandatory: set it to a random string of at least 32
characters. The server has no built-in signing key on purpose.

**Why do I get a "connection refused" error when running the agent?**
Ensure the server is running and that the port (default 8443) is open in your
server's firewall. The agent takes the address at runtime, not at compile time:
`agent.exe -s 192.168.1.100 -p 8443`.

## Usage

**How do I build the agent?**
The agent is a Go program, built from source, and it takes the server address
at runtime:
```bash
cd agent
go build -ldflags="-s -w" -o ../build/agent.exe .
cd ../build
./agent.exe -s 192.168.1.100 -p 8443
```

**How do I run signed scripts?**
Put the marketplace public key that the server prints at startup in the
`C2_SCRIPT_PUBKEY` environment variable before starting the agent. Without it
the scripting runtime stays disabled and unsigned scripts are never executed.