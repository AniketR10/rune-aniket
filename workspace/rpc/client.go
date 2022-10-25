package rpc

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ernestrc/blue/iterator"
	"google.golang.org/grpc"
	"unstable.build/go-tui/workspace"
)

const defaultTimeout = 10 * time.Second

var _ workspace.API = (*Client)(nil)

type Client struct {
	cc     grpc.ClientConnInterface
	client WorkspaceClient
	openRemoveClientImpl
}

// NewClient allocates storage for a new workspace.Client and
// initializes it with cc. Client satisfies workspace.API
// by connecting to a Server via the given grpc connection.
func NewClient(cc grpc.ClientConnInterface) *Client {
	ret := new(Client)
	ret.Init(cc)
	return ret
}

// Init initializes this client with cc.
func (c *Client) Init(cc grpc.ClientConnInterface) {
	client := NewWorkspaceClient(cc)
	c.cc = cc
	c.client = client
	c.openRemoveClientImpl.executorClientImpl.init(c.client)
	c.openRemoveClientImpl.client = client
}

// Open satisfies workspace.API.
func (c *Client) Open(path string, flag int, mode os.FileMode) (workspace.File, *workspace.Error) {
	return c.openRemoveClientImpl.Open(path, flag, mode)
}

// Remove satisfies workspace.API.
func (c *Client) Remove(path string) error {
	return c.openRemoveClientImpl.Remove(path)
}

// URI satisfies workspace.API.
func (c *Client) URI(path string) (workspace.URI, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := URIRequest{Path: path}
	resp, err := c.client.URI(ctx, &req)
	if err != nil {
		return workspace.URI{}, err
	}
	uri, err := workspace.ParseURI(resp.GetUri())
	if err != nil {
		return workspace.URI{}, fmt.Errorf("Could not parse URI response from server: %w", err)
	}
	return uri, nil
}

// Getwd satisfies workspace.API.
func (c *Client) Getwd() (workspace.URI, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := GetwdRequest{}
	resp, err := c.client.Getwd(ctx, &req)
	if err != nil {
		return workspace.URI{}, err
	}
	uri, err := workspace.ParseURI(resp.GetUri())
	if err != nil {
		return workspace.URI{}, fmt.Errorf("Could not parse URI response from server: %w", err)
	}
	return uri, nil
}

func (c *Client) ListFiles(ctx context.Context) (iterator.Iterator[string], error) {
	req := ListFilesRequest{}
	stream, err := c.client.ListFiles(ctx, &req)
	if err != nil {
		return nil, err
	}
	return &listFilesIterator{stream: stream}, nil
}

// Close closes all resources associated with this client.
func (c *Client) Close() (err error) {
	if closer, ok := c.cc.(io.Closer); ok {
		ccErr := closer.Close()
		if ccErr != nil {
			err = ccErr
		}
	}
	c.executorClientImpl.Close()

	return err
}

func ctxWithTimeout() (context.Context, func()) {
	return context.WithTimeout(context.Background(), defaultTimeout)
}
