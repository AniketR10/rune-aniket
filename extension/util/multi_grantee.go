package util

import (
	"github.com/ernestrc/go-multierror"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/proto"
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

func (m multiGrantee) Connected(b proto.MuxBroker, cfg config.Config) {
	for _, child := range m.children {
		child.Connected(b, cfg)
	}
}

func (m multiGrantee) PermissionGranted(grants []extension.Grant) {
	for _, child := range m.children {
		child.PermissionGranted(grants)
	}
}

func (m multiGrantee) PermissionDenied(perms []extension.Permission) {
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
