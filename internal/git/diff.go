package git

import (
	"regexp"
	"strconv"
	"strings"
)

// DiffLineType classifies a line in a unified diff.
type DiffLineType int

const (
	DiffLineContext DiffLineType = iota
	DiffLineAdd
	DiffLineDelete
)

// DiffLine is a single line in a diff hunk.
type DiffLine struct {
	Type    DiffLineType
	Content string // line content without the +/-/space prefix
}

// Hunk represents a contiguous region of changes in a file.
type Hunk struct {
	OldStart int    // 1-based start line in original
	OldCount int    // number of lines in original
	NewStart int    // 1-based start line in modified
	NewCount int    // number of lines in modified
	Header   string // the full @@ line
	Lines    []DiffLine
}

// FileDiff represents the diff for a single file.
type FileDiff struct {
	Path   string // file path relative to repo root
	Status rune   // 'M', 'A', 'D', 'R'
	Hunks  []Hunk
	Binary bool

	lineStatusCache map[int]rune // lazily built cache: 1-based line -> status
}

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// Diff returns the diff between the working tree and the given ref for all files.
func (r *Repo) Diff(ref string) ([]FileDiff, error) {
	out, err := Run(r.Root, "diff", ref)
	if err != nil {
		return nil, err
	}
	return ParseUnifiedDiff(out), nil
}

// DiffCached returns the diff between the index (staged) and the given ref.
func (r *Repo) DiffCached(ref string) ([]FileDiff, error) {
	out, err := Run(r.Root, "diff", "--cached", ref)
	if err != nil {
		return nil, err
	}
	return ParseUnifiedDiff(out), nil
}

// DiffFile returns the diff for a single file.
func (r *Repo) DiffFile(ref, path string) (*FileDiff, error) {
	out, err := Run(r.Root, "diff", ref, "--", path)
	if err != nil {
		return nil, err
	}
	diffs := ParseUnifiedDiff(out)
	if len(diffs) == 0 {
		return nil, nil
	}
	return &diffs[0], nil
}

