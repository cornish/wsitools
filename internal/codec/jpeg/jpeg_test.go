package jpeg

import (
	"bytes"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"testing"

	"github.com/cornish/wsi-tools/internal/codec"
)

func TestJPEGEncoderRoundTrip(t *testing.T) {
	// 256x256 RGB tile with a gradient.
	rgb := make([]byte, 256*256*3)
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			off := (y*256 + x) * 3
			rgb[off+0] = byte(x)
			rgb[off+1] = byte(y)
			rgb[off+2] = 128
		}
	}

	fac := Factory{}
	enc, err := fac.NewEncoder(codec.LevelGeometry{
		TileWidth: 256, TileHeight: 256,
		PixelFormat: codec.PixelFormatRGB8,
		ColorSpace:  codec.ColorSpace{Name: "sRGB"},
	}, codec.Quality{Knobs: map[string]string{"q": "85"}})
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	defer enc.Close()

	tile, err := enc.EncodeTile(rgb, 256, 256, nil)
	if err != nil {
		t.Fatalf("EncodeTile: %v", err)
	}
	if enc.TIFFCompressionTag() != 7 {
		t.Errorf("TIFFCompressionTag: got %d, want 7", enc.TIFFCompressionTag())
	}

	tables := enc.LevelHeader()
	if len(tables) == 0 {
		t.Fatal("LevelHeader: empty")
	}
	// Splice tables (drop EOI) + tile (drop SOI) for stdlib decode.
	spliced := spliceForDecode(tables, tile)
	im, err := stdjpeg.Decode(bytes.NewReader(spliced))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if im.Bounds() != image.Rect(0, 0, 256, 256) {
		t.Errorf("bounds: %v", im.Bounds())
	}
	c := im.At(10, 20)
	r, g, b, _ := c.RGBA()
	got := color.RGBA{R: byte(r >> 8), G: byte(g >> 8), B: byte(b >> 8)}
	// JPEG drift tolerance ±10 per channel at q=85.
	if abs(int(got.R)-10) > 10 || abs(int(got.G)-20) > 10 || abs(int(got.B)-128) > 10 {
		t.Errorf("pixel (10,20) round-trip drift too large: got %v", got)
	}
}

func spliceForDecode(tables, tile []byte) []byte {
	if !bytes.HasSuffix(tables, []byte{0xFF, 0xD9}) || !bytes.HasPrefix(tile, []byte{0xFF, 0xD8}) {
		return nil
	}
	out := make([]byte, 0, len(tables)+len(tile)-4)
	out = append(out, tables[:len(tables)-2]...) // tables minus EOI
	out = append(out, tile[2:]...)               // tile minus SOI
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
