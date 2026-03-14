package command

import "gioui.org/io/key"

// RegisterBuiltinCommands registers all built-in editor commands.
// The handlers are typically closures that capture the app state.
type CommandHandlers struct {
	Save          func() error
	Undo          func() error
	Redo          func() error
	Cut           func() error
	Copy          func() error
	Paste         func() error
	SelectAll     func() error
	Find          func() error
	FindReplace   func() error
	OpenFile      func() error
	CloseTab      func() error
	NewFile       func() error
	ToggleSidebar func() error
	CommandPalette func() error
	FuzzyFinder   func() error
	Quit          func() error
}

// RegisterAll registers all built-in commands and their default keybindings.
func RegisterAll(reg *Registry, km *KeybindingManager, h CommandHandlers) {
	cmds := []struct {
		id    string
		title string
		handler Handler
		key   string
	}{
		{"file.save", "File: Save", h.Save, "Cmd+S"},
		{"file.open", "File: Open", h.OpenFile, "Cmd+O"},
		{"file.new", "File: New", h.NewFile, "Cmd+N"},
		{"file.close", "File: Close Tab", h.CloseTab, "Cmd+W"},
		{"edit.undo", "Edit: Undo", h.Undo, "Cmd+Z"},
		{"edit.redo", "Edit: Redo", h.Redo, "Cmd+Shift+Z"},
		{"edit.cut", "Edit: Cut", h.Cut, "Cmd+X"},
		{"edit.copy", "Edit: Copy", h.Copy, "Cmd+C"},
		{"edit.paste", "Edit: Paste", h.Paste, "Cmd+V"},
		{"edit.selectAll", "Edit: Select All", h.SelectAll, "Cmd+A"},
		{"edit.find", "Edit: Find", h.Find, "Cmd+F"},
		{"edit.findReplace", "Edit: Find and Replace", h.FindReplace, "Cmd+Shift+F"},
		{"view.sidebar", "View: Toggle Sidebar", h.ToggleSidebar, "Cmd+B"},
		{"view.commandPalette", "View: Command Palette", h.CommandPalette, "Cmd+Shift+P"},
		{"view.fuzzyFinder", "View: Quick Open", h.FuzzyFinder, "Cmd+P"},
		{"app.quit", "Application: Quit", h.Quit, "Cmd+Q"},
	}

	for _, c := range cmds {
		cmd := &Command{
			ID:         c.id,
			Title:      c.title,
			Handler:    c.handler,
			Keybinding: c.key,
		}
		reg.Register(cmd)

		// Parse and bind keybinding
		if c.key != "" {
			keyName, mods, err := ParseKeybinding(c.key)
			if err == nil {
				km.Bind(Keybinding{Key: keyName, Modifiers: mods, CommandID: c.id})
			}
		}
	}
}

// DefaultKeyName returns the key name for common key constants.
func DefaultKeyName(name string) key.Name {
	return key.Name(name)
}
