package component

import (
	"bytes"
	"context"
	"errors"
	"time"

	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

const (
	defaultFPS = 30
	lenPrefix  = 8
)

var _ (tui.Component) = (*Animation)(nil)

// DecodeAnimation decodes the given raw compressed animation into
// an Animation tui.Component. If raw is empty, an empty animation is returned.
// This method returns an error if raw is somehow corrupted or is missing data.
func DecodeAnimation(
	raw []byte, fps int,
	interrupter term.Interrupter,
) (*Animation, error) {
	if len(raw) < lenPrefix {
		a := NewAnimation(interrupter,
			nil /* frames */, nil /* sequence */, fps)
		return a, nil
	}

	frames := make([]string, 0)
	sequence := make([]int, 0)

	for len(raw) != 0 {
		lenFrame, times := decodeLen(raw)
		if len(raw) < lenPrefix+int(lenFrame) {
			return nil, errors.New("corrupted animation data: missing frame data")
		}

		frame := raw[lenPrefix : lenPrefix+lenFrame]
		for i := 0; i < int(times); i++ {
			sequence = append(sequence, len(frames))
		}
		frames = append(frames, string(frame))

		raw = raw[lenPrefix+lenFrame:]
	}

	return NewAnimation(interrupter, frames, sequence, fps), nil
}

// EncodeAnimation encodes the given Animation as a compressed slice of bytes
// that can be stored or transferred.
func EncodeAnimation(a *Animation) []byte {
	padding := [lenPrefix]byte{0, 0, 0, 0, 0, 0, 0, 0}

	raw := make([]string, len(a.frames))
	for i, frame := range a.frames {
		cells := frame.floatingWithAttributes.(backgroundStrWrapper).
			Background.root.(*Span).
			content.C.(*stringComp).cells
		raw[i] = cell.CellsToString(cells)
	}

	var ret bytes.Buffer
	var n int
	var times int32
	for i, sequenceID := range a.sequence {
		times++

		if i != len(a.sequence)-1 && a.sequence[i+1] == sequenceID {
			// do not flush until we change frames
			continue
		}

		frame := raw[sequenceID]
		lenFrame := len([]byte(frame))
		ret.Write(padding[:])
		encodeLen(ret.Bytes()[n:n+lenPrefix], int32(lenFrame), times)
		ret.WriteString(frame)

		n += (lenPrefix + lenFrame)
		times = 0
	}
	return ret.Bytes()
}

// Animation is a tui.Component that renders a series
// of frames on loop at the specified fps.
type Animation struct {
	interrupter   term.Interrupter
	frames        []String
	sequence      []int
	fps           int
	i             int
	ctx           context.Context
	cancelCtx     func()
	waitInterrupt chan struct{}
}

// NewAnimation allocates storage for a new Animation and initializes it.
// See Init for more details.
func NewAnimation(
	interrupter term.Interrupter,
	frames []string, sequence []int, fps int,
) *Animation {
	ret := new(Animation)
	ret.Init(interrupter, frames, sequence, fps)
	return ret
}

// Init initializes this animation with the given interrupter,
// frames, fps. It fires a goroutine which will call the
// given interrupter at the specified fps.
//
// Close should be called to cleanup resources and stop the
// interrupt goroutine.
func (a *Animation) Init(
	interrupter term.Interrupter,
	frames []string, sequence []int, fps int,
) {
	// assert frames and sequence are consistent with each other
	// so we panic on Init to indicate programmer error
	// and not halfway through the animation.
	for _, sequenceID := range sequence {
		if sequenceID > len(frames)-1 {
			panic("invalid sequence and frames pair")
		}
	}
	if fps == 0 {
		fps = defaultFPS
	}
	a.waitInterrupt = make(chan struct{})
	a.interrupter = interrupter
	a.fps = fps
	a.sequence = sequence

	a.frames = make([]String, len(frames))
	for i, frame := range frames {
		// NOTE: changing underlying String implementation here
		// requires refactor of encode routine!
		a.frames[i] = NewStringWithConfig(frame, StringConfig{
			Alignment: SpanAlignmentCentered,
			// term.Attributes
			// FrameCharSet
			// BackgroundAttributes term.Attributes
			// BackgroundRune       rune
			// Tabspaces            int
		})
	}

	a.ctx, a.cancelCtx = context.WithCancel(context.Background())

	go a.interrupt()
}

// Resize satisfies tui.Component.
func (a *Animation) Resize(width, height int) {
	for _, frame := range a.frames {
		frame.Resize(width, height)
	}
}

// Draw satisfies tui.Component.
func (a *Animation) Draw(w term.Writer) {
	if len(a.sequence) == 0 {
		return
	}
	sequenceIdx := a.i % len(a.sequence)
	sequenceID := a.sequence[sequenceIdx]
	a.frames[sequenceID].Draw(w)
	a.i++
}

// Close cleans all resources associated with this Animation.
func (a *Animation) Close() error {
	a.cancelCtx()
	<-a.waitInterrupt
	return nil
}

func (a *Animation) interrupt() {
	defer close(a.waitInterrupt)
	for a.interruptFullSequence() {
	}
}

func (a *Animation) interruptFullSequence() bool {
	cadence := time.Duration(int(time.Second) / a.fps)
	ticker := time.NewTicker(cadence)
	defer ticker.Stop()

	if len(a.sequence) == 0 {
		return false
	}

	for i := 0; i < len(a.sequence); i++ {
		select {
		case <-ticker.C:
			_ = a.interrupter.Interrupt()
		case <-a.ctx.Done():
			return false
		}
	}

	return true
}

func decodeLen(b []byte) (int32, int32) {
	return int32(b[3]) | int32(b[2])<<8 | int32(b[1])<<16 | int32(b[0])<<24,
		int32(b[7]) | int32(b[6])<<8 | int32(b[5])<<16 | int32(b[4])<<24
}

func encodeLen(b []byte, length, times int32) {
	b[0] = byte(length >> 24 & 0x00FF)
	b[1] = byte(length >> 16 & 0x00FF)
	b[2] = byte(length >> 8 & 0x00FF)
	b[3] = byte(length)
	b[4] = byte(times >> 24 & 0x00FF)
	b[5] = byte(times >> 16 & 0x00FF)
	b[6] = byte(times >> 8 & 0x00FF)
	b[7] = byte(times)
}
