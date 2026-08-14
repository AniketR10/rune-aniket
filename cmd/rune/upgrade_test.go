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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/ide/upgradeshell"
)

// TestRegisterUpgradeCommandWithoutManager covers the configuration
// where no manifest URL is available: the manager is nil and there is
// nothing to register, which must not be an error.
func TestRegisterUpgradeCommandWithoutManager(t *testing.T) {
	require.NoError(t, registerUpgradeCommand(nil, nil))
}

// TestUpgradeAliasTargetsConsoleCommand pins the command-prompt entry
// point for the console `upgrade` command: `:upgrade` must keep
// working now that the ex-command is gone.
func TestUpgradeAliasTargetsConsoleCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.star")
	require.NoError(t, os.WriteFile(path, nil, 0o644))

	cfg, err := ide.Config(path, runeDefaultConfig())
	require.NoError(t, err)
	command, err := cfg.GetConfig("command")
	require.NoError(t, err)
	aliases, err := command.GetConfig("aliases")
	require.NoError(t, err)
	got, err := aliases.GetString(upgradeshell.CommandName)
	require.NoError(t, err)
	require.Equal(t, "console "+upgradeshell.CommandName, got)
}
