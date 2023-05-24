package util

import (
	"github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
)

// MultiGrantee combines together a series of plugin.Grantee, which will share
// Permissions and Grants.
func MultiGrantee(first plugin.Grantee, extra ...plugin.Grantee) plugin.Grantee {
	children := make([]plugin.Grantee, len(extra)+1)
	children[0] = first
	copy(children[1:], extra)
	return multiGrantee{children: children}
}

type multiGrantee struct {
	children []plugin.Grantee
}

func (m multiGrantee) Connected(b proto.MuxBroker, cfg config.Config) {
	for _, child := range m.children {
		child.Connected(b, cfg)
	}
}

func (m multiGrantee) PermissionGranted(grants []plugin.Grant) {
	for _, child := range m.children {
		child.PermissionGranted(grants)
	}
}

func (m multiGrantee) PermissionDenied(perms []plugin.Permission) {
	for _, child := range m.children {
		child.PermissionDenied(perms)
	}
}

func (m multiGrantee) Shutdown(reason string) (ret error) {
	for _, child := range m.children {
		if err := child.Shutdown(reason); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (m multiGrantee) Health() (ret error) {
	for _, child := range m.children {
		if err := child.Health(); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}
