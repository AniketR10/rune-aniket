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
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"golang.org/x/oauth2"
)

func TestCachedTokenToken(t *testing.T) {
	t.Run("uses sourcer if no token is cached", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		sourcer, token := goodSourcer()
		source := NewCachedTokenSource(sourcer, svc, nopNotifications{})
		actualToken, err := source.Token()
		require.NoError(t, err)
		assertEqualToken(t, token, actualToken)
	})

	t.Run("reuses token from cache if called again, within same session", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		sourcer, token := goodSourcer()
		source := NewCachedTokenSource(sourcer, svc, nopNotifications{})

		for i := 0; i < 2; i++ {
			actualToken, err := source.Token()
			require.NoError(t, err)
			assertEqualToken(t, token, actualToken)
		}

		assert.Equal(t, int32(1), sourcer.called.Load())
	})

	t.Run("reuses token from cache if called again, accross sessions", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()

		for i := 0; i < 2; i++ {
			sourcer, token := goodSourcer()
			source := NewCachedTokenSource(sourcer, svc, nopNotifications{})

			actualToken, err := source.Token()
			require.NoError(t, err)

			if i == 0 {
				assert.Equal(t, int32(1), sourcer.called.Load())
				assertEqualToken(t, token, actualToken)
			} else {
				assert.Equal(t, int32(0), sourcer.called.Load())
			}
		}

	})

	t.Run("acquires new token if token from cache is expired", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()

		for i := 0; i < 2; i++ {
			sourcer, token := goodSourcerWithExpiry(0)
			source := NewCachedTokenSource(sourcer, svc, nopNotifications{})

			actualToken, err := source.Token()
			require.NoError(t, err)

			// new token every session
			assert.Equal(t, int32(1), sourcer.called.Load())
			assertEqualToken(t, token, actualToken)
		}

	})

	// use-case: two concurrent CachedTokenSource using the same underlying storage
	t.Run("before acquiring new token, it checks storage if token from inmemory cache is expired", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()

		badSourcer, expiredToken := goodSourcerWithExpiry(-1)
		source := NewCachedTokenSource(badSourcer, svc, nopNotifications{})

		actualToken, err := source.Token()
		require.NoError(t, err)

		assert.Equal(t, int32(1), badSourcer.called.Load())
		assertEqualToken(t, expiredToken, actualToken)

		goodSourcer, storageToken := goodSourcerWithExpiry(30 * time.Minute)
		// force store token in svc
		actualToken, err = NewCachedTokenSource(goodSourcer, svc, nopNotifications{}).Token()
		require.NoError(t, err)

		assert.Equal(t, int32(1), goodSourcer.called.Load())
		assertEqualToken(t, storageToken, actualToken)

		actualToken, err = source.Token()
		require.NoError(t, err)

		assert.Equal(t, int32(1), badSourcer.called.Load())
		assertEqualToken(t, storageToken, actualToken)
	})

	t.Run("is goroutine safe", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		// token Valid returns false if we are within 10 seconds of
		// expiry, and this cannot be changed.
		sourcer, _ := goodSourcerWithExpiry(1 * time.Minute)
		source := NewCachedTokenSource(sourcer, svc, nopNotifications{})

		const n = 1000
		var wg sync.WaitGroup

		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				_, _ = source.Token()
			}()
		}
		wg.Wait()

		assert.Equal(t, int32(1), sourcer.called.Load())
	})

	t.Run("ignores document service errors, within same session", func(t *testing.T) {
		svc := failingDocumentService{}
		sourcer, token := goodSourcer()
		source := NewCachedTokenSource(sourcer, svc, nopNotifications{})

		for i := 0; i < 2; i++ {
			actualToken, err := source.Token()
			require.NoError(t, err)
			assertEqualToken(t, token, actualToken)
		}

		assert.Equal(t, int32(1), sourcer.called.Load())
	})

	t.Run("ignores document service errors, accross sessions", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			sourcer, token := goodSourcer()
			source := NewCachedTokenSource(sourcer, failingDocumentService{}, nopNotifications{})

			actualToken, err := source.Token()
			require.NoError(t, err)

			// new token every session
			assert.Equal(t, int32(1), sourcer.called.Load())
			assertEqualToken(t, token, actualToken)
		}
	})

	t.Run("bubbles up Sourcer errors", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		sourcer, _ := badSourcer()
		source := NewCachedTokenSource(sourcer, svc, nopNotifications{})
		_, actualErr := source.Token()
		require.EqualError(t, actualErr, "token source: boom")
	})

	t.Run("bubbles up oauth2.TokenSource errors", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		sourcer, _ := badSourceSourcer()
		source := NewCachedTokenSource(sourcer, svc, nopNotifications{})
		_, actualErr := source.Token()
		require.EqualError(t, actualErr, "acquire token: boom")
	})

	t.Run("retries without refresh token if cached refresh token fails to acquire a token", func(t *testing.T) {
		svc := storagestub.NewInMemoryService()
		cachedToken := new(oauth2.Token)
		cachedToken.Expiry = time.Now().Add(-24 * time.Hour)
		cachedToken.AccessToken = "1234"
		err := svc.Set(context.Background(), tokenDocumentID, newStoredToken(cachedToken))
		require.NoError(t, err)
		sourcer := badRefreshSourceSourcer()
		source := NewCachedTokenSource(sourcer, svc, nopNotifications{})
		_, err = source.Token()
		assert.NoError(t, err)
	})
}

