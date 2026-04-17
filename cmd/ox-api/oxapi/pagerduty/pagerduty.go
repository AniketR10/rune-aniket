// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2025 Unstable Build, All Rights Reserved.
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

// Package pagerduty implements oxapi.Pager using PagerDuty Events API v2.
package pagerduty

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	blueauth "github.com/unstablebuild/blue/auth"
	"unstable.build/go-tui/cmd/ox-api/oxapi"
)

const (
	routingKeySecretID = "pagerduty-routing-key-prod"
	eventsURL          = "https://events.pagerduty.com/v2/enqueue"
	defaultTimeout     = 10 * time.Second
)

type pager struct {
	secretStore blueauth.SecretStore
	endpoint    string
	client      *http.Client
}

// New creates a PagerDuty-backed oxapi.Pager. The PagerDuty Events API routing
// key is read from secretStore when a page is sent. If client is nil, a client
// with a conservative timeout is used.
func New(secretStore blueauth.SecretStore) oxapi.Pager {
	return newWithEndpoint(secretStore, eventsURL, nil)
}

func newWithEndpoint(
	secretStore blueauth.SecretStore, endpoint string, client *http.Client,
) oxapi.Pager {
	if secretStore == nil {
		panic("pagerduty.New: secretStore must not be nil")
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		panic("pagerduty.New: endpoint must not be empty")
	}
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &pager{
		secretStore: secretStore,
		endpoint:    endpoint,
		client:      client,
	}
}

func (p *pager) Page(ctx context.Context, page oxapi.Page) error {
	routingKey, err := p.routingKey(ctx)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(newEvent(routingKey, page)); err != nil {
		return fmt.Errorf("encode pagerduty event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, &body)
	if err != nil {
		return fmt.Errorf("new pagerduty request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send pagerduty event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return fmt.Errorf("pagerduty event status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

func (p *pager) routingKey(ctx context.Context) (string, error) {
	routingKeyBytes, err := p.secretStore.AccessSecret(ctx, routingKeySecretID)
	if err != nil {
		return "", fmt.Errorf("access pagerduty routing key secret %q: %w", routingKeySecretID, err)
	}
	routingKey := strings.TrimSpace(string(routingKeyBytes))
	if routingKey == "" {
		return "", fmt.Errorf("pagerduty routing key secret %q is empty", routingKeySecretID)
	}
	return routingKey, nil
}

type event struct {
	RoutingKey  string  `json:"routing_key"`
	EventAction string  `json:"event_action"`
	DedupKey    string  `json:"dedup_key,omitempty"`
	Payload     payload `json:"payload"`
	Links       []link  `json:"links,omitempty"`
}

type payload struct {
	Summary       string         `json:"summary"`
	Source        string         `json:"source"`
	Severity      string         `json:"severity"`
	Component     string         `json:"component,omitempty"`
	Group         string         `json:"group,omitempty"`
	Class         string         `json:"class,omitempty"`
	CustomDetails map[string]any `json:"custom_details,omitempty"`
}

type link struct {
	Href string `json:"href"`
	Text string `json:"text,omitempty"`
}

func newEvent(routingKey string, page oxapi.Page) event {
	links := make([]link, 0, len(page.Links))
	for _, pageLink := range page.Links {
		links = append(links, link{
			Href: pageLink.Href,
			Text: pageLink.Text,
		})
	}

	return event{
		RoutingKey:  routingKey,
		EventAction: "trigger",
		DedupKey:    strings.TrimSpace(page.DedupKey),
		Payload: payload{
			Summary:       defaultString(page.Summary, "Rune page"),
			Source:        defaultString(page.Source, "ox-api"),
			Severity:      defaultString(page.Severity, oxapi.PageSeverityError),
			Component:     strings.TrimSpace(page.Component),
			Group:         strings.TrimSpace(page.Group),
			Class:         strings.TrimSpace(page.Class),
			CustomDetails: page.Details,
		},
		Links: links,
	}
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
