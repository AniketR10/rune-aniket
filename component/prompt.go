package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// Prompt implements a prompt / question with options component.
type Prompt struct {
	optComp      []WithAttributes
	effective    tui.Component
	makeOptionFn func(msg string, cfg PromptConfig) WithAttributes
}

// PromptConfig holds configuration for initializing a Prompt.
type PromptConfig struct {
	Message string
	Options []string
	Frame   FrameCharSet
}

func (p *Prompt) initOptions(cfg PromptConfig, wm *WindowManager, win Window) {
	if len(cfg.Options) == 0 {
		panic("Options should be greater than zero")
	}
	// allow for Init to be used as reset
	p.optComp = make([]WithAttributes, 0)
	comp0 := p.makeOptionFn(cfg.Options[0], cfg)
	win, _ = wm.SplitHorizontal(win, comp0)
	p.optComp = append(p.optComp, comp0)

	for _, opt := range cfg.Options[1:] {
		compi := p.makeOptionFn(opt, cfg)
		p.optComp = append(p.optComp, compi)
		win, _ = wm.SplitVertical(win, compi)
	}
}

func (p *Prompt) init(
	makeOptionFn func(string, PromptConfig) WithAttributes,
	cfg PromptConfig,
) {
	if cfg.Message == "" {
		panic("Message cannot be empty")
	}
	message := StringCentered(cfg.Message)

	wm, win := NewWindowManager(message, WindowManagerConfig{})
	p.makeOptionFn = makeOptionFn
	p.initOptions(cfg, wm, win)

	if cfg.Frame != (FrameCharSet{}) {
		frame := NewFrame(wm)
		frame.FrameCharSet = cfg.Frame
		p.effective = frame
	} else {
		p.effective = wm
	}
}

// Init initializes this prompt with cfg. Note that PromptConfig.Options must
// always contain at least one option and PromptConfig.Message must not be empty.
// If one of these two rules is violated this method panics.
func (p *Prompt) Init(cfg PromptConfig) {
	p.init(func(msg string, cfg PromptConfig) WithAttributes {
		cells := cell.StringToCells(msg)
		return newStringComp(cells, term.Attributes{},
			0, term.Attributes{}, cfg.Frame, 2, 0, SpanAlignmentCentered)
	}, cfg)
}

// NewPrompt allocates storage for a new prompt and initializes it.
// See Init for more details.
func NewPrompt(cfg PromptConfig) *Prompt {
	p := new(Prompt)
	p.Init(cfg)
	return p
}

// SetOptionAttr sets the attributes of option at index i.
func (p *Prompt) SetOptionAttr(i int, attr term.Attributes) {
	p.optComp[i].SetAttr(attr)
}

// Resize satisfies tui.Component
func (p *Prompt) Resize(width, height int) {
	p.effective.Resize(width, height)
}

// Draw satisfies tui.Component
func (p *Prompt) Draw(w term.Writer) {
	p.effective.Draw(w)
}
