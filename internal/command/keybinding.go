package command

import (
	"fmt"
	"strings"

	"gioui.org/io/key"
)

// Keybinding maps a key combination to a command ID.
type Keybinding struct {
	Key       key.Name
	Modifiers key.Modifiers
	CommandID string
}

// KeybindingManager manages key-to-command mappings.
type KeybindingManager struct {
	bindings []Keybinding
	registry *Registry
}

// NewKeybindingManager creates a keybinding manager for the given registry.
func NewKeybindingManager(reg *Registry) *KeybindingManager {
	return &KeybindingManager{
		registry: reg,
	}
}

// Bind adds a keybinding.
func (km *KeybindingManager) Bind(binding Keybinding) {
	// Remove existing binding for the same key combo
	for i, b := range km.bindings {
		if b.Key == binding.Key && b.Modifiers == binding.Modifiers {
			km.bindings = append(km.bindings[:i], km.bindings[i+1:]...)
			break
		}
	}
	km.bindings = append(km.bindings, binding)
}

// Match returns the command ID for a key event, or empty string.
func (km *KeybindingManager) Match(name key.Name, mods key.Modifiers) string {
	for _, b := range km.bindings {
		if b.Key == name && b.Modifiers == mods {
			return b.CommandID
		}
	}
	return ""
}

// Execute matches a key event and executes the corresponding command.
// Returns true if a command was executed.
func (km *KeybindingManager) Execute(name key.Name, mods key.Modifiers) (bool, error) {
	cmdID := km.Match(name, mods)
	if cmdID == "" {
		return false, nil
	}
	return true, km.registry.Execute(cmdID)
}

// HasConflict checks if a key combo is already bound.
func (km *KeybindingManager) HasConflict(name key.Name, mods key.Modifiers) (bool, string) {
	for _, b := range km.bindings {
		if b.Key == name && b.Modifiers == mods {
			return true, b.CommandID
		}
	}
	return false, ""
}

// All returns all keybindings.
func (km *KeybindingManager) All() []Keybinding {
	result := make([]Keybinding, len(km.bindings))
	copy(result, km.bindings)
	return result
}

// ParseKeybinding parses a string like "Cmd+S" or "Cmd+Shift+P" into Key and Modifiers.
func ParseKeybinding(s string) (key.Name, key.Modifiers, error) {
	parts := strings.Split(s, "+")
	if len(parts) == 0 {
		return "", 0, fmt.Errorf("empty keybinding")
	}

	var mods key.Modifiers
	var keyName key.Name

	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch strings.ToLower(part) {
		case "cmd", "command", "super":
			mods |= key.ModShortcut
		case "shift":
			mods |= key.ModShift
		case "ctrl", "control":
			mods |= key.ModCtrl
		case "alt", "option":
			mods |= key.ModAlt
		default:
			// This is the key itself
			keyName = parseKeyName(part)
		}
	}

	if keyName == "" {
		return "", 0, fmt.Errorf("no key found in %q", s)
	}

	return keyName, mods, nil
}

func parseKeyName(s string) key.Name {
	switch strings.ToLower(s) {
	case "left":
		return key.NameLeftArrow
	case "right":
		return key.NameRightArrow
	case "up":
		return key.NameUpArrow
	case "down":
		return key.NameDownArrow
	case "enter", "return":
		return key.NameReturn
	case "escape", "esc":
		return key.NameEscape
	case "backspace":
		return key.NameDeleteBackward
	case "delete":
		return key.NameDeleteForward
	case "tab":
		return key.NameTab
	case "space":
		return key.NameSpace
	case "home":
		return key.NameHome
	case "end":
		return key.NameEnd
	case "pageup":
		return key.NamePageUp
	case "pagedown":
		return key.NamePageDown
	default:
		// Single letter keys are uppercase
		if len(s) == 1 {
			return key.Name(strings.ToUpper(s))
		}
		return key.Name(s)
	}
}

// FormatKeybinding returns a display string for a keybinding.
func FormatKeybinding(name key.Name, mods key.Modifiers) string {
	var parts []string
	if mods&key.ModShortcut != 0 {
		parts = append(parts, "Cmd")
	}
	if mods&key.ModShift != 0 {
		parts = append(parts, "Shift")
	}
	if mods&key.ModCtrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&key.ModAlt != 0 {
		parts = append(parts, "Alt")
	}
	parts = append(parts, string(name))
	return strings.Join(parts, "+")
}
