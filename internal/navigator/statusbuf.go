package navigator

import (
	"fmt"
	"strings"

	"github.com/kristianweb/zephyr/internal/git"
)

// SectionKind identifies a section in the status buffer.
type SectionKind int

const (
	SectionUnstaged  SectionKind = iota
	SectionStaged
	SectionUntracked
	SectionRecent
)

// StatusEntry represents a file entry in the status buffer.
type StatusEntry struct {
	Path     string
	Status   rune // 'M', 'A', 'D', '?'
	Added    int
	Deleted  int
	Expanded bool   // inline diff visible
	DiffText string // cached inline diff text
}

// StatusSection is a collapsible section in the status buffer.
type StatusSection struct {
	Kind      SectionKind
	Title     string
	Entries   []StatusEntry
	Collapsed bool
}

// StatusBuffer holds the data for the git status buffer.
type StatusBuffer struct {
	Branch   string
	Hash     string
	Upstream string
	Ahead    int
	Behind   int
	Sections []StatusSection

	// Line mapping for navigation
	lineMap []lineRef // maps buffer line -> section/entry
}

type lineRef struct {
	section int // -1 = header
	entry   int // -1 = section header, >=0 = entry index
	isDiff  bool
}

// NewStatusBuffer creates a status buffer from the repo state.
func NewStatusBuffer(repo *git.Repo, cache *git.Cache) (*StatusBuffer, error) {
	branch, hash, err := repo.Head()
	if err != nil {
		return nil, err
	}

	upstream, ahead, behind, _ := repo.Upstream()

	statuses, err := cache.Status()
	if err != nil {
		return nil, err
	}

	diffStat, _ := cache.DiffStat()

	sb := &StatusBuffer{
		Branch:   branch,
		Hash:     hash,
		Upstream: upstream,
		Ahead:    ahead,
		Behind:   behind,
	}

	// Build sections from statuses
	var unstaged, staged, untracked []StatusEntry

	for _, s := range statuses {
		stats := diffStat[s.Path]

		if s.Index == '?' && s.Worktree == '?' {
			untracked = append(untracked, StatusEntry{
				Path:   s.Path,
				Status: '?',
			})
			continue
		}

		// Staged changes (index status)
		if s.Index != ' ' && s.Index != '?' {
			staged = append(staged, StatusEntry{
				Path:    s.Path,
				Status:  s.Index,
				Added:   stats[0],
				Deleted: stats[1],
			})
		}

		// Unstaged changes (worktree status)
		if s.Worktree != ' ' && s.Worktree != '?' {
			unstaged = append(unstaged, StatusEntry{
				Path:    s.Path,
				Status:  s.Worktree,
				Added:   stats[0],
				Deleted: stats[1],
			})
		}
	}

	if len(unstaged) > 0 {
		totalAdd, totalDel := sumStats(unstaged)
		sb.Sections = append(sb.Sections, StatusSection{
			Kind:    SectionUnstaged,
			Title:   fmt.Sprintf("Unstaged changes (%d)%s", len(unstaged), formatTotalStats(totalAdd, totalDel)),
			Entries: unstaged,
		})
	}
	if len(staged) > 0 {
		totalAdd, totalDel := sumStats(staged)
		sb.Sections = append(sb.Sections, StatusSection{
			Kind:    SectionStaged,
			Title:   fmt.Sprintf("Staged changes (%d)%s", len(staged), formatTotalStats(totalAdd, totalDel)),
			Entries: staged,
		})
	}
	if len(untracked) > 0 {
		sb.Sections = append(sb.Sections, StatusSection{
			Kind:    SectionUntracked,
			Title:   fmt.Sprintf("Untracked files (%d)", len(untracked)),
			Entries: untracked,
		})
	}

	sb.buildLineMap()
	return sb, nil
}

func sumStats(entries []StatusEntry) (int, int) {
	var a, d int
	for _, e := range entries {
		a += e.Added
		d += e.Deleted
	}
	return a, d
}

func formatTotalStats(added, deleted int) string {
	if added == 0 && deleted == 0 {
		return ""
	}
	parts := []string{}
	if added > 0 {
		parts = append(parts, fmt.Sprintf("+%d", added))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("-%d", deleted))
	}
	return "  " + strings.Join(parts, " ")
}

