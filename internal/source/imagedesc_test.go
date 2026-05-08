package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadSourceImageDescription_Aperio(t *testing.T) {
	td := testdir(t)
	path := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	desc, err := ReadSourceImageDescription(path)
	if err != nil {
		t.Fatalf("ReadSourceImageDescription: %v", err)
	}
	if !strings.HasPrefix(desc, "Aperio") {
		end := 60
		if end > len(desc) {
			end = len(desc)
		}
		t.Errorf("expected Aperio prefix, got %q", desc[:end])
	}
	if !strings.Contains(desc, "AppMag") {
		t.Errorf("expected AppMag in description, got %q", desc)
	}
}

func TestReadSourceImageDescription_NotTIFF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "garbage.bin")
	os.WriteFile(path, []byte("not a TIFF"), 0644)
	desc, err := ReadSourceImageDescription(path)
	if err == nil {
		t.Errorf("expected error for non-TIFF, got nil (desc=%q)", desc)
	}
}
