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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/ox-api/api/account"
	"unstable.build/go-tui/cmd/ox-api/api/user"
)

var (
	accountOne = account.Account{
		ID:            account.ID("1234"),
		Admin:         user.ID("xxxz"),
		CreatedAt:     makeFixedTime(),
		Deactivated:   true,
		DeactivatedAt: makeFixedTime(),
	}
)

func TestGetUserAccount(t *testing.T) {
	suite := []struct {
		description      string
		mockStore        func(t *testing.T) account.Store
		userID           string
		expectedStatus   int
		expectedResponse response[account.Account]
	}{
		{
			description:      "returns 200 OK with account if account with given user id exists",
			mockStore:        goodAccountStoreGetByUserID(accountOne),
			userID:           testBase64Encode("xxxz"),
			expectedStatus:   http.StatusOK,
			expectedResponse: response[account.Account]{Data: &accountOne},
		},
		{
			description:      "returns 502 if store returns unknown error",
			mockStore:        errorAccountStore,
			userID:           testBase64Encode("xxxz"),
			expectedStatus:   http.StatusBadGateway,
			expectedResponse: response[account.Account]{Message: "store get: boom"},
		},
		{
			description:      "returns 400 if user id is missing from URI",
			mockStore:        errorAccountStore,
			userID:           "",
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: response[account.Account]{Message: "missing 'user_id' param"},
		},
		{
			description:      "returns 404 if account doesn't exist",
			mockStore:        notFoundAccountStore,
			userID:           testBase64Encode("xxxz"),
			expectedStatus:   http.StatusNotFound,
			expectedResponse: response[account.Account]{Message: "store get: document not found"},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			mockStore := test.mockStore(t)
			router := setupTestRouter(mockStore, &mockUserStore{})

			mockWriter := httptest.NewRecorder()
			req := httptest.NewRequest("GET",
				fmt.Sprintf("http://127.0.0.1:8080/api/user/%s/account", test.userID), nil)

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

func TestUpdateUser(t *testing.T) {
	suite := []struct {
		description      string
		mockStore        func(t *testing.T) *mockUserStore
		userID           string
		payload          io.Reader
		expectedStatus   int
		expectedResponse response[user.User]
	}{
		{
			description: "returns 200 OK if update all fields",
			mockStore: goodUserStoreUpdate("xxxz",
				map[string]any{"email": "one", "family_name": "family", "picture": "2134", "given_name": "given"},
			),
			userID: testBase64Encode("xxxz"),
			payload: makeJSONPayload(
				map[string]any{"Email": "one", "LastName": "family", "PictureURL": "2134", "FirstName": "given"},
			),
			expectedStatus:   http.StatusOK,
			expectedResponse: response[user.User]{},
		},
		{
			description: "returns 200 OK if update one field",
			mockStore: goodUserStoreUpdate("xxxz",
				map[string]any{"email": "one"},
			),
			userID: testBase64Encode("xxxz"),
			payload: makeJSONPayload(
				map[string]any{"Email": "one"},
			),
			expectedStatus:   http.StatusOK,
			expectedResponse: response[user.User]{},
		},
		{
			description:      "returns 400 if no fields were passed",
			mockStore:        errorUserStore,
			userID:           testBase64Encode("xxxz"),
			payload:          makeJSONPayload(map[string]any{}),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: response[user.User]{Message: "All fields are empty"},
		},
		{
			description:      "returns 400 if payload is not valid json",
			mockStore:        errorUserStore,
			userID:           testBase64Encode("xxxz"),
			payload:          strings.NewReader("\x00"),
			expectedStatus:   http.StatusBadRequest,
			expectedResponse: response[user.User]{Message: "unmarshal req body: invalid character '\\x00' looking for beginning of value"},
		},
		{
			description:      "returns 502 if store returns error",
			mockStore:        errorUserStore,
			userID:           testBase64Encode("xxxz"),
			payload:          makeJSONPayload(map[string]any{"Email": "one"}),
			expectedStatus:   http.StatusBadGateway,
			expectedResponse: response[user.User]{Message: "user service update: boom"},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			router := httprouter.New()
			AddAccountHTTPHandles(router, &mockAccountStore{})
			AddUserHTTPHandles(router, &mockAccountStore{}, test.mockStore(t))

			mockWriter := httptest.NewRecorder()
			req := httptest.NewRequest("PUT",
				fmt.Sprintf("http://127.0.0.1:8080/api/user/%s", test.userID),
				test.payload)

			router.ServeHTTP(mockWriter, req)
			resp := mockWriter.Result()

			require.Equal(t, test.expectedStatus, resp.StatusCode)
			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var actualResponse response[user.User]

			require.NoError(t, json.Unmarshal(data, &actualResponse))
			assert.Equal(t, test.expectedResponse, actualResponse)
		})
	}
}

func testBase64Encode(userID string) string {
	str := base64.StdEncoding.EncodeToString([]byte(userID))
	return str
}
