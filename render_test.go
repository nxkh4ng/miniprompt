package miniprompt

import (
	"errors"
	"testing"
)

func TestGlyphText(t *testing.T) {
	tests := []struct {
		name string
		g    string
		want string
	}{
		{"default", "", "? "},
		{"hidden", " ", ""},
		{"custom", "[?]", "[?] "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := glyphText(tt.g, "?"); got != tt.want {
				t.Fatalf("glyphText(%q) = %q; want %q", tt.g, got, tt.want)
			}
		})
	}
}

func TestHeaderText(t *testing.T) {
	tests := []struct {
		name string
		g    Glyphs
		want string
	}{
		{"default", Glyphs{}, "? Name"},
		{"hidden", Glyphs{Pending: " "}, "Name"},
		{"custom", Glyphs{Pending: "→"}, "→ Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := headerText(tt.g, "Name"); got != tt.want {
				t.Fatalf("headerText() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestDoneText(t *testing.T) {
	tests := []struct {
		name string
		g    Glyphs
		want string
	}{
		{"default", Glyphs{}, "✓ Name Alice"},
		{"hidden", Glyphs{Done: " "}, "Name Alice"},
		{"custom", Glyphs{Done: "[✓]"}, "[✓] Name Alice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doneText(tt.g, "Name", "Alice"); got != tt.want {
				t.Fatalf("doneText() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestFailText(t *testing.T) {
	tests := []struct {
		name string
		g    Glyphs
		msg  string
		cols int
		want string
	}{
		{"default", Glyphs{}, "too short", 40, "✗ too short"},
		{"hidden", Glyphs{Fail: " "}, "too short", 40, "too short"},
		{"custom", Glyphs{Fail: "[x]"}, "no", 40, "[x] no"},
		{"newline flattened", Glyphs{}, "a\nb\r\nc", 40, "✗ a b  c"},
		{"cut to width", Glyphs{}, "abcdef", 4, "✗ ab"},
		{"zero width", Glyphs{}, "no", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := failText(tt.g, errors.New(tt.msg), tt.cols)
			if got != tt.want {
				t.Fatalf("failText() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestOneLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "plain", "plain"},
		{"newline", "a\nb", "a b"},
		{"crlf", "a\r\nb", "a  b"},
		{"tab", "a\tb", "a b"},
		{"nul", "a\x00b", "a b"},
		{"del", "a\x7fb", "a b"},
		{"unicode kept", "Tên lỗi", "Tên lỗi"},
		{"c1 control", "a\u0085b", "a b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := oneLine(tt.in); got != tt.want {
				t.Fatalf("oneLine(%q) = %q; want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFrameRows(t *testing.T) {
	tests := []struct {
		name string
		f    frame
		want int
	}{
		{"empty", frame{}, 1},
		{"one line", frame{lines: []string{"a"}}, 1},
		{"two lines", frame{lines: []string{"a", "b"}}, 2},
		{"footer only", frame{footer: "x"}, 1},
		{"line and footer", frame{lines: []string{"a"}, footer: "x"}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.rows(); got != tt.want {
				t.Fatalf("rows() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestFrameCursor(t *testing.T) {
	// cursorRow and cursorCol are 0-based and do not change the row count.
	f := frame{
		lines:     []string{"? Name", "  a"},
		cursorRow: 1,
		cursorCol: 4,
	}

	if got := f.rows(); got != 2 {
		t.Fatalf("rows() = %d; want 2", got)
	}

	if f.cursorRow != 1 || f.cursorCol != 4 {
		t.Fatalf("cursor = %d,%d; want 1,4", f.cursorRow, f.cursorCol)
	}
}
