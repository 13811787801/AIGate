package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// DevPublicKeyB64 is replaced in release builds with vendor public key.
// Paired private key MUST never ship to customers. Leave license.file empty to run without one.
const DevPublicKeyB64 = "" // empty => license file optional in dev

type Payload struct {
	Customer  string          `json:"customer"`
	ExpiresAt string          `json:"expires_at"` // RFC3339; empty = never
	Modules   map[string]bool `json:"modules"`
	MaxUsers  int             `json:"max_users"`
	IssuedAt  string          `json:"issued_at"`
}

type File struct {
	Payload   Payload `json:"payload"`
	Signature string  `json:"signature"` // base64 ed25519 sig over canonical payload JSON
}

type State struct {
	Loaded   bool
	Valid    bool
	Reason   string
	Payload  Payload
	Ceiling  map[string]bool
	MaxUsers int
}

func Load(path, publicKeyB64 string) State {
	st := State{Loaded: false, Valid: true, Reason: "no license file (dev/open mode)", Ceiling: nil, MaxUsers: 0}
	if strings.TrimSpace(path) == "" {
		return st
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st
		}
		return State{Loaded: true, Valid: false, Reason: "cannot read license: " + err.Error()}
	}
	st.Loaded = true
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return State{Loaded: true, Valid: false, Reason: "bad license json"}
	}
	pubB64 := strings.TrimSpace(publicKeyB64)
	if pubB64 == "" {
		pubB64 = DevPublicKeyB64
	}
	if pubB64 == "" {
		// No public key configured: accept payload without crypto (NOT for production).
		st.Valid = true
		st.Reason = "license loaded without signature verify (configure license.public_key_b64)"
		st.Payload = f.Payload
		st.Ceiling = f.Payload.Modules
		st.MaxUsers = f.Payload.MaxUsers
		if expired(f.Payload.ExpiresAt) {
			return State{Loaded: true, Valid: false, Reason: "license expired", Payload: f.Payload}
		}
		return st
	}
	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return State{Loaded: true, Valid: false, Reason: "invalid public key"}
	}
	canon, err := json.Marshal(f.Payload)
	if err != nil {
		return State{Loaded: true, Valid: false, Reason: "payload marshal failed"}
	}
	sig, err := base64.StdEncoding.DecodeString(f.Signature)
	if err != nil {
		return State{Loaded: true, Valid: false, Reason: "bad signature encoding"}
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), canon, sig) {
		return State{Loaded: true, Valid: false, Reason: "license signature invalid"}
	}
	if expired(f.Payload.ExpiresAt) {
		return State{Loaded: true, Valid: false, Reason: "license expired", Payload: f.Payload}
	}
	st.Valid = true
	st.Reason = "ok"
	st.Payload = f.Payload
	st.Ceiling = f.Payload.Modules
	st.MaxUsers = f.Payload.MaxUsers
	return st
}

func expired(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return true
	}
	return time.Now().After(t)
}

// Sign creates a license file bytes (vendor-side tool).
func Sign(priv ed25519.PrivateKey, p Payload) ([]byte, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("bad private key")
	}
	if p.IssuedAt == "" {
		p.IssuedAt = time.Now().UTC().Format(time.RFC3339)
	}
	canon, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(priv, canon)
	f := File{Payload: p, Signature: base64.StdEncoding.EncodeToString(sig)}
	return json.MarshalIndent(f, "", "  ")
}

func GenerateKeyPair() (pubB64, privB64 string, err error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(pub), base64.StdEncoding.EncodeToString(priv), nil
}

func ParsePrivateKey(b64 string) (ed25519.PrivateKey, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, err
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("want %d bytes private key", ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(b), nil
}
