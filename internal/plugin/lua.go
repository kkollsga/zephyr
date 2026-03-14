package plugin

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// LuaPlugin represents a Lua script plugin.
type LuaPlugin struct {
	Name    string
	Path    string
	state   *lua.LState
	api     *EditorAPI
}

// NewLuaPlugin creates a new Lua plugin instance.
func NewLuaPlugin(name, path string, api *EditorAPI) *LuaPlugin {
	return &LuaPlugin{
		Name: name,
		Path: path,
		api:  api,
	}
}

// Load initializes and loads the Lua plugin.
func (lp *LuaPlugin) Load() error {
	L := lua.NewState(lua.Options{
		SkipOpenLibs: false,
	})

	// Sandbox: remove dangerous functions
	L.SetGlobal("os", lua.LNil)
	L.SetGlobal("io", lua.LNil)
	L.SetGlobal("loadfile", lua.LNil)
	L.SetGlobal("dofile", lua.LNil)

	// Register the editor API
	lp.registerAPI(L)

	lp.state = L

	// Execute the plugin file
	if err := L.DoFile(lp.Path); err != nil {
		return fmt.Errorf("plugin %s: %w", lp.Name, err)
	}

	// Call init() if defined
	init := L.GetGlobal("init")
	if init != lua.LNil {
		if err := L.CallByParam(lua.P{Fn: init, NRet: 0, Protect: true}); err != nil {
			return fmt.Errorf("plugin %s init: %w", lp.Name, err)
		}
	}

	return nil
}

func (lp *LuaPlugin) registerAPI(L *lua.LState) {
	// Create the "zephyr" module
	mod := L.NewTable()

	// zephyr.get_text() -> string
	L.SetField(mod, "get_text", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(lp.api.GetText()))
		return 1
	}))

	// zephyr.insert_text(text)
	L.SetField(mod, "insert_text", L.NewFunction(func(L *lua.LState) int {
		text := L.CheckString(1)
		lp.api.InsertText(text)
		return 0
	}))

	// zephyr.get_selection() -> string
	L.SetField(mod, "get_selection", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(lp.api.GetSelection()))
		return 1
	}))

	// zephyr.get_cursor() -> line, col
	L.SetField(mod, "get_cursor", L.NewFunction(func(L *lua.LState) int {
		line, col := lp.api.GetCursorPosition()
		L.Push(lua.LNumber(line))
		L.Push(lua.LNumber(col))
		return 2
	}))

	// zephyr.set_cursor(line, col)
	L.SetField(mod, "set_cursor", L.NewFunction(func(L *lua.LState) int {
		line := L.CheckInt(1)
		col := L.CheckInt(2)
		lp.api.SetCursorPosition(line, col)
		return 0
	}))

	// zephyr.register_command(id, title, handler)
	L.SetField(mod, "register_command", L.NewFunction(func(L *lua.LState) int {
		id := L.CheckString(1)
		title := L.CheckString(2)
		fn := L.CheckFunction(3)
		lp.api.RegisterCommand(id, title, func() error {
			return L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true})
		})
		return 0
	}))

	// zephyr.show_message(msg)
	L.SetField(mod, "show_message", L.NewFunction(func(L *lua.LState) int {
		msg := L.CheckString(1)
		lp.api.ShowMessage(msg)
		return 0
	}))

	// zephyr.get_file_path() -> string
	L.SetField(mod, "get_file_path", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(lp.api.GetFilePath()))
		return 1
	}))

	L.SetGlobal("zephyr", mod)
}

// Close cleans up the Lua state.
func (lp *LuaPlugin) Close() {
	if lp.state != nil {
		lp.state.Close()
	}
}

// CallEvent calls a named event handler in the plugin if defined.
func (lp *LuaPlugin) CallEvent(eventName string, args ...lua.LValue) error {
	if lp.state == nil {
		return nil
	}
	fn := lp.state.GetGlobal("on_" + eventName)
	if fn == lua.LNil {
		return nil
	}
	return lp.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    0,
		Protect: true,
	}, args...)
}