func assertEqualToken(t *testing.T, expected, actual *oauth2.Token) {
	assert.Equal(t, expected.AccessToken, actual.AccessToken)
	assert.Equal(t, expected.TokenType, actual.TokenType)
	assert.Equal(t, expected.RefreshToken, actual.RefreshToken)
	assert.WithinDuration(t, expected.Expiry.Truncate(time.Second),
		actual.Expiry.Truncate(time.Second), 0)
	assert.Equal(t, expected.Extra("extra"), actual.Extra("extra"))
}

func badSourcer() (*testSourcer, error) {
	err := errors.New("boom")
	return &testSourcer{retErr: err}, err
}

func badSourceSourcer() (*testSourcer, error) {
	err := errors.New("boom")
	return &testSourcer{retSource: &testSource{retErr: err}}, err
}

func badRefreshSourceSourcer() TokenSourcer {
	return FuncTokenSourcer(
		func(_ context.Context, token *oauth2.Token) (oauth2.TokenSource, error) {
			if token == nil {
				return &testSource{retToken: new(oauth2.Token), retErr: nil}, nil
			}
			return &testSource{retErr: errors.New("boom")}, nil
		})
}

func goodSourcer() (*testSourcer, *oauth2.Token) {
	return goodSourcerWithExpiry(1 * time.Hour)
}

func goodSourcerWithExpiry(expiry time.Duration) (*testSourcer, *oauth2.Token) {
	goodSource, token := goodSourceWithExpiry(expiry)
	return &testSourcer{retSource: goodSource}, token
}

func goodSourceWithExpiry(expiry time.Duration) (*testSource, *oauth2.Token) {
	token := &oauth2.Token{
		AccessToken:  uuid.New().String(),
		TokenType:    "bearer",
		RefreshToken: "1235",
		Expiry:       time.Now().Add(expiry),
	}
	token = token.WithExtra(map[string]any{"extra": map[string]any{
		"Email":   "myEmail",
		"Account": "myAccount",
		"ID":      "myID",
		"Role":    RoleAdmin,
	}})
	return &testSource{retToken: token}, token
}

type testSourcer struct {
	called    atomic.Int32
	retSource oauth2.TokenSource
	retErr    error
}

func (t *testSourcer) TokenSource(context.Context, *oauth2.Token) (oauth2.TokenSource, error) {
	t.called.Add(1)
	return t.retSource, t.retErr
}

type testSource struct {
	called   atomic.Int32
	retToken *oauth2.Token
	retErr   error
}

func (t *testSource) Token() (*oauth2.Token, error) {
	t.called.Add(1)
	return t.retToken, t.retErr
}

type failingDocumentService struct {
}

func (failingDocumentService) Create(ctx context.Context, ID string, doc interface{}) error {
	return errors.New("oopsie")
}
func (failingDocumentService) Set(ctx context.Context, ID string, doc interface{}) error {
	return errors.New("oopsie")
}
func (failingDocumentService) Update(ctx context.Context, ID string,
	updates []storageapi.Update, precond ...storageapi.Precondition) error {
	return errors.New("oopsie")
}
func (failingDocumentService) Get(ctx context.Context, ID string, doc interface{}) error {
	return errors.New("oopsie")
}
func (failingDocumentService) Delete(ctx context.Context, ID string) error {
	return errors.New("oopsie")
}
func (failingDocumentService) List(ctx context.Context, filters []storageapi.Filter) (storageapi.Iterator, error) {
	return nil, errors.New("oopsie")
}

func (failingDocumentService) Close() error {
	return nil
}

type nopNotifications struct {
}

func (a nopNotifications) Notify(
	level browserapi.NotificationLevel, msg string, args ...interface{},
) (string, error) {
	return "", nil
}

func (a nopNotifications) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...interface{},
) (string, error) {
	return "", nil
}

func (a nopNotifications) UpdateNotificationProgress(
	id, message string, progress, total int64,
) error {
	return nil
}
