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

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
	"go.uber.org/atomic"
	"unstable.build/go-tui/cmd/ox-api/oxapi"
	"unstable.build/go-tui/cmd/rune/api/account"
	"unstable.build/go-tui/cmd/rune/api/user"
	"unstable.build/go-tui/cmd/rune/auth"
)

var (
	testUser = user.User{
		ID:        user.ID("goodCitizen"),
		FirstName: "Fulanitu",
		LastName:  "De Tal",
	}
	testAccount = account.Account{
		Admin: user.ID("goodCitizen"),
	}
)

func TestAuth(t *testing.T) {
	priv := loadKey(t, "./testdata/jwk-priv.json")
	pub1 := loadKey(t, "./testdata/jwk-pub1.json.pub")
	pub2 := loadKey(t, "./testdata/jwk-pub2.json.pub")
	pub3 := loadKey(t, "./testdata/jwk-pub3.json.pub")
	multiKeys := blueauth.StaticAsymmetricKeys(priv, pub1, pub2, pub3)
	validToken, err := blueauth.SignToken[auth.WebUser](priv, "goodCitizen", "citizen@unstable.build",
		auth.WebUser{}, 1*time.Hour)
	require.NoError(t, err)
	otherUserToken, err := blueauth.SignToken[auth.WebUser](priv, "otherUser", "n48thy@unstable.build",
		auth.WebUser{}, 1*time.Hour)
	require.NoError(t, err)

	suite := []struct {
		description         string
		url                 string
		method              string
		payload             io.Reader
		authorizationHeader string
		expectedStatus      int
	}{
		{
			description:         "rejects GET account request for unauthenticated user",
			url:                 "/api/account/1234",
			method:              "GET",
			authorizationHeader: "",
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "rejects DELETE account request for unauthenticated user",
			url:                 "/api/account/1234",
			method:              "DELETE",
			authorizationHeader: "",
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "rejects POST account request for unauthenticated user",
			url:                 "/api/account",
			method:              "POST",
			authorizationHeader: "",
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "rejects GET account request for authenticated user, non-admin",
			url:                 "/api/account/%s",
			method:              "GET",
			authorizationHeader: "Bearer " + otherUserToken,
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "rejects DELETE account request for authenticated user, non-admin",
			url:                 "/api/account/%s",
			method:              "DELETE",
			authorizationHeader: "Bearer " + otherUserToken,
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "rejects GET user request for authenticated user, not itself",
			url:                 fmt.Sprintf("/api/user/%s/account", testBase64Encode("goodCitizen")),
			method:              "GET",
			authorizationHeader: "Bearer " + otherUserToken,
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "rejects DELETE user request for authenticated user, not itself",
			url:                 fmt.Sprintf("/api/user/%s", testBase64Encode("goodCitizen")),
			method:              "DELETE",
			authorizationHeader: "Bearer " + otherUserToken,
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "rejects PUT user request for authenticated user, not itself",
			url:                 fmt.Sprintf("/api/user/%s", testBase64Encode("goodCitizen")),
			method:              "PUT",
			authorizationHeader: "Bearer " + otherUserToken,
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "rejects GET user request for unauthenticated user",
			url:                 fmt.Sprintf("/api/user/%s/account", testBase64Encode("goodCitizen")),
			method:              "GET",
			authorizationHeader: "",
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "rejects DELETE user request for unauthenticated user",
			url:                 fmt.Sprintf("/api/user/%s", testBase64Encode("goodCitizen")),
			method:              "DELETE",
			authorizationHeader: "",
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "rejects PUT user request for unauthenticated user",
			url:                 fmt.Sprintf("/api/user/%s", testBase64Encode("goodCitizen")),
			method:              "PUT",
			authorizationHeader: "",
			expectedStatus:      http.StatusForbidden,
		},
		{
			description:         "accepts GET account request for authenticated admin user",
			url:                 "/api/account/%s",
			method:              "GET",
			authorizationHeader: "Bearer " + validToken,
			expectedStatus:      http.StatusOK,
		},
		{
			description:         "accepts DELETE account request for authenticated admin user",
			url:                 "/api/account/%s",
			method:              "DELETE",
			authorizationHeader: "Bearer " + validToken,
			expectedStatus:      http.StatusOK,
		},
		{
			description:         "accepts POST account request for authenticated user",
			url:                 "/api/account",
			method:              "POST",
			payload:             strings.NewReader(`{"Admin": { "ID": "goodCitizen", "Email": "email", "Issuer": "bla" }}`),
			authorizationHeader: "Bearer " + validToken,
			expectedStatus:      http.StatusOK,
		},
		{
			description:         "rejects POST account request for authenticated user, if claims doesn't match user ID",
			url:                 "/api/account",
			method:              "POST",
			payload:             strings.NewReader(`{"Admin": { "ID": "naughtyCitizen", "Email": "email", "Issuer": "bla" }}`),
			authorizationHeader: "Bearer " + validToken,
			expectedStatus:      http.StatusBadRequest,
		},
		{
			description:         "accepts GET user request for authenticated user",
			url:                 fmt.Sprintf("/api/user/%s/account", testBase64Encode("goodCitizen")),
			method:              "GET",
			authorizationHeader: "Bearer " + validToken,
			expectedStatus:      http.StatusOK,
		},
		{
			description:         "accepts PUT user request for authenticated user",
			url:                 fmt.Sprintf("/api/user/%s", testBase64Encode("goodCitizen")),
			method:              "PUT",
			authorizationHeader: "Bearer " + validToken,
			payload:             strings.NewReader(`{"LastName": "Fallout"}`),
			expectedStatus:      http.StatusOK,
		},
	}
	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			apiURL, err := url.Parse("http://localhost:3000")
			require.NoError(t, err)
			signupURL, err := url.Parse("http://localhost:3000/signup")
			require.NoError(t, err)
			accDB := document.NewInMemoryService()
			secretStore := blueauth.MapSecretStore(map[string][]byte{
				"auth0-ox-api-prod": []byte("1234"), /* unused */
			}, "client_id", "xxxx")
			// used by auth0 store
			var accountID atomic.String
			addr := newDummyServer(t, &accountID)
			testAuthConfig := auth.DefaultConfig()
			testAuthConfig.APIURL = "http://" + addr + "/api"
			testAuthConfig.Endpoint.TokenURL = "http://" + addr + "/o/oauth2/token"
			testAuthConfig.Endpoint.AuthURL = "http://" + addr + "/o/oauth2/auth"
			testAuthConfig.JWKSURL = "http://" + addr + "/o/oauth2/jwk"

			mockWriter := httptest.NewRecorder()
			handler, err := oxapi.NewHTTPApi(accDB, multiKeys,
				multiKeys, secretStore, 10*time.Hour, signupURL, apiURL, testAuthConfig,
				[]oxapi.ArchRelease{{
					Arch:    "darwin-arm64",
					Manager: stubReleaseManager{},
					Signer:  stubSigner{},
				}}, oxapi.RPCAuthorizer("issues", []string{"releases"}))
			require.NoError(t, err)

			if test.url != "/api/account" {
				// setup an account for tests, except for create account test
				actualAccountID, err := handler.AccountStore().Create(context.Background(), testUser, testAccount)
				require.NoError(t, err)
				accountID.Store(string(actualAccountID))

				if strings.Contains(test.url, "%s") {
					test.url = fmt.Sprintf(test.url, actualAccountID)
				}
			}

			req := httptest.NewRequest(test.method, test.url, test.payload)
			if test.authorizationHeader != "" {
				req.Header.Set("Authorization", test.authorizationHeader)
			}

			// sut
			handler.ServeHTTP(mockWriter, req)

			resp := mockWriter.Result()
			require.Equal(t, test.expectedStatus, resp.StatusCode)
		})
	}

}

