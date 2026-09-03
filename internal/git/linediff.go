package git

import (
	"fmt"
	"strings"
)

// lineDiffContext is the number of unchanged lines kept on each side of a
// change, matching git's default -U3.
const lineDiffContext = 3

// lineDiffMaxDistance caps the Myers search at an edit distance of 1000. The
// trace the backtrack needs costs O(D^2) memory, so an uncapped D on two texts
// with nothing in common would allocate without bound. Past the cap the two
// versions share too little for a line-level diff to say anything useful, and
// the differing middle is reported as one wholesale replacement instead.
const lineDiffMaxDistance = 1000

// lineDiffMaxWork bounds the steps the Myers search may take before it gives
// up and reports a wholesale replacement. The distance cap alone does not
// bound the running time: each round walks its diagonals and each diagonal can
// run a long snake, so two texts built from long repeated runs cost O(D^2 * N)
// even at a modest D. This call happens on the UI thread, so it needs a
// ceiling in work, not only in memory. Ten million steps is tens of
// milliseconds, and well clear of what a real file pair costs.
const lineDiffMaxWork = 10_000_000

// LineDiff diffs two texts by line and returns the result in the same shape
// ParseUnifiedDiff produces, so the gutter markers and hunk navigation read a
// diff computed here exactly as they read git's own.
//
// Both texts are split on "\n" the way the editor counts buffer lines: "a\n"
// is two lines, the second empty. A missing trailing newline therefore shows
// up as a change to the last line rather than vanishing.
//
// Cost is Myers' O(ND) greedy edit script over the lines left after the common
// prefix and suffix are trimmed — N their combined length, D the edit distance
// — plus O(D^2) memory for the backtrack trace. D is bounded by
// lineDiffMaxDistance.
func LineDiff(oldText, newText string) *FileDiff {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")
	return &FileDiff{
		Status: 'M',
		Hunks:  hunksFromScript(lineEditScript(oldLines, newLines), lineDiffContext),
	}
}

// lineEditScript returns the edit script turning a into b as a flat run of
// context, delete and add entries in file order.
func lineEditScript(a, b []string) []DiffLine {
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix &&
		a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	midA, midB := a[prefix:len(a)-suffix], b[prefix:len(b)-suffix]

	script := make([]DiffLine, 0, len(a)+len(b))
	for _, line := range a[:prefix] {
		script = append(script, DiffLine{Type: DiffLineContext, Content: line})
	}
	middle, ok := myersScript(midA, midB)
	if !ok {
		// Over the distance cap: replace the middle wholesale.
		middle = make([]DiffLine, 0, len(midA)+len(midB))
		for _, line := range midA {
			middle = append(middle, DiffLine{Type: DiffLineDelete, Content: line})
		}
		for _, line := range midB {
			middle = append(middle, DiffLine{Type: DiffLineAdd, Content: line})
		}
	}
	script = append(script, middle...)
	for _, line := range b[len(b)-suffix:] {
		script = append(script, DiffLine{Type: DiffLineContext, Content: line})
	}
	return script
}

// myersScript runs Myers' greedy algorithm over a and b, returning the edit
// script and whether the shortest edit distance stayed inside the cap.
func myersScript(a, b []string) ([]DiffLine, bool) {
	n, m := len(a), len(b)
	maxD := n + m
	if maxD > lineDiffMaxDistance {
		maxD = lineDiffMaxDistance
	}
	offset := maxD + 1
	v := make([]int, 2*maxD+3)
	trace := make([][]int, 0, maxD+1)
	work := 0

	for d := 0; d <= maxD; d++ {
		// The snapshot is the state before round d. Only diagonals within
		// d+1 of the centre can be read back at this depth, so the rest of v
		// is not worth copying.
		radius := d + 1
		snapshot := make([]int, 2*radius+1)
		copy(snapshot, v[offset-radius:offset+radius+1])
		trace = append(trace, snapshot)

		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[offset+k-1] < v[offset+k+1]) {
				x = v[offset+k+1]
			} else {
				x = v[offset+k-1] + 1
			}
			y := x - k
			snakeStart := x
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			work += 1 + (x - snakeStart)
			if work > lineDiffMaxWork {
				return nil, false
			}
			v[offset+k] = x
			if x >= n && y >= m {
				return backtrackScript(a, b, trace), true
			}
		}
	}
	return nil, false
}

// backtrackScript walks the saved traces back from the end of both inputs,
// turning each step into the edit it stands for.
func backtrackScript(a, b []string, trace [][]int) []DiffLine {
	x, y := len(a), len(b)
	reversed := make([]DiffLine, 0, len(a)+len(b))
	for d := len(trace) - 1; d >= 0; d-- {
		v, radius := trace[d], d+1
		k := x - y
		var prevK int
		if k == -d || (k != d && v[k-1+radius] < v[k+1+radius]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := v[prevK+radius]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			reversed = append(reversed, DiffLine{Type: DiffLineContext, Content: a[x-1]})
			x--
			y--
		}
		if d == 0 {
			break
		}
		if x > prevX {
			reversed = append(reversed, DiffLine{Type: DiffLineDelete, Content: a[x-1]})
		} else if y > prevY {
			reversed = append(reversed, DiffLine{Type: DiffLineAdd, Content: b[y-1]})
		}
		x, y = prevX, prevY
	}

	script := make([]DiffLine, len(reversed))
	for i, dl := range reversed {
		script[len(reversed)-1-i] = dl
	}
	return script
}

// hunksFromScript groups an edit script into hunks carrying ctx unchanged
// lines on each side. Two changes separated by at most 2*ctx unchanged lines
// land in one hunk — their context regions meet — which is the grouping git
// itself produces at the same context width.
func hunksFromScript(script []DiffLine, ctx int) []Hunk {
	var changed []int
	for i, dl := range script {
		if dl.Type != DiffLineContext {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return nil
	}

	// Old and new lines consumed before each script entry, so a hunk's start
	// line is a lookup rather than a rescan.
	oldBefore := make([]int, len(script)+1)
	newBefore := make([]int, len(script)+1)
	for i, dl := range script {
		oldBefore[i+1], newBefore[i+1] = oldBefore[i], newBefore[i]
		if dl.Type != DiffLineAdd {
			oldBefore[i+1]++
		}
		if dl.Type != DiffLineDelete {
			newBefore[i+1]++
		}
	}

	var hunks []Hunk
	for i := 0; i < len(changed); {
		j := i
		for j+1 < len(changed) && changed[j+1]-changed[j]-1 <= 2*ctx {
			j++
		}
		start := max(changed[i]-ctx, 0)
		end := min(changed[j]+ctx, len(script)-1)
		hunks = append(hunks, newHunk(script[start:end+1], oldBefore[start], newBefore[start]))
		i = j + 1
	}
	return hunks
}

// newHunk builds one hunk from its slice of the script and the number of old
// and new lines that precede it.
func newHunk(lines []DiffLine, oldBefore, newBefore int) Hunk {
	h := Hunk{OldStart: oldBefore + 1, NewStart: newBefore + 1, Lines: lines}
	for _, dl := range lines {
		if dl.Type != DiffLineAdd {
			h.OldCount++
		}
		if dl.Type != DiffLineDelete {
			h.NewCount++
		}
	}
	// Both counts are always at least 1: each text splits into at least one
	// line, and a hunk's window reaches ctx entries past its last change, so
	// neither side can come out empty the way a git hunk header can.
	h.Header = fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OldStart, h.OldCount, h.NewStart, h.NewCount)
	return h
}
