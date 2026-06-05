package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type HTTPProvider struct {
	name         string
	typ          string
	displayName  string
	clientID     string
	clientSecret string
	authURL      string
	tokenURL     string
	userURL      string
	scopes       []string
	httpClient   *http.Client
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

func NewGoogleProvider(clientID, clientSecret string) *HTTPProvider {
	return &HTTPProvider{name: "google", typ: "oidc", displayName: "Google", clientID: clientID, clientSecret: clientSecret, authURL: "https://accounts.google.com/o/oauth2/v2/auth", tokenURL: "https://oauth2.googleapis.com/token", userURL: "https://openidconnect.googleapis.com/v1/userinfo", scopes: []string{"openid", "email", "profile"}, httpClient: &http.Client{Timeout: 10 * time.Second}}
}
func NewGithubProvider(clientID, clientSecret string) *HTTPProvider {
	return &HTTPProvider{name: "github", typ: "oauth2", displayName: "GitHub", clientID: clientID, clientSecret: clientSecret, authURL: "https://github.com/login/oauth/authorize", tokenURL: "https://github.com/login/oauth/access_token", userURL: "https://api.github.com/user", scopes: []string{"read:user", "user:email"}, httpClient: &http.Client{Timeout: 10 * time.Second}}
}

func (p *HTTPProvider) Name() string        { return p.name }
func (p *HTTPProvider) Type() string        { return p.typ }
func (p *HTTPProvider) DisplayName() string { return p.displayName }
func (p *HTTPProvider) Enabled() bool       { return p.clientID != "" && p.clientSecret != "" }

func (p *HTTPProvider) BuildAuthURL(state, redirectURI, codeChallenge string) string {
	q := url.Values{}
	q.Set("client_id", p.clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(p.scopes, " "))
	q.Set("state", state)
	if codeChallenge != "" {
		q.Set("code_challenge", codeChallenge)
		q.Set("code_challenge_method", "S256")
	}
	return p.authURL + "?" + q.Encode()
}

func (p *HTTPProvider) Exchange(ctx context.Context, code, redirectURI, codeVerifier string) (*domain.OAuthIdentity, error) {
	form := url.Values{}
	form.Set("client_id", p.clientID)
	form.Set("client_secret", p.clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	if p.name == "google" {
		form.Set("grant_type", "authorization_code")
	}
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if p.name == "github" {
		req.Header.Set("Accept", "application/json")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, errors.New("oauth token exchange failed")
	}
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}
	if token.AccessToken == "" {
		return nil, errors.New("empty access token")
	}
	return p.loadIdentity(ctx, token.AccessToken)
}

func (p *HTTPProvider) loadIdentity(ctx context.Context, accessToken string) (*domain.OAuthIdentity, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, p.userURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oauth userinfo failed: %d", resp.StatusCode)
	}
	if p.name == "google" {
		var u struct {
			Sub           string `json:"sub"`
			Email         string `json:"email"`
			EmailVerified bool   `json:"email_verified"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
			return nil, err
		}
		return &domain.OAuthIdentity{Provider: p.name, ProviderUserID: u.Sub, Email: u.Email, EmailVerified: u.EmailVerified}, nil
	}
	var gh struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gh); err != nil {
		return nil, err
	}
	if gh.Email == "" {
		email, err := p.loadGithubPrimaryEmail(ctx, accessToken)
		if err != nil {
			return nil, err
		}
		gh.Email = email
	}
	return &domain.OAuthIdentity{Provider: p.name, ProviderUserID: strconv.FormatInt(gh.ID, 10), Email: gh.Email, EmailVerified: true}, nil
}

func (p *HTTPProvider) loadGithubPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("github emails failed: %d", resp.StatusCode)
	}
	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	for _, email := range emails {
		if email.Primary && email.Verified && email.Email != "" {
			return email.Email, nil
		}
	}
	for _, email := range emails {
		if email.Verified && email.Email != "" {
			return email.Email, nil
		}
	}
	return "", errors.New("github verified email missing")
}
