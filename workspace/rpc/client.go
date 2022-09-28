package rpc

import (
	"context"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	"unstable.build/go-tui/workspace"
)

const defaultTimeout = 10 * time.Second

var _ workspace.API = (*Client)(nil)

type Client struct {
	cc     grpc.ClientConnInterface
	client WorkspaceClient
	executorClientImpl
}

func NewClient(cc grpc.ClientConnInterface) *Client {
	ret := new(Client)
	ret.Init(cc)
	return ret
}

func (c *Client) Init(cc grpc.ClientConnInterface) {
	c.cc = cc
	c.client = NewWorkspaceClient(cc)
	c.executorClientImpl.init(c.client)
}

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
