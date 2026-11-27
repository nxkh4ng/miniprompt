package miniprompt

import (
	"testing"
)

// ins returns an op that types one rune.
func ins(r rune) func(*editor) {
	return func(e *editor) { e.insert(r) }
}

func TestEditorEdits(t *testing.T) {
	tests := []struct {
		name  string
		start string
		// cursor starts at the end of start.
		ops  []func(*editor)
		want string
		at   int
	}{
		{
			name: "insert runes",
			ops:  []func(*editor){ins('a'), ins('b')},
			want: "ab",
			at:   2,
		},
		{
			name:  "insert in middle",
			start: "ac",
			ops:   []func(*editor){(*editor).left, ins('b')},
			want:  "abc",
			at:    2,
		},
		{
			name:  "insert at start",
			start: "bc",
			ops:   []func(*editor){(*editor).home, ins('a')},
			want:  "abc",
			at:    1,
		},
		{
			name:  "backspace",
			start: "abc",
			ops:   []func(*editor){(*editor).backspace},
			want:  "ab",
			at:    2,
		},
		{
			name:  "backspace at start does nothing",
			start: "abc",
			ops:   []func(*editor){(*editor).home, (*editor).backspace},
			want:  "abc",
			at:    0,
		},
		{
			name:  "delete",
			start: "abc",
			ops:   []func(*editor){(*editor).home, (*editor).delete},
			want:  "bc",
			at:    0,
		},
		{
			name:  "delete at end does nothing",
			start: "abc",
			ops:   []func(*editor){(*editor).delete},
			want:  "abc",
			at:    3,
		},
		{
			name:  "left at start does nothing",
			start: "abc",
			ops:   []func(*editor){(*editor).home, (*editor).left},
			want:  "abc",
			at:    0,
		},
		{
			name:  "right at end does nothing",
			start: "abc",
			ops:   []func(*editor){(*editor).right},
			want:  "abc",
			at:    3,
		},
		{
			name:  "word left",
			start: "foo bar baz",
			ops:   []func(*editor){(*editor).wordLeft},
			want:  "foo bar baz",
			at:    8,
		},
		{
			name:  "word left twice",
			start: "foo bar baz",
			ops:   []func(*editor){(*editor).wordLeft, (*editor).wordLeft},
			want:  "foo bar baz",
			at:    4,
		},
		{
			name:  "word left skips punctuation",
			start: "foo/bar",
			ops:   []func(*editor){(*editor).wordLeft},
			want:  "foo/bar",
			at:    4,
		},
		{
			name:  "word right",
			start: "foo bar",
			ops:   []func(*editor){(*editor).home, (*editor).wordRight},
			want:  "foo bar",
			at:    3,
		},
		{
			name:  "word right twice reaches end",
			start: "foo bar",
			ops:   []func(*editor){(*editor).home, (*editor).wordRight, (*editor).wordRight},
			want:  "foo bar",
			at:    7,
		},
		{
			name:  "kill word",
			start: "foo bar",
			ops:   []func(*editor){(*editor).killWordLeft},
			want:  "foo ",
			at:    4,
		},
		{
			name:  "kill word skips punctuation",
			start: "foo/bar",
			ops:   []func(*editor){(*editor).killWordLeft},
			want:  "foo/",
			at:    4,
		},
		{
			name:  "select all then type replaces",
			start: "abc",
			ops:   []func(*editor){(*editor).selectAll, ins('x')},
			want:  "x",
			at:    1,
		},
		{
			name:  "select all then backspace clears",
			start: "abc",
			ops:   []func(*editor){(*editor).selectAll, (*editor).backspace},
			want:  "",
			at:    0,
		},
		{
			name:  "select all then delete clears",
			start: "abc",
			ops:   []func(*editor){(*editor).selectAll, (*editor).delete},
			want:  "",
			at:    0,
		},
		{
			name:  "select all then kill word clears",
			start: "abc def",
			ops:   []func(*editor){(*editor).selectAll, (*editor).killWordLeft},
			want:  "",
			at:    0,
		},
		{
			name:  "select all then left collapses to start",
			start: "abc",
			ops:   []func(*editor){(*editor).selectAll, (*editor).left},
			want:  "abc",
			at:    0,
		},
		{
			name:  "select all then right collapses to end",
			start: "abc",
			ops:   []func(*editor){(*editor).home, (*editor).selectAll, (*editor).right},
			want:  "abc",
			at:    3,
		},
		{
			name: "paste text",
			ops:  []func(*editor){func(e *editor) { e.insertText("hello") }},
			want: "hello",
			at:   5,
		},
		{
			name:  "paste replaces selection",
			start: "abc",
			ops:   []func(*editor){(*editor).selectAll, func(e *editor) { e.insertText("hi") }},
			want:  "hi",
			at:    2,
		},
		{
			name:  "unicode word left",
			start: "Việt Nam",
			ops:   []func(*editor){(*editor).wordLeft},
			want:  "Việt Nam",
			at:    5,
		},
		{
			name:  "unicode backspace removes whole rune",
			start: "a🙂",
			ops:   []func(*editor){(*editor).backspace},
			want:  "a",
			at:    1,
		},
		{
			name:  "kill word right",
			start: "foo bar",
			ops:   []func(*editor){(*editor).home, (*editor).killWordRight},
			want:  " bar",
			at:    0,
		},
		{
			name:  "kill word right from middle",
			start: "foo bar",
			ops: []func(*editor){
				(*editor).home,
				(*editor).right,
				(*editor).right,
				(*editor).right,
				(*editor).right,
				(*editor).killWordRight,
			},
			want: "foo ",
			at:   4,
		},
		{
			name:  "kill word right skips punctuation",
			start: "foo/bar",
			ops: []func(*editor){
				(*editor).home,
				(*editor).right,
				(*editor).right,
				(*editor).right,
				(*editor).killWordRight,
			},
			want: "foo",
			at:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &editor{buf: []rune(tt.start)}
			e.setCursor(len(e.buf))

			for _, op := range tt.ops {
				op(e)
			}

			if e.cursor != tt.at {
				t.Fatalf("cursor = %d; want %d", e.cursor, tt.at)
			}
		})
	}
}

