package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileNode represents a file or directory in the tree.
type FileNode struct {
	Name     string
	Path     string
	IsDir    bool
	Children []*FileNode
	Expanded bool
	Depth    int
}

// FileTree manages the file tree data model.
type FileTree struct {
	Root     *FileNode
	RootPath string
}

// NewFileTree creates a file tree rooted at the given directory.
func NewFileTree(rootPath string) (*FileTree, error) {
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	root := &FileNode{
		Name:     filepath.Base(absPath),
		Path:     absPath,
		IsDir:    true,
		Expanded: true,
		Depth:    0,
	}

	if err := loadChildren(root); err != nil {
		return nil, err
	}

	return &FileTree{Root: root, RootPath: absPath}, nil
}

// loadChildren reads directory entries and populates node.Children.
func loadChildren(node *FileNode) error {
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		return err
	}

	node.Children = nil
	for _, entry := range entries {
		name := entry.Name()
		// Skip hidden files and common noise
		if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" {
			continue
		}

		child := &FileNode{
			Name:  name,
			Path:  filepath.Join(node.Path, name),
			IsDir: entry.IsDir(),
			Depth: node.Depth + 1,
		}
		node.Children = append(node.Children, child)
	}

	// Sort: directories first, then alphabetical
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].IsDir != node.Children[j].IsDir {
			return node.Children[i].IsDir
		}
		return strings.ToLower(node.Children[i].Name) < strings.ToLower(node.Children[j].Name)
	})

	return nil
}

// ToggleExpand expands or collapses a directory node.
func (ft *FileTree) ToggleExpand(node *FileNode) error {
	if !node.IsDir {
		return nil
	}
	if node.Expanded {
		node.Expanded = false
		return nil
	}
	node.Expanded = true
	if len(node.Children) == 0 {
		return loadChildren(node)
	}
	return nil
}

// FlattenVisible returns a flat list of all visible nodes for rendering.
func (ft *FileTree) FlattenVisible() []*FileNode {
	var result []*FileNode
	flattenNode(ft.Root, &result)
	return result
}

func flattenNode(node *FileNode, result *[]*FileNode) {
	*result = append(*result, node)
	if node.IsDir && node.Expanded {
		for _, child := range node.Children {
			flattenNode(child, result)
		}
	}
}
