package miniprompt

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

var kindNames = map[eventKind]string{
	evUnknown:       "evUnknown",
	evEnter:         "evEnter",
	evBackspace:     "evBackspace",
	evDelete:        "evDelete",
	evHome:          "evHome",
	evEnd:           "evEnd",
	evLeft:          "evLeft",
	evRight:         "evRight",
	evWordLeft:      "evWordLeft",
	evWordRight:     "evWordRight",
	evKillWordLeft:  "evKillWordLeft",
	evKillWordRight: "evKillWordRight",
	evSelectAll:     "evSelectAll",
	evInterrupt:     "evInterrupt",
	evRune:          "evRune",
}

func kindName(k eventKind) string {
	if name, ok := kindNames[k]; ok {
		return name
	}
	return fmt.Sprintf("eventKind(%d)", k)
}

// fmtEvents renders events readably in failure messages.
func fmtEvents(evs []event) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, ev := range evs {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(kindName(ev.kind))
		if ev.kind == evRune {
			fmt.Fprintf(&sb, "(%q)", ev.r)
		}
	}
	sb.WriteByte(']')
	return sb.String()
}

// feed decodes chunks in order and fails when bytes are left pending.
func feed(t *testing.T, chunks ...[]byte) []event {
	t.Helper()

	var d decoder
	var got []event
	for _, c := range chunks {
		got = append(got, d.decode(c)...)
	}
	if len(d.pending) != 0 {
		t.Fatalf("pending should be empty, got % x", d.pending)
	}
	return got
}

func TestDecodeKeyPrintableASCII(t *testing.T) {
	for b := 0x20; b < 0x7f; b++ {
		ev, used, needMore := decodeKey([]byte{byte(b)})
		if needMore || used != 1 {
			t.Fatalf("byte %#02x: used=%d needMore=%v, want 1 false", b, used, needMore)
		}
		if ev.kind != evRune || ev.r != rune(b) {
			t.Fatalf("byte %#02x: got %s, want evRune(%q)", b, fmtEvents([]event{ev}), rune(b))
		}
	}
}

func TestDecodeKeySpaceIsText(t *testing.T) {
	// Space is printable, not a control byte: the 0x20 boundary must be strict.
	got := feed(t, []byte("a b"))
	want := []event{{kind: evRune, r: 'a'}, {kind: evRune, r: ' '}, {kind: evRune, r: 'b'}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %s, want %s", fmtEvents(got), fmtEvents(want))
	}
}

func TestDecodeKeyControl(t *testing.T) {
	want := map[byte]eventKind{
		'\r': evEnter,
		'\n': evEnter,
		0x08: evBackspace,
		0x7f: evBackspace,
		0x01: evSelectAll,
		0x03: evInterrupt,
		0x17: evKillWordLeft,
	}

	for b := 0; b <= 0xff; b++ {
		// ESC starts a multi-byte sequence, covered by the escape tests.
		if b == 0x1b {
			continue
		}
		// Printable ASCII is covered by TestDecodeKeyPrintableASCII.
		if b >= 0x20 && b != 0x7f {
			continue
		}

		wantKind, ok := want[byte(b)]
		if !ok {
			wantKind = evUnknown
		}

		ev, used, needMore := decodeKey([]byte{byte(b)})
		if needMore || used != 1 {
			t.Errorf("byte %#02x: used=%d needMore=%v, want 1 false", b, used, needMore)
			continue
		}
		if ev.kind != wantKind {
			t.Errorf("byte %#02x: got %s, want %s", b, kindName(ev.kind), kindName(wantKind))
		}
		if ev.r != 0 {
			t.Errorf("byte %#02x: r=%q set on non-rune event", b, ev.r)
		}
	}
}

func TestDecodeKeyTabIsUnknown(t *testing.T) {
	// No evTab exists, so 0x09 falls into the "ignore other control bytes" branch.
	ev, used, needMore := decodeKey([]byte{0x09})
	if needMore || used != 1 || ev.kind != evUnknown {
		t.Fatalf("got %s used=%d needMore=%v, want evUnknown 1 false", kindName(ev.kind), used, needMore)
	}
}

func TestDecodeKeyESCAlone(t *testing.T) {
	ev, used, needMore := decodeKey([]byte{0x1b})
	if !needMore || used != 0 {
		t.Fatalf("got used=%d needMore=%v, want 0 true", used, needMore)
	}
	if ev != (event{}) {
		t.Fatalf("got %s, want zero value while needMore", fmtEvents([]event{ev}))
	}
}

func TestDecodeESCThenPlainByte(t *testing.T) {
	// ESC followed by a non-CSI/SS3 byte: ESC is dropped, the byte is text.
	got := feed(t, []byte{0x1b, 'x'})
	want := []event{{kind: evUnknown}, {kind: evRune, r: 'x'}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %s, want %s", fmtEvents(got), fmtEvents(want))
	}
}

