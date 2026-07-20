// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
