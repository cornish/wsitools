//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	opentile "github.com/cornish/opentile-go"
	_ "github.com/cornish/opentile-go/formats/all"
)

var v02Codecs = []string{"jpegxl", "avif", "webp", "htj2k"}

func TestTranscode_PerCodec_CMU1Small(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	bin := buildOnce(t)
	for _, c := range v02Codecs {
		c := c
		t.Run(c, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out.tiff")
			cmd := exec.Command(bin, "transcode", "--codec", c, "-o", out, src)
			if b, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("transcode --codec %s: %v\n%s", c, err, b)
			}
			tlr, err := opentile.OpenFile(out)
			if err != nil {
				t.Fatalf("opentile.OpenFile(out): %v", err)
			}
			defer tlr.Close()
			if len(tlr.Levels()) == 0 {
				t.Errorf("output has no levels")
			}
		})
	}
	// Bonus: verify the v0.1 jpeg codec also still works as a transcode target.
	t.Run("jpeg", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.svs")
		cmd := exec.Command(bin, "transcode", "--codec", "jpeg", "-o", out, src)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("transcode --codec jpeg: %v\n%s", err, b)
		}
		tlr, err := opentile.OpenFile(out)
		if err != nil {
			t.Fatalf("opentile.OpenFile(out): %v", err)
		}
		defer tlr.Close()
	})
}

func TestTranscode_PerSourceFormat_ToJPEGXL(t *testing.T) {
	td := testdir(t)
	bin := buildOnce(t)

	cases := []struct {
		name   string
		path   string
		wantOK bool
	}{
		{"svs", filepath.Join(td, "svs", "CMU-1-Small-Region.svs"), true},
		{"philips", filepath.Join(td, "philips-tiff", "Philips-1.tiff"), true},
		{"ome-tiled", filepath.Join(td, "ome-tiff", "Leica-1.ome.tiff"), true},
		{"bif", filepath.Join(td, "bif", "Ventana-1.bif"), true},
		{"ife", filepath.Join(td, "ife", "cervix_2x_jpeg.iris"), true},
		{"generic-tiff", filepath.Join(td, "generic-tiff", "CMU-1.tiff"), true},
		{"ndpi-rejection", filepath.Join(td, "ndpi", "CMU-1.ndpi"), false},
		{"leica-scn-rejection", filepath.Join(td, "scn", "Leica-1.scn"), false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if _, err := os.Stat(c.path); err != nil {
				t.Skipf("fixture missing: %v", err)
			}
			out := filepath.Join(t.TempDir(), "out.tiff")
			cmd := exec.Command(bin, "transcode", "--codec", "jpegxl", "-o", out, c.path)
			b, err := cmd.CombinedOutput()
			if c.wantOK && err != nil {
				t.Fatalf("expected OK, got: %v\n%s", err, b)
			}
			if !c.wantOK && err == nil {
				t.Fatalf("expected failure for %s, transcode succeeded", c.name)
			}
			if !c.wantOK {
				if !strings.Contains(string(b), "format unsupported") &&
					!strings.Contains(string(b), "ErrUnsupportedFormat") {
					t.Errorf("expected sanity-gate failure mention; got:\n%s", b)
				}
			}
		})
	}
}

func TestTranscode_BigTIFFFixture(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "svs_40x_bigtiff.svs")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("BigTIFF fixture missing: %v", err)
	}
	bin := buildOnce(t)
	out := filepath.Join(t.TempDir(), "out.tiff")
	// Use webp for BigTIFF: smallest codec footprint, still tile-by-tile streaming.
	cmd := exec.Command(bin, "transcode", "--codec", "webp", "-o", out, src)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("transcode of 4.8 GB BigTIFF failed: %v\n%s", err, b)
	}
	// If we got here without OOM, streaming works.
}
