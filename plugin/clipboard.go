package plugin

import (
	"net/rpc"
	"os"
	"os/exec"

	"github.com/ernestrc/go-tui/editor"
	hPlugin "github.com/hashicorp/go-plugin"
	log "github.com/sirupsen/logrus"
)

// ClipboardPlugin satisfies plugin.Plugin
type ClipboardPlugin struct {
	editor.Clipboard
}

// Server must return an RPC server for this plugin
// type. We construct a rpcClipboardServer for this.
func (p *ClipboardPlugin) Server(*hPlugin.MuxBroker) (interface{}, error) {
	return &editor.RPCClipboardServer{Clipboard: p.Clipboard}, nil
}

// Client must return an implementation of our interface that communicates
// over an RPC client. We return rpcClipboardClient for this.
func (ClipboardPlugin) Client(b *hPlugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &editor.RPCClipboardClient{Client: c}, nil
}

// ServeClipboard serves the given Clipboard implementation as a plugin.
// This should be called from within the main function of a program compiled
// as a separate executable. This function never returns.
func ServeClipboard(impl editor.Clipboard) {
	// this ensures plugin host captures output
	SetLoggingOutput(os.Stderr)

	level, err := log.ParseLevel(os.Getenv(envLogLevel))
	if err != nil {
		panic(err)
	}
	SetLoggingLevel(level)

	impl = editor.NewLoggingClipboard(pluginLogger, impl)

	pluginMap := map[string]hPlugin.Plugin{
		"clipboard": &ClipboardPlugin{Clipboard: impl},
	}

	hPlugin.Serve(&hPlugin.ServeConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
		Logger:          NewHCLogLogrus(pluginLogger),
	})
}

// NewClipboard returns an implementation of editor.Clipboard which
// uses the clipboard plugin executable compiled at path. It also returns
// a function to close the plugin client.
//
// Note that the clipboard plugin executable should use ServeClipboard function
// for this method to spawn a clipboard RPC client and connect to it.
func NewClipboard(path string) (editor.Clipboard, func(), error) {
	// pluginMap is the map of plugins we can dispense.
	pluginMap := map[string]hPlugin.Plugin{
		"clipboard": &ClipboardPlugin{},
	}

	cmd := exec.Command(path)
	cmd.Env = append(cmd.Env, pluginEnv...)
	cmd.Env = append(cmd.Env, makeEnvVar(envLogLevel, pluginLogger.Level.String()))

	client := hPlugin.NewClient(&hPlugin.ClientConfig{
		HandshakeConfig: handshakeConfig,
		Plugins:         pluginMap,
		Cmd:             cmd,
		Logger:          NewHCLogLogrus(pluginLogger),
		// TODO we should validate integrity of plugins
		// SecureConfig:    &secureCfg,
	})

	rpcClient, err := client.Client()
	if err != nil {
		return nil, nil, err
	}

	raw, err := rpcClient.Dispense("clipboard")
	if err != nil {
		return nil, nil, err
	}

	return raw.(editor.Clipboard), client.Kill, nil
}
