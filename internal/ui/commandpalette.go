package ui

import "github.com/kristianweb/zephyr/internal/command"

// CommandPalette manages the command palette overlay state.
type CommandPalette struct {
	Visible   bool
	Query     string
	Results   []*command.Command
	Selected  int
	Registry  *command.Registry
}

// NewCommandPalette creates a new command palette.
func NewCommandPalette(reg *command.Registry) *CommandPalette {
	return &CommandPalette{
		Registry: reg,
	}
}

// Open shows the command palette.
func (cp *CommandPalette) Open() {
	cp.Visible = true
	cp.Query = ""
	cp.Selected = 0
	cp.Results = cp.Registry.All()
}

// Close hides the command palette.
func (cp *CommandPalette) Close() {
	cp.Visible = false
	cp.Query = ""
	cp.Results = nil
}

// UpdateQuery filters commands based on the current query.
func (cp *CommandPalette) UpdateQuery(query string) {
	cp.Query = query
	cp.Results = cp.Registry.Search(query)
	cp.Selected = 0
}

// MoveUp moves selection up.
func (cp *CommandPalette) MoveUp() {
	if cp.Selected > 0 {
		cp.Selected--
	}
}

// MoveDown moves selection down.
func (cp *CommandPalette) MoveDown() {
	if cp.Selected < len(cp.Results)-1 {
		cp.Selected++
	}
}

// Execute runs the selected command and closes the palette.
func (cp *CommandPalette) Execute() error {
	if cp.Selected < 0 || cp.Selected >= len(cp.Results) {
		cp.Close()
		return nil
	}
	cmd := cp.Results[cp.Selected]
	cp.Close()
	if cmd.Handler != nil {
		return cmd.Handler()
	}
	return nil
}

// SelectedCommand returns the currently highlighted command, or nil.
func (cp *CommandPalette) SelectedCommand() *command.Command {
	if cp.Selected >= 0 && cp.Selected < len(cp.Results) {
		return cp.Results[cp.Selected]
	}
	return nil
}
