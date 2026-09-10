// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package codex

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	sensiblebrowser "github.com/ernestrc/sensible/browser"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/rune/internal/llm/authcallback"
)

const (
	credentialDocumentID = "provider:codex:credential"

	oauthAuthURL     = "https://auth.openai.com/oauth/authorize"
	oauthTokenURL    = "https://auth.openai.com/oauth/token"
	oauthClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	defaultCallback  = ":1455"
	defaultRedirect  = "http://localhost:1455/auth/callback"
	defaultLoginWait = 5 * time.Minute
	refreshLead      = 5 * time.Minute
	codexOriginator  = "codex_cli_rs"
)

// ErrCredentialNotFound is returned when no Codex credential has been saved.
var ErrCredentialNotFound = errors.New("codex credential not found")

// Credential is the token data needed to authenticate Codex API calls.
type Credential struct {
	AccessToken    string
	RefreshToken   string
	IDToken        string
	TokenType      string
	Expiry         time.Time
	LastRefresh    time.Time
	Email          string
	AccountID      string
	PlanType       string
	UserID         string
	FedRAMP        bool
	InstallationID string
}

// AuthStatus describes the currently stored Codex credential, if any.
type AuthStatus struct {
	Authenticated   bool
	Expired         bool
	Email           string
	AccountID       string
	PlanType        string
	Expiry          time.Time
	LastRefresh     time.Time
	HasRefreshToken bool
}

// ClientHeaders returns Codex-specific headers inferred from the credential.
func (c Credential) ClientHeaders() map[string]string {
	headers := map[string]string{
		"originator": codexOriginator,
		"User-Agent": codexOriginator + "/0.0.0 (Rune Agent)",
	}
	if c.AccountID != "" {
		headers["ChatGPT-Account-ID"] = c.AccountID
	}
	if c.FedRAMP {
		headers["X-OpenAI-Fedramp"] = "true"
	}
	return headers
}

func (c Credential) expired(now time.Time) bool {
	return !c.Expiry.IsZero() && !now.Before(c.Expiry)
}

func (c Credential) needsRefresh(now time.Time) bool {
	return !c.Expiry.IsZero() && !now.Add(refreshLead).Before(c.Expiry)
}

// SaveCredential stores the Codex credential in extension storage.
func SaveCredential(ctx context.Context, storage storageapi.Service, cred Credential) error {
	if storage == nil {
		return errors.New("codex credential storage is unavailable")
	}
	if cred.InstallationID == "" {
		id, err := randomUUID()
		if err != nil {
			return fmt.Errorf("generate Codex installation ID: %w", err)
		}
		cred.InstallationID = id
	}
	return storage.Set(ctx, credentialDocumentID, credentialRecordFromPublic(cred))
}

// LoadCredential loads the stored Codex credential.
func LoadCredential(ctx context.Context, storage storageapi.Service) (Credential, error) {
	if storage == nil {
		return Credential{}, ErrCredentialNotFound
	}
	var rec credentialRecord
	if err := storage.Get(ctx, credentialDocumentID, &rec); err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			return Credential{}, ErrCredentialNotFound
		}
		return Credential{}, fmt.Errorf("load codex credential: %w", err)
	}
	return rec.public(), nil
}

// CredentialForClient returns a non-expired Codex credential, refreshing it when needed.
func CredentialForClient(ctx context.Context, storage storageapi.Service) (Credential, error) {
	cred, err := LoadCredential(ctx, storage)
	if err != nil {
		return Credential{}, err
	}
	if cred.AccessToken == "" {
		return Credential{}, errors.New("codex credential missing access token")
	}
	if !cred.needsRefresh(time.Now()) {
		return ensureInstallationID(ctx, storage, cred)
	}
	if cred.RefreshToken == "" {
		return Credential{}, errors.New("codex credential expired and has no refresh token")
	}
	refreshed, err := refreshCredential(ctx, http.DefaultClient, cred)
	if err != nil {
		return Credential{}, err
	}
	if err := SaveCredential(ctx, storage, refreshed); err != nil {
		return Credential{}, err
	}
	return ensureInstallationID(ctx, storage, refreshed)
}

