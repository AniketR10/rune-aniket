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
	"fmt"

	log "github.com/sirupsen/logrus"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"unstable.build/go-tui/cmd/rune/auth"
)

// RPCAuthorizer returns an auth.Authorizer of an rpc User.
func RPCAuthorizer(issuesCollection, releaseCollection string) blueauth.Authorizer[auth.RPCUser] {
	return authorizer{
		paths: map[string]func(blueauth.UserClaims[auth.RPCUser]) bool{
			"/api/health": func(u blueauth.UserClaims[auth.RPCUser]) bool { return true },
			fmt.Sprintf("/proto.DocumentStore.%s/Create", issuesCollection): roleGreaterBasic,
			fmt.Sprintf("/proto.DocumentStore.%s/Update", issuesCollection): roleGreaterBasic,
			fmt.Sprintf("/proto.DocumentStore.%s/Set", issuesCollection):    roleGreaterBasic,
			fmt.Sprintf("/proto.DocumentStore.%s/Get", issuesCollection):    roleGreaterBasic,
			fmt.Sprintf("/proto.DocumentStore.%s/List", issuesCollection):   roleGreaterBasic,
			// needed for .swp file manipulation
			fmt.Sprintf("/proto.DocumentStore.%s/Delete", issuesCollection): roleGreaterBasic,

			fmt.Sprintf("/proto.DocumentStore.%s/Create", releaseCollection): roleIsAdmin,
			fmt.Sprintf("/proto.DocumentStore.%s/Update", releaseCollection): roleIsAdmin,
			fmt.Sprintf("/proto.DocumentStore.%s/Set", releaseCollection):    roleIsAdmin,
			fmt.Sprintf("/proto.DocumentStore.%s/Get", releaseCollection):    roleGreaterBasic,
			fmt.Sprintf("/proto.DocumentStore.%s/List", releaseCollection):   roleGreaterBasic,
			fmt.Sprintf("/proto.DocumentStore.%s/Delete", releaseCollection): roleIsAdmin,
			"/workspace.Scheme/URI":      roleGreaterBasic,
			"/workspace.Scheme/Open":     roleGreaterBasic,
			"/workspace.Scheme/OpenFile": roleGreaterBasic,
			"/workspace.Scheme/Create":   roleGreaterBasic,
			"/workspace.Scheme/Remove":   roleGreaterBasic,
			"/workspace.Scheme/Rename":   roleGreaterBasic,
			"/workspace.Scheme/Stat":     roleGreaterBasic,
			"/workspace.Scheme/ReadLink": roleGreaterBasic,
			"/workspace.Scheme/ReadDir":  roleGreaterBasic,
			"/workspace.Scheme/Root":     roleGreaterBasic,
			"/workspace.Scheme/Join":     roleGreaterBasic,
			"/workspace.Scheme/TempFile": roleGreaterBasic,
			"/workspace.Scheme/Symlink":  roleGreaterBasic,
			"/workspace.Scheme/Chroot":   roleGreaterBasic,
		},
	}
}

type authorizer struct {
	paths map[string]func(blueauth.UserClaims[auth.RPCUser]) bool
}

func (a authorizer) Authorize(
	ctx context.Context, claims blueauth.UserClaims[auth.RPCUser], resource string,
) (err error) {
	const authorizeCallType = "Authorize"
	traceID, _ := trace.FromContextOrNew(ctx)
	fields := []logging.Field{
		{Key: "userID", Value: claims.UserID},
		{Key: "resource", Value: resource},
	}
	attemptAt := logging.LogAttempt(traceID, authorizeCallType, fields...)
	defer func() {
		logging.LogResultLevel(log.DebugLevel, log.WarnLevel, err, attemptAt,
			traceID, authorizeCallType, fields...)
	}()
	authFn, ok := a.paths[resource]
	if !ok || !authFn(claims) {
		err = blueauth.ErrForbidden
		return err
	}
	return nil
}

func roleGreaterBasic(u blueauth.UserClaims[auth.RPCUser]) bool {
	return u.Extra.Role > auth.RoleBasic
}

func roleIsAdmin(u blueauth.UserClaims[auth.RPCUser]) bool {
	return u.Extra.Role == auth.RoleAdmin
}
