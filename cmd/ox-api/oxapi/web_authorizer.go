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

package oxapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"unstable.build/go-tui/cmd/rune/api/account"
	"unstable.build/go-tui/cmd/rune/api/user"
	"unstable.build/go-tui/cmd/rune/auth"
)

// WebAuthorizer returns an auth.Authorizer of an rpc User.
func WebAuthorizer(store account.Store) blueauth.Authorizer[auth.WebUser] {
	return webAuthorizer{
		store: store,
	}
}

type webAuthorizer struct {
	store account.Store
}

func (a webAuthorizer) Authorize(
	ctx context.Context, claims blueauth.UserClaims[auth.WebUser], resource string,
) (err error) {
	// NOTE: claims here is not really our claims so the extra, non-jwt official claims
	// will be empty.
	userID := user.IDFromClaimsSubject(claims.Claims.Subject)

	const authorizeCallType = "Authorize"
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := []logging.Field{
		{Key: "userID", Value: string(userID)},
		{Key: "resource", Value: resource},
	}
	attemptAt := logging.LogAttempt(traceID, authorizeCallType, fields...)
	defer func() {
		logging.LogResultLevel(log.DebugLevel, log.WarnLevel, err, attemptAt,
			traceID, authorizeCallType, fields...)
		if err != nil { // log details, return generic error
			err = blueauth.ErrForbidden
		}
	}()

	// only POST is able to access a naked resource,
	// so authorize all authenticated users to create an account
	if resource == "/api/account" {
		return
	}

	if strings.HasPrefix(resource, "/api/account") {
		tokens := strings.Split(resource, "/")
		if len(tokens) < 4 {
			err = fmt.Errorf("invalid account path: %s", resource)
			return
		}
		err = a.validateAccount(ctx, userID, tokens[3])
		return
	}

	if strings.HasPrefix(resource, "/api/user") {
		tokens := strings.Split(resource, "/")
		if len(tokens) < 4 {
			err = fmt.Errorf("invalid user path: %s", resource)
			return
		}
		err = a.validateUser(ctx, userID, tokens[3])
		return
	}

	err = fmt.Errorf("invalid api path: %s", resource)
	return
}

func (a webAuthorizer) validateAccount(ctx context.Context, authenticatedUserID user.ID, targetAccountID string) error {
	acc, err := a.store.GetByAccountID(ctx, account.ID(targetAccountID))
	err = doValidateAccount(authenticatedUserID, acc, err)
	return err
}

func (a webAuthorizer) validateUser(ctx context.Context, authenticatedUserID user.ID, userIDFromPath string) error {
	targetUserID, err := user.IDFromURIPath(userIDFromPath)
	if err != nil {
		return fmt.Errorf("user ID from URI path: %w", err)
	}
	if authenticatedUserID != targetUserID {
		return fmt.Errorf("auth user %s not allowed to access user %s", authenticatedUserID, targetUserID)
	}
	acc, err := a.store.GetByUserID(ctx, targetUserID)
	err = doValidateAccount(targetUserID, acc, err)
	return err
}

func doValidateAccount(userID user.ID, acc account.Account, err error) error {
	if errors.Is(err, document.ErrNotFound) {
		err = fmt.Errorf("account for user %s does not exist", userID)
	}
	if err != nil {
		return err
	}
	if acc.Admin != userID {
		return fmt.Errorf("this user %s is not authorized to manage this account %s", userID, acc.ID)
	}
	return nil
}
