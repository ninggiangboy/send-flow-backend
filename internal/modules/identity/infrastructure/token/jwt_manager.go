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

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type JWTManager struct {
	issuer        string
	accessSecret  string
	refreshSecret string
	accessTTL     time.Duration
	refreshTTL    time.Duration
}

func NewJWTManager(issuer, accessSecret, refreshSecret string, accessTTL, refreshTTL time.Duration) *JWTManager {
	return &JWTManager{issuer: issuer, accessSecret: accessSecret, refreshSecret: refreshSecret, accessTTL: accessTTL, refreshTTL: refreshTTL}
}

func (m *JWTManager) Issue(userID, sessionID string, now time.Time) (ports.TokenPair, string, string, error) {
	accessJTI := id.Must(id.NewUUIDGenerator())
	refreshJTI := id.Must(id.NewUUIDGenerator())
	accessExp := now.Add(m.accessTTL)
	refreshExp := now.Add(m.refreshTTL)
	access, err := m.sign(ports.AccessClaims{Subject: userID, SessionID: sessionID, JWTID: accessJTI, Issuer: m.issuer, ExpiresAt: accessExp.Unix(), IssuedAt: now.Unix(), Type: "access"}, m.accessSecret)
	if err != nil {
		return ports.TokenPair{}, "", "", err
	}
	refresh, err := m.sign(ports.AccessClaims{Subject: userID, SessionID: sessionID, JWTID: refreshJTI, Issuer: m.issuer, ExpiresAt: refreshExp.Unix(), IssuedAt: now.Unix(), Type: "refresh"}, m.refreshSecret)
	if err != nil {
		return ports.TokenPair{}, "", "", err
	}
	return ports.TokenPair{AccessToken: access, RefreshToken: refresh, AccessExpiresAt: accessExp, RefreshExpiresAt: refreshExp}, accessJTI, refreshJTI, nil
}

func (m *JWTManager) ParseAccess(token string) (*ports.AccessClaims, error) {
	claims, err := m.parse(token, m.accessSecret)
	if err != nil {
		return nil, err
	}
	if claims.Type != "access" {
		return nil, errors.New("invalid token type")
	}
	if time.Now().UTC().Unix() > claims.ExpiresAt {
		return nil, errors.New("token expired")
	}
	return claims, nil
}

func (m *JWTManager) ParseRefresh(token string) (*ports.AccessClaims, error) {
	claims, err := m.parse(token, m.refreshSecret)
	if err != nil {
		return nil, err
	}
	if claims.Type != "refresh" {
		return nil, errors.New("invalid token type")
	}
	if time.Now().UTC().Unix() > claims.ExpiresAt {
		return nil, errors.New("token expired")
	}
	return claims, nil
}

func (m *JWTManager) sign(claims ports.AccessClaims, secret string) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	h, _ := json.Marshal(header)
	p, _ := json.Marshal(claims)
	left := base64.RawURLEncoding.EncodeToString(h)
	right := base64.RawURLEncoding.EncodeToString(p)
	unsigned := left + "." + right
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}

func (m *JWTManager) parse(token, secret string) (*ports.AccessClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}
	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(unsigned))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims ports.AccessClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse payload: %w", err)
	}
	return &claims, nil
}
