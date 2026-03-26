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

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	blueauth "github.com/unstablebuild/blue/auth"
	"unstable.build/go-tui/cmd/rune/api/account"
	"unstable.build/go-tui/cmd/rune/api/user"
	"unstable.build/go-tui/cmd/rune/auth"
)

func TestCreateAccount(t *testing.T) {
	suite := []struct {
		description      string
		mockStore        func(t *testing.T) account.Store
		payload          io.Reader
		claims           blueauth.UserClaims[auth.WebUser]
		expectedStatus   int
		expectedResponse response[account.Account]
	}{
		{
			description: "returns 200 OK if user A tries to create account for user A",
			mockStore: goodAccountStoreCreate(
				user.User{
					ID:         "userID",
					Email:      "a@unstable.build",
					LastName:   "family",
					FirstName:  "given",
					PictureURL: "bla.com",
					Issuer:     "dev.com",
				},
				account.Account{Admin: user.ID("userID")},
				account.ID("AccountID"),
			),
			claims: makeUserClaims("userID"),
			payload: makeJSONPayload(
				map[string]any{
					"Admin": map[string]any{
						"ID":         "userID",
						"Email":      "a@unstable.build",
						"LastName":   "family",
						"PictureURL": "bla.com",
						"FirstName":  "given",
						"Issuer":     "dev.com",
					},
				},
			),
			expectedStatus: http.StatusOK,
			expectedResponse: response[account.Account]{
				Data: &account.Account{
					ID:    account.ID("AccountID"),
					Admin: user.ID("userID"),
				},
			},
		},
		{
			description: "returns 400 if user A tries to create account for user B",
			mockStore:   errorAccountStore,
			claims:      makeUserClaims("naughtyUser"),
			payload: makeJSONPayload(
				map[string]any{
					"Admin": map[string]any{
						"ID":         "userID",
						"Email":      "a@unstable.build",
						"LastName":   "family",
						"PictureURL": "bla.com",
						"FirstName":  "given",
						"Issuer":     "dev.com",
					},
				},
			),
			expectedStatus: http.StatusBadRequest,
			expectedResponse: response[account.Account]{
				Message: "naughty naughty! user is not authorized " +
					"to create an account for someone else",
			},
		},
		{
			description: "returns 400 if Admin is missing ID",
			mockStore:   errorAccountStore,
			claims:      makeUserClaims("userID"),
			payload: makeJSONPayload(
				map[string]any{
					"Admin": map[string]any{
						"Email":      "a@unstable.build",
						"LastName":   "family",
						"PictureURL": "bla.com",
						"FirstName":  "given",
						"Issuer":     "dev.com",
					},
				},
			),
			expectedStatus: http.StatusBadRequest,
			expectedResponse: response[account.Account]{
				Message: "empty 'Admin.ID' field",
			},
		},
		{
			description: "returns 400 if Admin is missing Email",
			mockStore:   errorAccountStore,
			claims:      makeUserClaims("userID"),
			payload: makeJSONPayload(
				map[string]any{
					"Admin": map[string]any{
						"ID":         "userID",
						"LastName":   "family",
						"PictureURL": "bla.com",
						"FirstName":  "given",
						"Issuer":     "dev.com",
					},
				},
			),
			expectedStatus: http.StatusBadRequest,
			expectedResponse: response[account.Account]{
				Message: "empty 'Admin.Email' field",
			},
		},
		{
			description: "returns 502 if store fails to create account",
			mockStore:   errorAccountStore,
			claims:      makeUserClaims("userID"),
			payload: makeJSONPayload(
				map[string]any{
					"Admin": map[string]any{
						"ID":         "userID",
						"Email":      "bla",
						"LastName":   "family",
						"PictureURL": "bla.com",
						"FirstName":  "given",
						"Issuer":     "dev.com",
					},
				},
			),
			expectedStatus: http.StatusBadGateway,
			expectedResponse: response[account.Account]{
				Message: "store create: boom",
			},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			mockStore := test.mockStore(t)
			router := setupTestRouter(mockStore, &mockUserStore{})

			mockWriter := httptest.NewRecorder()
			req := httptest.NewRequest("POST",
				"http://127.0.0.1:8080/api/account", test.payload)

			req = req.WithContext(blueauth.ContextWithClaims[auth.WebUser](req.Context(), test.claims))
			router.ServeHTTP(mockWriter, req)
			resp := mockWriter.Result()

			require.Equal(t, test.expectedStatus, resp.StatusCode)
			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var actualResponse response[account.Account]

			require.NoError(t, json.Unmarshal(data, &actualResponse))
			assert.Equal(t, test.expectedResponse, actualResponse)
		})
	}
}

