package vim

// RegisterFile manages vim registers.
type RegisterFile struct {
	Named   map[rune]string // "a-"z named registers
	Unnamed string          // " register (default)
	Yank    string          // 0 register (last yank)
	Delete  [9]string       // 1-9 registers (last deletes, shifted)
	Small   string          // - register (small delete < 1 line)
	Search  string          // / register (last search)
}

// NewRegisterFile creates an empty register file.
func NewRegisterFile() RegisterFile {
	return RegisterFile{
		Named: make(map[rune]string),
	}
}

// Get returns the contents of a register.
func (rf *RegisterFile) Get(reg rune) string {
	switch {
	case reg == '"':
		return rf.Unnamed
	case reg == '0':
		return rf.Yank
	case reg >= '1' && reg <= '9':
		return rf.Delete[reg-'1']
	case reg == '-':
		return rf.Small
	case reg == '/':
		return rf.Search
	case reg == '+' || reg == '*':
		// System clipboard — handled by the app layer
		return ""
	case reg >= 'a' && reg <= 'z':
		return rf.Named[reg]
	case reg >= 'A' && reg <= 'Z':
		// Uppercase = append to named register (read returns same as lowercase)
		return rf.Named[reg-'A'+'a']
	}
	return ""
}

// Set stores text into a register.
func (rf *RegisterFile) Set(reg rune, text string) {
	switch {
	case reg == '"':
		rf.Unnamed = text
	case reg == '0':
		rf.Yank = text
	case reg == '-':
		rf.Small = text
	case reg == '/':
		rf.Search = text
	case reg >= 'a' && reg <= 'z':
		rf.Named[reg] = text
	case reg >= 'A' && reg <= 'Z':
		// Uppercase = append
		lower := reg - 'A' + 'a'
		rf.Named[lower] += text
	}
}

// RecordYank stores yanked text in the unnamed and yank registers.
func (rf *RegisterFile) RecordYank(text string, targetReg rune) {
	rf.Yank = text
	rf.Unnamed = text
	if targetReg != '"' {
		rf.Set(targetReg, text)
	}
}

// RecordDelete stores deleted text in the unnamed register and shifts the
// numbered delete registers (1-9). Small deletes (< 1 line) go to "-".
func (rf *RegisterFile) RecordDelete(text string, linewise bool, targetReg rune) {
	rf.Unnamed = text
	if targetReg != '"' {
		rf.Set(targetReg, text)
		return
	}
	if !linewise && len(text) > 0 {
		// Check if it's a "small" delete (no newline)
		hasNewline := false
		for _, r := range text {
			if r == '\n' {
				hasNewline = true
				break
			}
		}
		if !hasNewline {
			rf.Small = text
			return
		}
	}
	// Shift numbered registers
	for i := 8; i > 0; i-- {
		rf.Delete[i] = rf.Delete[i-1]
	}
	rf.Delete[0] = text
}
