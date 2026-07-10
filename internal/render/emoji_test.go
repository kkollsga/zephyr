package render

import "testing"

func TestAppendEmojiFamily(t *testing.T) {
	if emojiFamily == "" {
		// On platforms without an emoji family the input is returned verbatim.
		got := AppendEmojiFamily("Charter, Georgia, serif")
		if got != "Charter, Georgia, serif" {
			t.Fatalf("expected unchanged input, got %q", got)
		}
		return
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "insert before trailing generic",
			in:   "Charter, Georgia, serif",
			want: "Charter, Georgia, " + emojiFamily + ", serif",
		},
		{
			name: "append when no generic keyword",
			in:   "Menlo",
			want: "Menlo, " + emojiFamily,
		},
		{
			name: "monospace generic",
			in:   "Menlo, monospace",
			want: "Menlo, " + emojiFamily + ", monospace",
		},
		{
			name: "already present is unchanged",
			in:   "Charter, " + emojiFamily + ", serif",
			want: "Charter, " + emojiFamily + ", serif",
		},
		{
			name: "already present case-insensitive",
			in:   "Charter, apple color emoji, serif",
			want: "Charter, apple color emoji, serif",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AppendEmojiFamily(tt.in); got != tt.want {
				t.Errorf("AppendEmojiFamily(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