// GenerateText produces the buffer content for the status view.
func (sb *StatusBuffer) GenerateText() string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("Head:     %s (%s)\n", sb.Branch, sb.Hash))
	if sb.Upstream != "" {
		upInfo := sb.Upstream
		if sb.Ahead > 0 || sb.Behind > 0 {
			upInfo += fmt.Sprintf(" (ahead %d, behind %d)", sb.Ahead, sb.Behind)
		}
		b.WriteString(fmt.Sprintf("Upstream: %s\n", upInfo))
	}
	b.WriteString("\n")

	for _, sec := range sb.Sections {
		b.WriteString(sec.Title)
		b.WriteString("\n")

		if !sec.Collapsed {
			for _, e := range sec.Entries {
				stats := ""
				if e.Added > 0 || e.Deleted > 0 {
					stats = fmt.Sprintf("+%d -%d", e.Added, e.Deleted)
				}
				line := fmt.Sprintf("  %c  %s", e.Status, e.Path)
				if stats != "" {
					padding := 50 - len(line) - len(stats)
					if padding < 2 {
						padding = 2
					}
					line += strings.Repeat(" ", padding) + stats
				}
				b.WriteString(line)
				b.WriteString("\n")

				// Expanded inline diff
				if e.Expanded && e.DiffText != "" {
					for _, dline := range strings.Split(e.DiffText, "\n") {
						b.WriteString("     ")
						b.WriteString(dline)
						b.WriteString("\n")
					}
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

func (sb *StatusBuffer) buildLineMap() {
	sb.lineMap = nil
	line := 0

	// Header lines
	line++ // Head:
	if sb.Upstream != "" {
		line++ // Upstream:
	}
	line++ // blank
	for line > 0 {
		sb.lineMap = append(sb.lineMap, lineRef{section: -1, entry: -1})
		line--
	}

	// Recount using text
	text := sb.GenerateText()
	lines := strings.Split(text, "\n")
	sb.lineMap = make([]lineRef, len(lines))

	currentSection := -1
	currentEntry := -1
	inDiff := false

	for i, l := range lines {
		// Check if this line is a section header
		isSectionHeader := false
		for si, sec := range sb.Sections {
			if l == sec.Title {
				currentSection = si
				currentEntry = -1
				inDiff = false
				isSectionHeader = true
				break
			}
		}

		if isSectionHeader {
			sb.lineMap[i] = lineRef{section: currentSection, entry: -1}
			continue
		}

		// Check if this is an entry line (starts with "  X  ")
		if currentSection >= 0 && len(l) > 4 && l[0] == ' ' && l[1] == ' ' && l[3] == ' ' && l[4] == ' ' {
			currentEntry++
			inDiff = false
			if currentEntry < len(sb.Sections[currentSection].Entries) {
				sb.lineMap[i] = lineRef{section: currentSection, entry: currentEntry}
			} else {
				sb.lineMap[i] = lineRef{section: currentSection, entry: -1}
			}
			continue
		}

		// Check if this is expanded diff text
		if inDiff || (len(l) > 5 && l[:5] == "     ") {
			inDiff = true
			sb.lineMap[i] = lineRef{section: currentSection, entry: currentEntry, isDiff: true}
			continue
		}

		// Blank line or header
		if l == "" && currentSection >= 0 {
			currentEntry = -1
			inDiff = false
		}
		sb.lineMap[i] = lineRef{section: currentSection, entry: -1}
	}
}

// EntryAtLine returns the StatusEntry at the given buffer line (0-based), or nil.
func (sb *StatusBuffer) EntryAtLine(line int) (*StatusEntry, *StatusSection) {
	if line < 0 || line >= len(sb.lineMap) {
		return nil, nil
	}
	ref := sb.lineMap[line]
	if ref.section < 0 || ref.section >= len(sb.Sections) {
		return nil, nil
	}
	sec := &sb.Sections[ref.section]
	if ref.entry < 0 || ref.entry >= len(sec.Entries) {
		return nil, sec
	}
	return &sec.Entries[ref.entry], sec
}

// SectionAtLine returns the section at the given line, or nil.
func (sb *StatusBuffer) SectionAtLine(line int) *StatusSection {
	if line < 0 || line >= len(sb.lineMap) {
		return nil
	}
	ref := sb.lineMap[line]
	if ref.section < 0 || ref.section >= len(sb.Sections) {
		return nil
	}
	return &sb.Sections[ref.section]
}

// NextSection returns the line number of the next section header after currentLine.
func (sb *StatusBuffer) NextSection(currentLine int) int {
	for i := currentLine + 1; i < len(sb.lineMap); i++ {
		ref := sb.lineMap[i]
		if ref.section >= 0 && ref.entry == -1 && !ref.isDiff {
			// Check if this is actually a section header (not a blank line)
			if i < len(sb.lineMap) {
				for _, sec := range sb.Sections {
					text := sb.GenerateText()
					lines := strings.Split(text, "\n")
					if i < len(lines) && lines[i] == sec.Title {
						return i
					}
				}
			}
		}
	}
	return currentLine
}

// PrevSection returns the line number of the previous section header before currentLine.
func (sb *StatusBuffer) PrevSection(currentLine int) int {
	text := sb.GenerateText()
	lines := strings.Split(text, "\n")

	for i := currentLine - 1; i >= 0; i-- {
		for _, sec := range sb.Sections {
			if i < len(lines) && lines[i] == sec.Title {
				return i
			}
		}
	}
	return currentLine
}

// ToggleCollapse toggles the collapse state of the section at the given line.
func (sb *StatusBuffer) ToggleCollapse(line int) {
	sec := sb.SectionAtLine(line)
	if sec != nil {
		sec.Collapsed = !sec.Collapsed
		sb.buildLineMap()
	}
}

// Refresh reloads the status buffer data.
func (sb *StatusBuffer) Refresh(repo *git.Repo, cache *git.Cache) error {
	cache.Invalidate()
	newBuf, err := NewStatusBuffer(repo, cache)
	if err != nil {
		return err
	}
	*sb = *newBuf
	return nil
}
