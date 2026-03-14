package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kristianweb/zephyr/internal/buffer"
	"github.com/kristianweb/zephyr/internal/command"
	"github.com/kristianweb/zephyr/internal/editor"
)

func testAPI() *EditorAPI {
	ed := editor.NewEditor(buffer.NewFromString("hello world"), "test.go")
	reg := command.NewRegistry()
	return NewEditorAPI(ed, reg)
}

func writeLuaFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lua")
	os.WriteFile(path, []byte(content), 0644)
	return path
}

func TestLuaPlugin_LoadAndInit(t *testing.T) {
	api := testAPI()
	path := writeLuaFile(t, `
		function init()
			zephyr.show_message("Plugin loaded!")
		end
	`)
	p := NewLuaPlugin("test", path, api)
	if err := p.Load(); err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if len(api.Messages) != 1 || api.Messages[0] != "Plugin loaded!" {
		t.Fatalf("expected message, got %v", api.Messages)
	}
}

func TestLuaPlugin_RegisterCommand(t *testing.T) {
	api := testAPI()
	path := writeLuaFile(t, `
		function init()
			zephyr.register_command("plugin.hello", "Plugin: Hello", function()
				zephyr.show_message("Hello from plugin!")
			end)
		end
	`)
	p := NewLuaPlugin("test", path, api)
	if err := p.Load(); err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	cmd := api.Registry.Get("plugin.hello")
	if cmd == nil {
		t.Fatal("expected command to be registered")
	}

	api.Registry.Execute("plugin.hello")
	found := false
	for _, msg := range api.Messages {
		if msg == "Hello from plugin!" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'Hello from plugin!' message, got %v", api.Messages)
	}
}

func TestLuaPlugin_InsertText(t *testing.T) {
	api := testAPI()
	path := writeLuaFile(t, `
		function init()
			zephyr.insert_text(" inserted")
		end
	`)
	p := NewLuaPlugin("test", path, api)
	if err := p.Load(); err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if api.Editor.Buffer.Text() != " insertedhello world" {
		t.Fatalf("got %q", api.Editor.Buffer.Text())
	}
}

func TestLuaPlugin_GetSelection(t *testing.T) {
	api := testAPI()
	api.Editor.Selection.Start(editor.Cursor{Line: 0, Col: 0})
	api.Editor.Selection.Update(editor.Cursor{Line: 0, Col: 5})

	path := writeLuaFile(t, `
		function init()
			local sel = zephyr.get_selection()
			zephyr.show_message("Selected: " .. sel)
		end
	`)
	p := NewLuaPlugin("test", path, api)
	if err := p.Load(); err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	found := false
	for _, msg := range api.Messages {
		if msg == "Selected: hello" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected selection message, got %v", api.Messages)
	}
}

func TestLuaPlugin_OnEvent_BufferChange(t *testing.T) {
	api := testAPI()
	path := writeLuaFile(t, `
		function on_buffer_change()
			zephyr.show_message("Buffer changed!")
		end
	`)
	p := NewLuaPlugin("test", path, api)
	if err := p.Load(); err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	p.CallEvent("buffer_change")
	found := false
	for _, msg := range api.Messages {
		if msg == "Buffer changed!" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected buffer change event, got %v", api.Messages)
	}
}

func TestLuaPlugin_SandboxSecurity(t *testing.T) {
	api := testAPI()
	// Attempt to use os module (should be nil/sandboxed)
	path := writeLuaFile(t, `
		function init()
			if os == nil then
				zephyr.show_message("sandboxed")
			else
				zephyr.show_message("not sandboxed")
			end
		end
	`)
	p := NewLuaPlugin("test", path, api)
	if err := p.Load(); err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	found := false
	for _, msg := range api.Messages {
		if msg == "sandboxed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected sandbox message, got %v", api.Messages)
	}
}

func TestLuaPlugin_ErrorHandling(t *testing.T) {
	api := testAPI()
	path := writeLuaFile(t, `
		function init()
			error("intentional error")
		end
	`)
	p := NewLuaPlugin("test", path, api)
	err := p.Load()
	if err == nil {
		t.Fatal("expected error from plugin")
	}
	p.Close()
}
