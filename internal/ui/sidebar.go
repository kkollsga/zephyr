package ui

// Sidebar manages the file tree sidebar state.
type Sidebar struct {
	Visible  bool
	Tree     *FileTree
	Width    int
	ScrollY  int
	Selected int // index in flattened visible list
}

// NewSidebar creates a hidden sidebar.
func NewSidebar() *Sidebar {
	return &Sidebar{
		Width:    220,
		Selected: -1,
	}
}

// Toggle shows or hides the sidebar.
func (s *Sidebar) Toggle() {
	s.Visible = !s.Visible
}

// LoadDirectory loads a directory tree into the sidebar.
func (s *Sidebar) LoadDirectory(path string) error {
	tree, err := NewFileTree(path)
	if err != nil {
		return err
	}
	s.Tree = tree
	return nil
}
