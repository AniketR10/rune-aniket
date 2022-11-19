package rpc

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ernestrc/blue/iterator"
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
	openRemoveClientImpl
}

type openRemoveClientImpl struct {
	executorClientImpl
	client interface {
		Open(ctx context.Context, in *OpenRequest, opts ...grpc.CallOption) (*OpenResponse, error)
		Remove(ctx context.Context, in *RemoveRequest, opts ...grpc.CallOption) (*RemoveResponse, error)
		Close(ctx context.Context, in *CloseFileRequest, opts ...grpc.CallOption) (*CloseFileResponse, error)
		Read(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (*ReadResponse, error)
		Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*WriteResponse, error)
		Sync(ctx context.Context, in *SyncRequest, opts ...grpc.CallOption) (*SyncResponse, error)
		Truncate(ctx context.Context, in *TruncateRequest, opts ...grpc.CallOption) (*TruncateResponse, error)
		Seek(ctx context.Context, in *SeekRequest, opts ...grpc.CallOption) (*SeekResponse, error)
		Stat(ctx context.Context, in *StatRequest, opts ...grpc.CallOption) (*StatResponse, error)
	}
}

type fileClient struct {
	client interface {
		Sync(ctx context.Context, in *SyncRequest, opts ...grpc.CallOption) (*SyncResponse, error)
		Truncate(ctx context.Context, in *TruncateRequest, opts ...grpc.CallOption) (*TruncateResponse, error)
		Seek(ctx context.Context, in *SeekRequest, opts ...grpc.CallOption) (*SeekResponse, error)
		Close(ctx context.Context, in *CloseFileRequest, opts ...grpc.CallOption) (*CloseFileResponse, error)
		Read(ctx context.Context, in *ReadRequest, opts ...grpc.CallOption) (*ReadResponse, error)
		Write(ctx context.Context, in *WriteRequest, opts ...grpc.CallOption) (*WriteResponse, error)
		Stat(ctx context.Context, in *StatRequest, opts ...grpc.CallOption) (*StatResponse, error)
	}
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
	client := NewSchemeClient(cc)
	c.client = client
	c.openRemoveClientImpl.executorClientImpl.init(c.client)
	c.openRemoveClientImpl.client = client
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

func makeOpenRequest(name string, flag int, perm os.FileMode) *OpenRequest {
	return &OpenRequest{
		Filename: name,
		Mode:     int32(perm),
		O_RDONLY: flag&^(os.O_APPEND|os.O_CREATE|os.O_EXCL|os.O_SYNC|os.O_TRUNC) == os.O_RDONLY,
		O_WRONLY: flag&^(os.O_APPEND|os.O_CREATE|os.O_EXCL|os.O_SYNC|os.O_TRUNC) == os.O_WRONLY,
		O_APPEND: flag&os.O_APPEND != 0,
		O_CREATE: flag&os.O_CREATE != 0,
		O_EXCL:   flag&os.O_EXCL != 0,
		O_SYNC:   flag&os.O_SYNC != 0,
		O_TRUNC:  flag&os.O_TRUNC != 0,
	}
}

type errResponse interface {
	GetIsExistErr() bool
	GetIsNotExistErr() bool
	GetIsPermissionErr() bool
}

func isTypedError(resp errResponse) (*workspace.Error, bool) {
	if resp.GetIsExistErr() || resp.GetIsNotExistErr() || resp.GetIsPermissionErr() {
		return &workspace.Error{
			IsExist:      resp.GetIsExistErr(),
			IsNotExist:   resp.GetIsNotExistErr(),
			IsPermission: resp.GetIsPermissionErr(),
		}, true
	}
	return nil, false
}

func (c *openRemoveClientImpl) Open(name string, flag int, perm os.FileMode) (
	workspace.File, *workspace.Error,
) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := makeOpenRequest(name, flag, perm)
	resp, err := c.client.Open(ctx, req)
	if err != nil {
		return nil, &workspace.Error{Err: err}
	}
	if werr, ok := isTypedError(resp); ok {
		return nil, werr
	}
	// workspace.Pid is not necessary (and/or available) for files
	// because it's only used for Wait cleanup
	return c.newFileClient(-1, resp.GetFilename(), resp.GetHandlerId()), nil
}

func (c *openRemoveClientImpl) Remove(name string) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := RemoveRequest{Filename: name}
	resp, err := c.client.Remove(ctx, &req)
	if err != nil {
		return err
	}
	if werr, ok := isTypedError(resp); ok {
		return werr.ToError()
	}
	return nil
}

func (c *schemeClientImpl) Rename(oldpath, newpath string) error {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := RenameRequest{Filename: oldpath, Newfilename: newpath}
	resp, err := c.client.Rename(ctx, &req)
	if err != nil {
		return err
	}
	if werr, ok := isTypedError(resp); ok {
		return werr.ToError()
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
	if werr, ok := isTypedError(resp); ok {
		return nil, werr.ToError()
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
	if werr, ok := isTypedError(resp); ok {
		return nil, werr.ToError()
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

type listFilesIterator struct {
	stream Scheme_ListFilesClient
	err    error
}

func (i *listFilesIterator) Next() (string, bool) {
	resp, err := i.stream.Recv()
	if err == nil {
		return resp.GetPath(), true
	}
	if err != io.EOF {
		i.err = err
	}
	return "", false
}

func (i *listFilesIterator) Err() error {
	return i.err
}

func (c *schemeClientImpl) ListFiles(ctx context.Context) (iterator.Iterator[string], error) {
	req := ListFilesRequest{}
	stream, err := c.client.ListFiles(ctx, &req)
	if err != nil {
		return nil, err
	}
	return &listFilesIterator{stream: stream}, nil
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

func protoTimeToStd(ts *timestamppb.Timestamp) time.Time {
	return time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
}
