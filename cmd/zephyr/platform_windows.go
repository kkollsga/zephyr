//go:build windows

package main

import (
	"image/color"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/kristianweb/zephyr/internal/ui"
)

var (
	comdlg32         = syscall.NewLazyDLL("comdlg32.dll")
	shell32          = syscall.NewLazyDLL("shell32.dll")
	ole32            = syscall.NewLazyDLL("ole32.dll")
	getSaveFileNameW = comdlg32.NewProc("GetSaveFileNameW")
	shBrowseForFolder = shell32.NewProc("SHBrowseForFolderW")
	shGetPathFromIDList = shell32.NewProc("SHGetPathFromIDListW")
	coInitialize     = ole32.NewProc("CoInitialize")
	coUninitialize   = ole32.NewProc("CoUninitialize")
)

// platformDecorated returns true — Windows uses native window chrome.
func platformDecorated() bool { return true }

// platformThemeToggleLeft returns true — on Windows the toggle is on the left.
func platformThemeToggleLeft() bool { return true }

// platformHasFinderTags returns false — Finder tags are macOS-only.
func platformHasFinderTags() bool { return false }

// warningColor returns the orange color used for overwrite warnings.
func warningColor() color.NRGBA {
	return color.NRGBA{R: 0xFF, G: 0x9F, B: 0x0A, A: 0xFF}
}

// OPENFILENAMEW is the Win32 OPENFILENAME struct for file dialogs.
type openFileNameW struct {
	structSize      uint32
	owner           uintptr
	instance        uintptr
	filter          *uint16
	customFilter    *uint16
	maxCustomFilter uint32
	filterIndex     uint32
	file            *uint16
	maxFile         uint32
	fileTitle       *uint16
	maxFileTitle    uint32
	initialDir      *uint16
	title           *uint16
	flags           uint32
	fileOffset      uint16
	fileExtension   uint16
	defExt          *uint16
	custData        uintptr
	hook            uintptr
	templateName    *uint16
	pvReserved      uintptr
	dwReserved      uint32
	flagsEx         uint32
}

const (
	ofnOverwritePrompt = 0x00000002
	ofnNoChangeDir     = 0x00000008
	ofnPathmustExist   = 0x00000800
	ofnExplorer        = 0x00080000
)

// BROWSEINFOW is the Win32 BROWSEINFO struct for folder picker.
type browseInfoW struct {
	owner        uintptr
	root         uintptr
	displayName  *uint16
	title        *uint16
	flags        uint32
	callback     uintptr
	param        uintptr
	image        int32
}

const (
	bifReturnonlyfsdirs = 0x00000001
	bifNewdialogstyle   = 0x00000040
)

// pickSaveDir opens the Windows folder picker and updates the save dir.
func (st *appState) pickSaveDir() {
	go func() {
		coInitialize.Call(0)
		defer coUninitialize.Call()

		displayName := make([]uint16, syscall.MAX_PATH)
		title, _ := syscall.UTF16PtrFromString("Save in")

		bi := browseInfoW{
			displayName: &displayName[0],
			title:       title,
			flags:       bifReturnonlyfsdirs | bifNewdialogstyle,
		}

		pidl, _, _ := shBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
		if pidl == 0 {
			return
		}

		path := make([]uint16, syscall.MAX_PATH)
		shGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&path[0])))

		dir := syscall.UTF16ToString(path)
		if dir != "" {
			st.saveMenu.dir = dir
			if st.window != nil {
				st.window.Invalidate()
			}
		}
	}()
}

// saveTabAs shows the Windows Save As file dialog.
func (st *appState) saveTabAs(tab *ui.Tab) bool {
	defaultName := tab.Title
	if defaultName == "" || tab.IsUntitled {
		ts := st.tabStates[tab.Editor]
		if ts != nil && ts.langLabel != "" && ts.langLabel != "Plain Text" {
			defaultName = "Untitled" + langToExtension(ts.langLabel)
		} else {
			defaultName = "Untitled.txt"
		}
	}

	coInitialize.Call(0)
	defer coUninitialize.Call()

	fileBuffer := make([]uint16, syscall.MAX_PATH)
	// Copy default name into the buffer
	defaultNameUTF16, _ := syscall.UTF16FromString(defaultName)
	copy(fileBuffer, defaultNameUTF16)

	filter, _ := syscall.UTF16PtrFromString("All Files (*.*)\x00*.*\x00\x00")
	title, _ := syscall.UTF16PtrFromString("Save As")

	var initialDir *uint16
	if tab.Editor.FilePath != "" {
		dir := filepath.Dir(tab.Editor.FilePath)
		initialDir, _ = syscall.UTF16PtrFromString(dir)
	}

	ofn := openFileNameW{
		structSize: uint32(unsafe.Sizeof(openFileNameW{})),
		file:       &fileBuffer[0],
		maxFile:    uint32(len(fileBuffer)),
		filter:     filter,
		title:      title,
		initialDir: initialDir,
		flags:      ofnOverwritePrompt | ofnNoChangeDir | ofnPathmustExist | ofnExplorer,
	}

	r, _, _ := getSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return false
	}

	path := syscall.UTF16ToString(fileBuffer)
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	return st.saveTabToPath(tab, path)
}

// applyFinderTags is a no-op on Windows (no Finder tags concept).
func (st *appState) applyFinderTags(path string) {}
