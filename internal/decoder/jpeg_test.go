package decoder

import (
	"bytes"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"testing"
)

func TestJPEGDecoderFullScale(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			im.SetRGBA(x, y, color.RGBA{R: byte(x), G: byte(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	stdjpeg.Encode(&buf, im, &stdjpeg.Options{Quality: 90})

	d := NewJPEG()
	dst := make([]byte, 256*256*3)
	got, err := d.DecodeTile(buf.Bytes(), dst, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 256*256*3 {
		t.Errorf("decoded length: got %d, want %d", len(got), 256*256*3)
	}
}

func TestJPEGDecoderHalfScale(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			im.SetRGBA(x, y, color.RGBA{R: byte(x), G: byte(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	stdjpeg.Encode(&buf, im, &stdjpeg.Options{Quality: 90})

	d := NewJPEG()
	dst := make([]byte, 128*128*3)
	got, err := d.DecodeTile(buf.Bytes(), dst, 1, 2) // 1/2 scale
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 128*128*3 {
		t.Errorf("decoded length: got %d, want %d", len(got), 128*128*3)
	}
}
