package extension

import (
	"context"

	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/rpc"
)

// Permission represents a type of resource access.
type Permission string

// Permissions is a set of Permission.
type Permissions map[Permission]struct{}

// Grant binds a granted Permission with a Token that
// can be used with the extension API.
//
// The Context of this grant can be used to close
// associated resources when the grant is no longer valid.
type Grant struct {
	Token string
	Permission
	context.Context
}

// Grantee needs to be implemented by extensions that want
// to access extension host resources.
type Grantee interface {
	Connected(context.Context, rpc.MuxBroker, config.Config) error
	PermissionGranted(context.Context, []Grant) error
	PermissionDenied(context.Context, []Permission) error
	Shutdown(ctx context.Context, reason string) error
	Health(context.Context) error
}
