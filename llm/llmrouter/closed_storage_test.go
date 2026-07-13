// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

// Unstable Build LLC ("COMPANY") CONFIDENTIAL
package llmrouter

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"unstable.build/go-tui/llm"
)

// closedRootStorage mimics a firstmover-backed storage whose root Service
// has been closed: every Get returns the exact sentinel the host surfaces.
type closedRootStorage struct {
	storageapi.Service
}

func (closedRootStorage) Get(context.Context, string, any) error {
	return errors.New("firstmover: Partition on closed Service")
}

// TestGetModel_PropagatesClosedStorageError reproduces the rune-agent start
// failure surfaced by `extensions info rune-agent`. The agent resolves its
// default model during ExtendWorkspace via the host LLM router's GetModel,
// which reads the alias doc from the host's main storage. If that storage is
// closed, GetModel must surface "Partition on closed Service" — which the SDK
// turns into a fatal extension exit, recorded as the extension's Last error.
func TestGetModel_PropagatesClosedStorageError(t *testing.T) {
	cfg := llm.DefaultConfig()
	cfg.Gemini.APIKey = "test-key"
	cfg.Gemini.BaseURL = "http://127.0.0.1:1"
	r, err := New(cfg, t.TempDir(),
		closedRootStorage{Service: storagestub.NewInMemoryService()},
		&fakeLocalService{})
	require.NoError(t, err)

	_, err = r.GetModel(context.Background(), llmapi.ModelEntry{Name: llmapi.DefaultModel})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Partition on closed Service",
		"a closed host alias-store must surface to GetModel so the agent's "+
			"startup model resolution fails with the firstmover sentinel")
}
