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

package process

import (
	"fmt"
	"os/exec"
	"strings"

	goplugin "github.com/hashicorp/go-plugin"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/workspace"
)

const (
	envLogLevel = "go_tui_log_level"
	envDataDir  = "go_tui_data_dir"
)

func makeLogLevelEnv(l log.Level) string {
	return fmt.Sprintf("%s=%s", envLogLevel, l)
}

func makeDataDirEnv(dataDir string) string {
	return fmt.Sprintf("%s=%s", envDataDir, dataDir)
}

func goExtensionGranteeBuilder(m *Manager, dataDir string) extensionBuilder {
	return func(extensionID, path string, grantor extension.Grantor) (*granteeClient, error) {
		pluginsMap := map[string]goplugin.Plugin{
			typeGranteeExtension: &granteeExtension{broker: m.broker, grantor: grantor},
		}

		// allow args to be passed to extensions
		argv := strings.Split(path, " ")
		cmd := exec.Command(argv[0], argv[1:]...)
		// if local workspace, then do set dir in a best effort for
		// extensions that do not use APIs and call os functions directly.
		if m.config.workspace.Scheme() == workspace.FileScheme {
			// if URI is zero-valued, then Path returns an empty string
			// which fits the default in exec.Cmd.Dir which is to not
			// set the command's dir.
			var err error
			cmd.Dir, err = workspaceapi.ExpandPathWithURI(m.config.workspace.Path(), m.config.workspace)
			if err != nil {
				return nil, fmt.Errorf("could not expand workspace path: %q", m.config.workspace.Path())
			}
		}

		log.WithField(logging.KeyClass, "extension.Manager").
			Debugf("extension command Cmd=%#v for workspace=%q", cmd, m.config.workspace)

		cmd.Env = append(cmd.Env, makeDataDirEnv(dataDir))
		cmd.Env = append(cmd.Env, makeLogLevelEnv(log.GetLevel()))

		config := &goplugin.ClientConfig{
			HandshakeConfig:  handshakeConfig,
			Plugins:          pluginsMap,
			Cmd:              cmd,
			Logger:           newHCLogLogrus(extensionID, log.StandardLogger()),
			AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
			// TODO we should validate integrity of extensions
			// SecureConfig:    &secureCfg,
		}
		client := goplugin.NewClient(config)

		rpcClient, err := client.Client()
		if err != nil {
			return nil, err
		}

		raw, err := rpcClient.Dispense(typeGranteeExtension)
		if err != nil {
			return nil, err
		}

		grantee := raw.(*granteeClient)
		grantee.bindExtensionClient(client)

		return grantee, nil
	}
}
