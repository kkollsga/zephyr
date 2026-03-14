package ui

// AppLayout defines the layout regions for the editor UI.
// The layout is: [optional sidebar] | [optional tab strip + editor + status line]
type AppLayout struct {
	Sidebar    *Sidebar
	TabBar     *TabBar
	SidebarX   int // pixel x offset where sidebar ends
	EditorX    int // pixel x offset where editor starts
	TabStripH  int // height of tab strip (0 if single file)
	StatusBarH int // height of status bar
}

// NewAppLayout creates a new layout with default configuration.
func NewAppLayout() *AppLayout {
	return &AppLayout{
		Sidebar:    NewSidebar(),
		TabBar:     NewTabBar(),
		StatusBarH: 24,
	}
}

// EditorRect returns the available editor area given window dimensions.
func (al *AppLayout) EditorRect(windowW, windowH int) (x, y, w, h int) {
	x = 0
	y = 0

	if al.Sidebar.Visible {
		x = al.Sidebar.Width
	}

	if al.TabBar.TabCount() > 1 {
		y = al.TabStripH
	}

	w = windowW - x
	h = windowH - y - al.StatusBarH
	return
}
