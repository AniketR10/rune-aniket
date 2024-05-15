package util

import (
	"context"

	"github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/rpc"
)

// MultiGrantee combines together a series of extension.Grantee, which will share
// Permissions and Grants.
func MultiGrantee(first extension.Grantee, extra ...extension.Grantee) extension.Grantee {
	children := make([]extension.Grantee, len(extra)+1)
	children[0] = first
	copy(children[1:], extra)
	return multiGrantee{children: children}
}

type multiGrantee struct {
	children []extension.Grantee
}

func (m multiGrantee) Connected(ctx context.Context, b rpc.MuxBroker, cfg config.Config) (ret error) {
	for _, child := range m.children {
		if err := child.Connected(ctx, b, cfg); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return nil
}

func (m multiGrantee) PermissionGranted(ctx context.Context, grants []extension.Grant) (ret error) {
	for _, child := range m.children {
		if err := child.PermissionGranted(ctx, grants); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return nil
}

func (m multiGrantee) PermissionDenied(ctx context.Context, perms []extension.Permission) (ret error) {
	for _, child := range m.children {
		if err := child.PermissionDenied(ctx, perms); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return nil
}

func (m multiGrantee) Shutdown(ctx context.Context, reason string) (ret error) {
	for _, child := range m.children {
		if err := child.Shutdown(ctx, reason); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (m multiGrantee) Health(ctx context.Context) (ret error) {
	for _, child := range m.children {
		if err := child.Health(ctx); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}
