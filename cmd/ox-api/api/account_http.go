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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/go-tui/cmd/ox-api/api/account"
	"unstable.build/go-tui/cmd/ox-api/api/user"
	"unstable.build/go-tui/cmd/ox-api/auth"
)

const (
	accountIDParam = "account_id"
)

type accountHandler struct {
	store account.Store
}

// AddAccountHTTPHandles adds account and completion routes to the
// given router.
func AddAccountHTTPHandles(router *httprouter.Router, store account.Store) {
	h := accountHandler{store: store}
	router.POST("/api/account", h.createAccount)
	router.DELETE(fmt.Sprintf("/api/account/:%s", accountIDParam), h.deleteAccount)
	router.GET(fmt.Sprintf("/api/account/:%s", accountIDParam), h.getAccount)
}

func (a accountHandler) getAccount(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
) {
	traceID, ctx := trace.FromContextOrNew(r.Context())
	attemptAt := logAttempt(r, traceID)
	id := ps.ByName(accountIDParam)
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeAccountResponse(ctx, traceID, attemptAt, w, r,
			response[account.Account]{Message: fmt.Sprintf("missing '%s' param",
				accountIDParam)})
		return
	}

	acc, err := a.store.GetByAccountID(ctx, account.ID(id))
	if err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}
		writeAccountResponse(ctx, traceID, attemptAt, w, r,
			response[account.Account]{Message: fmt.Sprintf("store get: %v", err.Error())})
		return
	}

	writeAccountResponse(ctx, traceID, attemptAt, w, r, response[account.Account]{
		Data: &acc,
	})
}

func (a accountHandler) createAccount(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
) {
	traceID, ctx := trace.FromContextOrNew(r.Context())
	attemptAt := logAttempt(r, traceID)

	// NOTE: Body is never nil
	accountStr, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeAccountResponse(ctx, traceID, attemptAt, w, r, response[account.Account]{
			Message: fmt.Sprintf("read request body: %v", err.Error()),
		})
		return
	}

	var req createAccountReq
	if err := json.Unmarshal([]byte(accountStr), &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeAccountResponse(ctx, traceID, attemptAt, w, r, response[account.Account]{
			Message: fmt.Sprintf("unmarshal account: %v", err.Error()),
		})
		return
	}

	msg, ok := validateCreateAccountReq(ctx, req)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		writeAccountResponse(ctx, traceID, attemptAt, w, r, response[account.Account]{
			Message: msg,
		})
		return
	}

	acc := account.Account{
		Admin: req.Admin.ID,
	}
	admin := req.Admin
	id, err := a.store.Create(ctx, admin, acc)
	if err != nil {
		if errors.Is(err, storageapi.ErrAlreadyExists) {
			w.WriteHeader(http.StatusConflict)
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}
		writeAccountResponse(ctx, traceID, attemptAt, w, r,
			response[account.Account]{Message: fmt.Sprintf("store create: %v", err.Error())})
		return
	}

	acc.ID = id

	writeAccountResponse(ctx, traceID, attemptAt, w, r, response[account.Account]{
		Data: &acc,
	})
}

func (a accountHandler) deleteAccount(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
) {
	traceID, ctx := trace.FromContextOrNew(r.Context())
	attemptAt := logAttempt(r, traceID)

	id := ps.ByName(accountIDParam)
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeAccountResponse(ctx, traceID, attemptAt, w, r,
			response[account.Account]{Message: fmt.Sprintf("missing '%s' param",
				accountIDParam)})
		return
	}

	if err := a.store.Delete(ctx, account.ID(id)); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		writeAccountResponse(ctx, traceID, attemptAt, w, r,
			response[account.Account]{Message: fmt.Sprintf("store delete: %v", err.Error())})
		return
	}

	writeAccountResponse(ctx, traceID, attemptAt, w, r, response[account.Account]{})
}

func logAttempt(req *http.Request, traceID trace.ID) time.Time {
	fields := []logging.Field{
		{Key: logging.KeyClass, Value: "account.accountHandler"},
		{Key: "Method", Value: req.Method},
		{Key: "URL", Value: req.URL.String()},
	}
	return logging.LogAttempt(traceID, httpCallType, fields...)
}

func validateCreateAccountReq(ctx context.Context, req createAccountReq) (msg string, ok bool) {
	if req.Admin.ID == "" {
		msg = "empty 'Admin.ID' field"
		return
	}

	if req.Admin.Email == "" {
		msg = "empty 'Admin.Email' field"
		return
	}

	if req.Admin.Issuer == "" {
		msg = "empty 'Admin.Issuer' field"
		return
	}
	var claims blueauth.UserClaims[auth.WebUser]
	claims, ok = blueauth.ClaimsFromContext[auth.WebUser](ctx)
	if !ok {
		return
	}
	ok = false
	// this endpoint requires extra authorization steps; WebAuthorizer
	// has no access to the req payload, so a malicious user could pass authorization over
	// POST to /api/account and attempt to create an account for some other user.
	if user.IDFromClaimsSubject(claims.Subject) != req.Admin.ID {
		msg = "naughty naughty! user is not authorized to create an account for someone else"
		return
	}

	ok = true
	return
}

type createAccountReq struct {
	Admin user.User
}

func writeAccountResponse(
	ctx context.Context, traceID trace.ID,
	attemptAt time.Time, w http.ResponseWriter,
	req *http.Request, r response[account.Account],
) {
	writeResponse(ctx, traceID, attemptAt, w, req, r, "AccountID")
}
