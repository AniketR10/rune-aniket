package workspace

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	workspacepb "github.com/ernestrc/go-tui/workspace/proto"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultTimeout = 10 * time.Second

type Client struct {
	cc     grpc.ClientConnInterface
	client workspacepb.WorkspaceClient

	files map[int32]*FileClient
}

type FileClient struct {
	client    workspacepb.WorkspaceClient
	handlerID int32
	filename  string
}

type fileClientInfo struct {
	workspacepb.StatResponse
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

func NewClient(cc grpc.ClientConnInterface) *Client {
	ret := new(Client)
	ret.Init(cc)
	return ret
}

func (c *Client) Init(cc grpc.ClientConnInterface) {
	c.cc = cc
	c.client = workspacepb.NewWorkspaceClient(cc)
}

func (c *Client) newFileClient(filename string, handlerID int32) *FileClient {
	return &FileClient{
		client:    c.client,
		handlerID: handlerID,
		filename:  filename,
	}
}

func (c *Client) Open(name string, flag int, perm os.FileMode) (
	*FileClient, error,
) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.OpenRequest{Filename: name, Flag: int32(flag), Mode: int32(perm)}
	resp, err := c.client.Open(ctx, &req)
	if err != nil {
		return nil, &osError{error: err}
	}
	if resp.GetIsExistErr() || resp.GetIsNotExistErr() || resp.GetIsPermissionErr() {
		return nil, &osError{
			isExist:      resp.GetIsExistErr(),
			isNotExist:   resp.GetIsNotExistErr(),
			isPermission: resp.GetIsPermissionErr(),
		}
	}
	return c.newFileClient(name, resp.GetHandlerId()), nil
}

func (c *Client) Remove(name string) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.RemoveRequest{Filename: name}
	_, err := c.client.Remove(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Rename(oldpath, newpath string) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.RenameRequest{Filename: oldpath, Newfilename: newpath}
	_, err := c.client.Rename(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Stat(name string) (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.StatRequest{Filename: name}
	resp, err := c.client.Stat(ctx, &req)
	if err != nil {
		return nil, err
	}
	return fileClientInfo{StatResponse: *resp}, nil
}

func (c *Client) LStat(name string) (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.StatRequest{Filename: name, Lstat: true}
	resp, err := c.client.Stat(ctx, &req)
	if err != nil {
		return nil, err
	}
	return fileClientInfo{StatResponse: *resp}, nil
}

func (c *Client) Close() (err error) {
	if closer, ok := c.cc.(io.Closer); ok {
		ccErr := closer.Close()
		if ccErr != nil {
			err = ccErr
		}
	}
	for _, res := range c.files {
		resErr := res.Close()
		if resErr != nil {
			err = resErr
		}
	}

	c.files = nil

	return err
}

func (c *FileClient) Name() string {
	return c.filename
}

func (c *FileClient) Stat() (os.FileInfo, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.StatRequest{Filename: c.filename}
	resp, err := c.client.Stat(ctx, &req)
	if err != nil {
		return nil, err
	}
	return fileClientInfo{StatResponse: *resp}, nil
}

func (c *FileClient) Sync() error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.SyncRequest{HandlerId: c.handlerID}
	_, err := c.client.Sync(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *FileClient) Truncate(size int64) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.TruncateRequest{HandlerId: c.handlerID, Size: size}
	_, err := c.client.Truncate(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *FileClient) Seek(offset int64, whence int) (int64, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.SeekRequest{
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

func (c *FileClient) Read(p []byte) (n int, err error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.ReadRequest{HandlerId: c.handlerID, N: int64(len(p))}
	resp, err := c.client.Read(ctx, &req)
	if err != nil {
		return 0, err
	}
	data := resp.GetData()
	if len(data) > len(p) || int64(len(data)) != resp.GetN() {
		return 0, errors.New("server returned invalid data")
	}
	copy(p, []byte(data))
	if resp.IsEof {
		err = io.EOF
	} else {
		err = nil
	}
	return int(resp.GetN()), err
}

func (c *FileClient) Close() error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.CloseFileRequest{HandlerId: c.handlerID}
	_, err := c.client.Close(ctx, &req)
	if err != nil {
		return err
	}
	return nil
}

func (c *FileClient) Write(p []byte) (n int, err error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.WriteRequest{HandlerId: c.handlerID, Data: string(p)}
	resp, err := c.client.Write(ctx, &req)
	if err != nil {
		return 0, err
	}
	return int(resp.GetN()), nil
}

func (c *Client) ReadLink(filename string) (string, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := workspacepb.ReadLinkRequest{Filename: filename}
	resp, err := c.client.ReadLink(ctx, &req)
	if err != nil {
		return "", err
	}
	return resp.GetFilename(), nil
}

func ctxWithTimeout() (context.Context, func()) {
	return context.WithTimeout(context.Background(), defaultTimeout)
}

func protoTimeToStd(ts *timestamppb.Timestamp) time.Time {
	return time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
}
