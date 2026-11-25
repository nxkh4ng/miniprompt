package miniprompt

import "testing"

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
		width  int
	}{
		{
			name:   "default marker, empty",
			in:     Input{Label: "Name"},
			want:   "? Name ",
			cursor: 7,
			width:  7,
		},
		{
			name:   "custom marker",
			in:     Input{Marker: Marker{Pending: "[?]"}, Label: "Name"},
			want:   "[?] Name ",
			cursor: 9,
			width:  9,
		},
		{
			name:   "hidden marker",
			in:     Input{Marker: Marker{Pending: " "}, Label: "Name"},
			want:   "Name ",
			cursor: 5,
			width:  5,
		},
		{
			name:   "placeholder",
			in:     Input{Label: "Name", Placeholder: "nxkh4ng"},
			want:   "? Name " + sgrDim + "nxkh4ng" + sgrReset,
			cursor: 7,
			width:  14,
		},
		{
			name:   "text",
			in:     Input{Label: "Name"},
			text:   "Alice",
			pos:    5,
			want:   "? Name Alice",
			cursor: 12,
			width:  12,
		},
		{
			name:   "cursor in the middle",
			in:     Input{Label: "Name"},
			text:   "Alice",
			pos:    2,
			want:   "? Name Alice",
			cursor: 9,
			width:  12,
		},
		{
			name:   "placeholder ignored when text present",
			in:     Input{Label: "Name", Placeholder: "nxkh4ng"},
			text:   "A",
			pos:    1,
			want:   "? Name A",
			cursor: 8,
			width:  8,
		},
		{
			name:   "wide text",
			in:     Input{Label: "Name"},
			text:   "日本",
			pos:    2,
			want:   "? Name 日本",
			cursor: 11,
			width:  11,
		},
		{
			name:   "wide cursor in the middle",
			in:     Input{Label: "Name"},
			text:   "日本",
			pos:    1,
			want:   "? Name 日本",
			cursor: 9,
			width:  11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := tt.in.pendingFrame(lineEditor(tt.text, tt.pos))

			if f.line != tt.want {
				t.Fatalf("line = %q; want %q", f.line, tt.want)
			}
			if f.cursor != tt.cursor {
				t.Fatalf("cursor = %d; want %d", f.cursor, tt.cursor)
			}
			if f.width != tt.width {
				t.Fatalf("width = %d; want %d", f.width, tt.width)
			}
			if got, want := f.back(), tt.width-tt.cursor; got != want {
				t.Fatalf("back() = %d; want %d", got, want)
			}
		})
	}
}

func TestPendingFrameSelection(t *testing.T) {
	e := lineEditor("abc", 3)
	e.selectAll()

	f := Input{Label: "Name"}.pendingFrame(e)

	if want := "? Name " + sgrReverse + "abc" + sgrReset; f.line != want {
		t.Fatalf("line = %q; want %q", f.line, want)
	}
	if f.width != 10 {
		t.Fatalf("width = %d; want 10", f.width)
	}
}

func TestPendingFrameCursorNotSelected(t *testing.T) {
	e := lineEditor("abc", 1)
	e.anchor = 3

	f := Input{Label: "Name"}.pendingFrame(e)

	if want := "? Name a" + sgrReverse + "bc" + sgrReset; f.line != want {
		t.Fatalf("line = %q; want %q", f.line, want)
	}
	if f.cursor != 8 {
		t.Fatalf("cursor = %d; want 8", f.cursor)
	}
}
