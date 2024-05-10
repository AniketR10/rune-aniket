package scanner

const (
	MaxIntermediates = 2
	MaxOSCParams     = 16
	MaxOSCRaw        = 1024
	// MaxParams represents the max number of parameters
	// passed to the Perform ifc.
	MaxParams = 32
)

// State represents the scanner state.
type State uint8

const (
	Anywhere State = iota
	CsiEntry
	CsiIgnore
	CsiIntermediate
	CsiParam
	DcsEntry
	DcsIgnore
	DcsIntermediate
	DcsParam
	DcsPassthrough
	Escape
	EscapeIntermediate
	Ground
	OSCString
	SosPmApcString
	Utf8
)

// Action represents the scanner action.
type Action int

const (
	None Action = iota
	Clear
	Collect
	CSIDispatch
	ESCDispatch
	Execute
	Hook
	Ignore
	OSCEnd
	OSCPut
	OSCStart
	Param
	Print
	Put
	Unhook
	BeginUtf8
)

// unpack unpacks a uint8 into a State and Action.
func unpack(delta uint8) (State, Action) {
	return State(delta & 0x0f), Action(delta >> 4)
}

// nolint:unused
// pack packs a State and Action into a uint8.
func pack(state State, action Action) uint8 {
	return uint8(action)<<4 | uint8(state)
}