func TestDecodeUTF8Valid(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want rune
	}{
		{"ascii", "a", 'a'},
		{"2-byte", "é", 'é'},
		{"3-byte", "ế", 'ế'},
		{"4-byte", "😀", '😀'},
		{"replacement char", "\uFFFD", '\uFFFD'},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := []byte(c.in)
			ev, used, needMore := decodeUTF8(in)
			if needMore {
				t.Fatalf("decodeUTF8(% x) reported needMore", in)
			}
			if used != len(in) {
				t.Errorf("used = %d, want %d", used, len(in))
			}
			if ev.kind != evRune || ev.r != c.want {
				t.Errorf("got %s, want evRune(%q)", fmtEvents([]event{ev}), c.want)
			}
		})
	}
}

func TestDecodeUTF8Truncated(t *testing.T) {
	for _, s := range []string{"é", "ế", "😀"} {
		b := []byte(s)
		for i := 1; i < len(b); i++ {
			ev, used, needMore := decodeUTF8(b[:i])
			if !needMore || used != 0 {
				t.Errorf("%q truncated to %d bytes: used=%d needMore=%v, want 0 true",
					s, i, used, needMore)
			}
			if ev != (event{}) {
				t.Errorf("%q truncated to %d bytes: ev=%s, want zero value", s, i, fmtEvents([]event{ev}))
			}
		}
	}
}

func TestDecodeUTF8Invalid(t *testing.T) {
	// All of these are invalid on their own, so exactly 1 byte is dropped.
	bad := [][]byte{
		{0x80}, {0xbf}, // stray continuation bytes
		{0xc0}, {0xc1}, // overlong encodings
		{0xf5}, {0xff}, // above U+10FFFF
		{0xe1, 'A'}, // bad continuation
	}
	for _, b := range bad {
		ev, used, needMore := decodeUTF8(b)
		if needMore || used != 1 || ev.kind != evUnknown {
			t.Errorf("decodeUTF8(% x): got %s used=%d needMore=%v, want evUnknown 1 false",
				b, kindName(ev.kind), used, needMore)
		}
	}
}

func TestDecodeUTF8MatchesStdlibClassification(t *testing.T) {
	for b := 0x80; b <= 0xff; b++ {
		in := []byte{byte(b)}
		ev, used, needMore := decodeUTF8(in)

		wantMore := !utf8.FullRune(in)
		if needMore != wantMore {
			t.Errorf("byte %#02x: needMore=%v, want %v (!FullRune)", b, needMore, wantMore)
			continue
		}
		if needMore && used != 0 {
			t.Errorf("byte %#02x: used=%d while needMore, want 0", b, used)
		}
		if !needMore && (used != 1 || ev.kind != evUnknown) {
			t.Errorf("byte %#02x: got %s used=%d, want evUnknown 1", b, kindName(ev.kind), used)
		}
	}
}

func TestDecodeUTF8KeepsFollowingText(t *testing.T) {
	// A bad byte must not swallow the text after it.
	got := feed(t, []byte{0xe1, 'A'})
	want := []event{{kind: evUnknown}, {kind: evRune, r: 'A'}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %s, want %s", fmtEvents(got), fmtEvents(want))
	}
}

func TestDecodeKeySequences(t *testing.T) {
	for seq, want := range keySeqs {
		t.Run(fmt.Sprintf("%q", seq), func(t *testing.T) {
			ev, used, needMore := decodeKey([]byte(seq))
			if needMore {
				t.Fatalf("decodeKey(%q) reported needMore", seq)
			}
			if used != len(seq) {
				t.Errorf("used = %d, want %d", used, len(seq))
			}
			if ev.kind != want {
				t.Errorf("got %s, want %s", kindName(ev.kind), kindName(want))
			}
		})
	}
}

func TestKeySeqsPrefixes(t *testing.T) {
	if len(keySeqs) == 0 {
		t.Fatal("keySeqs is empty")
	}
	for seq := range keySeqs {
		if !strings.HasPrefix(seq, ansiCSI) && !strings.HasPrefix(seq, ansiSS3) {
			t.Errorf("sequence %q lacks an ANSI prefix", seq)
		}
	}
}