func ensureInstallationID(ctx context.Context, storage storageapi.Service, cred Credential) (Credential, error) {
	if cred.InstallationID != "" {
		return cred, nil
	}
	id, err := randomUUID()
	if err != nil {
		return Credential{}, fmt.Errorf("generate Codex installation ID: %w", err)
	}
	cred.InstallationID = id
	if err := SaveCredential(ctx, storage, cred); err != nil {
		return Credential{}, err
	}
	return cred, nil
}

// Status returns the stored Codex authentication status.
func Status(ctx context.Context, storage storageapi.Service) (AuthStatus, error) {
	cred, err := LoadCredential(ctx, storage)
	if err != nil {
		if errors.Is(err, ErrCredentialNotFound) {
			return AuthStatus{}, nil
		}
		return AuthStatus{}, err
	}
	return AuthStatus{
		Authenticated:   true,
		Expired:         cred.expired(time.Now()),
		Email:           cred.Email,
		AccountID:       cred.AccountID,
		PlanType:        cred.PlanType,
		Expiry:          cred.Expiry,
		LastRefresh:     cred.LastRefresh,
		HasRefreshToken: cred.RefreshToken != "",
	}, nil
}

type credentialRecord struct {
	AccessToken    string
	RefreshToken   string
	IDToken        string
	TokenType      string
	Expiry         time.Time
	LastRefresh    time.Time
	Email          string
	AccountID      string
	PlanType       string
	UserID         string
	FedRAMP        bool
	InstallationID string
}

func credentialRecordFromPublic(cred Credential) credentialRecord {
	return credentialRecord(cred)
}

func (r credentialRecord) public() Credential {
	return Credential(r)
}

// LoginSession is an in-progress Codex OAuth login.
type LoginSession struct {
	storage     storageapi.Service
	state       string
	pkce        pkceCodes
	authURL     string
	redirectURI string
	server      *http.Server
	listener    net.Listener
	callbackCh  chan oauthCallback
	httpClient  *http.Client
	timeout     time.Duration
	browserErr  error
	closeOnce   sync.Once
	closeErr    error
}

// AuthURL returns the OAuth URL the user must visit.
func (s *LoginSession) AuthURL() string { return s.authURL }

// BrowserError returns a non-fatal browser-opening error, if one occurred.
func (s *LoginSession) BrowserError() error { return s.browserErr }

// Wait waits for the OAuth callback, exchanges the authorization code, and saves the credential.
func (s *LoginSession) Wait(ctx context.Context) (Credential, error) {
	defer func() { _ = s.Close() }()
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()

	var cb oauthCallback
	select {
	case <-ctx.Done():
		return Credential{}, ctx.Err()
	case <-timer.C:
		return Credential{}, errors.New("timed out waiting for Codex authentication callback")
	case cb = <-s.callbackCh:
	}
	if cb.err != nil {
		return Credential{}, cb.err
	}
	if cb.state != s.state {
		return Credential{}, errors.New("codex authentication state mismatch")
	}

	cred, err := exchangeCode(ctx, s.httpClient, cb.code, s.pkce.verifier, s.redirectURI)
	if err != nil {
		return Credential{}, err
	}
	if err := SaveCredential(ctx, s.storage, cred); err != nil {
		return Credential{}, err
	}
	return cred, nil
}

// Close shuts down the temporary OAuth callback server.
func (s *LoginSession) Close() error {
	s.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		s.closeErr = s.server.Shutdown(ctx)
		if errors.Is(s.closeErr, http.ErrServerClosed) {
			s.closeErr = nil
		}
	})
	return s.closeErr
}

// LoginOption configures a Codex OAuth login.
type LoginOption func(*loginConfig)

type loginConfig struct {
	callbackAddr string
	redirectURI  string
	timeout      time.Duration
	httpClient   *http.Client
	openBrowser  func(string) error
}

// WithBrowserOpener overrides the browser opener. Passing nil disables browser opening.
func WithBrowserOpener(open func(string) error) LoginOption {
	return func(c *loginConfig) { c.openBrowser = open }
}

