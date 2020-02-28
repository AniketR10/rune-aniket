package plugin

import (
	"github.com/ernestrc/go-tui/proto"
	"github.com/hashicorp/go-plugin"
)

type Permission string

type Permissions map[Permission]struct{}

type Grantee interface {
	OnConnected(proto.MuxBroker)
	OnPermissionGranted(perm Permission)
	OnPermissionDenied(perm Permission)
	OnShutdown() error
	Health() error
}

// handshakeConfigs are used to just do a basic handshake between
// a plugin and host. If the handshake fails, a user friendly error is shown.
// This prevents users from executing bad plugins or executing a plugin
// directory. It is a UX feature, not a security feature.
var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "TUI_PLUGIN",
	MagicCookieValue: "kombucha_for_dogs",
}
