package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/nxkh4ng/miniprompt"
)

func main() {
	name, err := miniprompt.Input{
		Marker: miniprompt.Marker{
			Pending: "[?]",
			Done:    "[✓]",
			Fail:    "[✗]",
		},
		Label:       "Name",
		Placeholder: "nxkh4ng",
		Validate: func(s string) error {
			if s == "" {
				return errors.New("name cannot be empty")
			}
			return nil
		},
	}.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("Name: %q\n", name)
}
