package plugin

import (
	"github.com/ernestrc/go-tui/proto"
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
