// Copyright (C) 2017-2026 Unstable Build, LLC
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

package claude

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
	"unstable.build/rune/debug"
	"unstable.build/rune/llm/authcallback"
)

const (
	credentialDocumentID = "provider:claude:credential"

	// OAuth endpoints and client for the Claude Code subscription flow,
	// matching what the Claude Code CLI sends.
	oauthAuthURL    = "https://claude.ai/oauth/authorize"
	oauthClientID   = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	oauthScopes     = "user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"
	defaultCallback = ":54545"
	defaultRedirect = "http://localhost:54545/callback"

	defaultLoginWait = 5 * time.Minute
	refreshLead      = 5 * time.Minute

	// agentBetaHeader is the anthropic-beta value the Claude Code client
	// sends. oauth-2025-04-20 enables the OAuth bearer path and
	// claude-code-20250219 identifies the request as Claude Code so usage
	// attributes to the subscription's Agent-SDK credit pool.
	agentBetaHeader = "claude-code-20250219,oauth-2025-04-20,interleaved-thinking-2025-05-14"
	// anthropicVersion is the required Anthropic API version header.
	anthropicVersion = "2023-06-01"
)

// oauthTokenURL is the token-exchange endpoint. It is a package variable
// (not a const) so tests can point it at an httptest.Server via the
// injected *http.Client seam.
var oauthTokenURL = "https://api.anthropic.com/v1/oauth/token"

// ErrCredentialNotFound is returned when no Claude credential has been saved.
var ErrCredentialNotFound = errors.New("claude credential not found")

// Credential is the token data needed to authenticate Claude API calls
// using a Claude Code subscription.
type Credential struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
	LastRefresh  time.Time
	Email        string
	AccountID    string
	PlanType     string
	Scopes       string
}

// AuthStatus describes the currently stored Claude credential, if any.
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

// ClientHeaders returns the Agent-SDK identifying headers the Anthropic
// client must send when authenticating with a Claude Code subscription.
func (c Credential) ClientHeaders() map[string]string {
	return map[string]string{
		"anthropic-beta":    agentBetaHeader,
		"anthropic-version": anthropicVersion,
	}
}

func (c Credential) expired(now time.Time) bool {
	return !c.Expiry.IsZero() && !now.Before(c.Expiry)
}

func (c Credential) needsRefresh(now time.Time) bool {
	return !c.Expiry.IsZero() && !now.Add(refreshLead).Before(c.Expiry)
}

// SaveCredential stores the Claude credential in extension storage.
func SaveCredential(ctx context.Context, storage storageapi.Service, cred Credential) error {
	if storage == nil {
		return errors.New("claude credential storage is unavailable")
	}
	return storage.Set(ctx, credentialDocumentID, credentialRecordFromPublic(cred))
}

// LoadCredential loads the stored Claude credential.
func LoadCredential(ctx context.Context, storage storageapi.Service) (Credential, error) {
	if storage == nil {
		return Credential{}, ErrCredentialNotFound
	}
	var rec credentialRecord
	if err := storage.Get(ctx, credentialDocumentID, &rec); err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			return Credential{}, ErrCredentialNotFound
		}
		return Credential{}, fmt.Errorf("load claude credential: %w", err)
	}
	return rec.public(), nil
}

// CredentialForClient returns a non-expired Claude credential, refreshing it when needed.
func CredentialForClient(ctx context.Context, storage storageapi.Service) (Credential, error) {
	cred, err := LoadCredential(ctx, storage)
	if err != nil {
		return Credential{}, err
	}
	if cred.AccessToken == "" {
		return Credential{}, errors.New("claude credential missing access token")
	}
	if !cred.needsRefresh(time.Now()) {
		return cred, nil
	}
	if cred.RefreshToken == "" {
		return Credential{}, errors.New("claude credential expired and has no refresh token")
	}
	refreshed, err := refreshCredential(ctx, http.DefaultClient, cred)
	if err != nil {
		return Credential{}, err
	}
	if err := SaveCredential(ctx, storage, refreshed); err != nil {
		return Credential{}, err
	}
	return refreshed, nil
}

// Status returns the stored Claude authentication status.
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
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
	LastRefresh  time.Time
	Email        string
	AccountID    string
	PlanType     string
	Scopes       string
}

func credentialRecordFromPublic(cred Credential) credentialRecord {
	return credentialRecord(cred)
}

func (r credentialRecord) public() Credential {
	return Credential(r)
}

// LoginSession is an in-progress Claude OAuth login.
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
		return Credential{}, errors.New("timed out waiting for Claude authentication callback")
	case cb = <-s.callbackCh:
	}
	if cb.err != nil {
		return Credential{}, cb.err
	}
	if cb.state != s.state {
		return Credential{}, errors.New("claude authentication state mismatch")
	}

	cred, err := exchangeCode(ctx, s.httpClient, cb.code, cb.state, s.pkce.verifier, s.redirectURI)
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

// LoginOption configures a Claude OAuth login.
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

// WithHTTPClient overrides the HTTP client used for the token exchange.
func WithHTTPClient(client *http.Client) LoginOption {
	return func(c *loginConfig) { c.httpClient = client }
}

// WithCallbackAddr overrides the local callback listen address and redirect URI.
func WithCallbackAddr(addr, redirectURI string) LoginOption {
	return func(c *loginConfig) {
		c.callbackAddr = addr
		c.redirectURI = redirectURI
	}
}

