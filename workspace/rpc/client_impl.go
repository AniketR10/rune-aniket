package rpc

import (
	"context"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
	"unstable.build/go-tui/workspace"
)

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
		ReadDir(ctx context.Context, in *ReadDirRequest, opts ...grpc.CallOption) (*ReadDirResponse, error)
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

func (c *openRemoveClientImpl) Stat(name string) (os.FileInfo, error) {
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

func (c *openRemoveClientImpl) ReadDir(name string) ([]os.DirEntry, error) {
	ctx, cleanup := ctxWithTimeout()
	defer cleanup()

	req := ReadDirRequest{Root: name}
	resp, err := c.client.ReadDir(ctx, &req)
	if err != nil {
		return nil, err
	}
	if werr, ok := isTypedError(resp); ok {
		return nil, werr.ToError()
	}
	respp := resp.GetPath()
	ret := make([]os.DirEntry, 0, len(respp))
	for _, entry := range respp {
		ret = append(ret, dirEntry{
			c:        c,
			name:     entry.Name,
			isDir:    entry.IsDir,
			modeType: entry.Mode,
		})
	}

	return ret, nil
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

type dirEntry struct {
	c        *openRemoveClientImpl
	name     string
	isDir    bool
	modeType int32
}

func (e dirEntry) Name() string {
	return e.name
}

func (e dirEntry) IsDir() bool {
	return e.isDir
}

func (e dirEntry) Type() os.FileMode {
	return os.FileMode(e.modeType)
}

func (e dirEntry) Info() (os.FileInfo, error) {
	return e.c.Stat(e.Name())
}

func protoTimeToStd(ts *timestamppb.Timestamp) time.Time {
	return time.Unix(ts.GetSeconds(), int64(ts.GetNanos()))
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
