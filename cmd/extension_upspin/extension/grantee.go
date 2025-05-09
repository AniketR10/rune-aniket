// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package extension

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/api/schemeapi"
	"unstable.build/go-tui/api/schemeapi/schemeext"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/rpc"
)

var (
	requiredPermissions = []extension.Permission{
		extension.PermissionSchemeManager,
	}
)

// Grantee returns this extension's Grantee and the permission required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	return new(upspinGrantee), requiredPermissions
}

type upspinGrantee struct {
	mu     sync.Mutex
	broker rpc.MuxBroker
	m      schemeapi.SchemeManager
}

func (e *upspinGrantee) Connected(
	ctx context.Context, broker rpc.MuxBroker, pconfig config.Config,
) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Debugf("extension connected; config: %#v", pconfig)
	e.broker = broker

	return nil
}

func (e *upspinGrantee) PermissionGranted(
	ctx context.Context, grants []extension.Grant,
) error {
	log.Debugf("permissions granted: %v", grants)

	for _, g := range grants {
		switch g.Permission {
		case extension.PermissionSchemeManager:
			m, err := schemeext.SchemeManager(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire scheme manager: %w", err)
			}
			err = m.RegisterScheme(upspinScheme, newScheme)
			if err != nil && err != schemeapi.ErrSchemeAlreadyRegistered {
				return fmt.Errorf("register scheme: %w", err)
			}
			// store so finalizer doesn't kill the scheme RPC pipeline
			e.m = m
		}
	}
	return nil
}

func (e *upspinGrantee) PermissionDenied(
	ctx context.Context, perms []extension.Permission,
) error {
	return fmt.Errorf("missing critical permissions: "+
		"denied: %v; required: %v", perms, requiredPermissions)
}

func (e *upspinGrantee) Shutdown(ctx context.Context, reason string) error {
	log.Debugf("extension being shutdown: %s", reason)
	return nil
}

func (e *upspinGrantee) Health(context.Context) error {
	return nil
}
