package miniprompt

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

var errEmptyName = errors.New("name cannot be empty")

func TestRunWithPlainSubmit(t *testing.T) {
	var out bytes.Buffer

	got, err := Input{Label: "Name"}.RunWith(strings.NewReader("Alice\n"), &out)
	if err != nil || got != "Alice" {
		t.Fatalf("RunWith = %q, %v; want Alice, nil", got, err)
	}

	if !strings.Contains(out.String(), "? Name ") {
		t.Fatalf("output = %q; want prompt", out.String())
	}
}

func TestRunWithPlainCRLF(t *testing.T) {
	got, err := Input{Label: "Name"}.RunWith(strings.NewReader("Alice\r\n"), io.Discard)
	if err != nil || got != "Alice" {
		t.Fatalf("RunWith = %q, %v; want Alice, nil", got, err)
	}
}

func TestRunWithPlainPartialLine(t *testing.T) {
	got, err := Input{Label: "Name"}.RunWith(strings.NewReader("Alice"), io.Discard)
	if err != nil || got != "Alice" {
		t.Fatalf("RunWith = %q, %v; want Alice, nil", got, err)
	}
}

func TestRunWithPlainPreservesSpaces(t *testing.T) {
	got, err := Input{Label: "Name"}.RunWith(strings.NewReader("  Alice  \n"), io.Discard)
	if err != nil || got != "  Alice  " {
		t.Fatalf("RunWith = %q, %v; want spaces preserved", got, err)
	}
}

func TestRunWithPlainEOF(t *testing.T) {
	tests := []struct {
		name     string
		validate func(string) error
		want     string
		wantErr  error
	}{
		{name: "empty accepted"},
		{
			name: "empty rejected",
			validate: func(s string) error {
				if s == "" {
					return errEmptyName
				}
				return nil
			},
			wantErr: errEmptyName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Input{Label: "Name", Validate: tt.validate}.
				RunWith(strings.NewReader(""), io.Discard)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v; want %v", err, tt.wantErr)
				}
				if got != "" {
					t.Fatalf("value = %q; want empty on error", got)
				}
				return
			}

			if err != nil || got != tt.want {
				t.Fatalf("RunWith = %q, %v; want %q, nil", got, err, tt.want)
			}
		})
	}
}

func TestRunWithPlainValidate(t *testing.T) {
	boom := errors.New("boom")
	in := Input{
		Label: "Name",
		Validate: func(string) error {
			return boom
		},
	}

	got, err := in.RunWith(strings.NewReader("x\n"), io.Discard)
	if got != "" {
		t.Fatalf("value = %q; want empty on error", got)
	}
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v; want boom", err)
	}
}

func TestRunWithPlainReadError(t *testing.T) {
	boom := errors.New("boom")

	_, err := Input{Label: "Name"}.RunWith(errReader{err: boom}, io.Discard)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v; want boom", err)
	}
}

func TestRunWithPlainWriteError(t *testing.T) {
	boom := errors.New("boom")

	_, err := Input{Label: "Name"}.RunWith(
		strings.NewReader("Alice\n"),
		errWriter{err: boom},
	)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v; want boom", err)
	}
}

func TestPromptText(t *testing.T) {
	tests := []struct {
		name   string
		marker Marker
		want   string
	}{
		{"default", Marker{}, "? Name "},
		{"custom", Marker{Pending: "[?]"}, "[?] Name "},
		{"hidden", Marker{Pending: " "}, "Name "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := Input{Marker: tt.marker, Label: "Name"}
			if got := in.promptText(); got != tt.want {
				t.Fatalf("promptText() = %q; want %q", got, tt.want)
			}
		})
	}
}

type errReader struct {
	err error
}

func (r errReader) Read([]byte) (int, error) {
	return 0, r.err
}

type errWriter struct {
	err error
}

func (w errWriter) Write([]byte) (int, error) {
	return 0, w.err
}
