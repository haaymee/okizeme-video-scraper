package models

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// EncodeMoveName converts a move command into a lossless, filesystem-safe key.
//
// It keeps [A-Za-z0-9] plus '-' and '_' unchanged and percent-encodes every
// other byte as "%XX". Unlike a "slug", this stays injective: commands that
// differ only by punctuation (e.g. "1+2" vs "1,2"; "+" = simultaneous press,
// "," = sequence) map to different keys, and the result never contains bytes
// that are illegal in Windows filenames (* ? : " < > | \ /).
func EncodeMoveName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c >= '0' && c <= '9',
			c == '-', c == '_':
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func Capitalize(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