func TestDecodeUnknownSeqIsConsumed(t *testing.T) {
	cases := []struct {
		name string
		in   string
		used int
	}{
		{"csi unmapped final", "\x1b[Z", 3},   // backtab, not in keySeqs
		{"csi with params", "\x1b[1;2C", 6},   // shift+right, not in keySeqs
		{"csi private mode", "\x1b[?25l", 6},  // dec mode set, not a key
		{"csi cursor up", "\x1b[A", 3},        // no evUp: must not leak text
		{"ss3 unmapped", "\x1bOP", 3},         // F1
		{"ss3 cursor up", "\x1bOA", 3},        // no evUp
		{"esc then bracket only", "\x1b[", 0}, // incomplete
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev, used, needMore := decodeKey([]byte(c.in))
			if c.used == 0 {
				if !needMore || used != 0 {
					t.Fatalf("got used=%d needMore=%v, want 0 true", used, needMore)
				}
				return
			}
			if needMore || used != c.used {
				t.Fatalf("got used=%d needMore=%v, want %d false", used, needMore, c.used)
			}
			if ev.kind != evUnknown {
				t.Fatalf("got %s, want evUnknown", kindName(ev.kind))
			}
		})
	}
}

func TestDecodeUnknownSeqDoesNotLeakText(t *testing.T) {
	// The whole sequence must disappear, not become "[Z" in the buffer.
	got := feed(t, []byte("\x1b[Za"))
	want := []event{{kind: evUnknown}, {kind: evRune, r: 'a'}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %s, want %s", fmtEvents(got), fmtEvents(want))
	}
}

func TestDecodeCSITruncated(t *testing.T) {
	full := []byte("\x1b[1;5C")
	for i := 2; i < len(full); i++ {
		ev, used, needMore := decodeKey(full[:i])
		if !needMore || used != 0 {
			t.Errorf("%d bytes: used=%d needMore=%v, want 0 true", i, used, needMore)
		}
		if ev != (event{}) {
			t.Errorf("%d bytes: ev=%s, want zero value", i, fmtEvents([]event{ev}))
		}
	}
}

func TestDecodeSS3Truncated(t *testing.T) {
	in := []byte("\x1bO")
	ev, used, needMore := decodeKey(in)
	if !needMore || used != 0 {
		t.Fatalf("%q: used=%d needMore=%v, want 0 true", in, used, needMore)
	}
	if ev != (event{}) {
		t.Fatalf("%q: ev=%s, want zero value", in, fmtEvents([]event{ev}))
	}
}

func TestDecodePlainNewlineAndCRLF(t *testing.T) {
	got := feed(t, []byte("a\r\nb"))
	want := []event{
		{kind: evRune, r: 'a'},
		{kind: evEnter},
		{kind: evEnter},
		{kind: evRune, r: 'b'},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %s, want %s", fmtEvents(got), fmtEvents(want))
	}
}

func TestDecodeVietnameseOneByteAtATime(t *testing.T) {
	const input = "Tiếng Việt"
	var chunks [][]byte
	for i := range len(input) {
		chunks = append(chunks, []byte{input[i]})
	}

	got := feed(t, chunks...)
	if len(got) != len([]rune(input)) {
		t.Fatalf("got %d events, want %d: %s", len(got), len([]rune(input)), fmtEvents(got))
	}
	for i, want := range []rune(input) {
		if got[i].kind != evRune || got[i].r != want {
			t.Fatalf("event %d: got %s, want evRune(%q)", i, fmtEvents(got[i:i+1]), want)
		}
	}
}

func TestDecodeAllSplits(t *testing.T) {
	input := []byte("\x1b[1;5C" + "ế" + "a" + "\x1b[D" + "😀" + "\r" + "\x1b[3~")
	want := []event{
		{kind: evWordRight},
		{kind: evRune, r: 'ế'},
		{kind: evRune, r: 'a'},
		{kind: evLeft},
		{kind: evRune, r: '😀'},
		{kind: evEnter},
		{kind: evDelete},
	}

	if got := feed(t, input); !slices.Equal(got, want) {
		t.Fatalf("one chunk: got %s, want %s", fmtEvents(got), fmtEvents(want))
	}

	for i := 0; i <= len(input); i++ {
		got := feed(t, input[:i], input[i:])
		if !slices.Equal(got, want) {
			t.Fatalf("split at %d: got %s, want %s", i, fmtEvents(got), fmtEvents(want))
		}
	}

	var chunks [][]byte
	for i := range input {
		chunks = append(chunks, input[i:i+1])
	}
	if got := feed(t, chunks...); !slices.Equal(got, want) {
		t.Fatalf("byte at a time: got %s, want %s", fmtEvents(got), fmtEvents(want))
	}
}

func TestDecoderKeepsPending(t *testing.T) {
	var d decoder

	if got := d.decode([]byte{0xe1}); len(got) != 0 {
		t.Fatalf("got %s, want no events yet", fmtEvents(got))
	}
	if len(d.pending) != 1 {
		t.Fatalf("pending = % x, want 1 byte", d.pending)
	}

	got := d.decode([]byte{0xba, 0xbf})
	want := []event{{kind: evRune, r: 'ế'}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %s, want %s", fmtEvents(got), fmtEvents(want))
	}
	if len(d.pending) != 0 {
		t.Fatalf("pending = % x, want empty when idle", d.pending)
	}
}

