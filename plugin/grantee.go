package plugin

import (
	"context"

	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/proto"
)

// Permission represents a type of resource access.
type Permission string

// Permissions is a set of Permission.
type Permissions map[Permission]struct{}

// Grant binds a granted Permission with a Token that
// can be used with the plugin API.
//
// The Context of this grant can be used to close
// associated resources when the grant is no longer valid.
type Grant struct {
	Token string
	Permission
	context.Context
}

// Grantee needs to be implemented by plugins that want
// to access plugin host resources.
type Grantee interface {
	Connected(proto.MuxBroker, config.Config)
	PermissionGranted([]Grant)
	PermissionDenied([]Permission)
	Shutdown(reason string) error
	Health() error
}
