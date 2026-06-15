package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOutputFilename_stripsBz2Suffix(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"bz2 URL strips suffix", "https://example.com/match.dem.bz2", "match.dem"},
		{"plain dem URL unchanged", "https://example.com/match.dem", "match.dem"},
		{"bz2 with path", "https://cdn.example.com/demos/final.dem.bz2", "final.dem"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := outputFilename(tc.input); got != tc.want {
				t.Errorf("outputFilename(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestPrepareLocal_returnsExistingDemPath(t *testing.T) {
	dir := t.TempDir()
	demPath := filepath.Join(dir, "match.dem")
	if err := os.WriteFile(demPath, []byte("demo"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := PrepareLocal(demPath)
	if err != nil {
		t.Fatalf("PrepareLocal() error: %v", err)
	}
	if got != demPath {
		t.Errorf("PrepareLocal() = %q, want %q", got, demPath)
	}
}