// StartLogin starts a Codex OAuth login and returns the URL to display to the user.
func StartLogin(ctx context.Context, storage storageapi.Service, opts ...LoginOption) (*LoginSession, error) {
	if storage == nil {
		return nil, errors.New("codex credential storage is unavailable")
	}
	cfg := loginConfig{
		callbackAddr: defaultCallback,
		redirectURI:  defaultRedirect,
		timeout:      defaultLoginWait,
		httpClient:   http.DefaultClient,
		openBrowser:  openWithSensibleBrowser,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	pkce, err := generatePKCECodes()
	if err != nil {
		return nil, err
	}
	state, err := randomURLSafe(32)
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", cfg.callbackAddr)
	if err != nil {
		return nil, fmt.Errorf("start Codex OAuth callback server: %w", err)
	}

	callbackCh := make(chan oauthCallback, 1)
	session := &LoginSession{
		storage:     storage,
		state:       state,
		pkce:        pkce,
		authURL:     authURL(state, pkce.challenge, cfg.redirectURI),
		redirectURI: cfg.redirectURI,
		listener:    listener,
		callbackCh:  callbackCh,
		httpClient:  cfg.httpClient,
		timeout:     cfg.timeout,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", session.handleCallback)
	mux.HandleFunc("/success", handleSuccess)
	session.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	serveErr := make(chan error, 1)
	go func() {
		if err := session.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	select {
	case <-ctx.Done():
		_ = session.Close()
		return nil, ctx.Err()
	case err := <-serveErr:
		if err != nil {
			_ = session.Close()
			return nil, fmt.Errorf("start Codex OAuth callback server: %w", err)
		}
		_ = session.Close()
		return nil, errors.New("codex OAuth callback server stopped unexpectedly")
	default:
	}

	if cfg.openBrowser != nil {
		session.browserErr = cfg.openBrowser(session.authURL)
	}
	return session, nil
}

func authURL(state, challenge, redirectURI string) string {
	v := url.Values{}
	v.Set("client_id", oauthClientID)
	v.Set("response_type", "code")
	v.Set("redirect_uri", redirectURI)
	v.Set("scope", "openid email profile offline_access")
	v.Set("state", state)
	v.Set("code_challenge", challenge)
	v.Set("code_challenge_method", "S256")
	v.Set("prompt", "login")
	v.Set("id_token_add_organizations", "true")
	v.Set("codex_cli_simplified_flow", "true")
	return oauthAuthURL + "?" + v.Encode()
}

func openWithSensibleBrowser(rawURL string) error {
	b, err := sensiblebrowser.FindBrowser()
	if err != nil {
		return err
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	return b.Start(u)
}

type oauthCallback struct {
	code  string
	state string
	err   error
}

func (s *LoginSession) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	q := r.URL.Query()
	if oauthErr := q.Get("error"); oauthErr != "" {
		desc := q.Get("error_description")
		if desc == "" {
			desc = oauthErr
		}
		s.sendCallback(oauthCallback{err: fmt.Errorf("codex authentication failed: %s", desc)})
		http.Error(w, desc, http.StatusBadRequest)
		return
	}
	code := q.Get("code")
	state := q.Get("state")
	if code == "" || state == "" {
		s.sendCallback(oauthCallback{err: errors.New("codex authentication callback missing code or state")})
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}
	s.sendCallback(oauthCallback{code: code, state: state})
	http.Redirect(w, r, "/success", http.StatusFound)
}

func (s *LoginSession) sendCallback(cb oauthCallback) {
	select {
	case s.callbackCh <- cb:
	default:
	}
}

func handleSuccess(w http.ResponseWriter, _ *http.Request) {
	authcallback.WriteSuccess(w, authcallback.Page{
		Label:   "rune",
		Title:   "Codex authentication complete — Rune",
		Heading: "Done",
		Message: "Codex authentication complete. You can return to Rune.",
	})
}

type pkceCodes struct {
	verifier  string
	challenge string
}

func generatePKCECodes() (pkceCodes, error) {
	verifier, err := randomURLSafe(96)
	if err != nil {
		return pkceCodes{}, err
	}
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])
	return pkceCodes{verifier: verifier, challenge: challenge}, nil
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// NewSessionID returns a freshly-generated UUID v4 suitable for use as
// the ChatGPT Codex backend's session_id / x-client-request-id header.
// Used by the router as a per-resolve default PromptCacheKey for
// callers that do not supply their own conversation correlation key.
func NewSessionID() (string, error) {
	return randomUUID()
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func exchangeCode(
	ctx context.Context, client *http.Client, code, verifier, redirectURI string,
) (Credential, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", oauthClientID)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", verifier)
	resp, err := postTokenForm(ctx, client, form)
	if err != nil {
		return Credential{}, err
	}
	return credentialFromTokenResponse(resp, Credential{}, time.Now())
}

func refreshCredential(ctx context.Context, client *http.Client, current Credential) (Credential, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", oauthClientID)
	form.Set("refresh_token", current.RefreshToken)
	form.Set("scope", "openid profile email")
	resp, err := postTokenForm(ctx, client, form)
	if err != nil {
		return Credential{}, err
	}
	return credentialFromTokenResponse(resp, current, time.Now())
}

func postTokenForm(ctx context.Context, client *http.Client, form url.Values) (tokenResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("codex token exchange: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("read Codex token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokenResponse{}, fmt.Errorf("codex token exchange failed: status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var token tokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return tokenResponse{}, fmt.Errorf("decode Codex token response: %w", err)
	}
	if token.AccessToken == "" {
		return tokenResponse{}, errors.New("codex token response missing access_token")
	}
	return token, nil
}

func credentialFromTokenResponse(resp tokenResponse, fallback Credential, now time.Time) (Credential, error) {
	cred := fallback
	cred.AccessToken = resp.AccessToken
	if resp.RefreshToken != "" {
		cred.RefreshToken = resp.RefreshToken
	}
	if resp.IDToken != "" {
		cred.IDToken = resp.IDToken
	}
	if resp.TokenType != "" {
		cred.TokenType = resp.TokenType
	} else if cred.TokenType == "" {
		cred.TokenType = "Bearer"
	}
	if resp.ExpiresIn > 0 {
		cred.Expiry = now.Add(time.Duration(resp.ExpiresIn) * time.Second)
	}
	cred.LastRefresh = now
	if cred.IDToken != "" {
		claims, err := parseIDTokenClaims(cred.IDToken)
		if err != nil {
			return Credential{}, err
		}
		if claims.Email != "" {
			cred.Email = claims.Email
		}
		if claims.PlanType != "" {
			cred.PlanType = claims.PlanType
		}
		if claims.UserID != "" {
			cred.UserID = claims.UserID
		}
		if claims.AccountID != "" {
			cred.AccountID = claims.AccountID
		}
		cred.FedRAMP = claims.FedRAMP
	}
	return cred, nil
}

type parsedClaims struct {
	Email     string
	PlanType  string
	UserID    string
	AccountID string
	FedRAMP   bool
}

func parseIDTokenClaims(jwt string) (parsedClaims, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 || parts[1] == "" {
		return parsedClaims{}, errors.New("invalid Codex id_token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return parsedClaims{}, fmt.Errorf("decode Codex id_token: %w", err)
	}
	var claims struct {
		Email   string `json:"email"`
		Profile struct {
			Email string `json:"email"`
		} `json:"https://api.openai.com/profile"`
		Auth struct {
			PlanType  string `json:"chatgpt_plan_type"`
			UserID    string `json:"chatgpt_user_id"`
			UserIDAlt string `json:"user_id"`
			AccountID string `json:"chatgpt_account_id"`
			FedRAMP   bool   `json:"chatgpt_account_is_fedramp"`
		} `json:"https://api.openai.com/auth"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return parsedClaims{}, fmt.Errorf("parse Codex id_token: %w", err)
	}
	ret := parsedClaims{
		Email:     claims.Email,
		PlanType:  claims.Auth.PlanType,
		UserID:    claims.Auth.UserID,
		AccountID: claims.Auth.AccountID,
		FedRAMP:   claims.Auth.FedRAMP,
	}
	if ret.Email == "" {
		ret.Email = claims.Profile.Email
	}
	if ret.UserID == "" {
		ret.UserID = claims.Auth.UserIDAlt
	}
	return ret, nil
}
