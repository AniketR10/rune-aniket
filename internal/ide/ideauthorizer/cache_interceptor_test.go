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

package ideauthorizer

import (
	"context"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	blueauth "github.com/unstablebuild/blue/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"unstable.build/rune/internal/extension/extensionv2/peerprocess"
)

func TestAuthCacheReusesVerification(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	stub := &stubInterceptor{claims: claimsWithExpiry(now.Add(time.Hour))}
	cache := newTestCache(stub, func() time.Time { return now })

	ctx := incomingCtx("Bearer tok")
	callUnary(t, cache, ctx, "/Scheme/Open")
	callUnary(t, cache, ctx, "/Scheme/Open")

	assert.Equal(t, 1, stub.calls, "the wrapped interceptor runs once for a repeated token+method")
}

func TestAuthCacheAuthorizesEachMethod(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	stub := &stubInterceptor{claims: claimsWithExpiry(now.Add(time.Hour))}
	cache := newTestCache(stub, func() time.Time { return now })

	ctx := incomingCtx("Bearer tok")
	callUnary(t, cache, ctx, "/Scheme/Open")
	callUnary(t, cache, ctx, "/Scheme/ReadDir")

	assert.Equal(t, 2, stub.calls, "each method is authorized once")
}

func TestAuthCacheAuthorizesEachPeer(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	stub := &stubInterceptor{claims: claimsWithExpiry(now.Add(time.Hour))}
	cache := newTestCache(stub, func() time.Time { return now })

	const method = "/browser.WindowManager/NewWindow"
	// Two processes share a bearer token but differ in peer identity, as
	// with two plugin subprocesses. Each must be authorized independently.
	callUnary(t, cache, ctxWithPeer(incomingCtx("Bearer tok"), 1), method)
	callUnary(t, cache, ctxWithPeer(incomingCtx("Bearer tok"), 2), method)

	assert.Equal(t, 2, stub.calls, "a distinct peer is authorized separately")
}

func TestAuthCacheReverifiesDifferentToken(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	stub := &stubInterceptor{claims: claimsWithExpiry(now.Add(time.Hour))}
	cache := newTestCache(stub, func() time.Time { return now })

	const method = "/Scheme/Open"
	callUnary(t, cache, incomingCtx("Bearer a"), method)
	callUnary(t, cache, incomingCtx("Bearer b"), method)

	assert.Equal(t, 2, stub.calls, "a distinct token is verified separately")
}

func TestAuthCacheReverifiesAfterTTL(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	// Token exp is far in the future so the clamp to maxAuthCacheTTL governs.
	stub := &stubInterceptor{claims: claimsWithExpiry(now.Add(time.Hour))}
	current := now
	cache := newTestCache(stub, func() time.Time { return current })

	const method = "/Scheme/Open"
	callUnary(t, cache, incomingCtx("Bearer tok"), method)
	current = now.Add(maxAuthCacheTTL + time.Second)
	callUnary(t, cache, incomingCtx("Bearer tok"), method)

	assert.Equal(t, 2, stub.calls, "an expired cache entry forces re-verification")
}

func TestAuthCacheReverifiesAfterTokenExpiry(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	// Token exp is sooner than maxAuthCacheTTL, so it governs eviction.
	stub := &stubInterceptor{claims: claimsWithExpiry(now.Add(30 * time.Second))}
	current := now
	cache := newTestCache(stub, func() time.Time { return current })

	const method = "/Scheme/Open"
	callUnary(t, cache, incomingCtx("Bearer tok"), method)
	current = now.Add(31 * time.Second)
	callUnary(t, cache, incomingCtx("Bearer tok"), method)

	assert.Equal(t, 2, stub.calls, "an expired token forces re-verification")
}

func TestAuthCacheDoesNotCacheFailures(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	stub := &stubInterceptor{err: blueauth.ErrForbidden}
	cache := newTestCache(stub, func() time.Time { return now })

	ctx := incomingCtx("Bearer tok")
	const method = "/Scheme/Open"
	_, err := cache.Unary(ctx, nil, unaryInfo(method),
		func(context.Context, any) (any, error) { return nil, nil })
	require.Error(t, err)
	_, err = cache.Unary(ctx, nil, unaryInfo(method),
		func(context.Context, any) (any, error) { return nil, nil })
	require.Error(t, err)

	assert.Equal(t, 2, stub.calls, "a denied request must not be cached")
}

