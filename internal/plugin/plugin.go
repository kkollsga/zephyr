package plugin

// Plugin represents a loaded plugin.
type Plugin struct {
	Name    string
	Path    string
	Enabled bool
}

// PluginAPI is the interface that plugins can use to interact with the editor.
type PluginAPI interface {
	// Buffer operations
	GetText() string
	InsertText(text string)
	GetSelection() string
	GetCursorPosition() (line, col int)
	SetCursorPosition(line, col int)

	// Command operations
	RegisterCommand(id, title string, handler func() error)
	ExecuteCommand(id string) error

	// UI operations
	ShowMessage(msg string)
	GetFilePath() string
}
