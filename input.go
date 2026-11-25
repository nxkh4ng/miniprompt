package miniprompt

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

var ErrCanceled = errors.New("miniprompt: canceled")

type Input struct {
	Marker      Marker
	Label       string
	Placeholder string
	Validate    func(string) error
}

type Marker struct {
	Pending string
	Done    string
	Fail    string
}

func (in Input) Run() (string, error) {
	return in.RunWith(os.Stdin, os.Stdout)
}

// RunWith reads from r and writes to w.
// It uses live mode when both are terminals,
// plain mode otherwise.
func (in Input) RunWith(r io.Reader, w io.Writer) (string, error) {
	rf, rok := r.(*os.File)
	wf, wok := w.(*os.File)

	if rok && wok && term.IsTerminal(int(rf.Fd())) && term.IsTerminal(int(wf.Fd())) {
		return in.runLive(rf, wf)
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

func (in Input) check(value string) error {
	if in.Validate == nil {
		return nil
	}
	return in.Validate(value)
}
