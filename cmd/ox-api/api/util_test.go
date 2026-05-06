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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/go-tui/cmd/ox-api/api/account"
	"unstable.build/go-tui/cmd/ox-api/api/user"
	"unstable.build/go-tui/cmd/ox-api/auth"
)

func setupTestRouter(accStore account.Store, userStore user.Store) *httprouter.Router {
	router := httprouter.New()
	AddAccountHTTPHandles(router, accStore)
	AddUserHTTPHandles(router, accStore, userStore)
	return router
}

func makeJSONPayload(m map[string]any) io.Reader {
	data, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(data)
}

func errorUserStore(t *testing.T) *mockUserStore {
	return &mockUserStore{t: t, returnErr: errors.New("boom")}
}

func goodUserStoreUpdate(userID user.ID, updates map[string]any) func(*testing.T) *mockUserStore {
	return func(t *testing.T) *mockUserStore {
		return &mockUserStore{t: t, expectUserID: userID, expectUpdates: updates}
	}
}

type mockUserStore struct {
	t             *testing.T
	expectUserID  user.ID
	expectUser    user.User
	expectUpdates map[string]any
	returnErr     error
	returnUser    user.User
}

func (m *mockUserStore) Create(_ context.Context, id user.ID, user user.User) error {
	if m.returnErr == nil {
		assert.Equal(m.t, m.expectUserID, id)
		assert.Equal(m.t, m.expectUser, user)
	}
	return m.returnErr
}

func (m *mockUserStore) Update(_ context.Context, id user.ID, updates map[string]any) error {
	if len(updates) == 0 {
		panic("empty updates")
	}
	if m.returnErr == nil {
		assert.Equal(m.t, m.expectUserID, id)
		assert.Equal(m.t, m.expectUpdates, updates)
	}
	return m.returnErr
}

func (m *mockUserStore) Get(_ context.Context, id user.ID) (user.User, error) {
	if m.returnErr == nil {
		assert.Equal(m.t, m.expectUserID, id)
	}
	return m.returnUser, m.returnErr
}

func (m *mockUserStore) Health(context.Context) error {
	return m.returnErr
}

func makeFixedTime() time.Time {
	t, err := time.Parse("2006-01-02T15:04:05.000Z", "2024-05-21T06:48:57.792Z")
	if err != nil {
		panic(err)
	}
	return t
}

func errorAccountStore(t *testing.T) account.Store {
	return &mockAccountStore{t: t, returnErr: errors.New("boom")}
}

func notFoundAccountStore(t *testing.T) account.Store {
	return &mockAccountStore{t: t, returnErr: storageapi.ErrNotFound}
}

func goodAccountStoreGetByUserID(
	returnAccount account.Account,
) func(*testing.T) account.Store {
	return func(t *testing.T) account.Store {
		return &mockAccountStore{
			t:             t,
			expectUserID:  returnAccount.Admin,
			returnAccount: returnAccount,
		}
	}
}

func goodAccountStoreCreate(
	expectUser user.User,
	expectAccount account.Account,
	returnAccountID account.ID,
) func(*testing.T) account.Store {
	return func(t *testing.T) account.Store {
		return &mockAccountStore{
			t:             t,
			expectUser:    expectUser,
			expectAccount: expectAccount,
			returnID:      returnAccountID,
		}
	}
}

func goodAccountStoreDelete(expectAccountID account.ID) func(*testing.T) account.Store {
	return func(t *testing.T) account.Store {
		return &mockAccountStore{
			t:               t,
			expectAccountID: expectAccountID,
		}
	}
}

func makeUserClaims(userID string) blueauth.UserClaims[auth.WebUser] {
	return blueauth.UserClaims[auth.WebUser]{
		UserID: userID,
		Claims: jwt.Claims{
			Subject: userID,
		},
	}
}

func goodAccountStoreGet(
	expectAccountID account.ID, returnAccount account.Account,
) func(*testing.T) account.Store {
	return func(t *testing.T) account.Store {
		return &mockAccountStore{
			t:               t,
			expectAccountID: expectAccountID,
			returnAccount:   returnAccount,
		}
	}
}

type mockAccountStore struct {
	t               *testing.T
	returnErr       error
	returnAccount   account.Account
	returnID        account.ID
	expectUser      user.User
	expectAccount   account.Account
	expectAccountID account.ID
	expectUserID    user.ID
}

func (m *mockAccountStore) Health(context.Context) error {
	return m.returnErr
}

func (m *mockAccountStore) Create(_ context.Context, user user.User, acc account.Account) (account.ID, error) {
	if m.returnErr == nil {
		assert.Equal(m.t, m.expectUser, user)
		assert.Equal(m.t, m.expectAccount, acc)
	}
	return m.returnID, m.returnErr
}

func (m *mockAccountStore) Set(_ context.Context, id account.ID, acc account.Account) error {
	if m.returnErr == nil {
		assert.Equal(m.t, m.expectAccountID, id)
		assert.Equal(m.t, m.expectAccount, acc)
	}
	return m.returnErr
}

func (m *mockAccountStore) GetByAccountID(_ context.Context, id account.ID) (account.Account, error) {
	if m.returnErr == nil {
		assert.Equal(m.t, m.expectAccountID, id)
	}
	return m.returnAccount, m.returnErr
}

func (m *mockAccountStore) GetByUserID(_ context.Context, id user.ID) (account.Account, error) {
	if m.returnErr == nil {
		assert.Equal(m.t, m.expectUserID, id)
	}
	return m.returnAccount, m.returnErr
}

func (m *mockAccountStore) Delete(_ context.Context, id account.ID) error {
	if m.returnErr == nil {
		assert.Equal(m.t, m.expectAccountID, id)
	}
	return m.returnErr
}

func (m *mockAccountStore) List(context.Context) (iterator.Iterator[account.Account], error) {
	panic("unimplemented")
}
