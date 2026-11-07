package models

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func Slug(s string) string {
	if u, err := url.QueryUnescape(s); err == nil {
		s = u
	}
	s = strings.ToLower(s)
	s = nonSlug.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

func Capitalize(s string) string {
	if s == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
