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
	"fmt"
	"strings"
	"sync"
	"time"

	blueauth "github.com/unstablebuild/blue/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const maxAuthCacheTTL = 2 * time.Minute

type authCache struct {
	unary  grpc.UnaryServerInterceptor
	stream grpc.StreamServerInterceptor
	now    func() time.Time

	mu      sync.Mutex
	entries map[string]*authCacheEntry
}

func newAuthCache(
	unary grpc.UnaryServerInterceptor, stream grpc.StreamServerInterceptor,
) *authCache {
	return &authCache{
		unary:   unary,
		stream:  stream,
		now:     time.Now,
		entries: make(map[string]*authCacheEntry),
	}
}

// Unary satisfies grpc.UnaryServerInterceptor. On a cache hit it installs
// the cached claims and calls the handler directly; otherwise it delegates
// to the wrapped interceptor and records the verified, authorized claims.
func (c *authCache) Unary(
	ctx context.Context, req any,
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (any, error) {
	token := bearerTokenFromContext(ctx)
	key := authKey(ctx, info.FullMethod)
	if token != "" {
		if claims, ok := c.hit(token, key); ok {
			return handler(blueauth.ContextWithClaims(ctx, claims), req)
		}
	}
	return c.unary(ctx, req, info, func(ctx context.Context, req any) (any, error) {
		if claims, ok := blueauth.ClaimsFromContext[Extension](ctx); ok && token != "" {
			c.store(token, key, claims)
		}
		return handler(ctx, req)
	})
}

// Stream satisfies grpc.StreamServerInterceptor. On a cache hit it installs
// the cached claims and calls the handler directly; otherwise it delegates
// to the wrapped interceptor and records the verified, authorized claims.
func (c *authCache) Stream(
	srv any, ss grpc.ServerStream,
	info *grpc.StreamServerInfo, handler grpc.StreamHandler,
) error {
	token := bearerTokenFromContext(ss.Context())
	key := authKey(ss.Context(), info.FullMethod)
	if token != "" {
		if claims, ok := c.hit(token, key); ok {
			ctx := blueauth.ContextWithClaims(ss.Context(), claims)
			return handler(srv, cacheServerStream{ServerStream: ss, ctx: ctx})
		}
	}
	return c.stream(srv, ss, info, func(srv any, ss grpc.ServerStream) error {
		if claims, ok := blueauth.ClaimsFromContext[Extension](ss.Context()); ok && token != "" {
			c.store(token, key, claims)
		}
		return handler(srv, ss)
	})
}

func (c *authCache) evict() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, entry := range c.entries {
		entry.authorized = make(map[string]struct{})
	}
}

func (c *authCache) hit(token, authKey string) (blueauth.UserClaims[Extension], bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[token]
	if entry == nil {
		return blueauth.UserClaims[Extension]{}, false
	}
	if !c.now().Before(entry.expires) {
		delete(c.entries, token)
		return blueauth.UserClaims[Extension]{}, false
	}
	if _, ok := entry.authorized[authKey]; !ok {
		return blueauth.UserClaims[Extension]{}, false
	}
	return entry.claims, true
}

func (c *authCache) store(token, authKey string, claims blueauth.UserClaims[Extension]) {
	now := c.now()
	expires := now.Add(maxAuthCacheTTL)
	if claims.Expiry != nil {
		if tokenExp := claims.Expiry.Time(); tokenExp.Before(expires) {
			expires = tokenExp
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[token]
	if entry == nil || !now.Before(entry.expires) {
		entry = &authCacheEntry{authorized: make(map[string]struct{})}
		c.entries[token] = entry
	}
	entry.claims = claims
	entry.expires = expires
	entry.authorized[authKey] = struct{}{}
}

type authCacheEntry struct {
	claims     blueauth.UserClaims[Extension]
	expires    time.Time
	authorized map[string]struct{}
}

type cacheServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s cacheServerStream) Context() context.Context {
	return s.ctx
}

func bearerTokenFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	// metadata keys are normalized to lowercase.
	values := md["authorization"]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func authKey(ctx context.Context, method string) string {
	return method + "\x00" + peerCacheKey(ctx)
}

func peerCacheKey(ctx context.Context) string {
	process, ok := peerProcessFromContext(ctx)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%d\x00%d\x00%s\x00%s",
		process.PID, process.UID, process.ProgramPath(),
		strings.Join(process.ProgramArgs(), "\x00"))
}