// StartLogin starts a Claude OAuth login and returns the URL to display to the user.
func StartLogin(ctx context.Context, storage storageapi.Service, opts ...LoginOption) (*LoginSession, error) {
	if storage == nil {
		return nil, errors.New("claude credential storage is unavailable")
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
		return nil, fmt.Errorf("start Claude OAuth callback server: %w", err)
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
	mux.HandleFunc("/callback", session.handleCallback)
	mux.HandleFunc("/success", handleSuccess)
	session.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	serveErr := make(chan error, 1)
	go debug.CapturePanicReport(func() {
		if err := session.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	})

	select {
	case <-ctx.Done():
		_ = session.Close()
		return nil, ctx.Err()
	case err := <-serveErr:
		if err != nil {
			_ = session.Close()
			return nil, fmt.Errorf("start Claude OAuth callback server: %w", err)
		}
		_ = session.Close()
		return nil, errors.New("claude OAuth callback server stopped unexpectedly")
	default:
	}

	if cfg.openBrowser != nil {
		session.browserErr = cfg.openBrowser(session.authURL)
	}
	return session, nil
}

func authURL(state, challenge, redirectURI string) string {
	v := url.Values{}
	v.Set("code", "true")
	v.Set("client_id", oauthClientID)
	v.Set("response_type", "code")
	v.Set("redirect_uri", redirectURI)
	v.Set("scope", oauthScopes)
	v.Set("state", state)
	v.Set("code_challenge", challenge)
	v.Set("code_challenge_method", "S256")
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
		s.sendCallback(oauthCallback{err: fmt.Errorf("claude authentication failed: %s", desc)})
		http.Error(w, desc, http.StatusBadRequest)
		return
	}
	code := q.Get("code")
	state := q.Get("state")
	if code == "" || state == "" {
		s.sendCallback(oauthCallback{err: errors.New("claude authentication callback missing code or state")})
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
		Title:   "Claude authentication complete — Rune",
		Heading: "Done",
		Message: "Claude authentication complete. You can return to Rune.",
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

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
	Account      struct {
		UUID         string `json:"uuid"`
		EmailAddress string `json:"email_address"`
	} `json:"account"`
	Organization struct {
		UUID         string `json:"uuid"`
		Name         string `json:"name"`
		BillingType  string `json:"billing_type"`
		Subscription string `json:"subscription_type"`
	} `json:"organization"`
}

func exchangeCode(
	ctx context.Context, client *http.Client, code, state, verifier, redirectURI string,
) (Credential, error) {
	// The authorization code may arrive as "code#state"; the fragment
	// state takes precedence when present (matches the Claude Code CLI).
	if rawCode, fragState, found := strings.Cut(code, "#"); found {
		code = rawCode
		if fragState != "" {
			state = fragState
		}
	}
	body := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     oauthClientID,
		"code":          code,
		"state":         state,
		"redirect_uri":  redirectURI,
		"code_verifier": verifier,
	}
	resp, err := postTokenForm(ctx, client, body)
	if err != nil {
		return Credential{}, err
	}
	return credentialFromTokenResponse(resp, Credential{}, time.Now()), nil
}

func refreshCredential(ctx context.Context, client *http.Client, current Credential) (Credential, error) {
	body := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     oauthClientID,
		"refresh_token": current.RefreshToken,
	}
	resp, err := postTokenForm(ctx, client, body)
	if err != nil {
		return Credential{}, err
	}
	return credentialFromTokenResponse(resp, current, time.Now()), nil
}

func postTokenForm(ctx context.Context, client *http.Client, body map[string]string) (tokenResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return tokenResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, oauthTokenURL, strings.NewReader(string(payload)))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("claude token exchange: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("read Claude token response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tokenResponse{}, fmt.Errorf("claude token exchange failed: status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var token tokenResponse
	if err := json.Unmarshal(raw, &token); err != nil {
		return tokenResponse{}, fmt.Errorf("decode Claude token response: %w", err)
	}
	if token.AccessToken == "" {
		return tokenResponse{}, errors.New("claude token response missing access_token")
	}
	return token, nil
}

func credentialFromTokenResponse(resp tokenResponse, fallback Credential, now time.Time) Credential {
	cred := fallback
	cred.AccessToken = resp.AccessToken
	if resp.RefreshToken != "" {
		cred.RefreshToken = resp.RefreshToken
	}
	if resp.TokenType != "" {
		cred.TokenType = resp.TokenType
	} else if cred.TokenType == "" {
		cred.TokenType = "Bearer"
	}
	if resp.ExpiresIn > 0 {
		cred.Expiry = now.Add(time.Duration(resp.ExpiresIn) * time.Second)
	}
	if resp.Scope != "" {
		cred.Scopes = resp.Scope
	}
	if resp.Account.EmailAddress != "" {
		cred.Email = resp.Account.EmailAddress
	}
	if resp.Organization.UUID != "" {
		cred.AccountID = resp.Organization.UUID
	}
	if resp.Organization.Subscription != "" {
		cred.PlanType = resp.Organization.Subscription
	} else if resp.Organization.BillingType != "" {
		cred.PlanType = resp.Organization.BillingType
	}
	cred.LastRefresh = now
	return cred
}
