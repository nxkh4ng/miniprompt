//go:build windows

package miniprompt

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT turns on ANSI escape processing for the console output.
// The returned func restores the previous mode.
//
// term.MakeRaw only changes the input handle, so output still needs
// this flag or escape sequences print as literal text.
func enableVT(f *os.File) (func(), error) {
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(f.Fd()), &mode); err != nil {
		return nil, err
	}

	vt := mode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err := windows.SetConsoleMode(windows.Handle(f.Fd()), vt); err != nil {
		return nil, err
	}

	return func() {
		_ = windows.SetConsoleMode(windows.Handle(f.Fd()), mode)
	}, nil
}
