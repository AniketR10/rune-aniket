package plugin

import (
	"fmt"
	"os/exec"

	"github.com/ernestrc/blue/logging"
	"github.com/hashicorp/go-plugin"
	log "github.com/sirupsen/logrus"
	workspaceapi "unstable.build/go-tui/api/workspace"
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

func goPluginGranteeBuilder(m *Manager, dataDir string) pluginBuilder {
	return func(pluginID, path string, grantor Grantor) (*granteeClient, error) {
		pluginMap := map[string]plugin.Plugin{
			typeGranteePlugin: &granteePlugin{broker: m.broker, grantor: grantor},
		}

		cmd := exec.Command(path)
		// if local workspace, then do set dir in a best effort for
		// plugins that do not use APIs and call os functions directly.
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

		log.WithField(logging.KeyClass, "plugin.Manager").
			Debugf("plugin command Cmd=%#v for workspace=%q", cmd, m.config.workspace)

		cmd.Env = append(cmd.Env, makeDataDirEnv(dataDir))
		cmd.Env = append(cmd.Env, makeLogLevelEnv(log.GetLevel()))

		config := &plugin.ClientConfig{
			HandshakeConfig:  handshakeConfig,
			Plugins:          pluginMap,
			Cmd:              cmd,
			Logger:           NewHCLogLogrus(log.StandardLogger()),
			AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
			// TODO we should validate integrity of plugins
			// SecureConfig:    &secureCfg,
		}
		client := plugin.NewClient(config)

		rpcClient, err := client.Client()
		if err != nil {
			return nil, err
		}

		raw, err := rpcClient.Dispense(typeGranteePlugin)
		if err != nil {
			return nil, err
		}

		grantee := raw.(*granteeClient)
		grantee.bindPluginClient(client)

		return grantee, nil
	}
}
