package miniprompt

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDecodeKeys(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []eventKind
	}{
		{"printable", "abc", []eventKind{evRune, evRune, evRune}},
		{"space", "a b", []eventKind{evRune, evRune, evRune}},
		{"enter-cr", "\r", []eventKind{evEnter}},
		{"enter-lf", "\n", []eventKind{evEnter}},
		{"backspace-del", "\x7f", []eventKind{evBackspace}},
		{"backspace-ctrl-h", "\x08", []eventKind{evBackspace}},
		{"interrupt", "\x03", []eventKind{evInterrupt}},
		{"ctrl-a-select", "\x01", []eventKind{evSelectAll}},
		{"ctrl-w-kill-word", "\x17", []eventKind{evKillWordLeft}},
		{"ctrl-d-ignored", "\x04", []eventKind{evUnknown}},
		{"tab", "\t", []eventKind{evTab}},
		{"nul-ignored", "\x00", []eventKind{evUnknown}},
		{"esc-alone-pending", "\x1b", nil},
		{"alt-key-drops-esc", "\x1ba", []eventKind{evUnknown, evRune}},
		{"left", "\x1b[D", []eventKind{evLeft}},
		{"right", "\x1b[C", []eventKind{evRight}},
		{"up", "\x1b[A", []eventKind{evUp}},
		{"down", "\x1b[B", []eventKind{evDown}},
		{"home", "\x1b[H", []eventKind{evHome}},
		{"end", "\x1b[F", []eventKind{evEnd}},
		{"home-1", "\x1b[1~", []eventKind{evHome}},
		{"end-4", "\x1b[4~", []eventKind{evEnd}},
		{"home-7", "\x1b[7~", []eventKind{evHome}},
		{"end-8", "\x1b[8~", []eventKind{evEnd}},
		{"delete", "\x1b[3~", []eventKind{evDelete}},
		{"ctrl-left", "\x1b[1;5D", []eventKind{evWordLeft}},
		{"ctrl-right", "\x1b[1;5C", []eventKind{evWordRight}},
		{"ctrl-left-short", "\x1b[5D", []eventKind{evWordLeft}},
		{"ctrl-right-short", "\x1b[5C", []eventKind{evWordRight}},
		{"ctrl-del-kill-word-right", "\x1b[3;5~", []eventKind{evKillWordRight}},
		{"ss3-left", "\x1bOD", []eventKind{evLeft}},
		{"ss3-home", "\x1bOH", []eventKind{evHome}},
		{"ss3-up", "\x1bOA", []eventKind{evUp}},
		{"ss3-down", "\x1bOB", []eventKind{evDown}},
		{"unknown-csi", "\x1b[Z", []eventKind{evUnknown}},
		{"unknown-ss3", "\x1bOP", []eventKind{evUnknown}},
		{"utf8", "Việt", []eventKind{evRune, evRune, evRune, evRune}},
		{"utf8-nfd", "Vie\u0302\u0323t", []eventKind{evRune, evRune, evRune, evRune, evRune, evRune}},
		{"emoji", "🙂", []eventKind{evRune}},
		{"invalid-byte", "\xff", []eventKind{evUnknown}},
		{"truncated-rune-pending", "\xe1\xbb", nil},
		{"paste", pasteStart + "hi" + pasteEnd, []eventKind{evPaste}},
		{"paste-then-rune", pasteStart + "hi" + pasteEnd + "x", []eventKind{evPaste, evRune}},
		{"rune-tab-rune", "a\tb", []eventKind{evRune, evTab, evRune}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeAll(tt.in, len(tt.in))
			if kinds := eventKinds(got); !slices.Equal(kinds, tt.want) {
				t.Fatalf("decode(%q) = %v; want %v", tt.in, kinds, tt.want)
			}
		})
	}
}

func TestDecodeKeysChunked(t *testing.T) {
	inputs := []string{
		"abc",
		"\r\x7f\x03",
		"\x1b[D\x1b[C\x1b[3~",
		"\x1b[1;5D",
		"\x1b[1;5C",
		"\x1bOA\x1bOD",
		"\x1b[",
		"\x1b[1;5",
		pasteStart + "line1\r\nline2" + pasteEnd,
		pasteStart + "Việt" + pasteEnd + "!",
		"Việt 🙂",
		"\x1ba",
		"\xff\xfe",
		"\x1b[A\x1b[B",
		"\x1bOB\x1bOA",
		"\x1b[3;5~",
	}

	for _, in := range inputs {
		want := decodeAll(in, len(in))

		for chunk := 1; chunk <= len(in); chunk++ {
			if got := decodeAll(in, chunk); !slices.Equal(got, want) {
				t.Fatalf("decode(%q) chunk %d = %v; want %v", in, chunk, got, want)
			}
		}
	}
}

