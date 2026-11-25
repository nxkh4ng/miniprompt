package miniprompt

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// byteReader hands out one byte per Read to exercise split sequences.
type byteReader struct {
	s string
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.s == "" {
		return 0, io.EOF
	}

	p[0] = r.s[0]
	r.s = r.s[1:]

	return 1, nil
}

func TestLiveLoop(t *testing.T) {
	tests := []struct {
		name    string
		in      Input
		input   string
		want    string
		wantErr error
		suffix  string
	}{
		{
			name:   "submit",
			in:     Input{Label: "Name"},
			input:  "Alice\r",
			want:   "Alice",
			suffix: "\r✓ Name Alice\x1b[K\r\n",
		},
		{
			name:   "submit empty",
			in:     Input{Label: "Name"},
			input:  "\r",
			want:   "",
			suffix: "\r✓ Name \x1b[K\r\n",
		},
		{
			name:    "interrupt keeps message on next line",
			in:      Input{Label: "Name"},
			input:   "ab\x03",
			wantErr: ErrCanceled,
			suffix:  "\r\n",
		},
		{
			name:  "backspace",
			in:    Input{Label: "Name"},
			input: "abc\x7f\r",
			want:  "ab",
		},
		{
			name:  "select all then type",
			in:    Input{Label: "Name"},
			input: "abc\x01x\r",
			want:  "x",
		},
		{
			name:  "left then type",
			in:    Input{Label: "Name"},
			input: "ab\x1b[Dc\r",
			want:  "acb",
		},
		{
			name:  "kill word",
			in:    Input{Label: "Name"},
			input: "foo bar\x17\r",
			want:  "foo ",
		},
		{
			name:  "home and end",
			in:    Input{Label: "Name"},
			input: "bc\x1b[Ha\x1b[F!\r",
			want:  "abc!",
		},
		{
			name:  "paste flattens newlines",
			in:    Input{Label: "Name"},
			input: pasteStart + "a\r\nb" + pasteEnd + "\r",
			want:  "a b",
		},
		{
			name:  "utf8",
			in:    Input{Label: "Name"},
			input: "ệ\r",
			want:  "ệ",
		},
		{
			name:    "eof",
			in:      Input{Label: "Name"},
			input:   "abc",
			wantErr: io.EOF,
			suffix:  "\r? Name abc\x1b[K",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out strings.Builder

			got, err := tt.in.liveLoop(strings.NewReader(tt.input), &out)

			switch {
			case tt.wantErr != nil && !errors.Is(err, tt.wantErr):
				t.Fatalf("err = %v; want %v", err, tt.wantErr)
			case tt.wantErr == nil && err != nil:
				t.Fatalf("err = %v; want nil", err)
			}

			if got != tt.want {
				t.Fatalf("value = %q; want %q", got, tt.want)
			}

			if tt.suffix != "" && !strings.HasSuffix(out.String(), tt.suffix) {
				t.Fatalf("output = %q; want suffix %q", out.String(), tt.suffix)
			}
		})
	}
}

func TestLiveLoopSplitReads(t *testing.T) {
	var out strings.Builder

	got, err := Input{Label: "Name"}.liveLoop(&byteReader{s: "ab\x1b[Dc\r"}, &out)
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}

	if got != "acb" {
		t.Fatalf("value = %q; want %q", got, "acb")
	}
}

func TestLiveLoopRetry(t *testing.T) {
	in := Input{
		Label: "Name",
		Validate: func(s string) error {
			if len(s) < 3 {
				return errors.New("too short")
			}

			return nil
		},
	}

	var out strings.Builder

	got, err := in.liveLoop(strings.NewReader("ab\rcde\r"), &out)
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}

	if got != "abcde" {
		t.Fatalf("value = %q; want %q", got, "abcde")
	}

	if !strings.Contains(out.String(), "\r\n✗ too short\r\n") {
		t.Fatalf("output = %q; want the fail line", out.String())
	}

	if !strings.HasSuffix(out.String(), "\r✓ Name abcde\x1b[K\r\n") {
		t.Fatalf("output = %q; want the done line", out.String())
	}
}

func TestLiveLoopPlaceholder(t *testing.T) {
	var out strings.Builder

	got, err := Input{Label: "Name", Placeholder: "nxkh4ng"}.liveLoop(strings.NewReader("\r"), &out)
	if err != nil {
		t.Fatalf("err = %v; want nil", err)
	}

	if got != "" {
		t.Fatalf("value = %q; want empty", got)
	}

	if !strings.HasPrefix(out.String(), "\r? Name \x1b[2mnxkh4ng\x1b[0m\x1b[K\x1b[7D") {
		t.Fatalf("output = %q; want the placeholder frame", out.String())
	}
}
