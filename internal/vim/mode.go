package vim

// Mode represents the current vim editing mode.
type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeVisual
	ModeVisualLine
	ModeVisualBlock
	ModeCommand // : command line
	ModeSearch  // / or ? search
	ModeReplace // single-char replace (r)
)

// String returns the display name of the mode.
func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return "NORMAL"
	case ModeInsert:
		return "INSERT"
	case ModeVisual:
		return "VISUAL"
	case ModeVisualLine:
		return "V-LINE"
	case ModeVisualBlock:
		return "V-BLOCK"
	case ModeCommand:
		return "COMMAND"
	case ModeSearch:
		return "SEARCH"
	case ModeReplace:
		return "REPLACE"
	default:
		return "NORMAL"
	}
}

// Operator represents a pending operator (d, c, y, etc.).
type Operator int

const (
	OpNone Operator = iota
	OpDelete
	OpChange
	OpYank
	OpIndent
	OpDedent
)

// State holds the complete vim state machine state.
type State struct {
	Mode     Mode
	PrevMode Mode // mode before entering command/search

	// Count and operator pending
	Count      int
	Operator   Operator
	PendingBuf string // accumulated keys for visual feedback

	// Register
	Register  rune
	Registers RegisterFile

	// Command/search line
	CommandLine   string
	CommandCursor int
	SearchDir     int    // +1 forward, -1 backward
	SearchPattern string // last search pattern

	// Dot repeat
	LastAction     Action
	LastInsertText string // text typed during last insert session

	// Visual mode anchor
	VisualAnchorLine int
	VisualAnchorCol  int

	// f/t/F/T character find
	FindChar        rune
	FindCharForward bool
	FindCharTill    bool

	// Waiting for a character (f, t, F, T, r)
	WaitingForChar     bool
	WaitingForCharType rune // 'f', 't', 'F', 'T', 'r'

	// Waiting for text object delimiter (after i or a in operator-pending)
	WaitingForTextObj     bool
	WaitingForTextObjType rune // 'i' (inner) or 'a' (around)

	// Navigator mode: when true, <Space> acts as leader key
	NavigatorEnabled bool
}

// NewState creates a new vim state in Normal mode.
func NewState() *State {
	return &State{
		Mode:      ModeNormal,
		Register:  '"',
		Registers: NewRegisterFile(),
		SearchDir: 1,
	}
}

// reset clears the pending command state.
func (s *State) reset() {
	s.Count = 0
	s.Operator = OpNone
	s.PendingBuf = ""
	s.WaitingForChar = false
	s.WaitingForTextObj = false
}

// pendingInput reports whether a key already pressed is waiting for the next
// keystroke to complete it: r/f/t/F/T waiting for their character, an operator
// waiting for a motion, i/a waiting for a text-object delimiter, or a two-key
// sequence (g, z, [, ], <Space>) waiting for its second key. A bare count is
// not pending input — it consumes nothing on its own.
func (s *State) pendingInput() bool {
	if s.WaitingForChar || s.WaitingForTextObj || s.Operator != OpNone {
		return true
	}
	for _, c := range s.PendingBuf {
		if c < '0' || c > '9' {
			return true
		}
	}
	return false
}
