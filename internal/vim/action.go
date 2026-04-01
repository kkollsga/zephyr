package vim

// ActionKind identifies what action the vim state machine wants the editor to perform.
type ActionKind int

const (
	ActionNone ActionKind = iota

	// Movement
	ActionMoveLeft          // h
	ActionMoveRight         // l
	ActionMoveUp            // k
	ActionMoveDown          // j
	ActionMoveWordForward   // w
	ActionMoveWordEnd       // e
	ActionMoveWordBackward  // b
	ActionMoveBigWordFwd    // W
	ActionMoveBigWordEnd    // E
	ActionMoveBigWordBack   // B
	ActionMoveLineStart     // 0
	ActionMoveLineEnd       // $
	ActionMoveFirstNonBlank // ^
	ActionMoveFileStart     // gg
	ActionMoveFileEnd       // G (no count)
	ActionMoveToLine        // {count}G or :{count}
	ActionMoveHalfPageDown  // Ctrl+d
	ActionMoveHalfPageUp    // Ctrl+u
	ActionMovePageDown      // Ctrl+f
	ActionMovePageUp        // Ctrl+b
	ActionMoveParagraphDown // }
	ActionMoveParagraphUp   // {
	ActionMoveBracketMatch  // %
	ActionMoveFindChar      // f{char}
	ActionMoveTillChar      // t{char}
	ActionMoveFindCharBack  // F{char}
	ActionMoveTillCharBack  // T{char}
	ActionRepeatFindChar    // ;
	ActionRepeatFindCharRev // ,

	// Scrolling (no cursor move)
	ActionScrollCenter // zz
	ActionScrollTop    // zt
	ActionScrollBottom // zb

	// Insert mode transitions
	ActionInsertBefore    // i
	ActionInsertAfter     // a
	ActionInsertLineStart // I
	ActionInsertLineEnd   // A
	ActionOpenBelow       // o
	ActionOpenAbove       // O
	ActionSubstChar       // s (delete char + insert)
	ActionSubstLine       // S (delete line + insert)

	// Editing
	ActionDelete    // d{motion}, dd, x, X
	ActionChange    // c{motion}, cc, C, s, S
	ActionYank      // y{motion}, yy, Y
	ActionPut       // p
	ActionPutBefore // P
	ActionReplace   // r{char}
	ActionJoinLines // J
	ActionUndo      // u
	ActionRedo      // Ctrl+r
	ActionRepeatLast // .
	ActionIndent    // >{motion}, >>
	ActionDedent    // <{motion}, <<

	// Visual mode
	ActionVisualStart     // v
	ActionVisualLineStart // V
	ActionVisualBlockStart // Ctrl+v
	ActionVisualEscape    // escape from visual

	// Command/Search
	ActionEnterCommand    // :
	ActionEnterSearch     // /
	ActionEnterSearchBack // ?
	ActionSearchNext      // n
	ActionSearchPrev      // N
	ActionSearchWordUnder // *
	ActionExecCommand     // Enter pressed in command mode
	ActionCancelCommand   // Escape in command/search mode

	// File operations (from : commands)
	ActionWrite     // :w
	ActionQuit      // :q
	ActionWriteQuit // :wq
	ActionForceQuit // :q!
	ActionTutor     // :Tutor

	// Mode transitions
	ActionEnterInsert // generic enter insert
	ActionEnterNormal // generic enter normal

	// Navigator: Hunk navigation
	ActionNavNextHunk      // ]c
	ActionNavPrevHunk      // [c
	ActionNavNextFile      // ]C — next changed file
	ActionNavPrevFile      // [C — previous changed file
	ActionNavToggleOriginal // go — toggle original/modified

	// Navigator: File navigation
	ActionNavOpenParent    // - — open parent directory
	ActionNavOpenEntry     // Enter/l in directory buffer
	ActionNavCloseSpecial  // q in directory/status buffer
	ActionNavOpenRoot      // <Space>e — open project root

	// Navigator: Status buffer
	ActionNavOpenStatus    // <Space>g — open git status
	ActionNavStage         // s in status buffer
	ActionNavUnstage       // u in status buffer
	ActionNavDiscard       // x in status buffer
	ActionNavToggleDiff    // = in status buffer
	ActionNavSectionNext   // n in status buffer
	ActionNavSectionPrev   // p in status buffer
	ActionNavRefresh       // R in status buffer

	// Navigator: Import & alternate
	ActionNavGoFile       // gf — go to file under cursor
	ActionNavGoImports    // gi — show imports
	ActionNavGoAlternate  // ga — alternate file (test <-> impl)

	// Navigator: Fuzzy finding
	ActionNavFindFiles   // <Space>f — find files
	ActionNavFindChanged // <Space>b — find changed files

	// Navigator: Help
	ActionNavHelp // g? — context help

	// Navigator: Directory buffer
	ActionNavToggleHidden  // . — toggle hidden files
	ActionNavToggleReadMode // <Space>r — toggle markdown read/edit

	// Generic keys used by special buffers
	ActionEnterKey     // Enter key pressed
	ActionTabKey       // Tab key pressed
	ActionBackspaceKey // Backspace key pressed
)

// MotionType distinguishes line-wise vs char-wise operations.
type MotionType int

const (
	MotionCharWise MotionType = iota
	MotionLineWise
)

// Action is the output of the vim state machine.
type Action struct {
	Kind       ActionKind
	Count      int        // repeat count (0 = not specified, treat as 1)
	Register   rune       // register for yank/put
	Motion     ActionKind // for operator+motion combos: the motion part
	MotionType MotionType // line-wise or char-wise
	Char       rune       // for f/t/r operations
	Text       string     // for command line, search term
	Line       int        // target line number for ActionMoveToLine (1-based)

	// Text object info (for ci", diw, etc.)
	TextObj     rune // delimiter: w, W, ", ', (, ), [, ], {, }, `, etc.
	TextObjType rune // 'i' (inner) or 'a' (around)
}

// EffectiveCount returns the count, defaulting to 1 if unspecified.
func (a Action) EffectiveCount() int {
	if a.Count <= 0 {
		return 1
	}
	return a.Count
}
