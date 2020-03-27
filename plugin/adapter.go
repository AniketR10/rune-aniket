package plugin

import (
	"github.com/ernestrc/go-tui/proto"
	"github.com/hashicorp/go-plugin"
)

// used to adapt to plugin.GRPCBroker Dial signature to proto.MuxBroker
type pluginBrokerAdapter struct {
	*plugin.GRPCBroker
}

func (p pluginBrokerAdapter) Dial(ID uint32) (
	conn proto.MuxConn, err error,
) {
	return p.GRPCBroker.Dial(ID)
}