type stubReleaseManager struct{}

func (stubReleaseManager) Create(context.Context, release.Package) error { return nil }
func (stubReleaseManager) DeletePackage(context.Context, string) error   { return nil }
func (stubReleaseManager) GetPackage(context.Context, string) (release.Package, error) {
	return release.Package{}, nil
}
func (stubReleaseManager) ListPackages(context.Context, map[string]string) (iterator.Iterator[release.Package], error) {
	return iterator.FromSlice([]release.Package{}), nil
}
func (stubReleaseManager) Upload(context.Context, release.Bundle, release.ProgressReader) error {
	return nil
}
func (stubReleaseManager) Get(context.Context, string, release.Version, release.ProgressWriter) (release.Bundle, error) {
	return release.Bundle{}, nil
}
func (stubReleaseManager) Delete(context.Context, string, release.Version) error { return nil }
func (stubReleaseManager) List(context.Context, string, map[string]string) (iterator.Iterator[release.Bundle], error) {
	return iterator.FromSlice([]release.Bundle{}), nil
}

type stubSigner struct{}

func (stubSigner) SignedDownloadURL(context.Context, string, release.Version) (string, error) {
	return "http://localhost/unused", nil
}

func loadKey(t *testing.T, filename string) blueauth.Key {
	data, err := os.ReadFile(filename)
	require.NoError(t, err)

	var key blueauth.Key
	if strings.HasSuffix(filename, ".pub") {
		key, err = blueauth.LoadPublicKey(data)
	} else {
		key, err = blueauth.LoadPrivateKey(data)
	}
	require.NoError(t, err)
	return key
}

func newDummyServer(t *testing.T, accountID *atomic.String) string {
	listener, err := newTCPListener()
	require.NoError(t, err)

	go func(t *testing.T) {
		_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "oauth2") {
				// always return a fake token so auth0 oauth2 client doesn't complain.
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`
{
  "access_token":"MTQ0NjJkZmQ5OTM2NDE1ZTZjNGZmZjI3",
  "token_type":"Bearer",
  "expires_in":3600,
  "scope":"create"
}
`))
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(
				`{"user_id":"goodCitizen", "user_metadata": {"account": "%s"}}`, accountID.Load())))
		}))
		// require.Equal(t, http.ErrServerClosed, err)
	}(t)

	t.Cleanup(func() {
		listener.Close()
	})

	return listener.Addr().String()
}

func newTCPListener() (net.Listener, error) {
	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:")
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	return listener, nil
}

func testBase64Encode(userID string) string {
	str := base64.StdEncoding.EncodeToString([]byte(userID))
	return str
}
