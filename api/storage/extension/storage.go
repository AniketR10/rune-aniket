package extension

import (
	"os"

	"github.com/ernestrc/blue/document"
	docrpc "github.com/ernestrc/blue/document/rpc"
	"github.com/ernestrc/blue/encoding/toml"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/proto"
)

// NOTE: this is exposing blue/document types which we might not
// want to do directly. If we ever open-source that library
// then remove this comment.
func dialStorage(grant extension.Grant, broker proto.MuxBroker) (
	document.Service, error,
) {
	conn, err := broker.DialChannel(grant.Token,
		os.Args[0], "storage", string(grant.Permission))
	if err != nil {
		return nil, err
	}
	c := new(docrpc.Client)
	c.Init(conn, toml.Marshaler())
	partition, ok := partitionFromContext(grant.Context)
	if !ok {
		partition = "default"
	}
	return document.WithPartition(c, partition), nil
}

// Storage acquires a client to persistent storage with
// the given token.
func Storage(grant extension.Grant, broker proto.MuxBroker) (
	document.Service, error,
) {
	return dialStorage(grant, broker)
}
