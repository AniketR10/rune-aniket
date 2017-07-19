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

type CellWriter interface {
	SetCell(x, y int, r rune, fg, bg Attribute) error
}

type Window interface {
	Resize(width, height int) error
	MoveTo(x, y int) error
	Draw(w CellWriter) error
	Height() int
	Width() int
	Position() (int, int)
}
