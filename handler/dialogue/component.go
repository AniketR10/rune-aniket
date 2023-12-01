package dialogue

import (
	"fmt"
	"math"
	"strings"

	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler/input"
	"unstable.build/go-tui/term"
)

// ComponentConfig holds configuration options for dialogue.Component.
type ComponentConfig struct {
	// MessagesRowConfig determines the positioning of the messages span
	// within dialogue component.
	MessagesRowConfig component.SpanConfig
	// InputRowColumns determines the width of the input row in columns,
	// from 1 to component.MaxCols. A zero value uses the default of 10.
	InputRowColumns int
	// SendMessageStringConfig determines the StringConfig of the sent messages in
	// the messages component.
	SendMessageStringConfig component.StringConfig
	// SendMessageSpanConfig allows for clients to add padding to send
	// messages and control its alignent in regards of this padding.
	SendMessageSpanConfig component.SpanConfig
	// ReceiveMessageStringConfig determines the StringConfig of the received
	// messages in the messages component.
	ReceiveMessageStringConfig component.StringConfig
	// ReceiveMessageSpanConfig allows for clients to add padding to receive
	// messages and control its alignent in regards of this padding.
	ReceiveMessageSpanConfig component.SpanConfig
	// InputConfig determines the configuration for the input prompt.
	InputConfig input.BoxConfig
}

// Component implements a dialogue tui.Component.
type Component struct {
	cfg          ComponentConfig
	messages     component.ResponsiveList
	spanMessages component.Span
	box          input.Box
	container    component.Container
	inputRow     *component.Row
	inputCol     *component.Virtual
	height       int

	// stream back model results
	msg  strings.Builder
	tail *component.ListNode
}

// NewComponent allocates storage for a new Component and initializes it.
func NewComponent(cfg ComponentConfig) *Component {
	ret := new(Component)
	ret.Init(cfg)
	return ret
}

// Init initializes this Component with cfg.
func (c *Component) Init(cfg ComponentConfig) {
	if cfg.InputRowColumns == 0 {
		cfg.InputRowColumns = 10
	}
	if cfg.InputRowColumns > component.MaxCols {
		panic(fmt.Sprintf("InputRowColumns must be > 0 and <= %d", component.MaxCols))
	}
	c.cfg = cfg

	c.messages.Init()
	c.messages.Alignment = component.SpanAlignmentBottom
	c.spanMessages.Init(&c.messages, c.cfg.MessagesRowConfig)
	messagesResponsive := component.FuncResponsive(&c.spanMessages, func(width int) int {
		// assumes c.height has been set prior to call to Container.Resize
		return int(math.Max(float64(c.height-c.box.Height(c.boxWidth(width))), 0))
	})
	c.container.AddRow().AddComponent(messagesResponsive, component.MaxCols)

	c.box.Init(cell.NewBuffer(), c.cfg.InputConfig)
	c.inputRow = c.container.AddRow()
	c.inputRow.AddComponent(component.NopResponsive(), (component.MaxCols-c.cfg.InputRowColumns)/2)
	c.inputCol = c.inputRow.AddComponent(&c.box, c.boxWidth(component.MaxCols))
}

// Draw satisfies tui.Component.
func (c *Component) Draw(w term.Writer) {
	c.container.Draw(w)
}

// Resize satisfies tui.Component.
func (c *Component) Resize(width, height int) {
	// store height for Container's call to responsive list's Height
	c.height = height
	c.container.Resize(width, height)
}

// Input returns the input.Box employed by this Component.
func (c *Component) Input() *input.Box {
	return &c.box
}

// InputPosition returns the offset of the input component.
func (c *Component) InputPosition() term.Coordinates {
	return term.Coordinates{
		Y: c.inputRow.Position().Y,
		X: c.inputCol.Position().X,
	}
}

// MessagesPosition returns the offset of the messages component.
func (c *Component) MessagesPosition() term.Coordinates {
	return c.spanMessages.ContentOffset()
}

// InputSubmit submits the contents of the input buffer as a send message,
// and returns it for delivery or returns false if there's no text
// in the input buffer. There is no need to call AddSendMessage
// with the return value.
func (c *Component) InputSubmit() (string, bool) {
	if c.box.Buffer().Size() == 0 {
		return "", false
	}

	str := c.box.Buffer().String()
	c.box.Reset()
	c.AddSendMessage(str)
	return str, true
}

// AddSendMessage adds the following msg as a sent message.
func (c *Component) AddSendMessage(msg string) {
	strComp := component.StringResponsive(msg,
		component.StringResponsiveConfig{
			NoSplitWords: true,
			StringConfig: c.cfg.SendMessageStringConfig,
		})
	strComp = component.WithAttrSetter(strComp)
	strComp = component.NewSpan(strComp, c.cfg.SendMessageSpanConfig)
	c.messages.PushBack(strComp)
}

// AddReceiveMessage adds the following message as a received message.
func (c *Component) AddReceiveMessage(msg string) {
	c.AddReceiveMessageChunk(msg)
	c.AddReceiveMessageBreak()
}

// AddReceiveMessageBreak breaks a receive message stream
// such that next call to AddReceiveMessageChunk will add
// a new chunk of data as a new message.
func (c *Component) AddReceiveMessageBreak() {
	c.tail = nil
	c.msg.Reset()
	return
}

// AddReceiveMessage adds the following message chunk
// as a received message. A new message is started
// by calling AddReceiveMessageBreak.
func (c *Component) AddReceiveMessageChunk(chunk string) {
	c.msg.WriteString(chunk)
	if c.tail != nil {
		c.messages.Remove(*c.tail)
	} else {
		c.tail = new(component.ListNode)
	}
	strComp := component.StringResponsive(c.msg.String(),
		component.StringResponsiveConfig{
			NoSplitWords: true,
			StringConfig: c.cfg.ReceiveMessageStringConfig,
		})
	strComp = component.WithAttrSetter(strComp)
	strComp = component.NewSpan(strComp, c.cfg.ReceiveMessageSpanConfig)
	*c.tail = c.messages.PushBack(strComp)
}

// SeekDown seeks the history panel down.
func (c *Component) SeekDown() bool {
	return c.messages.SeekDown()
}

// SeekUp seeks the history panel up.
func (c *Component) SeekUp() bool {
	return c.messages.SeekUp()
}

func (c *Component) boxWidth(width int) int {
	return int(float64(width) * float64(c.cfg.InputRowColumns) / float64(component.MaxCols))
}
