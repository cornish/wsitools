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

// v02Codecs and the substrings expected in `tiffinfo`'s Compression Scheme
// line. tiffinfo recognises some codecs by name (e.g. "WEBP" for tag 50001)
// and prints unrecognised codes as their numeric value. We accept either form
// per codec to be portable across libtiff versions.
//
// These four codec wrappers produce TIFFs that opentile-go v0.12's generic-TIFF
// reader does NOT yet recognise (it accepts JPEG / JP2K / LZW / Deflate / None
// compression values only). Until a future opentile-go release adds decoders
// for our private codes, the per-codec test validates structurally via
// tiffinfo rather than round-tripping through opentile-go.
var v02Codecs = []struct {
	name           string
	expectAnyOf    []string // any of these substrings indicates the right Compression Scheme
}{
	{"jpegxl", []string{"50002", "JPEG-XL", "JXL"}},        // Adobe-allocated draft
	{"avif", []string{"60001"}},                            // wsi-tools-private (no libtiff name)
	{"webp", []string{"WEBP", "50001"}},                    // Adobe-allocated; libtiff prints "WEBP"
	{"htj2k", []string{"60003"}},                           // wsi-tools-private
}

func TestTranscode_PerCodec_CMU1Small(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	if _, err := exec.LookPath("tiffinfo"); err != nil {
		t.Skip("tiffinfo not in PATH (brew install libtiff); skipping structural validation")
	}
	bin := buildOnce(t)

	for _, c := range v02Codecs {
		c := c
		t.Run(c.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out.tiff")
			cmd := exec.Command(bin, "transcode", "--codec", c.name, "-o", out, src)
			if b, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("transcode --codec %s: %v\n%s", c.name, err, b)
			}

			// Structural validation via tiffinfo. opentile-go v0.12's
			// generic-TIFF reader only recognises JPEG/JP2K/LZW/Deflate/None
			// compression values, so it would reject our private codes —
			// even though the file is structurally valid TIFF that
			// wsi-tools-aware viewers can decode.
			//
			// tiffinfo may exit non-zero with "WEBP compression support is
			// not configured" (or similar) for codecs libtiff knows by name
			// but wasn't built to decode. The structural directory dump
			// still goes to stdout, so we ignore the exit code and only
			// verify the captured output looks right.
			info, _ := exec.Command("tiffinfo", out).CombinedOutput()
			got := string(info)
			if len(got) == 0 {
				t.Fatalf("tiffinfo produced no output for codec %s", c.name)
			}
			// Must contain at least 2 TIFF Directories (pyramid L0 +
			// associated images at minimum).
			if strings.Count(got, "TIFF Directory") < 2 {
				t.Errorf("expected ≥2 TIFF Directories in tiffinfo output; got:\n%s", got)
			}
			// Compression Scheme line on the pyramid IFD should mention the
			// expected compression — accept any of the per-codec markers
			// (libtiff prints recognised codecs by name, unrecognised by
			// numeric tag value).
			matched := false
			for _, marker := range c.expectAnyOf {
				if strings.Contains(got, marker) {
					matched = true
					break
				}
			}
			if !matched {
				t.Errorf("expected one of %v in tiffinfo output for codec %s; got:\n%s",
					c.expectAnyOf, c.name, got)
			}
			// WSIImageType tag (65080) should appear at least once.
			if !strings.Contains(got, "65080") {
				t.Errorf("expected WSIImageType tag (65080) in tiffinfo output; got:\n%s", got)
			}
		})
	}

	// Bonus: the v0.1 jpeg codec produces standard JPEG tiles that opentile-go
	// CAN re-open; round-trip via opentile-go to verify SVS-shape integrity.
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
		if len(tlr.Levels()) == 0 {
			t.Errorf("output has no levels")
		}
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
