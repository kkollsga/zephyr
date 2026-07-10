package main

import (
	"testing"

	"gioui.org/io/pointer"
)

func TestVisualLineAtYIncludesPixelOffset(t *testing.T) {
	tests := []struct {
		name                               string
		first, offset, y, lineHeight, want int
	}{
		{name: "top", first: 10, y: 0, lineHeight: 20, want: 10},
		{name: "next line", first: 10, y: 20, lineHeight: 20, want: 11},
		{name: "partial scroll crosses boundary", first: 10, offset: 19, y: 1, lineHeight: 20, want: 11},
		{name: "partial scroll below boundary", first: 10, offset: 9, y: 10, lineHeight: 20, want: 10},
		{name: "negative y floors", first: 10, y: -1, lineHeight: 20, want: 9},
		{name: "negative y with offset", first: 10, offset: 5, y: -5, lineHeight: 20, want: 10},
		{name: "invalid height", first: 10, offset: 5, y: 100, lineHeight: 0, want: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := visualLineAtY(tt.first, tt.offset, tt.y, tt.lineHeight); got != tt.want {
				t.Fatalf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsPrimaryPointerPress(t *testing.T) {
	tests := []struct {
		name string
		ev   pointer.Event
		want bool
	}{
		{name: "primary mouse", ev: pointer.Event{Source: pointer.Mouse, Buttons: pointer.ButtonPrimary}, want: true},
		{name: "secondary mouse", ev: pointer.Event{Source: pointer.Mouse, Buttons: pointer.ButtonSecondary}, want: false},
		{name: "middle mouse", ev: pointer.Event{Source: pointer.Mouse, Buttons: pointer.ButtonTertiary}, want: false},
		{name: "touch", ev: pointer.Event{Source: pointer.Touch}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPrimaryPointerPress(tt.ev); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
