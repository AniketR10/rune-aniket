// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package webfetch

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// SafeClient is an HTTP client with SSRF protection.
type SafeClient struct {
	client        *http.Client
	maxRedirects  int
	userAgent     string
	allowLoopback bool
}

// NewSafeClient creates a SafeClient with SSRF-safe settings.
func NewSafeClient(cfg Config) *SafeClient {
	allowLB := cfg.AllowLoopback
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		DialContext: func(
			ctx context.Context, network, addr string,
		) (net.Conn, error) {
			return safeDialContext(
				ctx, dialer, network, addr, allowLB,
			)
		},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: cfg.Timeout,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
		// Disable automatic redirects; we follow them
		// manually so each hop is validated through the
		// SSRF pipeline.
		CheckRedirect: func(
			_ *http.Request, _ []*http.Request,
		) error {
			return http.ErrUseLastResponse
		},
	}

	return &SafeClient{
		client:        client,
		maxRedirects:  cfg.MaxRedirects,
		userAgent:     cfg.UserAgent,
		allowLoopback: cfg.AllowLoopback,
	}
}

// Do executes an HTTP GET request with SSRF protection and
// manual redirect following.
func (sc *SafeClient) Do(
	ctx context.Context, rawURL string,
) (*http.Response, error) {
	current := rawURL
	for i := range sc.maxRedirects + 1 {
		if err := validateURL(current, sc.allowLoopback); err != nil {
			return nil, fmt.Errorf(
				"blocked URL (hop %d): %w", i, err,
			)
		}

		req, err := http.NewRequestWithContext(
			ctx, http.MethodGet, current, nil,
		)
		if err != nil {
			return nil, fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("User-Agent", sc.userAgent)
		req.Header.Set("Accept",
			"text/html,application/xhtml+xml,"+
				"application/xml;q=0.9,*/*;q=0.8")

		resp, err := sc.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("do request: %w", err)
		}

		if !isRedirect(resp.StatusCode) {
			return resp, nil
		}
		resp.Body.Close() //nolint:errcheck

		loc := resp.Header.Get("Location")
		if loc == "" {
			return nil, fmt.Errorf(
				"redirect %d with no Location header",
				resp.StatusCode,
			)
		}

		resolved, err := resolveRedirect(current, loc)
		if err != nil {
			return nil, fmt.Errorf(
				"resolve redirect: %w", err,
			)
		}
		current = resolved
	}

	return nil, fmt.Errorf(
		"too many redirects (max %d)", sc.maxRedirects,
	)
}

func validateURL(rawURL string, allowLoopback bool) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty hostname")
	}

	if !allowLoopback && isBlockedHostname(host) {
		return fmt.Errorf("blocked hostname: %s", host)
	}

	return nil
}

func isBlockedHostname(host string) bool {
	lower := strings.ToLower(host)
	blocked := []string{
		"localhost",
		"metadata.google.internal",
		// known cloud metadata IP
		"169.254.169.254",
	}
	if slices.Contains(blocked, lower) {
		return true
	}

	suffixes := []string{
		".local",
		".internal",
		".localhost",
	}
	for _, s := range suffixes {
		if strings.HasSuffix(lower, s) {
			return true
		}
	}

	return false
}

// safeDialContext resolves the hostname and validates all
// resolved IPs before connecting.
func safeDialContext(
	ctx context.Context, dialer *net.Dialer,
	network, addr string, allowLoopback bool,
) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("split host/port: %w", err)
	}

	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no IPs for host %s", host)
	}

	if !allowLoopback {
		for _, ip := range ips {
			if isBlockedIP(ip.IP) {
				return nil, fmt.Errorf(
					"blocked IP %s for host %s",
					ip.IP, host,
				)
			}
		}
	}

	// DNS pinning: connect directly to validated IP.
	target := net.JoinHostPort(ips[0].IP.String(), port)
	return dialer.DialContext(ctx, network, target)
}

func isBlockedIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"0.0.0.0/8",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	// Check IPv4-mapped IPv6 addresses (::ffff:x.x.x.x).
	if ip4 := ip.To4(); ip4 != nil && !ip.Equal(ip4) {
		return isBlockedIP(ip4)
	}

	return false
}

func isRedirect(code int) bool {
	return code == 301 || code == 302 ||
		code == 303 || code == 307 || code == 308
}

func resolveRedirect(
	base, location string,
) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	locURL, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(locURL).String(), nil
}
