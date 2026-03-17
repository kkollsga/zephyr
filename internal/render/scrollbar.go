package render

import (
	"image"
	"image/color"
	"time"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// ScrollbarRenderer draws a VS Code-style scrollbar that fades out when idle.
type ScrollbarRenderer struct {
	TrackWidth   int
	ThumbWidth   int
	ThumbColor   color.NRGBA
	MinThumbH    int
	FadeDelay    time.Duration
	FadeDuration time.Duration

	lastScrollAt time.Time
	opacity      float32
}

// NewScrollbarRenderer creates a scrollbar with sensible defaults.
func NewScrollbarRenderer(thumbColor color.NRGBA) *ScrollbarRenderer {
	return &ScrollbarRenderer{
		TrackWidth:   10,
		ThumbWidth:   6,
		ThumbColor:   thumbColor,
		MinThumbH:    20,
		FadeDelay:    800 * time.Millisecond,
		FadeDuration: 400 * time.Millisecond,
	}
}

// NotifyScroll resets the fade timer — call on every scroll event.
func (sr *ScrollbarRenderer) NotifyScroll() {
	sr.lastScrollAt = time.Now()
	sr.opacity = 1.0
}

// Update computes the current opacity based on time since last scroll.
// Returns true if still animating (needs redraw).
func (sr *ScrollbarRenderer) Update() bool {
	if sr.opacity == 0 {
		return false
	}
	elapsed := time.Since(sr.lastScrollAt)
	if elapsed < sr.FadeDelay {
		sr.opacity = 1.0
		return true
	}
	fade := elapsed - sr.FadeDelay
	if fade >= sr.FadeDuration {
		sr.opacity = 0
		return false
	}
	sr.opacity = 1.0 - float32(fade)/float32(sr.FadeDuration)
	return true
}

// Render draws the scrollbar thumb.
func (sr *ScrollbarRenderer) Render(ops *op.Ops, editorW, editorH, firstLine, pixelOffset, visibleLines, totalLines, lineHeight int) {
	if sr.opacity <= 0 || totalLines <= visibleLines || totalLines == 0 || lineHeight == 0 {
		return
	}

	// Thumb height: proportional to visible/total, with minimum
	thumbH := editorH * visibleLines / totalLines
	if thumbH < sr.MinThumbH {
		thumbH = sr.MinThumbH
	}
	if thumbH > editorH {
		thumbH = editorH
	}

	// Scroll ratio: 0.0 at top, 1.0 at bottom
	maxScroll := (totalLines - visibleLines) * lineHeight
	if maxScroll <= 0 {
		return
	}
	scrollPos := firstLine*lineHeight + pixelOffset
	ratio := float32(scrollPos) / float32(maxScroll)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	thumbY := int(ratio * float32(editorH-thumbH))

	// Position: right edge, centered in track
	x := editorW - sr.TrackWidth + (sr.TrackWidth-sr.ThumbWidth)/2

	// Apply opacity to thumb color
	c := sr.ThumbColor
	c.A = uint8(float32(c.A) * sr.opacity)

	rect := clip.Rect{
		Min: image.Pt(x, thumbY),
		Max: image.Pt(x+sr.ThumbWidth, thumbY+thumbH),
	}.Push(ops)
	paint.ColorOp{Color: c}.Add(ops)
	paint.PaintOp{}.Add(ops)
	rect.Pop()
}
