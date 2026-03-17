package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kristianweb/zephyr/internal/highlight"
	"github.com/kristianweb/zephyr/internal/ui"
)

// saveAllBeforeQuit prompts the user for each unsaved tab.
// Returns true if the quit should proceed, false if the user cancelled.
func (st *appState) saveAllBeforeQuit() bool {
	for _, tab := range st.tabBar.Tabs {
		if !tab.Editor.Modified {
			continue
		}
		result := st.promptSaveDialog(tab.Title)
		switch result {
		case saveResultCancel:
			return false
		case saveResultDiscard:
			continue
		case saveResultSave:
			if !st.saveTab(tab) {
				return false
			}
		}
	}
	return true
}

type saveResult int

const (
	saveResultCancel  saveResult = iota
	saveResultDiscard
	saveResultSave
)

// promptSaveDialog shows a native "Do you want to save?" dialog.
// Buttons: Cancel (Escape) / Discard / Save
func (st *appState) promptSaveDialog(title string) saveResult {
	script := fmt.Sprintf(
		`display dialog "Do you want to save changes to \"%s\"?" buttons {"Discard", "Cancel", "Save"} default button "Save" cancel button "Cancel" with icon caution`,
		title,
	)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return saveResultCancel
	}
	result := strings.TrimSpace(string(out))
	switch {
	case strings.Contains(result, "Save"):
		return saveResultSave
	case strings.Contains(result, "Discard"):
		return saveResultDiscard
	default:
		return saveResultCancel
	}
}

// saveTab saves a tab to its existing path. Returns false if the file is
// untitled (caller should use saveTabAs instead).
func (st *appState) saveTab(tab *ui.Tab) bool {
	if tab.Editor.FilePath == "" {
		return st.saveTabAs(tab)
	}
	if err := tab.Editor.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "save error: %v\n", err)
		return false
	}
	return true
}

// saveTabAs shows a native Save As file dialog. Returns false if the user
// cancelled.
func (st *appState) saveTabAs(tab *ui.Tab) bool {
	defaultName := tab.Title
	if defaultName == "" || defaultName == "Untitled" {
		ts := st.tabStates[tab.Editor]
		if ts != nil && ts.langLabel != "" && ts.langLabel != "Plain Text" {
			defaultName = "Untitled" + langToExtension(ts.langLabel)
		} else {
			defaultName = "Untitled.txt"
		}
	}

	script := fmt.Sprintf(`set filePath to POSIX path of (choose file name with prompt "Save As" default name %q)
return filePath`, defaultName)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return false
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return false
	}
	if err := tab.Editor.SaveAs(path); err != nil {
		fmt.Fprintf(os.Stderr, "save error: %v\n", err)
		return false
	}
	tab.Title = filepath.Base(path)

	// Update highlighter for new extension
	ts := st.tabStates[tab.Editor]
	if ts != nil {
		ts.langLabel = detectLanguage(path)
		h := highlight.NewHighlighter(path)
		if h != nil {
			if ts.highlighter != nil {
				ts.highlighter.Close()
			}
			ts.highlighter = h
			h.Parse([]byte(tab.Editor.Buffer.Text()))
		}
	}
	return true
}

func (st *appState) hasUnsavedChanges() bool {
	for _, tab := range st.tabBar.Tabs {
		if tab.Editor.Modified {
			return true
		}
	}
	return false
}
