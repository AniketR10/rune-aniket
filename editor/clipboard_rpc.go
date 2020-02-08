package editor

import (
	"net/rpc"
)

// RPCClipboardClient satisfies Clipboard by talking to a clipboard server over RPC.
type RPCClipboardClient struct{ *rpc.Client }

// Get uses the underlying RPC client to call the remote's clipboard Get method and
// return its results.
func (c *RPCClipboardClient) Get() (Paste, error) {
	var resp Paste
	err := c.Client.Call("Plugin.ClipboardGet", new(interface{}), &resp)
	if err != nil {
		return Paste{}, err
	}

	return resp, nil
}

// Set uses the underlying RPC client to call the remote's clipboard Set method.
func (c *RPCClipboardClient) Set(data Paste) error {
	err := c.Client.Call("Plugin.ClipboardSet", data, new(interface{}))
	if err != nil {
		return err
	}

	return nil
}

// RPCClipboardServer exposes a Clipboard implementation over RPC.
type RPCClipboardServer struct {
	Clipboard Clipboard
}

// ClipboardGet can be called over RPC to call the underlying's Clipboard Get method.
func (s *RPCClipboardServer) ClipboardGet(args interface{}, resp *Paste) error {
	var err error
	*resp, err = s.Clipboard.Get()
	return err
}

// ClipboardSet can be called over RPC to call the underlying's Clipboard Set method.
func (s *RPCClipboardServer) ClipboardSet(data Paste, resp *interface{}) error {
	return s.Clipboard.Set(data)
}
