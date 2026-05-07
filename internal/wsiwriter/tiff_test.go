package wsiwriter

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	xtiff "golang.org/x/image/tiff"
)

// TestWriteMinimalTIFF writes the simplest possible classic-TIFF (8x8 RGB,
// uncompressed, single strip) and verifies it's structurally valid by re-opening
// it with golang.org/x/image/tiff (a pure-Go reader).
func TestWriteMinimalTIFF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "minimal.tiff")

	w, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	rgb := make([]byte, 8*8*3) // 8x8 RGB pixels, all-zero (black)
	if err := w.AddStrippedImage(StrippedSpec{
		ImageWidth:                8,
		ImageHeight:               8,
		Compression:               CompressionNone,
		PhotometricInterpretation: 2, // RGB
		StripBytes:                rgb,
	}); err != nil {
		t.Fatalf("AddStrippedImage: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	img, err := xtiff.Decode(f)
	if err != nil {
		t.Fatalf("xtiff.Decode: %v", err)
	}
	bounds := img.Bounds()
	if got, want := bounds, (image.Rect(0, 0, 8, 8)); got != want {
		t.Errorf("bounds: got %v, want %v", got, want)
	}
}
