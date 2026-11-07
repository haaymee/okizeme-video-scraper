package app

import (
	"os"
	"path/filepath"
	"testing"

	"okiscraper/internal/models"
)

func TestPlanCards(t *testing.T) {
	moves := []models.Move{
		{Key: "1_2", Command: "1,2"},
		{Key: "1%2B2", Command: "1+2"},
		{Key: "missing", Command: "zzz"},
	}
	webm := map[string]string{
		"1_2":   "webm/victor_1_2.webm",
		"1%2B2": "webm/victor_1%2B2.webm",
	}
	ready, skipped := planCards(moves, webm)
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready, got %d", len(ready))
	}
	if len(skipped) != 1 || skipped[0].Command != "zzz" {
		t.Fatalf("expected 1 skipped (zzz), got %v", skipped)
	}
	if ready[0].VideoPath != "webm/victor_1_2.webm" || ready[1].VideoPath != "webm/victor_1%2B2.webm" {
		t.Fatalf("wrong pairing: %+v", ready)
	}
}

func TestWebmPathsByKey(t *testing.T) {
	dir := t.TempDir()
	name := "victor_1%2B2.webm"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := webmPathsByKey(dir, "victor")
	if err != nil {
		t.Fatal(err)
	}
	if p := got["1%2B2"]; p == "" || !filepath.IsAbs(p) {
		t.Fatalf("expected an absolute path for key %q, got %q (map %v)", "1%2B2", p, got)
	}
}
