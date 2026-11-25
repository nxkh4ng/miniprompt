//go:build !windows

package miniprompt

import "os"

// enableVT is a no-op outside Windows:
// terminals process ANSI escapes already.
func enableVT(*os.File) (func(), error) {
	return func() {}, nil
}
