// Package token issues and validates signed edge WSS tokens (EDGE_FS.md).
package token

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims carried in the edge token.
type Claims struct {
	TenantID  string `json:"tenant_id"`
	SessionID string `json:"session_id,omitempty"`
	ProfileID string `json:"profile_id,omitempty"`
	Exp       int64  `json:"exp"`
}

var (
	ErrInvalid = errors.New("invalid edge token")
	ErrExpired = errors.New("edge token expired")
)

// Issue HMAC-SHA256 signs claims as base64url(payload).base64url(sig).
func Issue(secret []byte, c Claims, ttl time.Duration) (string, error) {
	if len(secret) == 0 {
		return "", fmt.Errorf("edge token secret required")
	}
	if c.SessionID == "" && c.ProfileID == "" {
		return "", fmt.Errorf("session_id or profile_id required")
	}
	if c.Exp == 0 {
		c.Exp = time.Now().Add(ttl).Unix()
	}
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	pEnc := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(pEnc))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return pEnc + "." + sig, nil
}

// Validate checks HMAC and expiry. now may be zero (uses time.Now).
func Validate(secret []byte, raw string, now time.Time) (Claims, error) {
	if len(secret) == 0 || raw == "" {
		return Claims{}, ErrInvalid
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return Claims{}, ErrInvalid
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(parts[0]))
	want := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(want, got) {
		return Claims{}, ErrInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalid
	}
	var c Claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return Claims{}, ErrInvalid
	}
	if now.IsZero() {
		now = time.Now()
	}
	if c.Exp > 0 && now.Unix() > c.Exp {
		return Claims{}, ErrExpired
	}
	if c.SessionID == "" && c.ProfileID == "" {
		return Claims{}, ErrInvalid
	}
	return c, nil
}
