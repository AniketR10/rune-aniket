package rpc

import (
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"unstable.build/go-tui/workspace"
)

// NewScheme returns a workspace.Scheme RPC-based client over
// the given connection. It expects a SchemeServer to be listening
// on the other side of the connection.
func NewScheme(cc grpc.ClientConnInterface) workspace.Scheme {
	ret := new(schemeClientImpl)
	ret.Init(cc)
	return ret
}

type schemeClientImpl struct {
	client SchemeClient
	executorClientImpl
}

type fileClient struct {
	client    SchemeClient
	handlerID int32
	filename  string
	ioClient
}

type fileClientInfo struct {
	StatResponse
}

func (f fileClientInfo) Name() string {
	return f.StatResponse.GetName()
}

func (f fileClientInfo) Size() int64 {
	return f.StatResponse.GetSize()
}

func (f fileClientInfo) Mode() os.FileMode {
	return os.FileMode(f.StatResponse.GetMode())
}

func (f fileClientInfo) ModTime() time.Time {
	return protoTimeToStd(f.StatResponse.GetModTime())
}

func (f fileClientInfo) IsDir() bool {
	return f.StatResponse.GetIsDir()
}
func (f fileClientInfo) Sys() interface{} {
	return nil
}

func (c *schemeClientImpl) Init(cc grpc.ClientConnInterface) {
	c.client = NewSchemeClient(cc)
	c.executorClientImpl.init(c.client)
}

func (c *schemeClientImpl) Stop() {
	c.executorClientImpl.Close()
}

func (c *schemeClientImpl) URI(path string) (workspace.URI, error) {
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

func (c *schemeClientImpl) Open(name string, flag int, perm os.FileMode) (
	workspace.File, *workspace.Error,
) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := OpenRequest{Filename: name, Flag: int64(flag), Mode: int32(perm)}
	resp, err := c.client.Open(ctx, &req)
	if err != nil {
		return nil, &workspace.Error{Err: err}
	}
	if resp.GetIsExistErr() || resp.GetIsNotExistErr() || resp.GetIsPermissionErr() {
		return nil, &workspace.Error{
			IsExist:      resp.GetIsExistErr(),
			IsNotExist:   resp.GetIsNotExistErr(),
			IsPermission: resp.GetIsPermissionErr(),
		}
	}
	return c.newFileClient(name, resp.GetHandlerId()), nil
}

func (c *schemeClientImpl) Remove(name string) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := RemoveRequest{Filename: name}
	_, err := c.client.Remove(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *schemeClientImpl) Rename(oldpath, newpath string) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := RenameRequest{Filename: oldpath, Newfilename: newpath}
	_, err := c.client.Rename(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *schemeClientImpl) Stat(name string) (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StatRequest{Filename: name}
	resp, err := c.client.Stat(ctx, &req)
	if err != nil {
		return nil, err
	}
	return fileClientInfo{StatResponse: *resp}, nil
}

func (c *schemeClientImpl) Lstat(name string) (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StatRequest{Filename: name, Lstat: true}
	resp, err := c.client.Stat(ctx, &req)
	if err != nil {
		return nil, err
	}
	return fileClientInfo{StatResponse: *resp}, nil
}

func (c *schemeClientImpl) ReadLink(filename string) (string, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := ReadLinkRequest{Filename: filename}
	resp, err := c.client.ReadLink(ctx, &req)
	if err != nil {
		return "", err
	}
	return resp.GetFilename(), nil
}

func (c *fileClient) Name() string {
	return c.filename
}

func (c *fileClient) Stat() (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := StatRequest{Filename: c.filename}
	resp, err := c.client.Stat(ctx, &req)
	if err != nil {
		return nil, err
	}
	return fileClientInfo{StatResponse: *resp}, nil
}

func (c *fileClient) Sync() error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := SyncRequest{HandlerId: c.handlerID}
	_, err := c.client.Sync(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *fileClient) Truncate(size int64) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := TruncateRequest{HandlerId: c.handlerID, Size: size}
	_, err := c.client.Truncate(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *fileClient) Seek(offset int64, whence int) (int64, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := SeekRequest{
		HandlerId: c.handlerID,
		Offset:    offset,
		Whence:    int64(whence),
	}
	resp, err := c.client.Seek(ctx, &req)
	if err != nil {
		return 0, err
	}
	return resp.GetNewOffset(), nil
}

func (c *schemeClientImpl) newFileClient(
	filename string, handlerID int32,
) *fileClient {
	ret := &fileClient{
		handlerID: handlerID,
		client:    c.client,
		filename:  filename,
		// satisfies Read/Write/Close
		ioClient: ioClient{
			client:    c.client,
			handlerID: handlerID,
		},
	}
	c.addCloser(handlerID, ret)
	return ret
}

func protoTimeToStd(ts *timestamppb.Timestamp) time.Time {
	return time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
}