func TestDecodePasteText(t *testing.T) {
	long := strings.Repeat("a", 256)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", pasteStart + "hello" + pasteEnd, "hello"},
		{"multiline", pasteStart + "a\r\nb\nc" + pasteEnd, "a b c"},
		{"tabs", pasteStart + "a\tb" + pasteEnd, "a b"},
		{"controls-dropped", pasteStart + "a\x00\x07b" + pasteEnd, "ab"},
		{"utf8", pasteStart + "Việt" + pasteEnd, "Việt"},
		{"empty", pasteStart + pasteEnd, ""},
		{"long", pasteStart + long + pasteEnd, long},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := decodeAll(tt.in, 1)

			if len(events) != 1 || events[0].kind != evPaste {
				t.Fatalf("events = %v; want one evPaste", events)
			}

			if events[0].text != tt.want {
				t.Fatalf("paste text = %q; want %q", events[0].text, tt.want)
			}
		})
	}
}

func TestDecodePasteUnterminated(t *testing.T) {
	d := &keyDecoder{}

	if events := d.decode([]byte(pasteStart + "abc")); len(events) != 0 {
		t.Fatalf("events before flush = %v; want none", events)
	}

	events := d.flush()
	if len(events) != 1 || events[0].kind != evPaste || events[0].text != "abc" {
		t.Fatalf("flush = %v; want one evPaste with %q", events, "abc")
	}

	if d.pasting || len(d.pending) != 0 {
		t.Fatalf("decoder state after flush: pasting=%v pending=%d", d.pasting, len(d.pending))
	}
}

func TestDecodeRuneValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want rune
	}{
		{"ascii", "a", 'a'},
		{"space", " ", ' '},
		{"vietnamese", "ệ", 'ệ'},
		{"emoji", "🙂", '🙂'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := (&keyDecoder{}).decode([]byte(tt.in))
			if len(events) != 1 || events[0].kind != evRune || events[0].r != tt.want {
				t.Fatalf("events = %+v; want rune %q", events, tt.want)
			}
		})
	}
}

func TestDecodeSS3Left(t *testing.T) {
	events := (&keyDecoder{}).decode([]byte("\x1bOD"))
	if len(events) != 1 || events[0].kind != evLeft {
		t.Fatalf("events = %+v; want evLeft", events)
	}
}

func FuzzDecodeKeys(f *testing.F) {
	for _, seed := range []string{
		"a",
		"\r",
		"\x03",
		"\x04",
		"\x01",
		"\x1b",
		"\x1b[A",
		"\x1b[1;5D",
		"\x1b[",
		"\x1bO",
		pasteStart + "ab\r\nc" + pasteEnd,
		"Việt",
		"\xe1\xbb",
		"\xff\x00\x09",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in string) {
		d := &keyDecoder{}
		var events []event

		for i := 0; i < len(in); i += 3 {
			events = append(events, d.decode([]byte(in[i:min(i+3, len(in))]))...)
		}
		events = append(events, d.flush()...)

		for _, ev := range events {
			if ev.kind == evRune && ev.r == 0 {
				t.Fatal("rune event with zero rune")
			}

			if ev.kind == evRune && !utf8.ValidRune(ev.r) {
				t.Fatalf("invalid rune %U", ev.r)
			}

			if ev.kind == evPaste && !utf8.ValidString(ev.text) {
				t.Fatalf("paste text is not valid UTF-8: %q", ev.text)
			}
		}

		if d.pasting || len(d.pending) != 0 {
			t.Fatalf("decoder state after flush: pasting=%v pending=%d", d.pasting, len(d.pending))
		}
	})
}

func decodeAll(in string, chunk int) []event {
	if chunk <= 0 {
		chunk = 1
	}

	d := &keyDecoder{}
	var events []event

	for i := 0; i < len(in); i += chunk {
		events = append(events, d.decode([]byte(in[i:min(i+chunk, len(in))]))...)
	}

	return append(events, d.flush()...)
}

func eventKinds(events []event) []eventKind {
	kinds := make([]eventKind, len(events))
	for i, ev := range events {
		kinds[i] = ev.kind
	}

	return kinds
}
