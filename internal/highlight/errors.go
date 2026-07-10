package highlight

import (
	"sort"

	sitter "github.com/smacker/go-tree-sitter"
)

// ErrorLines walks the current parse tree and returns the sorted, de-duplicated
// 0-based start lines of ERROR and MISSING nodes (tree-sitter's markers for
// syntax errors). The result is capped at max lines to bound gutter work.
//
// The walk descends only into subtrees that actually contain an error
// (Node.HasError), so a well-formed file returns nil after inspecting the root.
// Returns nil for simple (non-tree-sitter) languages or before the first parse.
func (h *Highlighter) ErrorLines(max int) []int {
	if h.simple || h.tree == nil || max <= 0 {
		return nil
	}
	root := h.tree.RootNode()
	if root == nil || (!root.HasError() && !root.IsMissing()) {
		return nil
	}

	seen := make(map[int]bool)
	var lines []int
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if len(lines) >= max {
			return
		}
		if n.IsError() || n.IsMissing() {
			row := int(n.StartPoint().Row)
			if !seen[row] {
				seen[row] = true
				lines = append(lines, row)
			}
		}
		count := int(n.ChildCount())
		for i := 0; i < count; i++ {
			if len(lines) >= max {
				return
			}
			c := n.Child(i)
			if c == nil {
				continue
			}
			// Prune healthy subtrees: only recurse where an error can live.
			if c.HasError() || c.IsError() || c.IsMissing() {
				walk(c)
			}
		}
	}
	walk(root)

	sort.Ints(lines)
	return lines
}
