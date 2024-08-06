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

package test_all

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/encoding/json"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/storage/schemedoc"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/docscheme"
)

func TestMemoryWorkspaceSchemeBackedByWorkspaceSchemeService(t *testing.T) {
	testWorkspaceSchemeSuite(t, func(t *testing.T) schemeapi.Scheme {
		workspaceURI, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)

		ctx := context.Background()
		scheme, err := workspace.NewMemoryScheme(
			ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		svc, err := schemedoc.NewDocumentService(scheme, json.Marshaler())
		require.NoError(t, err)

		s, err := docscheme.Scheme[testStruct](workspaceURI, svc, json.Marshaler(),
			errMissingID, "author")(ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)
		return s
	})
}
