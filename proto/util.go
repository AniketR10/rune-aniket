package proto

//go:generate mockgen -destination=./grpc_gomock.go -package proto google.golang.org/grpc ClientConnInterface

import (
	context "context"
	fmt "fmt"
	"io"
	math "math"
	"strings"
	"sync"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

var zeroCell = Cell{}

// NewDrawResponse converts a tui.Component into a DrawResponse.
func NewDrawResponse(comp tui.Component, width, height int) *DrawResponse {
	resp := &DrawResponse{
		Cursor: &DrawResponse_Cursor{
			Position: &Coordinates{},
		},
	}
	w := newDrawResponseWriter(width, height, resp)
	comp.Draw(w)
	return resp
}

// NewRawCellsResponse converts a buf into an RawCellsResponse.
func NewRawCellsResponse(cells [][]term.Cell) *RawCellsResponse {
	return &RawCellsResponse{Rows: rawCellsToProtoCells(cells)}
}

// BufferToEditRequest converts a buf into an EditRequest.
func BufferToEditRequest(buf *cell.Buffer) EditRequest {
	return EditRequest{Buffer: rawCellsToProtoCells(buf.RawCells())}
}

func rawCellsToProtoCells(cells [][]term.Cell) []*CellRow {
	var size int
	for _, row := range cells {
		size += len(row)
	}

	// these slabs reduce allocations from ~N (=num cells)
	// to 4 which reduces this function's ns/op from 60 to 80%
	rows := make([]*CellRow, len(cells))
	protoCellRowSlabPtr := make([]CellRow, len(cells))
	cellRowSlab := make([]*Cell, size)
	cellRowSlabIdx := 0
	cellSlabPtr := make([]Cell, size)
	cellSlabPtrIdx := 0

	for y, row := range cells {
		cells := cellRowSlab[cellRowSlabIdx : cellRowSlabIdx+len(row)]
		cellRowSlabIdx += len(row)
		for x, cell := range row {
			c := &cellSlabPtr[cellSlabPtrIdx]
			cellSlabPtrIdx++
			c.FromModel(cell)
			cells[x] = c
		}
		protoCellRowSlabPtr[y].Cells = cells
		rows[y] = &protoCellRowSlabPtr[y]
	}

	return rows
}

// EditRequestToBuffer converts an EditRequest into a cell.Buffer
func EditRequestToBuffer(in *EditRequest) *cell.Buffer {
	return rowsToBuffer(in.GetBuffer())
}

// RawCellsResponseToBuffer converts an EditRequest into a cell.Buffer
func RawCellsResponseToBuffer(in *RawCellsResponse) *cell.Buffer {
	return rowsToBuffer(in.GetRows())
}

func rowsToBuffer(in []*CellRow) *cell.Buffer {
	w := cell.NewBufferWriter(math.MaxInt32, math.MaxInt32)

	for y, rows := range in {
		for x, cell := range rows.Cells {
			w.SetCell(term.Coordinates{X: x, Y: y}, cell.ToModel())
		}
	}

	return &w.Buffer
}

type drawResponseWriter struct {
	width, height int
	res           *DrawResponse
}

func newDrawResponseWriter(width, height int, r *DrawResponse) drawResponseWriter {
	cellRowSlab := make([]CellRow, height)
	cellRowWidthSlab := make([]*Cell, height*width)
	r.Rows = make([]*CellRow, height)
	for i := 0; i < height; i++ {
		r.Rows[i] = &cellRowSlab[i]
		r.Rows[i].Cells = cellRowWidthSlab[i*width : (i+1)*width]
		for j := 0; j < width; j++ {
			// SetCell substitutes zeroCell for a newly allocated cell;
			// this allows us to speed up client/server communication
			r.Rows[i].Cells[j] = &zeroCell
		}
	}
	return drawResponseWriter{
		width:  width,
		height: height,
		res:    r,
	}
}

// SetCell satisfies term.Writer
func (r drawResponseWriter) SetCell(pos term.Coordinates, c term.Cell) {
	if pos.Y >= r.height || pos.X >= r.width || pos.X < 0 || pos.Y < 0 {
		return
	}

	var cell *Cell
	if r.res.Rows[pos.Y].Cells[pos.X] == &zeroCell {
		cell = new(Cell)
	} else {
		cell = r.res.Rows[pos.Y].Cells[pos.X]
	}
	cell.Character = uint32(c.Ch)
	cell.Foreground = uint32(c.Fg)
	cell.Background = uint32(c.Bg)

	r.res.Rows[pos.Y].Cells[pos.X] = cell
}

// Flush satisfies term.Writer
func (r drawResponseWriter) Flush() error {
	return nil
}

// Clear satisfies term.Writer
func (r drawResponseWriter) Clear(term.Attributes) error {
	return nil
}

// SetCursor satisfies term.Writer
func (r drawResponseWriter) SetCursor(term.Coordinates) {
}

// DrawResponseToTermString renders a DrawResponse  into str, with width and
// height dimensions.
func DrawResponseToTermString(r *DrawResponse) (str string, width, height int) {
	var builder strings.Builder

	rows := r.GetRows()
	for y, row := range rows {
		height++
		width = 0
		for _, c := range row.GetCells() {
			width++
			ch := c.GetCharacter()
			if ch == 0 {
				ch = ' '
			}
			_, _ = builder.WriteRune(rune(ch))
		}
		if y+1 != len(r.GetRows()) {
			_, _ = builder.WriteRune('\n')
		}
	}
	str = builder.String()
	return
}

// ForceCloseResource is a helper function to remove a resource
// from a resource server or client and close it, safely.
func ForceCloseResource(
	brokerID uint64, getResourcesFn func() map[uint64]io.Closer,
	logger *log.Logger, locker sync.Locker,
) (io.Closer, error) {
	locker.Lock()
	defer locker.Unlock()
	resources := getResourcesFn()
	res, ok := resources[brokerID]
	if !ok {
		if logger != nil {
			logger.Debugf("resource %d already closed", brokerID)
		}
		return nil, nil
	}
	delete(resources, brokerID)

	err := res.Close()
	if err != nil && logger != nil {
		logger.Errorf("resource.Close error: %v", err)
	}

	return res, err
}

// MonitorConnection blocks the calling goroutine and calls onClosed callback
// and returns only when connection state is shutdown, or it has been in a transient
// failure for too long.
func MonitorConnection(
	ctx context.Context, failureTimeout time.Duration,
	conn MuxConn, onClosed func(reason string),
) {

	for {
		state := conn.GetState()
		switch state {
		case connectivity.Idle, connectivity.Connecting, connectivity.Ready:
			conn.WaitForStateChange(ctx, state)
		case connectivity.TransientFailure:
			failureCtx, cancelFn := context.WithTimeout(ctx, failureTimeout)
			didChange := conn.WaitForStateChange(failureCtx, connectivity.TransientFailure)
			cancelFn()
			if !didChange {
				onClosed("timeout waiting for transient failure to recover")
				return
			}
		case connectivity.Shutdown:
			onClosed("grpc connection state = shutdown")
			return
		default:
			panic(fmt.Sprintf("unknown connection state: %v", state))
		}
	}
}

// AcceptAndServe calls the underlying broker's AcceptAndServe
// with a new brokerID, and potentially an instrumented grpc.Server,
// if and only if the level enabled at logger is Trace.
func AcceptAndServe(
	broker MuxBroker, logger *log.Logger,
	register func(uint32, MuxServer),
) (uint32, MuxServer) {
	brokerID := broker.NextId()

	var wg sync.WaitGroup
	var srv MuxServer
	serverFunc := func(opts []grpc.ServerOption) MuxServer {
		defer wg.Done()
		if logger != nil && logger.IsLevelEnabled(log.TraceLevel) {
			srv = LoggingGRPCServer(logger, opts...)
		} else {
			srv = GRPCServer(opts...)
		}
		register(brokerID, srv)
		return srv
	}

	wg.Add(1)
	go broker.AcceptAndServe(brokerID, serverFunc)
	wg.Wait()

	return brokerID, srv
}
