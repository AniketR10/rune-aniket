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
