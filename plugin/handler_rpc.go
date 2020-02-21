package plugin

import (
	"net/rpc"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// RPCHandlerClient satisfies Clipboard by talking to a clipboard server over RPC.
type RPCHandlerClient struct {
	height, width int
	errors        []error
	*rpc.Client
}

// ResizePayload is a Request of a Resize request.
type ResizePayload struct {
	Height, Width int
}

// HandleResponse is a response to a Handle request.
type HandleResponse struct {
	Exit, Handled bool
}

// CursorResponse to a Cursor request.
type CursorResponse struct {
	Pos  term.Coordinates
	Show bool
}

// TODO log and allow to retrieve somehow?
func (h *RPCHandlerClient) collectError(err error) {
	h.errors = append(h.errors, err)
}

// Resize satisfies tui.Handler
func (h *RPCHandlerClient) Resize(width, height int) {
	h.width, h.height = width, height
	req := ResizePayload{Height: height, Width: width}
	err := h.Client.Call("Plugin.HandlerResize", req, new(interface{}))
	if err != nil {
		h.collectError(err)
	}
}

// Draw satisfies tui.Handler
func (h *RPCHandlerClient) Draw(w tui.Writer) {
	var resp [][]term.Cell
	err := h.Client.Call("Plugin.HandlerDraw", new(interface{}), &resp)
	if err != nil {
		h.collectError(err)
		return
	}

	for y, row := range resp {
		for x, c := range row {
			w.SetCell(term.Coordinates{X: x, Y: y}, c)
		}
	}
}

// Handle satisfies tui.Handler
func (h *RPCHandlerClient) Handle(ev term.Event) (exit, handled bool) {
	var resp HandleResponse
	err := h.Client.Call("Plugin.HandlerHandle", ev, &resp)
	if err != nil {
		h.collectError(err)
		return
	}

	return resp.Exit, resp.Handled
}

// Cursor satisfies tui.Handler
func (h *RPCHandlerClient) Cursor() (pos term.Coordinates, show bool) {
	var resp CursorResponse
	err := h.Client.Call("Plugin.HandlerCursor", new(interface{}), &resp)
	if err != nil {
		h.collectError(err)
		return
	}

	return resp.Pos, resp.Show
}

// Man satisfies tui.Handler
func (h *RPCHandlerClient) Man() tui.Manual {
	var resp tui.Manual
	err := h.Client.Call("Plugin.HandlerMan", new(interface{}), &resp)
	if err != nil {
		h.collectError(err)
		return tui.Manual{}
	}

	return resp
}

// RPCHandlerServer exposes a tui.Handler implementation over RPC.
type RPCHandlerServer struct {
	Handler       tui.Handler
	width, height int
}

// HandlerResize is an RPC that handles request to an
// underlying Handler's Resize over RPC.
func (s *RPCHandlerServer) HandlerResize(args ResizePayload, resp *interface{}) error {
	s.height, s.width = args.Height, args.Width
	s.Handler.Resize(args.Width, args.Height)
	return nil
}

// HandlerDraw is an RPC that handles request to an underlying
// Handler's Draw over RPC.
func (s *RPCHandlerServer) HandlerDraw(args interface{}, resp *[][]term.Cell) error {
	w := cell.NewBufferWriter(s.height, s.width)
	s.Handler.Draw(w)
	*resp = w.RawCells()
	return nil
}

// HandlerHandle is an RPC that handles request to an underlying Handler's
// Handle over RPC.
func (s *RPCHandlerServer) HandlerHandle(args term.Event, resp *HandleResponse) error {
	exit, handled := s.Handler.Handle(args)
	*resp = HandleResponse{Exit: exit, Handled: handled}
	return nil
}

// HandlerCursor is an RPC that handles request to an underlying Handler's
// Cursor over RPC.
func (s *RPCHandlerServer) HandlerCursor(args interface{}, resp *CursorResponse) error {
	pos, show := s.Handler.Cursor()
	*resp = CursorResponse{Pos: pos, Show: show}
	return nil
}

// HandlerMan is an RPC that handles request to an underlying
// Handler's Man over RPC.
func (s *RPCHandlerServer) HandlerMan(args interface{}, resp *tui.Manual) error {
	man := s.Handler.Man()
	*resp = man
	return nil
}
