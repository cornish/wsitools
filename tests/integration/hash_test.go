//go:build integration

package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHash_FileMode(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out, err := exec.Command(bin, "hash", src).CombinedOutput()
	if err != nil {
		t.Fatalf("hash: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	// Compute expected file hash directly.
	want := computeFileHash(t, src)
	if !strings.HasPrefix(got, "sha256:") {
		t.Errorf("expected sha256: prefix, got: %q", got)
	}
	if !strings.Contains(got, want) {
		t.Errorf("expected hash %s in output, got: %q", want, got)
	}
}

func TestHash_PixelMode_Deterministic(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out1, err := exec.Command(bin, "hash", "--mode", "pixel", src).CombinedOutput()
	if err != nil {
		t.Fatalf("hash --mode pixel: %v\n%s", err, out1)
	}
	out2, err := exec.Command(bin, "hash", "--mode", "pixel", src).CombinedOutput()
	if err != nil {
		t.Fatalf("hash --mode pixel (run 2): %v\n%s", err, out2)
	}
	if string(out1) != string(out2) {
		t.Errorf("pixel hash not deterministic across runs:\nrun 1: %s\nrun 2: %s",
			out1, out2)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(out1)), "sha256-pixel:") {
		t.Errorf("expected sha256-pixel: prefix, got: %q", out1)
	}
}

func TestHash_JSON(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out, err := exec.Command(bin, "hash", "--json", src).CombinedOutput()
	if err != nil {
		t.Fatalf("hash --json: %v\n%s", err, out)
	}
	var got struct {
		Algorithm string `json:"algorithm"`
		Mode      string `json:"mode"`
		Hex       string `json:"hex"`
		Path      string `json:"path"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if got.Algorithm != "sha256" || got.Mode != "file" {
		t.Errorf("got %+v, want algorithm=sha256 mode=file", got)
	}
	if len(got.Hex) != 64 {
		t.Errorf("hex length = %d, want 64", len(got.Hex))
	}
}

func computeFileHash(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("hash file: %v", err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
