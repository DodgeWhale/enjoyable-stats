package downloader

import (
	"compress/bzip2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Download(rawURL, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("downloader: mkdir: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("downloader: get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloader: unexpected status %d for %s", resp.StatusCode, rawURL)
	}

	filename := outputFilename(rawURL)
	outPath := filepath.Join(destDir, filename)
	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("downloader: create file: %w", err)
	}
	defer f.Close()

	var reader io.Reader = resp.Body
	if strings.HasSuffix(rawURL, ".bz2") {
		reader = bzip2.NewReader(resp.Body)
	}

	if _, err := io.Copy(f, reader); err != nil {
		return "", fmt.Errorf("downloader: write: %w", err)
	}

	return outPath, nil
}

func outputFilename(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		parts := strings.Split(rawURL, "/")
		return strings.TrimSuffix(parts[len(parts)-1], ".bz2")
	}
	return strings.TrimSuffix(filepath.Base(parsed.Path), ".bz2")
}

// PrepareLocal returns the path to a raw .dem file for analysis.
// Plain .dem paths are returned as-is; .dem.bz2 files are decompressed alongside
// the source file (reusing an existing .dem if already present).
func PrepareLocal(path string) (string, error) {
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("downloader: stat local file: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("downloader: %s is a directory", path)
	}
	if strings.HasSuffix(path, ".bz2") {
		return decompressLocal(path)
	}
	return path, nil
}

func decompressLocal(bz2Path string) (string, error) {
	outPath := strings.TrimSuffix(bz2Path, ".bz2")
	if _, err := os.Stat(outPath); err == nil {
		return outPath, nil
	}

	in, err := os.Open(bz2Path)
	if err != nil {
		return "", fmt.Errorf("downloader: open bz2: %w", err)
	}
	defer in.Close()

	out, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("downloader: create dem: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, bzip2.NewReader(in)); err != nil {
		return "", fmt.Errorf("downloader: decompress: %w", err)
	}

	return outPath, nil
}
