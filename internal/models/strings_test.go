package models

import (
	"strings"
	"testing"
)

func TestEncodeMoveNameIsInjective(t *testing.T) {
	pairs := [][2]string{
		{"1+2", "1,2"},
		{"1+3", "1,3"},
		{"b+1+2", "b+1,2"},
		{"df+3+4", "df+3,4"},
		{"IAI.1+2", "IAI.1,2"},
		{"ub+2", "ub+2*"},
	}
	for _, p := range pairs {
		if EncodeMoveName(p[0]) == EncodeMoveName(p[1]) {
			t.Errorf("collision: %q and %q both encode to %q", p[0], p[1], EncodeMoveName(p[0]))
		}
	}
}

func TestEncodeMoveNameFilesystemSafe(t *testing.T) {
	unsafe := []string{"*", "?", ":", "\"", "<", ">", "|", "\\", "/", " "}
	for _, in := range []string{"ub+2*", "a/b", "a:b", "a?b", "Back throw", `a\b`} {
		out := EncodeMoveName(in)
		for _, ch := range unsafe {
			if strings.Contains(out, ch) {
				t.Errorf("EncodeMoveName(%q) = %q contains unsafe %q", in, out, ch)
			}
		}
	}
}

func TestEncodeMoveNameKeepsSafeChars(t *testing.T) {
	if got := EncodeMoveName("abc_123-XYZ"); got != "abc_123-XYZ" {
		t.Errorf("expected identity for safe input, got %q", got)
	}
	if got := EncodeMoveName(""); got != "" {
		t.Errorf("expected empty for empty input, got %q", got)
	}
	if got := EncodeMoveName("1+2"); got != "1%2B2" {
		t.Errorf("expected %q, got %q", "1%2B2", got)
	}
}
