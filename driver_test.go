package miniprompt

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
)

// fakeWidget returns a queued action per event.
type fakeWidget struct {
	lines   []string
	row     int
	col     int
	actions []action
	value   string
	verr    error
	done    []string
	seen    []eventKind
}

func (f *fakeWidget) view(int) frame {
	return frame{lines: f.lines, row: f.row, col: f.col}
}

func (f *fakeWidget) handle(ev event) action {
	f.seen = append(f.seen, ev.kind)

	if len(f.actions) == 0 {
		return actRedraw
	}
	a := f.actions[0]
	f.actions = f.actions[1:]

	return a
}

// submit returns verr once, so a retry can succeed.
func (f *fakeWidget) submit() (string, error) {
	err := f.verr
	f.verr = nil

	return f.value, err
}

func (f *fakeWidget) fail(error, int) string { return "bad" }

func (f *fakeWidget) final(string) []string { return f.done }

func TestRunSubmit(t *testing.T) {
	f := &fakeWidget{
		lines:   []string{"? Name"},
		actions: []action{actRedraw, actSubmit},
		value:   "Alice",
		done:    []string{"✓ Name Alice"},
	}

	var out strings.Builder
	got, err := run(f, strings.NewReader("a\r"), &out)
	if err != nil || got != "Alice" {
		t.Fatalf("run = %q, %v; want Alice, nil", got, err)
	}

	if !strings.HasSuffix(out.String(), "\r✓ Name Alice\x1b[K\r\n") {
		t.Fatalf("out = %q; want final block", out.String())
	}
}

func TestRunRetryThenSubmit(t *testing.T) {
	f := &fakeWidget{
		lines:   []string{"? Name"},
		actions: []action{actSubmit, actSubmit},
		value:   "x",
		verr:    errors.New("too short"),
		done:    []string{"✓ x"},
	}

	var out strings.Builder
	got, err := run(f, strings.NewReader("\r\r"), &out)
	if err != nil || got != "x" {
		t.Fatalf("run = %q, %v; want x, nil", got, err)
	}

	if !strings.Contains(out.String(), "\r? Name\x1b[K\r\nbad\x1b[K\x1b[1A\x1b[1G") {
		t.Fatalf("out = %q; want the error row", out.String())
	}

	if !strings.HasSuffix(out.String(), "\r✓ x\x1b[K\r\n\x1b[K") {
		t.Fatalf("out = %q; want the error row erased", out.String())
	}
}

func TestRunCancel(t *testing.T) {
	f := &fakeWidget{
		lines:   []string{"a", "b"},
		actions: []action{actCancel},
	}

	var out strings.Builder
	_, err := run(f, strings.NewReader("\x03"), &out)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v; want ErrCanceled", err)
	}

	if !strings.HasSuffix(out.String(), "\r\n\r\n\x1b[K") {
		t.Fatalf("out = %q; want cancel below the block", out.String())
	}
}

func TestRunFlushesPasteAtEOF(t *testing.T) {
	f := &fakeWidget{
		lines:   []string{"?"},
		actions: []action{actRedraw},
	}

	_, err := run(f, strings.NewReader(pasteStart+"ab"), io.Discard)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v; want EOF", err)
	}
	if !slices.Contains(f.seen, evPaste) {
		t.Fatalf("events = %v; want evPaste", f.seen)
	}
}

func TestRunWriteError(t *testing.T) {
	boom := errors.New("boom")
	f := &fakeWidget{lines: []string{"?"}}

	_, err := run(f, strings.NewReader(""), errWriter{err: boom})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v; want boom", err)
	}
}

func TestRunReadError(t *testing.T) {
	boom := errors.New("boom")
	f := &fakeWidget{lines: []string{"?"}}

	_, err := run(f, errReader{err: boom}, io.Discard)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v; want boom", err)
	}
}

func TestFinishBlockShrinksBlock(t *testing.T) {
	var out strings.Builder

	if err := finishBlock(&out, []string{"done"}, 4); err != nil {
		t.Fatalf("err = %v; want nil", err)
	}

	want := "\rdone\x1b[K\r\n\x1b[K\r\n\x1b[K\r\n\x1b[K\x1b[2A"
	if out.String() != want {
		t.Fatalf("out = %q; want %q", out.String(), want)
	}
}
