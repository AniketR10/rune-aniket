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
	"fmt"

	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"unstable.build/go-tui/cmd/rune/api/user"
	"unstable.build/go-tui/cmd/rune/auth"
)

// newGranter returns a Granter of RPCUser.
func newGranter(userStore user.Store, signupURL string) blueauth.Granter[auth.RPCUser] {
	return granter{userStore: userStore, signupURL: signupURL}
}

type granter struct {
	signupURL string
	userStore user.Store
}

func (a granter) Grant(ctx context.Context, claims *blueauth.ProviderClaims) (auth.RPCUser, error) {
	const grantCallType = "Grant"
	traceID, ctx := trace.FromContextOrNew(ctx)
	fields := []logging.Field{
		{Key: "Claims.Subject", Value: claims.Subject},
		{Key: "Claims.Issuer", Value: claims.Issuer},
	}
	attemptAt := logging.LogAttempt(traceID, grantCallType, fields...)
	user, err := a.userStore.Get(ctx, user.IDFromClaimsSubject(claims.Subject))
	if err != nil {
		err = fmt.Errorf("user store get: %w", err)
		logging.LogResult(err, attemptAt, traceID, grantCallType, fields...)
		return auth.RPCUser{}, err
	}

	fields = append(fields, logging.Field{Key: "Role", Value: user.Role.String()})
	fields = append(fields, logging.Field{Key: "Account", Value: user.Account})
	var ret auth.RPCUser
	if user.Role == 0 {
		ret = makeUnregisteredUser(claims)
	} else {
		ret = makeUserFromBackendUser(user)
	}

	logging.LogResult(err, attemptAt, traceID, grantCallType, fields...)
	return ret, nil
}

func makeUnregisteredUser(claims *blueauth.ProviderClaims) auth.RPCUser {
	return auth.RPCUser{
		ID:      claims.Subject,
		Email:   claims.Email,
		Role:    auth.RoleBasic,
		Account: "",
	}
}

func makeUserFromBackendUser(user user.User) auth.RPCUser {
	return auth.RPCUser{
		ID:      string(user.ID),
		Email:   string(user.Email),
		Role:    user.Role,
		Account: user.Account,
	}
}
