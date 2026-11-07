package app

import (
	"os"
	"path/filepath"
	"strings"
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
		"1%2B2": "webm/victor_1!2B2.webm",
	}
	ready, skipped := planCards(moves, webm)
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready, got %d", len(ready))
	}
	if len(skipped) != 1 || skipped[0].Command != "zzz" {
		t.Fatalf("expected 1 skipped (zzz), got %v", skipped)
	}
	if ready[0].VideoPath != "webm/victor_1_2.webm" || ready[1].VideoPath != "webm/victor_1!2B2.webm" {
		t.Fatalf("wrong pairing: %+v", ready)
	}
}

func TestWebmPathsByKey(t *testing.T) {
	dir := t.TempDir()
	name := "victor_1!2B2.webm"
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

func TestWebmFileKeyRoundTrip(t *testing.T) {
	encoded := models.EncodeMoveName("df+2")
	if !strings.Contains(encoded, "%") {
		t.Fatalf("expected the encoded key to contain a percent escape, got %q", encoded)
	}
	safe := webmFileKey(encoded)
	if strings.Contains(safe, "%") {
		t.Fatalf("expected no '%%' in the URL-safe filename fragment, got %q", safe)
	}
	if back := moveKeyFromWebmFile(safe); back != encoded {
		t.Fatalf("round trip mismatch: %q -> %q -> %q", encoded, safe, back)
	}
}

func TestDedupeMoves(t *testing.T) {
	moves := []models.Move{
		{Key: "b%2B3", Command: "b+3", FramesOnBlock: "N/A", FramesOnHit: "N/A"}, // stub
		{Key: "b%2B3", Command: "b+3", FramesOnBlock: "-7", FramesOnHit: "+4"},   // real
		{Key: "1", Command: "1", FramesOnBlock: "-3", FramesOnHit: "+2"},
	}
	got := dedupeMoves(moves)
	if len(got) != 2 {
		t.Fatalf("expected 2 moves after dedupe, got %d (%+v)", len(got), got)
	}
	if got[0].Key != "b%2B3" || got[0].FramesOnBlock != "-7" {
		t.Fatalf("expected the real b+3 entry to be kept, got %+v", got[0])
	}
	if got[1].Key != "1" {
		t.Fatalf("expected the unrelated move 1 to be kept, got %+v", got[1])
	}
}