func TestEditorSelection(t *testing.T) {
	e := &editor{buf: []rune("hello")}
	e.setCursor(5)

	if _, _, ok := e.selection(); ok {
		t.Fatal("selection() reports a selection; want none")
	}

	e.selectAll()

	start, end, ok := e.selection()
	if !ok || start != 0 || end != 5 {
		t.Fatalf("selection() = %d, %d, %v; want 0, 5, true", start, end, ok)
	}

	e.home()
	e.right()
	if _, _, ok := e.selection(); ok {
		t.Fatal("selection() still reports a selection after moving")
	}
}

func TestEditorAlwaysDropsSelection(t *testing.T) {
	ops := []struct {
		name string
		run  func(*editor)
	}{
		{"insert rune", ins('x')},
		{"insert text", func(e *editor) { e.insertText("xy") }},
		{"backspace", (*editor).backspace},
		{"delete", (*editor).delete},
		{"left", (*editor).left},
		{"right", (*editor).right},
		{"home", (*editor).home},
		{"end", (*editor).end},
		{"word left", (*editor).wordLeft},
		{"word right", (*editor).wordRight},
		{"kill word left", (*editor).killWordLeft},
		{"kill word right", (*editor).killWordRight},
	}

	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			e := &editor{buf: []rune("foo bar")}
			e.setCursor(3)
			e.selectAll()
			op.run(e)

			if e.anchor != e.cursor {
				t.Fatalf("anchor = %d, cursor = %d; want equal", e.anchor, e.cursor)
			}
		})
	}
}

func FuzzEditorOps(f *testing.F) {
	f.Add("hello world", uint8(7), 50)

	f.Fuzz(func(t *testing.T, start string, seed uint8, steps int) {
		if steps < 0 || steps > 200 {
			steps = 200
		}

		e := &editor{buf: []rune(start)}
		e.setCursor(len(e.buf))

		// Small deterministic PRNG so a failing case is replayable.
		s := uint32(seed)

		for range steps {
			s = s*1664525 + 1013904223

			switch s % 14 {
			case 0:
				e.insert('a' + rune(s%26))
			case 1:
				e.insertText("hi there")
			case 2:
				e.backspace()
			case 3:
				e.delete()
			case 4:
				e.left()
			case 5:
				e.right()
			case 6:
				e.home()
			case 7:
				e.end()
			case 8:
				e.wordLeft()
			case 9:
				e.wordRight()
			case 10:
				e.killWordLeft()
			case 11:
				e.selectAll()
			case 12:
				e.insert('ệ')
			case 13:
				e.killWordRight()
			}

			if e.cursor < 0 || e.cursor > len(e.buf) {
				t.Fatalf("cursor = %d; want 0..%d", e.cursor, len(e.buf))
			}
			if e.anchor < 0 || e.anchor > len(e.buf) {
				t.Fatalf("anchor = %d; want 0..%d", e.anchor, len(e.buf))
			}
		}
	})
}
