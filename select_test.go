package miniprompt

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestSelectView(t *testing.T) {
	w := newSelectWidget(Select{Label: "Pick", Options: []string{"a", "b", "c"}})
	f := w.view(80)

	want := []string{"? Pick", "> a", "  b", "  c"}
	if !slices.Equal(f.lines, want) {
		t.Fatalf("lines = %q; want %q", f.lines, want)
	}
	if f.cursorRow != 1 || f.cursorCol != 0 {
		t.Fatalf("cursor = %d,%d; want 1,0", f.cursorRow, f.cursorCol)
	}
}

func TestSelectWindow(t *testing.T) {
	w := newSelectWidget(Select{
		Label:   "Pick",
		Options: []string{"a", "b", "c", "d"},
		Height:  2,
	})

	w.handle(event{kind: evDown})
	w.handle(event{kind: evDown})

	f := w.view(80)
	if want := []string{"? Pick", "  b", "> c"}; !slices.Equal(f.lines, want) {
		t.Fatalf("lines = %q; want %q", f.lines, want)
	}
	if f.cursorRow != 2 {
		t.Fatalf("row = %d; want 2", f.cursorRow)
	}

	w.handle(event{kind: evDown})
	f = w.view(80)
	if want := []string{"? Pick", "  c", "> d"}; !slices.Equal(f.lines, want) {
		t.Fatalf("lines = %q; want %q", f.lines, want)
	}

	w.handle(event{kind: evUp})
	f = w.view(80)
	if want := []string{"? Pick", "> c", "  d"}; !slices.Equal(f.lines, want) {
		t.Fatalf("lines = %q; want %q", f.lines, want)
	}
	if f.cursorRow != 1 {
		t.Fatalf("row = %d; want 1", f.cursorRow)
	}
}

func TestSelectRun(t *testing.T) {
	var out strings.Builder

	got, err := run(
		newSelectWidget(Select{Label: "Pick", Options: []string{"a", "b", "c"}}),
		strings.NewReader("\x1b[B\x1b[B\r"),
		&out,
	)
	if err != nil || got != "c" {
		t.Fatalf("run = %q, %v; want c, nil", got, err)
	}

	want := "\r✓ Pick c\x1b[K\r\n\x1b[K\r\n\x1b[K\r\n\x1b[K\x1b[2A"
	if !strings.HasSuffix(out.String(), want) {
		t.Fatalf("out = %q; want suffix %q", out.String(), want)
	}
}

func TestSelectCancel(t *testing.T) {
	var out strings.Builder

	_, err := run(
		newSelectWidget(Select{Options: []string{"a", "b"}}),
		strings.NewReader("\x03"),
		&out,
	)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v; want ErrCanceled", err)
	}

	if !strings.HasSuffix(out.String(), "\r\n\r\n\x1b[K") {
		t.Fatalf("out = %q; want cancel below the block", out.String())
	}
}

func TestSelectErrors(t *testing.T) {
	if _, err := (Select{}).RunWith(strings.NewReader(""), &strings.Builder{}); err == nil {
		t.Fatal("err = nil; want no-options error")
	}

	_, err := Select{Options: []string{"a"}}.
		RunWith(strings.NewReader(""), &strings.Builder{})
	if !errors.Is(err, ErrNotTerminal) {
		t.Fatalf("err = %v; want ErrNotTerminal", err)
	}
}

func TestSelectCut(t *testing.T) {
	f := newSelectWidget(Select{
		Label:   "Pick",
		Options: []string{"abcdef"},
	}).view(4)

	for _, line := range f.lines {
		if displayWidth(line) > 4 {
			t.Fatalf("line = %q; width > 4", line)
		}
	}
}
