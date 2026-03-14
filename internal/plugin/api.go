package plugin

import (
	"github.com/kristianweb/zephyr/internal/command"
	"github.com/kristianweb/zephyr/internal/editor"
)

// EditorAPI implements PluginAPI for the editor.
type EditorAPI struct {
	Editor   *editor.Editor
	Registry *command.Registry
	Messages []string
}

// NewEditorAPI creates a plugin API for the given editor.
func NewEditorAPI(ed *editor.Editor, reg *command.Registry) *EditorAPI {
	return &EditorAPI{
		Editor:   ed,
		Registry: reg,
	}
}

func (api *EditorAPI) GetText() string {
	return api.Editor.Buffer.Text()
}

func (api *EditorAPI) InsertText(text string) {
	api.Editor.InsertText(text)
}

func (api *EditorAPI) GetSelection() string {
	return api.Editor.SelectedText()
}

func (api *EditorAPI) GetCursorPosition() (int, int) {
	return api.Editor.Cursor.Line, api.Editor.Cursor.Col
}

func (api *EditorAPI) SetCursorPosition(line, col int) {
	api.Editor.Cursor.SetPosition(api.Editor.Buffer, line, col)
}

func (api *EditorAPI) RegisterCommand(id, title string, handler func() error) {
	api.Registry.Register(&command.Command{
		ID:      id,
		Title:   title,
		Handler: handler,
	})
}

func (api *EditorAPI) ExecuteCommand(id string) error {
	return api.Registry.Execute(id)
}

func (api *EditorAPI) ShowMessage(msg string) {
	api.Messages = append(api.Messages, msg)
}

func (api *EditorAPI) GetFilePath() string {
	return api.Editor.FilePath
}
