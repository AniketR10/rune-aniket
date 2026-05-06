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

package user

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"golang.org/x/oauth2/clientcredentials"
	"unstable.build/go-tui/cmd/ox-api/auth"
)

const (
	serverCertClientIDKey    = "client_id"
	serviceInitializeTimeout = 5 * time.Second
)

// NewAuth0Store allocates storage and initializes a new store. It will fetch a valid
// secret identified with secretID in the given secretStore to be used in oauth2
// client_credentials requests against the configured auth0 domain.
func NewAuth0Store(
	secretStore blueauth.SecretStore, secretID string,
	cfg auth.Config,
) (Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), serviceInitializeTimeout)
	defer cancel()

	metadata, err := secretStore.GetSecretMetadata(ctx, secretID)
	if err != nil {
		return nil, fmt.Errorf("could not get secret %s metadata to "+
			"initialize auth0 service: %v", secretID, err)
	}

	clientID, ok := metadata[serverCertClientIDKey]
	if !ok {
		return nil, fmt.Errorf("secret %q does not contain metadata %q to get client id",
			secretID, serverCertClientIDKey)
	}

	clientSecret, err := secretStore.AccessSecret(ctx, secretID)
	if err != nil {
		return nil, fmt.Errorf("could not access secret %s data: %v",
			secretID, err)
	}

	apiURL, err := url.Parse(cfg.APIURL)
	if err != nil {
		return nil, fmt.Errorf("could not parse configured API url %q: %v", cfg.APIURL, err)
	}
	conf := clientcredentials.Config{
		ClientID:       clientID,
		ClientSecret:   string(clientSecret),
		Scopes:         cfg.Scopes,
		TokenURL:       cfg.Endpoint.TokenURL,
		AuthStyle:      cfg.Endpoint.AuthStyle,
		EndpointParams: url.Values{"audience": []string{cfg.APIURL}},
	}

	client := conf.Client(context.Background())

	return &store{api: apiURL, client: client}, nil
}

var _ Store = (*store)(nil)

// store implements a service to manage auth2 users.
type store struct {
	client *http.Client
	api    *url.URL
}

func (s *store) Update(ctx context.Context, id ID, updates map[string]interface{}) (err error) {
	if len(updates) == 0 {
		panic("empty updates")
	}

	// https://auth0.com/docs/api/management/v2/users/patch-users-by-id
	const method = "PATCH"
	const callType = "auth0.UpdateUser"

	url := s.api.JoinPath("users").JoinPath(string(id)).String()
	traceID, ctx := trace.FromContextOrNew(ctx)

	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "auth0.store"},
		{Key: "Method", Value: method},
		{Key: "URL", Value: url},
	}
	attemptAt := logging.LogAttempt(traceID, callType, fields...)
	defer func() {
		logging.LogResultLevel(log.DebugLevel, log.WarnLevel, err, attemptAt,
			traceID, callType, fields...)
	}()

	fields = append(fields, logging.Field{Key: "updates", Value: strconv.Itoa(len(updates))})

	data, err := json.Marshal(updates)
	if err != nil {
		err = fmt.Errorf("marshal updates: %v", err)
		return
	}

	r, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(data))
	if err != nil {
		err = fmt.Errorf("create http request: %v", err)
		return
	}
	r.Header.Add("Content-Type", "application/json")
	resp, err := s.client.Do(r)
	if err != nil {
		err = fmt.Errorf("send http request: %v", err)
		return
	}

	defer resp.Body.Close()

	/*if data, err := io.ReadAll(resp.Body); err == nil {
		// this is to aid with debugging. Never allow it in prod as it has PII and access tokens
		fields = append(fields, logging.Field{Key: "Response", Value: string(data)})
	}*/

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("http response status: %s", resp.Status)
		return
	}

	return nil
}

func (s store) Get(ctx context.Context, id ID) (user User, err error) {
	// https://auth0.com/docs/api/management/v2/users/get-users-by-id
	const (
		method   = "GET"
		callType = "auth0.GetUser"
	)

	url := s.api.JoinPath("users").JoinPath(string(id)).String()

	data, done, err := s.doRequestExpect200(ctx, callType, method, url)
	if err != nil {
		return user, err
	}
	defer func() {
		done(err)
	}()

	var u auth0User
	err = json.Unmarshal(data, &u)
	if err != nil {
		err = fmt.Errorf("unmarshal json auth0 user: %w", err)
		return
	}

	err = u.validate()
	if err != nil {
		return
	}

	user = u.toUser(s.api.String())
	return
}

func (s store) Create(ctx context.Context, id ID, u User) error {
	actual, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	// satisfy Create semantics
	if actual.Account != "" {
		return &ErrAlreadyExists{Account: actual.Account}
	}
	// user is already created in auth0, update it with metadata
	updates := map[string]any{
		"user_metadata": map[string]any{
			"role":    int(u.Role),
			"account": u.Account,
		},
	}
	if err := s.Update(ctx, id, updates); err != nil {
		// no need to rollback a simple field update above.
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (s store) Health(ctx context.Context) error {
	// https://auth0.com/docs/api/management/v2/tenants/tenant-settings-route
	const (
		method   = "GET"
		callType = "auth0.Health"
	)
	url := s.api.JoinPath("tenants").JoinPath("settings").String()
	_, done, err := s.doRequestExpect200(ctx, callType, method, url)
	if err != nil {
		return err
	}
	done(nil)
	return nil
}

func (s store) doRequestExpect200(ctx context.Context, callType, method, url string) (
	data []byte, done func(error), err error,
) {
	traceID, ctx := trace.FromContextOrNew(ctx)

	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "auth0.store"},
		{Key: "Method", Value: method},
		{Key: "URL", Value: url},
	}
	attemptAt := logging.LogAttempt(traceID, callType, fields...)
	done = func(err error) {
		logging.LogResultLevel(log.DebugLevel, log.WarnLevel, err, attemptAt,
			traceID, callType, fields...)
	}

	r, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		err = fmt.Errorf("create http request: %v", err)
		done(err)
		return
	}
	r.Header.Add("Content-Type", "application/json")
	resp, err := s.client.Do(r)
	if err != nil {
		err = fmt.Errorf("send http request: %v", err)
		done(err)
		return
	}

	defer resp.Body.Close()
	data, err = io.ReadAll(resp.Body)
	/*if err == nil {
		// this is to aid with debugging. Never allow it in prod as it has PII, and access tokens
		fields = append(fields, logging.Field{Key: "Response", Value: string(data)})
	}*/

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("http response status: %s", resp.Status)
		data = nil
		done(err)
	}
	return
}

// https://auth0.com/docs/api/management/v2/users/get-users-by-id
type auth0User struct {
	PictureURL string `json:"picture"`
	FirstName  string `json:"given_name"`
	LastName   string `json:"family_name"`
	Verified   bool   `json:"email_verified"`

	ID    string `json:"user_id"`
	Email string `json:"email"`

	Metadata auth0UserMetadata `json:"user_metadata"`
}

func (u auth0User) validate() error {
	if u.ID == "" {
		return errors.New("user returned by auth0 has no id")
	}
	return nil
}

func (u auth0User) toUser(apiURL string) User {
	return User{
		Issuer:     apiURL,
		PictureURL: u.PictureURL,
		FirstName:  u.FirstName,
		LastName:   u.LastName,
		Verified:   u.Verified,
		ID:         ID(u.ID),
		Email:      u.Email,
		Role:       auth.Role(u.Metadata.Role),
		Account:    u.Metadata.Account,
	}
}

type auth0UserMetadata struct {
	Role    int    `json:"role"`
	Account string `json:"account"`
}