// ParseUnifiedDiff parses unified diff output into structured FileDiffs.
func ParseUnifiedDiff(data []byte) []FileDiff {
	if len(data) == 0 {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	var diffs []FileDiff
	var current *FileDiff
	var currentHunk *Hunk

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// New file diff header
		if strings.HasPrefix(line, "diff --git ") {
			if current != nil {
				if currentHunk != nil {
					current.Hunks = append(current.Hunks, *currentHunk)
					currentHunk = nil
				}
				diffs = append(diffs, *current)
			}
			current = &FileDiff{}
			currentHunk = nil
			// Extract path from "diff --git a/path b/path"
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				current.Path = parts[1]
			}
			continue
		}

		if current == nil {
			continue
		}

		// Detect file status from diff header lines
		if strings.HasPrefix(line, "new file mode") {
			current.Status = 'A'
			continue
		}
		if strings.HasPrefix(line, "deleted file mode") {
			current.Status = 'D'
			continue
		}
		if strings.HasPrefix(line, "rename from ") {
			current.Status = 'R'
			continue
		}
		if strings.HasPrefix(line, "rename to ") {
			continue
		}
		if strings.HasPrefix(line, "similarity index") {
			continue
		}
		if strings.HasPrefix(line, "index ") {
			if current.Status == 0 {
				current.Status = 'M'
			}
			continue
		}
		if strings.HasPrefix(line, "Binary files") {
			current.Binary = true
			continue
		}
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}

		// Hunk header
		if matches := hunkHeaderRe.FindStringSubmatch(line); matches != nil {
			if currentHunk != nil {
				current.Hunks = append(current.Hunks, *currentHunk)
			}
			oldStart, _ := strconv.Atoi(matches[1])
			oldCount := 1
			if matches[2] != "" {
				oldCount, _ = strconv.Atoi(matches[2])
			}
			newStart, _ := strconv.Atoi(matches[3])
			newCount := 1
			if matches[4] != "" {
				newCount, _ = strconv.Atoi(matches[4])
			}
			currentHunk = &Hunk{
				OldStart: oldStart,
				OldCount: oldCount,
				NewStart: newStart,
				NewCount: newCount,
				Header:   line,
			}
			continue
		}

		// Diff content lines
		if currentHunk != nil {
			if strings.HasPrefix(line, "+") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    DiffLineAdd,
					Content: line[1:],
				})
			} else if strings.HasPrefix(line, "-") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    DiffLineDelete,
					Content: line[1:],
				})
			} else if strings.HasPrefix(line, " ") {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{
					Type:    DiffLineContext,
					Content: line[1:],
				})
			} else if strings.HasPrefix(line, `\`) {
				// "\ No newline at end of file" — skip
				continue
			}
		}
	}

	// Flush last file/hunk
	if current != nil {
		if currentHunk != nil {
			current.Hunks = append(current.Hunks, *currentHunk)
		}
		diffs = append(diffs, *current)
	}

	return diffs
}

// buildLineStatusCache pre-computes line status for all changed lines.
func (fd *FileDiff) buildLineStatusCache() {
	fd.lineStatusCache = make(map[int]rune)
	for _, h := range fd.Hunks {
		newLine := h.NewStart
		pendingDeletes := 0
		for _, dl := range h.Lines {
			switch dl.Type {
			case DiffLineDelete:
				pendingDeletes++
			case DiffLineAdd:
				if pendingDeletes > 0 {
					fd.lineStatusCache[newLine] = '~'
					pendingDeletes--
				} else {
					fd.lineStatusCache[newLine] = '+'
				}
				newLine++
			case DiffLineContext:
				pendingDeletes = 0
				newLine++
			}
		}
	}
}

// LineStatus returns the diff status for a 1-based new-file line number.
// Returns '+' for added, '~' for modified, or ' ' for unchanged.
// Uses a lazily-built cache for O(1) lookups after first call.
func (fd *FileDiff) LineStatus(line int) rune {
	if fd == nil {
		return ' '
	}
	if fd.lineStatusCache == nil {
		fd.buildLineStatusCache()
	}
	if s, ok := fd.lineStatusCache[line]; ok {
		return s
	}
	return ' '
}

// ChangedNewLines returns all 1-based new-file line numbers that are added or modified.
func (fd *FileDiff) ChangedNewLines() []int {
	if fd == nil {
		return nil
	}
	// Pre-allocate estimate
	total := 0
	for _, h := range fd.Hunks {
		total += h.NewCount
	}
	lines := make([]int, 0, total)
	for _, h := range fd.Hunks {
		newLine := h.NewStart
		for _, dl := range h.Lines {
			switch dl.Type {
			case DiffLineAdd:
				lines = append(lines, newLine)
				newLine++
			case DiffLineContext:
				newLine++
			case DiffLineDelete:
				// deleted lines don't have a new-file line number
			}
		}
	}
	return lines
}

// HunkAt returns the hunk containing the given 1-based new-file line, or nil.
func (fd *FileDiff) HunkAt(line int) *Hunk {
	if fd == nil {
		return nil
	}
	for i := range fd.Hunks {
		h := &fd.Hunks[i]
		if line >= h.NewStart && line < h.NewStart+h.NewCount {
			return h
		}
	}
	return nil
}

// HunkStartLines returns the first new-file line number of each hunk.
func (fd *FileDiff) HunkStartLines() []int {
	if fd == nil {
		return nil
	}
	starts := make([]int, 0, len(fd.Hunks))
	for _, h := range fd.Hunks {
		starts = append(starts, h.NewStart)
	}
	return starts
}

// Stats returns the total added and deleted line counts.
func (fd *FileDiff) Stats() (added, deleted int) {
	for _, h := range fd.Hunks {
		for _, dl := range h.Lines {
			switch dl.Type {
			case DiffLineAdd:
				added++
			case DiffLineDelete:
				deleted++
			}
		}
	}
	return
}
