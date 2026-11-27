package miniprompt

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
)

// Input asks the user for one line of text.
//
// The zero value works and uses the default markers.
type Input struct {
	// Marker customises the pending, done and fail glyphs.
	Marker Marker

	// Label is shown before the input.
	Label string

	// Placeholder is a dim hint shown while the line is empty.
	Placeholder string

	// Validate, when set, is called on submit. Returning an error keeps
	// the prompt open and shows the message under it.
	Validate func(string) error
}

// inputWidget adapts Input to the prompt driver.
type inputWidget struct {
	in Input
	ed editor
}

func newInputWidget(in Input) *inputWidget {
	return &inputWidget{in: in}
}

func (w *inputWidget) view(_ int) frame {
	return w.in.pendingFrame(&w.ed)
}

func (w *inputWidget) handle(ev event) action {
	switch ev.kind {
	case evRune:
		w.ed.insertRune(ev.r)
	case evPaste:
		w.ed.insertText(ev.text)
	case evBackspace:
		w.ed.backspace()
	case evDelete:
		w.ed.deleteForward()
	case evLeft:
		w.ed.left()
	case evRight:
		w.ed.right()
	case evWordLeft:
		w.ed.wordLeft()
	case evWordRight:
		w.ed.wordRight()
	case evHome:
		w.ed.home()
	case evEnd:
		w.ed.end()
	case evKillWordLeft:
		w.ed.killWordLeft()
	case evKillWordRight:
		w.ed.killWordRight()
	case evSelectAll:
		w.ed.selectAll()
	case evEnter:
		return actSubmit
	case evInterrupt:
		return actCancel
	}

	return actRedraw
}

func (w *inputWidget) submit() (string, error) {
	v := w.ed.text()
	return v, w.in.check(v)
}

func (w *inputWidget) fail(err error, cols int) string {
	return w.in.failLine(err, cols)
}

func (w *inputWidget) final(v string) []string {
	return []string{w.in.doneLine(v)}
}

// Run prompts on os.Stdin and os.Stdout.
func (in Input) Run() (string, error) {
	return in.RunWith(os.Stdin, os.Stdout)
}

// RunWith reads from r and writes to w.
// It uses live mode when both are terminals, plain mode otherwise.
func (in Input) RunWith(r io.Reader, w io.Writer) (string, error) {
	rf, wf, ok := terminalFiles(r, w)
	if ok {
		return liveSession[string](rf, wf, newInputWidget(in))
	}

	return in.runPlainMode(r, w)
}

func (in Input) runPlainMode(r io.Reader, w io.Writer) (string, error) {
	s := in.promptText()
	if _, err := io.WriteString(w, s); err != nil {
		return "", err
	}

	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	value := strings.TrimRight(line, "\r\n")
	if verr := in.check(value); verr != nil {
		return "", verr
	}

	return value, nil
}

func (in Input) promptText() string {
	return markerText(in.Marker.Pending, "?") + in.Label + " "
}

// check runs the optional validator.
func (in Input) check(value string) error {
	if in.Validate == nil {
		return nil
	}

	return in.Validate(value)
}
