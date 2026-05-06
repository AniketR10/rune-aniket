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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"golang.org/x/oauth2"
)

const (
	// ServeTokenPath is the path at which oauth2 token redemption is performed.
	ServeTokenPath = "/o/oauth2/token"

	// ServeConfigPath is the path at which oauth2 configuration is served over http.
	ServeConfigPath = "/o/oauth2/config"

	authStyle = oauth2.AuthStyleAutoDetect
	apiURL    = "https://dev-fv7z5qrer6vkxhxf.us.auth0.com/api/v2/"
	tokenURL  = "https://dev-fv7z5qrer6vkxhxf.us.auth0.com/oauth/token"
	authURL   = "https://dev-fv7z5qrer6vkxhxf.us.auth0.com/authorize"
	jWKSURL   = "https://dev-fv7z5qrer6vkxhxf.us.auth0.com/.well-known/jwks.json"
	clientID  = "AhY5YlLUiEjOXNFmmyw4Nve32Hp0ag22"
	signupURL = "https://rune.dev/signup"
)

// Config represents a full oauth2 configuration for clients and servers to use.
type Config struct {
	APIURL    string
	SignupURL string
	JWKSURL   string
	oauth2.Config
}

// DefaultConfig serves the default's oauth2 provider config for a backend service
// to communicate against an oauth2 provier's API.
func DefaultConfig() Config {
	return Config{
		APIURL:    apiURL,
		JWKSURL:   jWKSURL,
		SignupURL: signupURL,
		Config: oauth2.Config{
			ClientID: clientID,
			Scopes:   []string{"read:users", "update:users", "create:users", "delete:users"},
			Endpoint: oauth2.Endpoint{
				AuthStyle: authStyle,
				AuthURL:   authURL,
				TokenURL:  tokenURL,
			},
		},
	}
}

// DefaultNativeConfig returns the auth configuration for both server and native client.
//
// To deploy a new oauth2 provider, domain or simply a new private key the following
// steps must be done:
//
//  1. Create a new secret via bluectl cli. Be sure to add the certs_url, redeem_url
//     and login_url metadata keys for the secret:
//     - bluectl secret create -d "certs_url=<url>" -d "redeem_url=<url>" -d "login_url=<url>" <client-id>
//  2. Add the secret's secret via:
//     - bluectl secret rotate <client_id> <file_with_secret>
//  3. Update this configuration with the new JWKSURL, ClientID, AuthURL,
//     (TokenURL and Scopes too, if applicable).
//  4. Deploy a new version of the ox-api. All clients will pick up the new configuration
//     via FetchConfig.
func DefaultNativeConfig(api *url.URL) Config {
	tokenURL := api.JoinPath(ServeTokenPath).String()
	return Config{
		APIURL:    apiURL,
		JWKSURL:   jWKSURL,
		SignupURL: signupURL,
		Config: oauth2.Config{
			ClientID: clientID,
			Scopes: []string{
				"offline_access", /* ensure it returns a refresh token */
				"openid",         /* to ensure it returns an id token */
			},
			Endpoint: oauth2.Endpoint{
				AuthStyle: oauth2.AuthStyleInParams,
				AuthURL:   authURL,
				TokenURL:  tokenURL,
			},
		},
	}
}

// FetchConfig fetches the config at the given api url on ServeConfigPath and
// returns a valid configuration or an error.
func FetchConfig(api *url.URL) (Config, error) {
	var client http.Client
	client.Timeout = 5 * time.Second
	resp, err := client.Get(api.JoinPath(ServeConfigPath).String())
	if err != nil {
		return Config{}, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Config{}, errors.New("http get returned non 200 status")
	}

	var cfg Config
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config json: %v", err)
	}

	if err := validateConfig(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// ServeNativeConfig returns an http.Handler that serves DefaultConfig.
func ServeNativeConfig(logger *log.Logger, api *url.URL) (http.Handler, error) {
	const httpCallType = "ServeConfig"

	cfg := DefaultNativeConfig(api)
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("could not marshal config: %v", err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID, _ := trace.FromContextOrNew(r.Context())
		fields := []logging.Field{
			{Key: logging.KeyClass, Value: "auth.funcConfigHandler"},
			{Key: "Method", Value: r.Method},
			{Key: "URL", Value: r.URL.String()},
		}
		attemptAt := logging.LogAttempt(traceID, httpCallType, fields...)

		if r.Method != http.MethodGet {
			status := http.StatusBadRequest
			w.WriteHeader(status)
			fields = append(fields, logging.Field{Key: "Status", Value: http.StatusText(status)})
			logging.LogResult(nil, attemptAt, traceID, httpCallType, fields...)
			return
		}
		_, err = w.Write(data)
		logging.LogResult(err, attemptAt, traceID, httpCallType, fields...)
	}), nil
}

func validateConfig(cfg Config) error {
	if cfg.Config.ClientID == "" || cfg.Config.Endpoint.AuthURL == "" ||
		cfg.Config.Endpoint.TokenURL == "" {
		return errors.New("unusable oauth2 config: missing key fields")
	}
	return nil
}