func TestDeleteAccount(t *testing.T) {
	suite := []struct {
		description      string
		mockStore        func(t *testing.T) account.Store
		accountID        string
		payload          io.Reader
		expectedStatus   int
		expectedResponse response[account.Account]
	}{
		{
			description:      "returns 200 OK happy path",
			mockStore:        goodAccountStoreDelete("1234"),
			accountID:        "1234",
			payload:          nil,
			expectedStatus:   http.StatusOK,
			expectedResponse: response[account.Account]{},
		},
		{
			description:    "returns 502 if store errors out",
			mockStore:      errorAccountStore,
			accountID:      "1234",
			payload:        nil,
			expectedStatus: http.StatusBadGateway,
			expectedResponse: response[account.Account]{
				Message: "store delete: boom",
			},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			mockStore := test.mockStore(t)
			router := setupTestRouter(mockStore, &mockUserStore{})

			mockWriter := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE",
				fmt.Sprintf("http://127.0.0.1:8080/api/account/%s", test.accountID), test.payload)

			router.ServeHTTP(mockWriter, req)
			resp := mockWriter.Result()

			require.Equal(t, test.expectedStatus, resp.StatusCode)
			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var actualResponse response[account.Account]

			require.NoError(t, json.Unmarshal(data, &actualResponse))
			assert.Equal(t, test.expectedResponse, actualResponse)
		})
	}
}

func TestGetAccount(t *testing.T) {
	suite := []struct {
		description      string
		mockStore        func(t *testing.T) account.Store
		accountID        string
		payload          io.Reader
		expectedStatus   int
		expectedResponse response[account.Account]
	}{
		{
			description: "returns 200 OK happy path",
			mockStore: goodAccountStoreGet("1234", account.Account{
				ID:            "1234",
				Admin:         "zzzc",
				CreatedAt:     makeFixedTime(),
				DeactivatedAt: makeFixedTime(),
				Deactivated:   true,
			}),
			accountID:      "1234",
			payload:        nil,
			expectedStatus: http.StatusOK,
			expectedResponse: response[account.Account]{
				Data: &account.Account{
					ID:            "1234",
					Admin:         "zzzc",
					CreatedAt:     makeFixedTime(),
					DeactivatedAt: makeFixedTime(),
					Deactivated:   true,
				},
			},
		},
		{
			description:    "returns 502 if store errors out",
			mockStore:      errorAccountStore,
			accountID:      "1234",
			payload:        nil,
			expectedStatus: http.StatusBadGateway,
			expectedResponse: response[account.Account]{
				Message: "store get: boom",
			},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			mockStore := test.mockStore(t)
			router := setupTestRouter(mockStore, &mockUserStore{})

			mockWriter := httptest.NewRecorder()
			req := httptest.NewRequest("GET",
				fmt.Sprintf("http://127.0.0.1:8080/api/account/%s",
					test.accountID), test.payload)

			router.ServeHTTP(mockWriter, req)
			resp := mockWriter.Result()

			require.Equal(t, test.expectedStatus, resp.StatusCode)
			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var actualResponse response[account.Account]

			require.NoError(t, json.Unmarshal(data, &actualResponse))
			assert.Equal(t, test.expectedResponse, actualResponse)
		})
	}
}
