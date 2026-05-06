// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package auth

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"golang.org/x/oauth2"
)

const (
	defaultStorageTimeout = 2 * time.Second
	// this should be long enough to allow user to go through the oauth2 flow
	// comfortably. We only want to timeout to unlock the calling goroutine
	// in case something hangs or the flow cannot be completed.
	defaultSourceTimeout = 15 * time.Second
	tokenDocumentID      = "tokenv2"
)

// NewCachedTokenSource allocates storage for a new CachedTokenSource
// and initializes it with sourcer and storage.
func NewCachedTokenSource(
	sourcer TokenSourcer, storage storageapi.Service,
	notifications browserapi.Notifications,
) *CachedTokenSource {
	return &CachedTokenSource{sourcer: sourcer, storage: storage, notifications: notifications}
}

// CachedTokenSource is a oauth2.TokenSource that stores
// tokens in a document.Service to persist across sessions.
type CachedTokenSource struct {
	sourcer       TokenSourcer
	storage       storageapi.Service
	notifications browserapi.Notifications

	mu    sync.RWMutex
	token *oauth2.Token
}

// Purge purged the underlying storage from any cached data.
func (l *CachedTokenSource) Purge() error {
	ctx, cancel := context.WithTimeout(context.Background(),
		defaultStorageTimeout)
	defer cancel()

	l.mu.Lock()
	l.token = nil
	l.mu.Unlock()

	return l.storage.Delete(ctx, tokenDocumentID)
}

// Token satisfies oauth2.TokenSource.
func (l *CachedTokenSource) Token() (*oauth2.Token, error) {
	const callType = "Token"
	traceID, ctx := trace.FromContextOrNew(context.Background())

	l.mu.RLock()
	token := l.token
	l.mu.RUnlock()

	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "auth.CachedTokenSource"},
		{Key: "ptr", Value: fmt.Sprintf("%p", l)},
		{Key: "cache-valid", Value: strconv.FormatBool(token != nil && token.Valid())},
	}

	attemptAt := logging.LogAttempt(traceID, callType, fields...)
	ret, err := l.getToken(traceID, ctx, fields, token)
	// do not log ErrUnavailable as error
	if err == ErrUnavailable {
		logging.LogResultLevel(log.DebugLevel, log.DebugLevel, err, attemptAt,
			traceID, callType, fields...)
	} else {
		logging.LogResult(err, attemptAt, traceID, callType, fields...)
	}

	return ret, err
}

func (l *CachedTokenSource) getToken(
	traceID trace.ID, ctx context.Context, fields []logging.Field, token *oauth2.Token,
) (*oauth2.Token, error) {
	log := log.WithField("cached", fmt.Sprintf("%p", token))
	log = log.WithField(logging.KeyTraceID, traceID)
	for _, field := range fields {
		log = log.WithField(field.Key, field.Value)
	}

	if token == nil || !token.Valid() {
		// only allow one goroutine to proceed
		l.mu.Lock()

		// double check that while waiting for lock to be freed, some other goroutine
		// didn't complete the auth flow.
		if l.token == nil || !l.token.Valid() {
			ctx, cancel := context.WithTimeout(ctx, defaultStorageTimeout)
			defer cancel()
			var tokenForStorage storedToken
			err := l.storage.Get(ctx, tokenDocumentID, &tokenForStorage)
			log.Tracef("sourcing from storage: %v", err)
			if err == nil {
				token = tokenForStorage.oauth2()
				l.token = token
			} else {
				// ErrNotFound or other cache errors are ignored
				// signal below that a new token needs to be created
				token = nil
			}
		}

		l.mu.Unlock()
	}

	if token != nil && token.Valid() {
		log.Tracef("using cached in memory or in storage")
		return token, nil
	}

	log.Tracef("fetching a new one: using refresh_token %v", token != nil)

	// act as semaphore so only 1 goroutine is acquiring a new token at a time
	l.mu.Lock()
	defer l.mu.Unlock()

	// double check that while waiting for lock to be freed, some other goroutine
	// didn't complete the auth flow.
	if l.token != nil && l.token.Valid() {
		log.Tracef("some other goroutine was able to acquire one just in time")
		return l.token, nil
	}

	if l.sourcer == nil {
		return nil, ErrUnavailable
	}

	// if token is nil, then a new oauth2 flow is started
	// otherwise refresh_token grant is used to refresh token
	// under the hood.
	source, err := l.sourcer.TokenSource(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("token source: %v", err)
	}

	newToken, err := source.Token()
	if err != nil && token != nil {
		// try again without a refresh token; it could be expired
		// and internal oauth2 implementation doesn't ever retry without it
		token = nil
		source, err = l.sourcer.TokenSource(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("token source: %v", err)
		}
		newToken, err = source.Token()
	}
	if err != nil {
		// notify if refreshing in the background, but only notify once per session.
		// In order to do this, a unique UUID per session is passed, in this case it's
		// the hash of the address of this CachedTokenSource.
		if token != nil && !token.Valid() && token.RefreshToken != "" {
			id := hashString(fmt.Sprintf("%p", l))
			_, _ = l.notifications.NotifyOnce(browserapi.LevelWarn,
				"Failed to use oauth2 refresh token to authenticate. "+
					"Please logout and re-login to access API resources. (Source=%d)", id)
		}
		return nil, fmt.Errorf("acquire token: %v", err)
	}
	l.token = newToken

	ctx, cancel := context.WithTimeout(ctx, defaultStorageTimeout)
	defer cancel()

	// store for next session
	if err := l.storage.Set(ctx, tokenDocumentID, newStoredToken(l.token)); err != nil {
		log.Tracef("cached token source: storage set: %v", err)
	}

	if !l.token.Valid() {
		log.Errorf("token returned by source is not valid: %s", l.token.AccessToken)
	}

	return l.token, nil
}

func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// storedToken is a mirror of oauth2.Token, with the Extra field
// defined so it can be persisted.
type storedToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry,omitempty"`
	Extra        RPCUser   `json:"extra,omitempty"`
}

func newStoredToken(t *oauth2.Token) (ret storedToken) {
	ret.AccessToken = t.AccessToken
	ret.TokenType = t.TokenType
	ret.RefreshToken = t.RefreshToken
	ret.Expiry = t.Expiry

	extra, _ := t.Extra("extra").(map[string]any)
	ret.Extra.Email, _ = extra["Email"].(string)
	ret.Extra.ID, _ = extra["ID"].(string)
	ret.Extra.Role, _ = extra["Role"].(Role)
	ret.Extra.Account, _ = extra["Account"].(string)
	return ret
}

func (t storedToken) oauth2() *oauth2.Token {
	ret := new(oauth2.Token)
	ret.AccessToken = t.AccessToken
	ret.TokenType = t.TokenType
	ret.RefreshToken = t.RefreshToken
	ret.Expiry = t.Expiry

	// unfortunately to keep the server's http token handler
	// agnostic to extra, it serializes it under the field 'extra',
	// rather than adding all the extra properties as part
	// of the resposne payload.
	all := make(map[string]any)
	extra := make(map[string]any)
	all["extra"] = extra
	extra["Email"] = t.Extra.Email
	extra["ID"] = t.Extra.ID
	extra["Role"] = t.Extra.Role
	extra["Account"] = t.Extra.Account
	ret = ret.WithExtra(all)
	return ret
}
