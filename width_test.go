package miniprompt

import (
	"strings"
	"testing"
)

// TestWideRangesSorted guards the invariant isWide relies on: a binary search
// over wideRanges is only correct while the ranges are sorted and do not
// overlap. A range added in the wrong place would silently misclassify runes.
func TestWideRangesSorted(t *testing.T) {
	if len(wideRanges) == 0 {
		t.Fatal("wideRanges is empty")
	}

	for i, rg := range wideRanges {
		if rg.lo > rg.hi {
			t.Fatalf("range %d is inverted: [%#x, %#x]", i, rg.lo, rg.hi)
		}
	}

	for i := 1; i < len(wideRanges); i++ {
		prev, cur := wideRanges[i-1], wideRanges[i]
		if prev.hi >= cur.lo {
			t.Fatalf(
				"ranges %d and %d overlap or are out of order: [%#x, %#x] then [%#x, %#x]",
				i-1, i, prev.lo, prev.hi, cur.lo, cur.hi,
			)
		}
	}
}

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		name string
		r    rune
		want int
	}{
		{"ascii", 'a', 1},
		{"cjk", '日', 2},
		{"emoji", '😀', 2},
		{"ambiguous counts as one", '±', 1},
		{"non-spacing mark", '\u0301', 0},
		{"spacing mark advances", 0x1d165, 1}, // Mc: documented divergence from go-runewidth
		{"zero width joiner", '\u200d', 0},
		{"hangul jamo medial", 0x1161, 0},
		{"control", '\n', 0},
		{"del", 0x7f, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := runeWidth(tt.r); got != tt.want {
				t.Fatalf("runeWidth(%U) = %d; want %d", tt.r, got, tt.want)
			}
		})
	}
}

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"empty", "", 0},
		{"ascii", "abc", 3},
		{"cjk", "日本", 4},
		{"combining mark adds no cell", "e\u0301", 1},
		{"flag pair", "🇻🇳", 2},
		{"zwj sequence is per rune", "👨‍👩‍👧", 6}, // terminal draws 2; documented limit
		{"tab is zero", "\t", 0},                 // documented limit
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayWidth(tt.s); got != tt.want {
				t.Fatalf("displayWidth(%q) = %d; want %d", tt.s, got, tt.want)
			}
		})
	}
}

func TestCut(t *testing.T) {
	tests := []struct {
		name string
		s    string
		cols int
		want string
	}{
		{"zero cols", "abc", 0, ""},
		{"negative cols", "abc", -1, ""},
		{"exact fit", "abc", 3, "abc"},
		{"wider than cols", "abc", 10, "abc"},
		{"cuts ascii", "abcdef", 3, "abc"},
		{"does not split a wide rune", "日本", 3, "日"},
		{"keeps a wide rune that fits", "日本", 4, "日本"},
		{"keeps the mark after a fitting base", "abe\u0301", 3, "abe\u0301"},
		{"drops the base when its mark would not fit", "abe\u0301", 2, "ab"},
		{"mixed ascii and wide", "a日b本", 3, "a日"},
		{"empty string", "", 5, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cut(tt.s, tt.cols); got != tt.want {
				t.Fatalf("cut(%q, %d) = %q; want %q", tt.s, tt.cols, got, tt.want)
			}
		})
	}
}

// TestCutNeverExceedsCols pins the two properties every caller depends on:
// the result is a prefix of the input, and it never overflows the column.
func TestCutNeverExceedsCols(t *testing.T) {
	inputs := []string{"abc", "日本語です", "a日b本c", "e\u0301x", "🇻🇳🇻🇳", ""}

	for _, s := range inputs {
		for cols := range displayWidth(s) + 2 {
			got := cut(s, cols)

			if !strings.HasPrefix(s, got) {
				t.Fatalf("cut(%q, %d) = %q is not a prefix", s, cols, got)
			}

			if w := displayWidth(got); w > cols {
				t.Fatalf("cut(%q, %d) = %q occupies %d cells", s, cols, got, w)
			}
		}
	}
}
