package unsubscribetoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("unsubscribe token invalid")
	ErrTokenExpired = errors.New("unsubscribe token expired")
)

type Payload struct {
	WorkspaceID              string `json:"ws"`
	MessageID                string `json:"msg"`
	RecipientEmailNormalized string `json:"rem"`
	Scope                    string `json:"sc"`
	ExpiresAt                int64  `json:"exp"`
}

type Signer struct {
	secret []byte
}

func NewSigner(secret string) *Signer {
	return &Signer{secret: []byte(secret)}
}

func (s *Signer) Sign(payload Payload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	encoded := base64.RawURLEncoding.EncodeToString(data)

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(encoded))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return encoded + "." + signature, nil
}

func (s *Signer) Verify(token string) (*Payload, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	encoded := parts[0]
	signature := parts[1]

	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(encoded))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return nil, ErrInvalidToken
	}

	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var payload Payload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, ErrInvalidToken
	}

	if payload.ExpiresAt > 0 && time.Now().Unix() > payload.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return &payload, nil
}
