package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// Prompt implements a prompt / question with options component.
type Prompt struct {
	optComp   []WithAttributes
	effective tui.Component
}

// PromptConfig holds configuration for initializing a Prompt.
type PromptConfig struct {
	Message string
	Options []string
	Frame   FrameCharSet
}

func makeOption(msg string, cfg PromptConfig) (ret WithAttributes) {
	return stringBackgroundAttrFrame(msg, term.Attributes{},
		0, term.Attributes{}, cfg.Frame, 2, 0)
}

func (p *Prompt) initOptions(cfg PromptConfig, wm *WindowManager, win Window) {
	if len(cfg.Options) == 0 {
		panic("Options should be greater tha one")
	}
	comp0 := makeOption(cfg.Options[0], cfg)
	win, _ = wm.SplitHorizontal(win, comp0)
	p.optComp = append(p.optComp, comp0)

	for _, opt := range cfg.Options[1:] {
		compi := makeOption(opt, cfg)
		p.optComp = append(p.optComp, compi)
		win, _ = wm.SplitVertical(win, compi)
	}
}

// Init initializes this prompt cfg.
func (p *Prompt) Init(cfg PromptConfig) {
	message := StringCentered(cfg.Message)

	wm, win := NewWindowManager(message, WindowManagerConfig{})
	p.initOptions(cfg, wm, win)

	if cfg.Frame != (FrameCharSet{}) {
		frame := NewFrame(wm)
		frame.FrameCharSet = cfg.Frame
		p.effective = frame
	} else {
		p.effective = wm
	}
}

// NewPrompt allocates storage for a new prompt and initializes it.
func NewPrompt(cfg PromptConfig) *Prompt {
	p := new(Prompt)
	p.Init(cfg)
	return p
}

// SetOptionAttr sets the attributes of option at index i.
func (p *Prompt) SetOptionAttr(i int, attr term.Attributes) {
	if i < 0 || i >= len(p.optComp) {
		panic("option out of bounds")
	}
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
