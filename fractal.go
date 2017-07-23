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

type Writer interface {
	Write(x, y int, r rune, fg Attribute, bg Attribute) error
	Flush() error
	Clear(fg, bg Attribute) error
}

type Component interface {
	Resize(width, height int) error
	Move(x, y int) error
	Draw(w Writer) error
	Height() int
	Width() int
	Position() (int, int)
}

type Coordinates struct {
	X, Y int
}

type Cell struct {
	Coordinates
	Fg, Bg Attribute
	Ch     rune
}
