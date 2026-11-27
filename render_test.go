package miniprompt

import (
	"errors"
	"testing"
)

// lineEditor builds an editor with text and the cursor at pos.
func lineEditor(text string, pos int) *editor {
	e := &editor{buf: []rune(text)}
	e.setCursor(pos)

	return e
}

func TestMarkerText(t *testing.T) {
	tests := []struct {
		name string
		m    string
		def  string
		want string
	}{
		{"hidden", " ", "?", ""},
		{"default", "", "?", "? "},
		{"custom", "[?]", "?", "[?] "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := markerText(tt.m, tt.def); got != tt.want {
				t.Fatalf("markerText(%q, %q) = %q; want %q", tt.m, tt.def, got, tt.want)
			}
		})
	}
}

func TestPendingFrame(t *testing.T) {
	tests := []struct {
		name   string
		in     Input
		text   string
		pos    int
		want   string
		cursor int
	}{
		{
			name:   "default marker, empty",
			in:     Input{Label: "Name"},
			want:   "? Name ",
			cursor: 7,
		},
		{
			name:   "custom marker",
			in:     Input{Marker: Marker{Pending: "[?]"}, Label: "Name"},
			want:   "[?] Name ",
			cursor: 9,
		},
		{
			name:   "hidden marker",
			in:     Input{Marker: Marker{Pending: " "}, Label: "Name"},
			want:   "Name ",
			cursor: 5,
		},
		{
			name:   "placeholder",
			in:     Input{Label: "Name", Placeholder: "nxkh4ng"},
			want:   "? Name " + sgrDim + "nxkh4ng" + sgrReset,
			cursor: 7,
		},
		{
			name:   "text",
			in:     Input{Label: "Name"},
			text:   "Alice",
			pos:    5,
			want:   "? Name Alice",
			cursor: 12,
		},
		{
			name:   "cursor in the middle",
			in:     Input{Label: "Name"},
			text:   "Alice",
			pos:    2,
			want:   "? Name Alice",
			cursor: 9,
		},
		{
			name:   "placeholder ignored when text present",
			in:     Input{Label: "Name", Placeholder: "nxkh4ng"},
			text:   "A",
			pos:    1,
			want:   "? Name A",
			cursor: 8,
		},
		{
			name:   "wide text",
			in:     Input{Label: "Name"},
			text:   "日本",
			pos:    2,
			want:   "? Name 日本",
			cursor: 11,
		},
		{
			name:   "wide cursor in the middle",
			in:     Input{Label: "Name"},
			text:   "日本",
			pos:    1,
			want:   "? Name 日本",
			cursor: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.in.pendingFrame(lineEditor(tt.text, tt.pos))

			if f.lines[0] != tt.want {
				t.Fatalf("line = %q; want %q", f.lines[0], tt.want)
			}
			if f.cursorCol != tt.cursor {
				t.Fatalf("cursor = %d; want %d", f.cursorCol, tt.cursor)
			}
		})
	}
}

func TestPendingFrameSelection(t *testing.T) {
	e := lineEditor("abc", 3)
	e.selectAll()

	f := Input{Label: "Name"}.pendingFrame(e)

	if want := "? Name " + sgrReverse + "abc" + sgrReset; f.lines[0] != want {
		t.Fatalf("line = %q; want %q", f.lines[0], want)
	}
}

func TestPendingFrameCursorNotSelected(t *testing.T) {
	e := lineEditor("abc", 1)
	e.anchor = 3

	f := Input{Label: "Name"}.pendingFrame(e)

	if want := "? Name a" + sgrReverse + "bc" + sgrReset; f.lines[0] != want {
		t.Fatalf("line = %q; want %q", f.lines[0], want)
	}
	if f.cursorCol != 8 {
		t.Fatalf("cursor = %d; want 8", f.cursorCol)
	}
}

func TestFailLine(t *testing.T) {
	tests := []struct {
		name string
		m    Marker
		err  error
		cols int
		want string
	}{
		{"default marker", Marker{}, errors.New("too short"), 40, "✗ too short"},
		{"hidden marker", Marker{Fail: " "}, errors.New("too short"), 40, "too short"},
		{"custom marker", Marker{Fail: "[x]"}, errors.New("no"), 40, "[x] no"},
		{"newline flattened", Marker{}, errors.New("a\nb\r\nc"), 40, "✗ a b  c"},
		{"cut to width", Marker{}, errors.New("abcdefghij"), 6, "✗ abcd"},
		{"zero width", Marker{}, errors.New("no"), 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Input{Marker: tt.m}.failLine(tt.err, tt.cols)

			if got != tt.want {
				t.Fatalf("failLine() = %q; want %q", got, tt.want)
			}
		})
	}
}