func TestDecoderInvalidRuneAcrossChunks(t *testing.T) {
	// A truncated lead byte completed by a non-continuation byte must resync.
	got := feed(t, []byte{0xe1}, []byte{'A'})
	want := []event{{kind: evUnknown}, {kind: evRune, r: 'A'}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %s, want %s", fmtEvents(got), fmtEvents(want))
	}
}

func TestDecoderCopiesCallerBuffer(t *testing.T) {
	buf := make([]byte, utf8.UTFMax)
	buf[0] = 0xe1

	var d decoder
	d.decode(buf[:1])

	// The caller reuses its read buffer for the next Read.
	buf[0] = 0xff

	got := d.decode([]byte{0xba, 0xbf})
	want := []event{{kind: evRune, r: 'ế'}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %s, want %s (pending must not alias the caller buffer)", fmtEvents(got), fmtEvents(want))
	}
}

func TestDecoderEmptyChunk(t *testing.T) {
	var d decoder
	if got := d.decode(nil); len(got) != 0 {
		t.Fatalf("got %s, want nothing", fmtEvents(got))
	}
	if got := d.decode([]byte{}); len(got) != 0 {
		t.Fatalf("got %s, want nothing", fmtEvents(got))
	}
	if len(d.pending) != 0 {
		t.Fatalf("pending = % x, want empty", d.pending)
	}
}

func TestDecodeKeyAlwaysConsumes(t *testing.T) {
	// Contract: needMore == false must mean used >= 1, otherwise decode loops forever.
	for b := 0; b <= 0xff; b++ {
		_, used, needMore := decodeKey([]byte{byte(b)})
		if !needMore && used < 1 {
			t.Fatalf("byte %#02x: used=%d with needMore=false", b, used)
		}
	}
	for _, seq := range []string{"\x1b[", "\x1bO", "\x1b", "\x1b[1;5", "\xe1"} {
		_, used, needMore := decodeKey([]byte(seq))
		if !needMore && used < 1 {
			t.Fatalf("%q: used=%d with needMore=false", seq, used)
		}
	}
}

func TestDecodeLongUnknownCSITerminates(t *testing.T) {
	// A single ESC followed by digits stays buffered until a final byte arrives.
	var d decoder
	if got := d.decode([]byte("\x1b[123")); len(got) != 0 {
		t.Fatalf("got %s, want no events yet", fmtEvents(got))
	}
	got := d.decode([]byte("A"))
	want := []event{{kind: evUnknown}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %s, want %s", fmtEvents(got), fmtEvents(want))
	}
	if len(d.pending) != 0 {
		t.Fatalf("pending = % x, want empty", d.pending)
	}
}

func TestDecoderRuneInvariant(t *testing.T) {
	// evRune always carries a non-zero rune; other events never set one.
	input := []byte("aéế😀\x1b[1;5C\x1b[3~\x01\x03\x17\x08\r\n\xff\x80")
	got := feed(t, input)
	for i, ev := range got {
		if ev.kind == evRune && ev.r == 0 {
			t.Errorf("event %d: evRune with zero rune", i)
		}
		if ev.kind != evRune && ev.r != 0 {
			t.Errorf("event %d: %s has r=%q", i, kindName(ev.kind), ev.r)
		}
	}
}

func FuzzDecode(f *testing.F) {
	for _, seed := range []string{
		"",
		"a",
		"Tiếng Việt",
		"\x1b[1;5C",
		"\x1b[",
		"\x1b",
		"\x1bO",
		"\x1b[3~",
		"\xe1",
		"\xe1\xba\xbf",
		"\xff",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 64 {
			return
		}

		var d decoder
		got := d.decode(b)

		// Nothing is emitted by an empty follow-up read.
		if extra := d.decode(nil); len(extra) != 0 {
			t.Fatalf("second decode emitted %s", fmtEvents(extra))
		}

		for i, ev := range got {
			if ev.kind == evRune && ev.r == 0 {
				t.Fatalf("event %d: evRune with zero rune", i)
			}
			if ev.kind != evRune && ev.r != 0 {
				t.Fatalf("event %d: %s has r=%q", i, kindName(ev.kind), ev.r)
			}
		}

		// Splitting the input must never change the result.
		for i := 0; i <= len(b); i++ {
			var d2 decoder
			split := append(d2.decode(b[:i]), d2.decode(b[i:])...)
			if !slices.Equal(split, got) {
				t.Fatalf("split at %d: got %s, want %s", i, fmtEvents(split), fmtEvents(got))
			}
		}
	})
}
