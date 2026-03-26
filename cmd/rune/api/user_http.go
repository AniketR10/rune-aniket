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
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/logging/trace"
	"unstable.build/go-tui/cmd/rune/api/account"
	"unstable.build/go-tui/cmd/rune/api/user"
)

const (
	userIDParam = "user_id"
)

type userHandler struct {
	userStore user.Store
	accStore  account.Store
}

// AddUserHTTPHandles adds account and completion routes to the
// given router.
func AddUserHTTPHandles(router *httprouter.Router, accStore account.Store, userStore user.Store) {
	h := userHandler{userStore: userStore, accStore: accStore}
	router.GET(fmt.Sprintf("/api/user/:%s/account", userIDParam), h.getUserAccount)
	router.PUT(fmt.Sprintf("/api/user/:%s", userIDParam), h.updateUser)
}

func (a userHandler) getUserAccount(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
) {
	traceID, ctx := trace.FromContextOrNew(r.Context())
	attemptAt := logAttempt(r, traceID)
	userID := ps.ByName(userIDParam)
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeAccountResponse(ctx, traceID, attemptAt, w, r,
			response[account.Account]{Message: fmt.Sprintf("missing '%s' param",
				userIDParam)})
		return
	}

	decodedUserID, err := user.IDFromURIPath(userID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeUserResponse(ctx, traceID, attemptAt, w, r,
			response[user.User]{Message: fmt.Sprintf("decode user id in uri: %v", err.Error())})
		return
	}
	acc, err := a.accStore.GetByUserID(ctx, decodedUserID)
	if err != nil {
		if errors.Is(err, document.ErrNotFound) {
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

func (a userHandler) updateUser(
	w http.ResponseWriter, r *http.Request, ps httprouter.Params,
) {
	traceID, ctx := trace.FromContextOrNew(r.Context())
	attemptAt := logAttempt(r, traceID)

	userID := ps.ByName(userIDParam)
	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		writeUserResponse(ctx, traceID, attemptAt, w, r,
			response[user.User]{Message: fmt.Sprintf("missing '%s' param",
				userIDParam)})
		return
	}

	decodedUserID, err := user.IDFromURIPath(userID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeUserResponse(ctx, traceID, attemptAt, w, r,
			response[user.User]{Message: fmt.Sprintf("decode user id in uri: %v", err.Error())})
		return
	}

	// NOTE: Body is never nil
	profileUpdateStr, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeUserResponse(ctx, traceID, attemptAt, w, r, response[user.User]{
			Message: fmt.Sprintf("read request body: %v", err.Error()),
		})
		return
	}

	var req updateProfileRequest
	if err := json.Unmarshal([]byte(profileUpdateStr), &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeUserResponse(ctx, traceID, attemptAt, w, r, response[user.User]{
			Message: fmt.Sprintf("unmarshal req body: %v", err.Error()),
		})
		return
	}

	updates, msg, ok := validateUpdateProfileReq(ctx, req)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		writeUserResponse(ctx, traceID, attemptAt, w, r, response[user.User]{
			Message: msg,
		})
		return
	}

	if err := a.userStore.Update(ctx, decodedUserID, updates); err != nil {
		w.WriteHeader(http.StatusBadGateway)
		writeUserResponse(ctx, traceID, attemptAt, w, r,
			response[user.User]{Message: fmt.Sprintf("user service update: %v", err.Error())})
		return
	}

	writeUserResponse(ctx, traceID, attemptAt, w, r, response[user.User]{})
}

func validateUpdateProfileReq(ctx context.Context, req updateProfileRequest) (
	map[string]interface{}, string, bool,
) {
	if req.FirstName == "" && req.LastName == "" && req.PictureURL == "" && req.Email == "" {
		msg := "All fields are empty"
		return nil, msg, false
	}

	// https://auth0.com/docs/api/management/v2/users/patch-users-by-id
	updates := make(map[string]interface{})
	if req.LastName != "" {
		updates["family_name"] = req.LastName
	}
	if req.FirstName != "" {
		updates["given_name"] = req.FirstName
	}
	if req.PictureURL != "" {
		updates["picture"] = req.PictureURL
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	return updates, "", true
}

type updateProfileRequest struct {
	Email      string
	PictureURL string
	FirstName  string
	LastName   string
}

func writeUserResponse(
	ctx context.Context, traceID trace.ID,
	attemptAt time.Time, w http.ResponseWriter,
	req *http.Request, r response[user.User],
) {
	writeResponse(ctx, traceID, attemptAt, w, req, r, "UserID")
}
