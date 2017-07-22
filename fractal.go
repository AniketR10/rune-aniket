package fractal

type Attribute uint16

const (
	ColorDefault Attribute = iota
	ColorBlack
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite
)

const (
	AttrBold Attribute = 1 << (iota + 9)
	AttrUnderline
	AttrReverse
)

// TODO Printer
type Writer interface {
	Write(x, y int, r rune) error
	SetAttributes(x, y int, fg Attribute, bg Attribute)
	Flush() error
	Clear(fg, bg Attribute) error
}

// TODO change for component
type Window interface {
	Resize(width, height int) error
	MoveTo(x, y int) error
	// TODO Flush
	Draw(w Writer) error
	Height() int
	Width() int
	Position() (int, int)
}

// TODO use
type Coordinates struct {
	x, y int
}

// TODO use
type Attributes struct {
	FG Attribute
	BG Attribute
}

type Cell struct {
	X, Y   int
	Fg, Bg Attribute
	Ch     rune
}
