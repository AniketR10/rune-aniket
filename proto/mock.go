package proto

import (
	context "context"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	grpc "google.golang.org/grpc"
)

type MockHandlerClient struct {
	ResizeError   error
	height, width int
	Remote        tui.Handler
}

func (c *MockHandlerClient) Resize(
	ctx context.Context, in *ResizeRequest, opts ...grpc.CallOption,
) (*ResizeResponse, error) {
	c.width, c.height = int(in.Width), int(in.Height)
	c.Remote.Resize(c.width, c.height)
	if c.ResizeError != nil {
		return nil, c.ResizeError
	}
	return new(ResizeResponse), nil
}

func (c *MockHandlerClient) Draw(
	ctx context.Context, in *DrawRequest, opts ...grpc.CallOption,
) (*DrawResponse, error) {
	w := cell.NewBufferWriter(c.width, c.height)
	c.Remote.Draw(w)

	resp := new(DrawResponse)
	for _, row := range w.RawCells() {
		cRow := new(CellRow)
		for _, c := range row {
			cell := new(Cell)
			cell.FromModel(c)
			cRow.Cells = append(cRow.Cells, cell)
		}
		resp.Rows = append(resp.Rows, cRow)
	}

	return resp, nil
}

func (c *MockHandlerClient) Handle(
	ctx context.Context, in *HandleRequest, opts ...grpc.CallOption,
) (*HandleResponse, error) {
	resp := new(HandleResponse)
	ev, err := in.GetEvent().ToModel()
	if err != nil {
		return nil, err
	}
	resp.Quit, resp.Handled = c.Remote.Handle(ev)
	return resp, nil
}

func (c *MockHandlerClient) Cursor(
	ctx context.Context, in *CursorRequest, opts ...grpc.CallOption,
) (*CursorResponse, error) {
	pos, show := c.Remote.Cursor()

	protoPos := Coordinates{}
	protoPos.FromModel(pos)

	return &CursorResponse{Position: &protoPos, Show: show}, nil
}

func (c *MockHandlerClient) Man(
	ctx context.Context, in *ManRequest, opts ...grpc.CallOption,
) (*ManResponse, error) {
	man := c.Remote.Man()

	protoMan := Manual{}
	protoMan.FromModel(man)

	return &ManResponse{Man: &protoMan}, nil
}
