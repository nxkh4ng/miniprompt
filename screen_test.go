package miniprompt

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestScreenDraw(t *testing.T) {
	tests := []struct {
		name     string
		f        frame
		prev     int
		want     string
		wantRows int
		wantRow  int
	}{
		{
			name:     "one row",
			f:        frame{lines: []string{"? Name a"}, cursorCol: 8},
			want:     "\r? Name a\x1b[K\x1b[9G",
			wantRows: 1,
		},
		{
			name:     "error row",
			f:        frame{lines: []string{"? Name ab"}, footer: "✗ too short", cursorCol: 9},
			prev:     1,
			want:     "\r? Name ab\x1b[K\r\n✗ too short\x1b[K\x1b[1A\x1b[10G",
			wantRows: 2,
		},
		{
			name:     "clears the error row",
			f:        frame{lines: []string{"? Name abc"}, cursorCol: 10},
			prev:     2,
			want:     "\r? Name abc\x1b[K\r\n\x1b[K\x1b[1A\x1b[11G",
			wantRows: 2,
		},
		{
			name:     "cursor at column zero",
			f:        frame{lines: []string{"Name "}, cursorCol: 0},
			want:     "\rName \x1b[K\x1b[1G",
			wantRows: 1,
		},
		{
			name: "three lines",
			f: frame{
				lines:     []string{"one", "two", "three"},
				cursorRow: 1,
				cursorCol: 2,
			},
			want:     "\rone\x1b[K\r\ntwo\x1b[K\r\nthree\x1b[K\x1b[1A\x1b[3G",
			wantRows: 3,
			wantRow:  1,
		},
		{
			name:     "shrinks to one line",
			f:        frame{lines: []string{"done"}, cursorCol: 4},
			prev:     3,
			want:     "\rdone\x1b[K\r\n\x1b[K\r\n\x1b[K\x1b[2A\x1b[5G",
			wantRows: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder

			s := &screen{w: &out, rows: tt.prev}
			if err := s.draw(tt.f); err != nil {
				t.Fatalf("err = %v; want nil", err)
			}

			if out.String() != tt.want {
				t.Fatalf("out = %q; want %q", out.String(), tt.want)
			}

			if s.rows != tt.wantRows || s.row != tt.wantRow {
				t.Fatalf("state = %d,%d; want %d,%d", s.rows, s.row, tt.wantRows, tt.wantRow)
			}
		})
	}
}

// TestScreenRepaintsMultiLineInPlace guards the invariant that a repaint
// starts at the first row of the block, not at the cursor row of the
// previous frame.
func TestScreenRepaintsMultiLineInPlace(t *testing.T) {
	vt := &virtualTerm{}
	s := &screen{w: vt}

	first := frame{
		lines:     []string{"? Pick", "> a", "  b", "  c"},
		cursorRow: 1,
		cursorCol: 3,
	}
	second := frame{
		lines:     []string{"? Pick", "  a", "> b", "  c"},
		cursorRow: 2,
		cursorCol: 3,
	}

	if err := s.draw(first); err != nil {
		t.Fatalf("first draw: %v", err)
	}

	if err := s.draw(second); err != nil {
		t.Fatalf("second draw: %v", err)
	}

	want := []string{"? Pick", "  a", "> b", "  c"}
	if got := vt.grid(); !slices.Equal(got, want) {
		t.Fatalf("screen = %q; want %q", got, want)
	}

	if vt.row != 2 || vt.col != 3 {
		t.Fatalf("cursor = %d,%d; want 2,3", vt.row, vt.col)
	}
}

func TestScreenFinishShrinksBlock(t *testing.T) {
	var out strings.Builder

	s := &screen{w: &out, rows: 4, row: 2}
	if err := s.finish([]string{"done"}); err != nil {
		t.Fatalf("err = %v; want nil", err)
	}

	want := "\x1b[2A\rdone\x1b[K\r\n\x1b[K\r\n\x1b[K\r\n\x1b[K\x1b[2A"
	if out.String() != want {
		t.Fatalf("out = %q; want %q", out.String(), want)
	}

	if s.rows != 0 || s.row != 0 {
		t.Fatalf("state = %d,%d; want 0,0", s.rows, s.row)
	}
}

func TestScreenClearMovesBelowBlock(t *testing.T) {
	var out strings.Builder

	s := &screen{w: &out, rows: 3, row: 1}
	if err := s.clear(); err != nil {
		t.Fatalf("err = %v; want nil", err)
	}

	want := "\x1b[1A\r\n\r\n\r\n\x1b[K"
	if out.String() != want {
		t.Fatalf("out = %q; want %q", out.String(), want)
	}

	if s.rows != 0 || s.row != 0 {
		t.Fatalf("state = %d,%d; want 0,0", s.rows, s.row)
	}
}

// virtualTerm applies the cursor controls screen emits to a character grid,
// so tests can assert what the user would see instead of raw escape bytes.
type virtualTerm struct {
	rows [][]rune
	row  int
	col  int
}

func (v *virtualTerm) Write(p []byte) (int, error) {
	s := string(p)

	for i := 0; i < len(s); {
		switch s[i] {
		case 0x1b:
			i = v.escape(s, i)
		case '\r':
			v.col = 0
			i++
		case '\n':
			v.row++
			i++
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			v.put(r)
			i += size
		}
	}

	return len(p), nil
}

// escape parses ESC [ params final and returns the next byte offset.
func (v *virtualTerm) escape(s string, i int) int {
	j := i + 2
	for j < len(s) && (s[j] < csiFinalMin || s[j] > csiFinalMax) {
		j++
	}

	if j >= len(s) {
		return len(s)
	}

	n := 1
	if j > i+2 {
		if parsed, err := strconv.Atoi(s[i+2 : j]); err == nil {
			n = parsed
		}
	}

	switch s[j] {
	case 'A':
		v.row = max(v.row-n, 0)
	case 'G':
		v.col = n - 1
	case 'K':
		if v.row < len(v.rows) && v.col < len(v.rows[v.row]) {
			v.rows[v.row] = v.rows[v.row][:v.col]
		}
	}

	return j + 1
}

func (v *virtualTerm) put(r rune) {
	for len(v.rows) <= v.row {
		v.rows = append(v.rows, nil)
	}

	line := v.rows[v.row]
	for len(line) < v.col {
		line = append(line, ' ')
	}

	if v.col < len(line) {
		line[v.col] = r
	} else {
		line = append(line, r)
	}

	v.rows[v.row] = line
	v.col++
}

func (v *virtualTerm) grid() []string {
	lines := make([]string, len(v.rows))
	for i, row := range v.rows {
		lines[i] = strings.TrimRight(string(row), " ")
	}

	return lines
}
