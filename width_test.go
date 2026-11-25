package miniprompt

import "testing"

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "abc", 3},
		{"space", "a b", 3},
		{"vietnamese", "Việt", 4},
		{"combining", "e\u0302", 1},
		{"cjk", "日本", 4},
		{"kana", "カタ", 4},
		{"hangul", "한글", 4},
		{"fullwidth", "Ａ", 2},
		{"emoji", "🙂", 2},
		{"emoji-mixed", "a🙂b", 4},
		{"control", "a\tb", 2},
		{"zwsp", "a\u200bb", 2},
		{"variation-selector", "❤\ufe0f", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayWidth(tt.in); got != tt.want {
				t.Fatalf("displayWidth(%q) = %d; want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestDisplayWidthIsAdditive(t *testing.T) {
	text := "Việt 🙂 日本 e\u0302"

	sum := 0
	for _, r := range text {
		w := runeWidth(r)
		if w < 0 || w > 2 {
			t.Fatalf("runeWidth(%q) = %d; want 0..2", r, w)
		}
		sum += w
	}

	if got, want := displayWidth(text), sum; got != want {
		t.Fatalf("displayWidth(%q) = %d; want %d", text, got, want)
	}
}
