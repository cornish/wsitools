package wsiwriter

import (
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	xtiff "golang.org/x/image/tiff"
)

// TestWriteTiledTIFF writes a 16x16 RGB image laid out as four 8x8 uncompressed
// tiles, then re-decodes via golang.org/x/image/tiff to verify structural validity.
func TestWriteTiledTIFF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tiled.tiff")

	w, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	level, err := w.AddLevel(LevelSpec{
		ImageWidth:                16,
		ImageHeight:               16,
		TileWidth:                 8,
		TileHeight:                8,
		Compression:               CompressionNone,
		PhotometricInterpretation: 2, // RGB
	})
	if err != nil {
		t.Fatalf("AddLevel: %v", err)
	}

	// Four tiles, each 8x8x3 bytes. Fill each with a distinct value so we can
	// confirm tile-coordinate-to-bytes mapping after decode.
	for ty := uint32(0); ty < 2; ty++ {
		for tx := uint32(0); tx < 2; tx++ {
			tile := make([]byte, 8*8*3)
			for i := range tile {
				tile[i] = byte(ty*2 + tx + 1)
			}
			if err := level.WriteTile(tx, ty, tile); err != nil {
				t.Fatalf("WriteTile(%d,%d): %v", tx, ty, err)
			}
		}
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
	if got, want := img.Bounds(), image.Rect(0, 0, 16, 16); got != want {
		t.Errorf("bounds: got %v, want %v", got, want)
	}
	// Spot-check pixel (0,0) (in tile (0,0), filled with 1) vs pixel (8,8)
	// (in tile (1,1), filled with 4):
	r0, g0, b0, _ := img.At(0, 0).RGBA()
	r1, g1, b1, _ := img.At(8, 8).RGBA()
	if r0 == r1 && g0 == g1 && b0 == b1 {
		t.Errorf("tile (0,0) and (1,1) have identical pixels — tile layout wrong")
	}
}

// TestWriteBigTIFF writes a 16x16 RGB tiled BigTIFF and validates it using
// tiffinfo (libtiff's CLI tool). golang.org/x/image/tiff cannot read BigTIFF,
// so we shell out. The test is skipped if tiffinfo is not in PATH.
func TestWriteBigTIFF(t *testing.T) {
	if _, err := exec.LookPath("tiffinfo"); err != nil {
		t.Skip("tiffinfo not in PATH (brew install libtiff); skipping")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "big.tiff")

	w, err := Create(path, WithBigTIFF(true))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	level, err := w.AddLevel(LevelSpec{
		ImageWidth: 16, ImageHeight: 16,
		TileWidth: 8, TileHeight: 8,
		Compression: CompressionNone, PhotometricInterpretation: 2,
	})
	if err != nil {
		t.Fatalf("AddLevel: %v", err)
	}
	for ty := uint32(0); ty < 2; ty++ {
		for tx := uint32(0); tx < 2; tx++ {
			if err := level.WriteTile(tx, ty, make([]byte, 8*8*3)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("tiffinfo", path).CombinedOutput()
	if err != nil {
		t.Fatalf("tiffinfo: %v\n%s", err, out)
	}
	got := string(out)
	if !strings.Contains(got, "BigTIFF") && !strings.Contains(got, "Subfile") {
		t.Errorf("tiffinfo output doesn't mention BigTIFF or expected fields:\n%s", got)
	}
}

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
