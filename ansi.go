package miniprompt

const (
	ansiESC = "\x1b"
	ansiCSI = ansiESC + "["
	ansiSS3 = ansiESC + "O"

	csiFinalMin = 0x40
	csiFinalMax = 0x7e
)

// isCSIFinal reports whether b may terminate a CSI sequence.
func isCSIFinal(b byte) bool {
	return b >= csiFinalMin && b <= csiFinalMax
}
