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
- **Go** (version 1.18 or higher) to compile the backend.
- **.NET SDK** (version 6.0 or higher) to compile the agent and the GUI.
- **Supported OS**: Windows, Linux, or macOS for the server. The agent and GUI are Windows-only.

**Why do I get a "connection refused" error when running the agent?**
Ensure the server is running and that the port (default 8080) is open in your server's firewall. Also, double-check that the IP address and port were correctly specified during the agent compilation.

## Usage

**How do I generate an agent?**
You must compile the `agent` project with your server parameters:
```bash
cd agent
dotnet build -c Release -p:ServerIP=192.168.1.100 -p:ServerPort=8080