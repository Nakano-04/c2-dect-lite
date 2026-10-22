// Package crypto implements the C2-DECT Lite 2.0 transport envelope.
//
// Wire format:
//
//	key exchange : ECDH P-256, agent sends its public key, server answers with its own
//	key derivation: HKDF-SHA256(shared_secret, salt=session_id, info="c2-dect-lite v1 aes")
//	body         : AES-256-GCM, ciphertext = nonce || sealed, base64 encoded in JSON
//
// The envelope is symmetric: both sides derive the same key from the same
// ECDH shared secret, so no key material ever crosses the wire.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	// KDFInfo binds the derived key to this protocol version.
	KDFInfo = "c2-dect-lite v1 aes"
	// KeyLen is the AES-256 key size in bytes.
	KeyLen = 32
	// NonceLen is the AES-GCM nonce size in bytes.
	NonceLen = 12
)

var (
	// ErrBadEnvelope is returned when an envelope cannot be decoded.
	ErrBadEnvelope = errors.New("crypto: malformed envelope")
	// ErrAuth is returned when the GCM authentication tag does not verify.
	ErrAuth = errors.New("crypto: authentication failed")
)

// Curve is the ECDH group used by the key exchange.
func Curve() ecdh.Curve { return ecdh.P256() }

// GenerateKeyPair creates a new ECDH P-256 key pair.
func GenerateKeyPair() (*ecdh.PrivateKey, error) {
	return Curve().GenerateKey(cryptorand.Reader)
}

// PublicKeyHex renders a public key as lowercase hex.
func PublicKeyHex(pub *ecdh.PublicKey) string {
	if pub == nil {
		return ""
	}
	return hex.EncodeToString(pub.Bytes())
}

// ParsePublicKey decodes a hex encoded P-256 public key.
func ParsePublicKey(raw string) (*ecdh.PublicKey, error) {
	b, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("crypto: bad public key encoding: %w", err)
	}
	pub, err := Curve().NewPublicKey(b)
	if err != nil {
		return nil, fmt.Errorf("crypto: bad public key: %w", err)
	}
	return pub, nil
}

// DeriveKey performs the ECDH exchange and stretches the shared secret with
// HKDF-SHA256. Both agent and server use the exact same inputs, so the
// resulting keys are identical on both ends.
func DeriveKey(priv *ecdh.PrivateKey, peerPub *ecdh.PublicKey, sessionID string) ([]byte, error) {
	if priv == nil || peerPub == nil {
		return nil, ErrBadEnvelope
	}
	shared, err := priv.ECDH(peerPub)
	if err != nil {
		return nil, fmt.Errorf("crypto: ecdh failed: %w", err)
	}
	reader := hkdf.New(sha256.New, shared, []byte(sessionID), []byte(KDFInfo))
	key := make([]byte, KeyLen)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("crypto: hkdf failed: %w", err)
	}
	return key, nil
}

// Seal encrypts plaintext with AES-256-GCM and returns base64(nonce||ciphertext).
func Seal(key, plaintext []byte) (string, error) {
	if len(key) != KeyLen {
		return "", fmt.Errorf("crypto: key must be %d bytes", KeyLen)
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
	if _, err := io.ReadFull(cryptorand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Open decrypts a base64(nonce||ciphertext) payload produced by Seal.
func Open(key []byte, encoded string) ([]byte, error) {
	if len(key) != KeyLen {
		return nil, fmt.Errorf("crypto: key must be %d bytes", KeyLen)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: bad base64", ErrBadEnvelope)
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
		return nil, ErrBadEnvelope
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	out, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, ErrAuth
	}
	return out, nil
}

// Fingerprint returns a short, non secret identifier for a key. It is used for
// operational logging only and never for authentication.
func Fingerprint(key []byte) string {
	if len(key) == 0 {
		return "none"
	}
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:4])
}
