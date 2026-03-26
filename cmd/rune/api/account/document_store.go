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

package account

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"unstable.build/go-tui/cmd/rune/api/user"
	"unstable.build/go-tui/cmd/rune/auth"
)

const (
	createCallType    = "Create"
	updateCallType    = "Update"
	getCallType       = "Get"
	deleteCallType    = "Delete"
	storeLoggingClass = "account.store"
)

// NewDocumentStore allocates storage for a new Store and
// initializes it with the given account document.Service,
// and user.Store.
func NewDocumentStore(accSvc document.Service, userStore user.Store) Store {
	return store{accSvc: accSvc, userStore: userStore}
}

var _ Store = store{}

type store struct {
	accSvc    document.Service
	userStore user.Store
}

func (s store) Health(ctx context.Context) error {
	err := s.accSvc.Delete(ctx, "IDThatWillNeverExist")
	if err != nil {
		return fmt.Errorf("firestore: %w", err)
	}
	return nil
}

func (s store) Create(
	ctx context.Context,
	admin user.User,
	account Account,
) (ID, error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	id := uuid.New().String()
	fields := makeStoreLoggingFields(storeLoggingClass, id)
	attemptAt := logging.LogAttempt(traceID, createCallType, fields...)
	now := time.Now()

	doc := account
	doc.ID = ID(id)
	doc.CreatedAt = now
	doc.Deactivated = false
	doc.DeactivatedAt = time.Time{}
	doc.Admin = admin.ID
	userDoc := admin
	userDoc.Role = auth.RoleAdmin
	userDoc.Account = id
	userDoc.CreatedAt = now

	err := s.userStore.Create(ctx, userDoc.ID, userDoc)
	// idempotent user creation, handles case where acc create fails below
	// and user retries but user already created. Also, a user should
	// in principle be able to manage multiple accounts.
	if alreadyExists, ok := err.(*user.ErrAlreadyExists); ok {
		var createdAcc Account
		gerr := s.accSvc.Get(ctx, alreadyExists.Account, &createdAcc)
		if gerr != nil && !errors.Is(gerr, document.ErrNotFound) {
			return "", fmt.Errorf("user was already created but get account returned: %w", gerr)
		}
		if gerr == nil {
			return "", document.ErrAlreadyExists
		}
		// re-use id of previously created user
		// if multiple goroutines reach this point at the same time,
		// it should be fine as only one of them will succeed Create call below.
		doc.ID = ID(alreadyExists.Account)
		err = nil
	}
	if err != nil {
		return "", fmt.Errorf("user service create: %w", err)
	}

	err = s.accSvc.Create(ctx, string(doc.ID), &doc)
	logging.LogResult(err, attemptAt, traceID, createCallType, fields...)
	if err != nil {
		return "", fmt.Errorf("account service create: %w", err)
	}

	return ID(doc.ID), nil
}

func (s store) GetByUserID(
	ctx context.Context, userID user.ID,
) (Account, error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeStoreLoggingFields(storeLoggingClass, string(userID))
	attemptAt := logging.LogAttempt(traceID, getCallType, fields...)

	doc, err := s.userStore.Get(ctx, userID)
	if err != nil {
		logging.LogResult(err, attemptAt, traceID, getCallType, fields...)
		return Account{}, fmt.Errorf("document user get: %w", err)
	}

	var ret Account
	err = s.accSvc.Get(ctx, doc.Account, &ret)
	logging.LogResult(err, attemptAt, traceID, getCallType, fields...)
	if err != nil {
		return Account{}, fmt.Errorf("document account get: %w", err)
	}
	if ret.Deactivated {
		return Account{}, document.ErrNotFound
	}
	return ret, nil
}

func (s store) GetByAccountID(
	ctx context.Context, id ID,
) (Account, error) {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeStoreLoggingFields(storeLoggingClass, string(id))
	attemptAt := logging.LogAttempt(traceID, getCallType, fields...)

	var doc Account
	err := s.accSvc.Get(ctx, string(id), &doc)
	logging.LogResult(err, attemptAt, traceID, getCallType, fields...)
	if err != nil {
		return Account{}, fmt.Errorf("document service get: %w", err)
	}
	if doc.Deactivated {
		return Account{}, document.ErrNotFound
	}
	return doc, nil
}

func (s store) Delete(
	ctx context.Context, id ID,
) error {
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := makeStoreLoggingFields(storeLoggingClass, string(id))
	attemptAt := logging.LogAttempt(traceID, deleteCallType, fields...)

	acc, err := s.GetByAccountID(ctx, id)
	if err != nil {
		if errors.Is(err, document.ErrNotFound) {
			return document.ErrNotFound
		} else {
			return fmt.Errorf("get account by id: %w", err)
		}
	}

	if acc.Deactivated {
		return nil
	}

	acc.Deactivated = true
	acc.DeactivatedAt = time.Now()

	err = s.accSvc.Delete(ctx, string(id))
	logging.LogResult(err, attemptAt, traceID, deleteCallType, fields...)
	if err != nil {
		return fmt.Errorf("document service delete: %w", err)
	}
	return nil
}

func (s store) List(ctx context.Context) (iterator.Iterator[Account], error) {
	it, err := s.accSvc.List(ctx, []document.Filter{
		{Field: document.Field{FieldPath: []string{"Deactivated"}, Value: false}},
	})
	if err != nil {
		return nil, err
	}
	return iterator.FromDocumentIterator[Account](it), nil
}

func makeStoreLoggingFields(class string, id string) []logging.Field {
	return []logging.Field{
		{Key: logging.KeyClass, Value: class},
		{Key: "AccountID", Value: id},
	}
}