func TestAuthCacheEvictKeepsVerificationForcesReauthorization(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	stub := &stubInterceptor{claims: claimsWithExpiry(now.Add(time.Hour))}
	cache := newTestCache(stub, func() time.Time { return now })

	ctx := incomingCtx("Bearer tok")
	const method = "/Scheme/Open"
	callUnary(t, cache, ctx, method)

	cache.evict()

	callUnary(t, cache, ctx, method)

	// The wrapped interceptor still runs again because eviction drops the
	// authorization decision; there is no separate verify/authorize split
	// in the wrapper, so a re-authorization also re-verifies.
	assert.Equal(t, 2, stub.calls, "eviction forces the wrapped interceptor to run again")
}

func TestAuthCacheStreamCaches(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	stub := &stubInterceptor{claims: claimsWithExpiry(now.Add(time.Hour))}
	cache := newTestCache(stub, func() time.Time { return now })

	info := &grpc.StreamServerInfo{FullMethod: "/Executor/StartCommand"}
	handler := func(any, grpc.ServerStream) error { return nil }
	ss := nopServerStream{ctx: incomingCtx("Bearer tok")}

	require.NoError(t, cache.Stream(nil, ss, info, handler))
	require.NoError(t, cache.Stream(nil, ss, info, handler))

	assert.Equal(t, 1, stub.calls, "a repeated stream token+method is served from cache")
}

func TestAuthCacheDelegatesWhenNoToken(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	stub := &stubInterceptor{err: blueauth.ErrForbidden}
	cache := newTestCache(stub, func() time.Time { return now })

	// No authorization metadata: the cache cannot key on the request and
	// must delegate so the wrapped interceptor produces the auth error.
	_, err := cache.Unary(context.Background(), nil, unaryInfo("/Scheme/Open"),
		func(context.Context, any) (any, error) { return nil, nil })
	require.Error(t, err)
	assert.Equal(t, 1, stub.calls, "a request without a token is delegated to the wrapped interceptor")
}

// stubInterceptor stands in for the blue oauth2 interceptor. It counts how
// often it runs (a proxy for the ed25519/JWT verification and permission
// lookup) and installs claims in the context before calling the handler,
// exactly as the real interceptor does.
type stubInterceptor struct {
	calls  int
	claims blueauth.UserClaims[Extension]
	err    error
}

func (s *stubInterceptor) unary(
	ctx context.Context, req any,
	_ *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (any, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return handler(blueauth.ContextWithClaims(ctx, s.claims), req)
}

func (s *stubInterceptor) stream(
	srv any, ss grpc.ServerStream,
	_ *grpc.StreamServerInfo, handler grpc.StreamHandler,
) error {
	s.calls++
	if s.err != nil {
		return s.err
	}
	ctx := blueauth.ContextWithClaims(ss.Context(), s.claims)
	return handler(srv, cacheServerStream{ServerStream: ss, ctx: ctx})
}

// nopServerStream is a minimal grpc.ServerStream carrying a context.
type nopServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s nopServerStream) Context() context.Context { return s.ctx }

func claimsWithExpiry(expiry time.Time) blueauth.UserClaims[Extension] {
	var claims blueauth.UserClaims[Extension]
	claims.Expiry = jwt.NewNumericDate(expiry)
	return claims
}

func incomingCtx(token string) context.Context {
	md := metadata.New(map[string]string{"authorization": token})
	return metadata.NewIncomingContext(context.Background(), md)
}

// ctxWithPeer adds a peer process identity so authorization decisions key
// on the peer, mirroring plugin subprocesses.
func ctxWithPeer(ctx context.Context, pid int) context.Context {
	addr := PeerProcessAddr{Process: peerprocess.Process{PID: pid, Exe: "/plugin"}}
	return peer.NewContext(ctx, &peer.Peer{Addr: addr})
}

func newTestCache(stub *stubInterceptor, now func() time.Time) *authCache {
	cache := newAuthCache(stub.unary, stub.stream)
	cache.now = now
	return cache
}

func unaryInfo(method string) *grpc.UnaryServerInfo {
	return &grpc.UnaryServerInfo{FullMethod: method}
}

func callUnary(t *testing.T, cache *authCache, ctx context.Context, method string) {
	t.Helper()
	_, err := cache.Unary(ctx, nil, unaryInfo(method),
		func(context.Context, any) (any, error) { return nil, nil })
	require.NoError(t, err)
}
