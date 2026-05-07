# wsi-tools v0.2.0 Transcode Tool — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `wsi-tools transcode` end-to-end with 5 new codec wrappers (JPEG-XL, jpegli, AVIF, WebP, HTJ2K), 6 sane source formats (SVS, Philips-TIFF, OME-TIFF tiled, BIF, IFE, generic-TIFF), streaming pyramid build, self-describing TIFF tags (`WSIImageType` + level/source/version block), and a bundled fix for v0.1 downsample's associated-image IFD ordering bug.

**Architecture:** Per-tile streaming end-to-end: source bytes → decode (libjpeg-turbo or OpenJPEG, both already shipped) → identity pixel pass → encode (5 new codec wrappers via cgo) → write (existing `wsiwriter` plus new WSI metadata tags). No raster materialisation. Memory ceiling: workers × tile_bytes × 2, independent of slide size.

**Tech Stack:** Go 1.26+, `github.com/cornish/opentile-go` v0.11.x (the read side), `spf13/cobra` (CLI), libjxl (jpegli + jpegxl), libavif, libwebp, OpenJPH (cgo), libjpeg-turbo (encode/decode, already shipped), OpenJPEG (decode, already shipped), `vbauerster/mpb/v8` (progress), stdlib `log/slog`.

**Spec:** `docs/superpowers/specs/2026-05-07-wsi-tools-v02-design.md`

---

## File structure (delta from v0.1)

```
wsi-tools/
├── cmd/wsi-tools/
│   ├── transcode.go                                  # NEW — Task I1
│   └── downsample.go                                 # MODIFY — Tasks A2 (Format constants), J1+J2 (IFD ordering fix)
├── internal/
│   ├── source/
│   │   ├── source.go                                 # NEW — Task B1: Source/Level/AssociatedImage interfaces
│   │   ├── opentile.go                               # NEW — Task B2: opentile-go wrapper + sanity gate
│   │   ├── opentile_test.go                          # NEW — Task B2
│   │   ├── imagedesc.go                              # NEW — Task B3: tag-270 reader (promoted)
│   │   └── imagedesc_test.go                         # NEW — Task B3
│   ├── wsiwriter/
│   │   ├── tiff.go                                   # MODIFY — Tasks C2, C3: WithMake/Model/Software/DateTime/Source/Version
│   │   ├── wsitags.go                                # NEW — Task C1: WSI tag constants + helpers
│   │   └── wsitags_test.go                           # NEW — Task C1
│   ├── codec/
│   │   ├── jpegli/
│   │   │   ├── jpegli.go                             # NEW — Task D1
│   │   │   └── jpegli_test.go                        # NEW — Task D1
│   │   ├── jpegxl/
│   │   │   ├── jpegxl.go                             # NEW — Task E1
│   │   │   └── jpegxl_test.go                        # NEW — Task E1
│   │   ├── avif/
│   │   │   ├── avif.go                               # NEW — Task F1
│   │   │   └── avif_test.go                          # NEW — Task F1
│   │   ├── webp/
│   │   │   ├── webp.go                               # NEW — Task G1
│   │   │   └── webp_test.go                          # NEW — Task G1
│   │   ├── htj2k/
│   │   │   ├── htj2k.go                              # NEW — Task H1
│   │   │   └── htj2k_test.go                         # NEW — Task H1
│   │   └── all/all.go                                # MODIFY — Task I2: blank-import 5 new codecs
│   └── (decoder, pipeline, resample untouched)
├── docs/
│   ├── tiff-tags.md                                  # RENAME from compression-tags.md (Task C1)
│   ├── viewer-compat.md                              # MODIFY — Task L1
│   └── superpowers/
│       ├── specs/2026-05-07-wsi-tools-v02-design.md  # already exists
│       └── plans/2026-05-07-wsi-tools-v02-transcode.md # this file
├── tests/integration/
│   ├── downsample_test.go                            # MODIFY — Task J2 (Kind round-trip assertion)
│   └── transcode_test.go                             # NEW — Task K1
├── .github/workflows/
│   └── ci.yml                                        # MODIFY — Task L2 (codec deps)
└── (Makefile, README.md, CLAUDE.md, go.mod modified per Task A1)
```

---

## Conventions for the executor

1. **Always run unit tests with `-race -count=1`**. `count=1` defeats Go's test cache.
2. **`cgo` deps live in `// #cgo pkg-config: …` directives where possible.** OpenJPH lacks pkg-config on Homebrew; use explicit `#cgo CXXFLAGS:` and `#cgo LDFLAGS:`.
3. **One commit per task** (the `Step ... Commit` step). Message format: `<type>(<scope>): <one-line summary>`.
4. **Reference fixtures** live at `sample_files/` (gitignored symlink to `$HOME/GitHub/opentile-go/sample_files/`). Integration tests gated by `WSI_TOOLS_TESTDIR` (default `./sample_files`).
5. **No guessing on cgo APIs.** Read the installed library headers (`/opt/homebrew/include/jxl/encode.h`, `/opt/homebrew/include/avif/avif.h`, `/opt/homebrew/include/webp/encode.h`, `/opt/homebrew/include/openjph/ojph_*.h`) when in doubt. Each codec subpackage's spec section in the v0.2 design lists the canonical entry points.
6. **All codec wrappers must move libjpeg/libjxl/libavif/libwebp/OpenJPH state into a single C helper function** to avoid Go-pointer-to-Go-pointer cgo violations. Pattern established in v0.1's `internal/codec/jpeg/jpeg.go` (`wsi_encode` static helper). Mirror that pattern for each new codec.
7. **Per-codec quality knob**: read from `Quality.Knobs["q"]`, fall back to a sensible default (per-codec). Codec-opts namespaced (`jxl.distance`, `avif.speed`, `webp.lossless`, `htj2k.qstep`, `htj2k.layers`, `jpegli.distance`).

---

## Batch A — opentile-go v0.11 bump + Format constants migration (2 tasks)

### Task A1: Bump opentile-go to v0.11 and verify

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Update opentile-go**

```bash
cd /Users/cornish/GitHub/wsi-tools
go get github.com/cornish/opentile-go@v0.11.0
```

Expected: `go.mod` shows `github.com/cornish/opentile-go v0.11.0`. If v0.11.0 isn't tagged yet, use `@latest`; if neither resolves cleanly, escalate as BLOCKED.

- [ ] **Step 2: Verify nothing in v0.1 broke**

```bash
go test ./... -race -count=1
```

Expected: all wsiwriter, codec, decoder, pipeline, resample tests still pass. If any fail because of v0.11's API changes (rather than just format-string renames), inspect the error and either fix in this task or split fixes into a follow-up task. Format-string changes are handled in Task A2.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: bump opentile-go to v0.11.0"
```

---

### Task A2: Migrate format string comparisons to opentile.Format* constants

**Files:**
- Modify: `cmd/wsi-tools/downsample.go`

- [ ] **Step 1: Find all `Format()` string comparisons in the codebase**

```bash
grep -rn "Format()" --include="*.go" .
```

Expected: at least one hit in `cmd/wsi-tools/downsample.go` checking `src.Format() != "svs"`. Note the line numbers.

- [ ] **Step 2: Look up the v0.11 Format constant names**

```bash
grep -E "^const|Format[A-Z]" $HOME/go/pkg/mod/github.com/cornish/opentile-go@v0.11.*/opentile.go | head -20
```

If that doesn't find them, look at `$HOME/go/pkg/mod/github.com/cornish/opentile-go@v0.11.*/` directly:

```bash
ls $HOME/go/pkg/mod/github.com/cornish/opentile-go@v0.11.*/
grep -rE "Format[A-Z][A-Za-z]+ +[A-Z]?" $HOME/go/pkg/mod/github.com/cornish/opentile-go@v0.11.*/opentile.go
```

Confirm the names of the SVS, Philips-TIFF, OME-TIFF, BIF, IFE, generic-TIFF, NDPI constants. Likely `opentile.FormatSVS`, `opentile.FormatPhilipsTIFF`, etc.

- [ ] **Step 3: Modify downsample.go to use the constant**

In `cmd/wsi-tools/downsample.go`, find the line `if src.Format() != "svs" {` and replace with:

```go
import opentile "github.com/cornish/opentile-go"
// ...
if src.Format() != string(opentile.FormatSVS) {
    return fmt.Errorf("v0.1 downsample supports SVS only, got %q", src.Format())
}
```

(If `opentile.Format()` returns the constant type directly rather than a string, drop the `string(...)` cast and compare directly.)

- [ ] **Step 4: Verify build + tests**

```bash
go build ./...
go test ./... -race -count=1
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/wsi-tools/downsample.go
git commit -m "refactor(cli): use opentile.Format* constants instead of literal strings"
```

---

## Batch B — `internal/source` adapter (3 tasks)

### Task B1: Source/Level/AssociatedImage interfaces

**Files:**
- Create: `internal/source/source.go`

- [ ] **Step 1: Write the interface declarations**

Create `internal/source/source.go`:

```go
// Package source is a thin adapter between the wsi-tools CLI and opentile-go.
// It enforces the v0.2 sanity gate (rejecting NDPI and OME-OneFrame at the
// boundary) and exposes a unified streaming-friendly tile API. Whatever
// opentile-go's various format-specific quirks are, the CLI consumes them
// through the Source interface uniformly.
package source

import (
	"errors"
	"image"
	"time"
)

// Source is what the transcode CLI consumes. Wraps an opentile-go Tiler
// after the sanity gate.
type Source interface {
	// Format returns one of the opentile.Format* string values.
	Format() string

	// Levels returns the pyramid levels in order, L0 first.
	Levels() []Level

	// Associated returns the source's associated images (label, macro,
	// thumbnail, overview, probability, map) — the union of what
	// opentile-go's various format-specific readers expose.
	Associated() []AssociatedImage

	// Metadata returns cross-format scanner / acquisition facts.
	Metadata() Metadata

	// SourceImageDescription returns the L0 IFD's raw ImageDescription
	// string for TIFF-dialect sources, or "" for non-TIFF sources (IFE).
	// Errors are silenced — a missing or malformed tag yields "".
	SourceImageDescription() string

	Close() error
}

// Level is one pyramid level.
type Level interface {
	Index() int
	Size() image.Point     // image dimensions in pixels
	TileSize() image.Point // tile dimensions; preserved verbatim on output
	Grid() image.Point     // tilesX × tilesY
	Compression() Compression
	Tile(x, y int) ([]byte, error) // raw compressed bytes from opentile-go
}

// AssociatedImage is one of label / macro / thumbnail / overview /
// probability / map / associated.
type AssociatedImage interface {
	Kind() string
	Size() image.Point
	Compression() Compression
	Bytes() ([]byte, error) // self-contained encoded blob
}

// Compression mirrors opentile-go's Compression enum.
type Compression int

const (
	CompressionUnknown Compression = iota
	CompressionJPEG
	CompressionJPEG2000
	CompressionLZW
	CompressionDeflate
	CompressionNone
	CompressionAVIF
	CompressionIrisProprietary
)

func (c Compression) String() string {
	switch c {
	case CompressionJPEG:
		return "jpeg"
	case CompressionJPEG2000:
		return "jpeg2000"
	case CompressionLZW:
		return "lzw"
	case CompressionDeflate:
		return "deflate"
	case CompressionNone:
		return "none"
	case CompressionAVIF:
		return "avif"
	case CompressionIrisProprietary:
		return "iris-proprietary"
	}
	return "unknown"
}

// Metadata is the cross-format scanner / acquisition info, populated by
// the opentile-go adapter from t.Metadata() plus any per-format helpers.
type Metadata struct {
	Make, Model, Software, SerialNumber string
	Magnification                       float64
	MPP                                 float64 // micrometers per pixel; 0 if unknown
	AcquisitionDateTime                 time.Time
	Raw                                 map[string]string
}

var (
	// ErrUnsupportedFormat is returned by Open for source formats that
	// don't have intrinsic per-tile geometry (NDPI, OME-OneFrame).
	ErrUnsupportedFormat = errors.New("source: format unsupported at v0.2 (NDPI and OME-OneFrame are skipped)")

	// ErrUnsupportedSourceCompression is returned when a tile uses a
	// compression we can't decode (e.g., Iris-proprietary, or AVIF source
	// before v0.2.1).
	ErrUnsupportedSourceCompression = errors.New("source: source compression not decodable at v0.2.0")
)
```

- [ ] **Step 2: Verify it compiles standalone (no Open yet)**

```bash
go build ./internal/source/
```

Expected: succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/source/source.go
git commit -m "feat(source): Source/Level/AssociatedImage interfaces"
```

---

### Task B2: opentile-go adapter + sanity gate

**Files:**
- Create: `internal/source/opentile.go`
- Create: `internal/source/opentile_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/source/opentile_test.go`:

```go
package source

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func testdir(t *testing.T) string {
	t.Helper()
	d := os.Getenv("WSI_TOOLS_TESTDIR")
	if d == "" {
		d = "../../sample_files"
	}
	if _, err := os.Stat(d); err != nil {
		t.Skipf("WSI_TOOLS_TESTDIR=%s not accessible: %v", d, err)
	}
	return d
}

func TestOpen_SVS(t *testing.T) {
	td := testdir(t)
	src, err := Open(filepath.Join(td, "svs", "CMU-1-Small-Region.svs"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer src.Close()
	if src.Format() == "" {
		t.Errorf("empty format string")
	}
	if len(src.Levels()) == 0 {
		t.Errorf("zero levels")
	}
}

func TestOpen_NDPI_Rejects(t *testing.T) {
	td := testdir(t)
	candidate := filepath.Join(td, "ndpi", "CMU-1.ndpi")
	if _, err := os.Stat(candidate); err != nil {
		t.Skipf("NDPI fixture missing: %v", err)
	}
	_, err := Open(candidate)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestOpen_NotATIFF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "garbage.bin")
	if err := os.WriteFile(path, []byte("not a TIFF"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	if err == nil {
		t.Error("expected error opening non-TIFF garbage")
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./internal/source/ -count=1
```

Expected: compile error (`undefined: Open`).

- [ ] **Step 3: Implement `internal/source/opentile.go`**

```go
package source

import (
	"errors"
	"fmt"
	"image"

	opentile "github.com/cornish/opentile-go"
	_ "github.com/cornish/opentile-go/formats/all"
	svsfmt "github.com/cornish/opentile-go/formats/svs"
)

// Open is the entry point. Opens the file via opentile-go, then routes
// through the sanity gate.
func Open(path string) (Source, error) {
	t, err := opentile.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("source: open %s: %w", path, err)
	}
	// Sanity gate.
	switch t.Format() {
	case opentile.FormatNDPI:
		t.Close()
		return nil, fmt.Errorf("%w: NDPI", ErrUnsupportedFormat)
	case opentile.FormatOMETIFF:
		// OneFrame OMEs report TileSize == zero on all levels.
		if oneFrameOME(t) {
			t.Close()
			return nil, fmt.Errorf("%w: OME-OneFrame", ErrUnsupportedFormat)
		}
	}
	desc := readSourceImageDescription(path) // Task B3 helper; "" on non-TIFF or read error
	return &opentileSource{t: t, path: path, desc: desc}, nil
}

func oneFrameOME(t opentile.Tiler) bool {
	for _, lvl := range t.Levels() {
		if lvl.TileSize() == (image.Point{}) {
			return true
		}
	}
	return false
}

// opentileSource implements Source by wrapping an opentile.Tiler.
type opentileSource struct {
	t    opentile.Tiler
	path string
	desc string
}

func (s *opentileSource) Format() string                 { return string(s.t.Format()) }
func (s *opentileSource) SourceImageDescription() string { return s.desc }
func (s *opentileSource) Close() error                   { return s.t.Close() }

func (s *opentileSource) Levels() []Level {
	out := make([]Level, 0, len(s.t.Levels()))
	for i, lvl := range s.t.Levels() {
		out = append(out, &opentileLevel{lvl: lvl, index: i})
	}
	return out
}

func (s *opentileSource) Associated() []AssociatedImage {
	src := s.t.Associated()
	out := make([]AssociatedImage, 0, len(src))
	for _, a := range src {
		out = append(out, &opentileAssociated{a: a})
	}
	return out
}

func (s *opentileSource) Metadata() Metadata {
	md := s.t.Metadata()
	m := Metadata{
		Make:                md.ScannerManufacturer,
		Model:               md.ScannerModel,
		SerialNumber:        md.ScannerSerial,
		Magnification:       md.Magnification,
		AcquisitionDateTime: md.AcquisitionDateTime,
		Raw:                 map[string]string{},
	}
	if len(md.ScannerSoftware) > 0 {
		m.Software = md.ScannerSoftware[0]
	}
	// MPP via per-format accessor where available.
	if smd, ok := svsfmt.MetadataOf(s.t); ok {
		m.MPP = smd.MPP
		m.Raw["filename"] = smd.Filename
	}
	// Other format-specific MPP accessors can be added as needed; for v0.2.0
	// we expose the cross-format Magnification universally and SVS MPP.
	return m
}

// opentileLevel adapts opentile.Level to source.Level.
type opentileLevel struct {
	lvl   opentile.Level
	index int
}

func (l *opentileLevel) Index() int               { return l.index }
func (l *opentileLevel) Size() image.Point        { return l.lvl.Size() }
func (l *opentileLevel) TileSize() image.Point    { return l.lvl.TileSize() }
func (l *opentileLevel) Grid() image.Point        { return l.lvl.Grid() }
func (l *opentileLevel) Tile(x, y int) ([]byte, error) {
	return l.lvl.Tile(x, y)
}

func (l *opentileLevel) Compression() Compression {
	switch l.lvl.Compression() {
	case opentile.CompressionJPEG:
		return CompressionJPEG
	case opentile.CompressionJP2K:
		return CompressionJPEG2000
	case opentile.CompressionLZW:
		return CompressionLZW
	case opentile.CompressionDeflate:
		return CompressionDeflate
	case opentile.CompressionNone:
		return CompressionNone
	}
	return CompressionUnknown
}

// opentileAssociated adapts opentile.AssociatedImage to source.AssociatedImage.
type opentileAssociated struct {
	a opentile.AssociatedImage
}

func (a *opentileAssociated) Kind() string             { return a.a.Kind() }
func (a *opentileAssociated) Size() image.Point        { return a.a.Size() }
func (a *opentileAssociated) Bytes() ([]byte, error)   { return a.a.Bytes() }
func (a *opentileAssociated) Compression() Compression {
	switch a.a.Compression() {
	case opentile.CompressionJPEG:
		return CompressionJPEG
	case opentile.CompressionLZW:
		return CompressionLZW
	}
	return CompressionUnknown
}

// readSourceImageDescription is implemented in imagedesc.go (Task B3).
// Stub here so Open compiles; replaced when Task B3 lands.
var _ = errors.New // silence unused import if errors isn't otherwise used
```

**Note:** the constants `opentile.FormatNDPI`, `opentile.FormatOMETIFF`, `opentile.FormatSVS`, etc. are v0.11 names. If their actual names differ, adjust accordingly (the spec section on opentile-go v0.11 covers this contingency). Same for `opentile.CompressionJPEG`, `CompressionJP2K`, etc.

- [ ] **Step 4: Stub `readSourceImageDescription`**

For Task B2 to compile before Task B3 lands, add a temporary stub at the bottom of `internal/source/opentile.go`:

```go
// Temporary stub; real implementation lands in Task B3 (imagedesc.go).
func readSourceImageDescription(path string) string {
	return ""
}
```

We'll move this to `imagedesc.go` and replace with a real reader in Task B3.

- [ ] **Step 5: Run tests**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./internal/source/ -race -count=1 -v
```

Expected: `TestOpen_SVS` PASS, `TestOpen_NDPI_Rejects` PASS, `TestOpen_NotATIFF` PASS. If any fail because opentile-go v0.11 has a Compression enum value name we missed, add it to the switch and re-run.

- [ ] **Step 6: Commit**

```bash
git add internal/source/
git commit -m "feat(source): opentile-go adapter + NDPI/OME-OneFrame sanity gate"
```

---

### Task B3: ImageDescription helper (promote from cmd/wsi-tools/downsample.go)

**Files:**
- Create: `internal/source/imagedesc.go`
- Create: `internal/source/imagedesc_test.go`
- Modify: `internal/source/opentile.go` (drop the stub)
- Modify: `cmd/wsi-tools/downsample.go` (use the package version)

- [ ] **Step 1: Write failing tests**

Create `internal/source/imagedesc_test.go`:

```go
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
	desc := readSourceImageDescription(path)
	if !strings.HasPrefix(desc, "Aperio") {
		t.Errorf("expected Aperio prefix, got %q", desc[:min(60, len(desc))])
	}
	if !strings.Contains(desc, "AppMag") {
		t.Errorf("expected AppMag in description, got %q", desc)
	}
}

func TestReadSourceImageDescription_NotTIFF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "garbage.bin")
	os.WriteFile(path, []byte("not a TIFF"), 0644)
	desc := readSourceImageDescription(path)
	if desc != "" {
		t.Errorf("expected empty for non-TIFF, got %q", desc)
	}
}

func min(a, b int) int { if a < b { return a }; return b }
```

- [ ] **Step 2: Run, verify the existing stub passes the empty-string test but fails the Aperio test**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./internal/source/ -run TestReadSourceImageDescription -count=1 -v
```

Expected: `TestReadSourceImageDescription_Aperio` FAILS (stub returns ""), `TestReadSourceImageDescription_NotTIFF` PASSES.

- [ ] **Step 3: Implement `internal/source/imagedesc.go`**

The hand-rolled TIFF tag-270 reader. Pattern: read file header (8 bytes classic, 16 bytes BigTIFF), find first IFD offset, walk IFD entries until we find tag 270 (ImageDescription), read the ASCII value (inline if ≤4 bytes classic / ≤8 BigTIFF, else from out-of-band offset).

```go
package source

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// readSourceImageDescription opens path, reads TIFF tag 270 from IFD 0,
// returns the ASCII string. Returns "" on any error or non-TIFF input.
func readSourceImageDescription(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	hdr := make([]byte, 8)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return ""
	}
	var bo binary.ByteOrder
	switch {
	case hdr[0] == 'I' && hdr[1] == 'I':
		bo = binary.LittleEndian
	case hdr[0] == 'M' && hdr[1] == 'M':
		bo = binary.BigEndian
	default:
		return ""
	}
	magic := bo.Uint16(hdr[2:4])
	var ifdOffset int64
	bigtiff := false
	switch magic {
	case 42:
		ifdOffset = int64(bo.Uint32(hdr[4:8]))
	case 0x002B:
		bigtiff = true
		extra := make([]byte, 8)
		if _, err := io.ReadFull(f, extra); err != nil {
			return ""
		}
		// BigTIFF: bytes 4-5 = offset bytesize (0x0008), 6-7 = 0x0000, 8-15 = first-IFD offset.
		ifdOffset = int64(bo.Uint64(extra[:8]))
	default:
		return ""
	}

	desc, err := readTag270FromIFD(f, bo, ifdOffset, bigtiff)
	if err != nil {
		return ""
	}
	return desc
}

// readTag270FromIFD seeks to ifdOffset, reads the IFD entry count and walks
// entries until it finds tag 270, then reads the ASCII string (handling
// inline vs out-of-band).
func readTag270FromIFD(f *os.File, bo binary.ByteOrder, ifdOffset int64, bigtiff bool) (string, error) {
	if _, err := f.Seek(ifdOffset, io.SeekStart); err != nil {
		return "", err
	}
	var entryCount uint64
	if bigtiff {
		buf := make([]byte, 8)
		if _, err := io.ReadFull(f, buf); err != nil {
			return "", err
		}
		entryCount = bo.Uint64(buf)
	} else {
		buf := make([]byte, 2)
		if _, err := io.ReadFull(f, buf); err != nil {
			return "", err
		}
		entryCount = uint64(bo.Uint16(buf))
	}

	entrySize := 12
	if bigtiff {
		entrySize = 20
	}

	entry := make([]byte, entrySize)
	for i := uint64(0); i < entryCount; i++ {
		if _, err := io.ReadFull(f, entry); err != nil {
			return "", err
		}
		tag := bo.Uint16(entry[0:2])
		if tag != 270 {
			continue
		}
		// Tag 270 is ASCII (type 2). count = number of bytes including null terminator.
		// Value/offset field: classic = entry[8:12] (4 bytes); BigTIFF = entry[12:20] (8 bytes).
		var count uint64
		if bigtiff {
			count = bo.Uint64(entry[4:12])
		} else {
			count = uint64(bo.Uint32(entry[4:8]))
		}
		if count == 0 {
			return "", nil
		}
		inlineLimit := uint64(4)
		if bigtiff {
			inlineLimit = 8
		}
		if count <= inlineLimit {
			// Inline value.
			if bigtiff {
				return string(stripNull(entry[12 : 12+count])), nil
			}
			return string(stripNull(entry[8 : 8+count])), nil
		}
		// Out-of-band: value field is an offset.
		var off int64
		if bigtiff {
			off = int64(bo.Uint64(entry[12:20]))
		} else {
			off = int64(bo.Uint32(entry[8:12]))
		}
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			return "", err
		}
		buf := make([]byte, count)
		if _, err := io.ReadFull(f, buf); err != nil {
			return "", err
		}
		return string(stripNull(buf)), nil
	}
	return "", fmt.Errorf("tag 270 not found")
}

// stripNull removes a trailing NUL byte from a TIFF ASCII value if present.
func stripNull(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == 0 {
		return b[:len(b)-1]
	}
	return b
}
```

- [ ] **Step 4: Drop the stub from opentile.go**

Remove the temporary stub `func readSourceImageDescription(path string) string { return "" }` from `internal/source/opentile.go` (Task B2 added it).

- [ ] **Step 5: Update cmd/wsi-tools/downsample.go to use the package version**

Find `cmd/wsi-tools/downsample.go`'s existing local `readSourceImageDescription` helper and remove it. Wherever it was called, replace with `source.readSourceImageDescription(...)` — but that's package-internal; expose it as `source.ReadSourceImageDescription` instead (capitalize).

Update Task B3's `imagedesc.go` to export the function as `ReadSourceImageDescription` (capitalized), and update Task B2's `opentile.go` call to match. Update `downsample.go`'s call site.

- [ ] **Step 6: Run tests + build**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./... -race -count=1
go build ./cmd/wsi-tools/
```

Expected: all pass; binary builds.

- [ ] **Step 7: Commit**

```bash
git add internal/source/imagedesc.go internal/source/imagedesc_test.go internal/source/opentile.go cmd/wsi-tools/downsample.go
git commit -m "feat(source): TIFF tag-270 reader; promote from downsample.go"
```

---

## Batch C — `wsiwriter` extensions (3 tasks)

### Task C1: WSI tag block (constants, helpers, doc rename)

**Files:**
- Create: `internal/wsiwriter/wsitags.go`
- Create: `internal/wsiwriter/wsitags_test.go`
- Rename: `docs/compression-tags.md` → `docs/tiff-tags.md`

- [ ] **Step 1: Failing test**

Create `internal/wsiwriter/wsitags_test.go`:

```go
package wsiwriter

import (
	"bytes"
	"testing"
)

func TestMakeWSIImageTypeTag(t *testing.T) {
	// makeASCIITag is an existing helper (used for tag 270); makeWSIImageTypeTag
	// wraps it specifically for tag 65080 with input validation.
	w := &Writer{} // bo etc. don't matter for tag construction
	w.bo = newLE() // helper: returns binary.LittleEndian
	tag := w.makeWSIImageTypeTag(WSIImageTypeLabel)
	if tag.tag != TagWSIImageType {
		t.Errorf("tag: got %d, want %d", tag.tag, TagWSIImageType)
	}
	if !bytes.HasPrefix(tag.value, []byte("label\x00")) && tag.value[0] != 'l' {
		t.Errorf("value: got %q", tag.value)
	}
}

func newLE() binaryByteOrder { return leOrder{} }

// (Helper types newLE/leOrder/binaryByteOrder are stubs to satisfy the
// existing tag-construction-helpers signature. The test exists primarily
// to exercise the WSIImageType validation logic; the IFD-emission round-trip
// is covered by the integration tests in Task K1.)
```

Actually, simplify the test. The wsitags helpers are thin wrappers; round-trip behavior is more meaningful than unit-testing the byte construction. Replace the test above with:

```go
package wsiwriter

import (
	"strings"
	"testing"
)

func TestWSIImageTypeValidation(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid label", "label", false},
		{"valid pyramid", "pyramid", false},
		{"valid associated", "associated", false},
		{"empty", "", true},
		{"unknown", "FOOBAR", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateWSIImageType(c.input)
			if (err != nil) != c.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, c.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "wsi") {
				t.Errorf("error message should mention 'wsi': %v", err)
			}
		})
	}
}
```

- [ ] **Step 2: Run, verify compile failure**

```bash
go test ./internal/wsiwriter/ -run TestWSIImageType -count=1
```

Expected: compile error (`undefined: ValidateWSIImageType`, `undefined: WSIImageTypeLabel`, etc.).

- [ ] **Step 3: Implement wsitags.go**

```go
package wsiwriter

import "fmt"

// WSI-specific TIFF tag values (private range, ≥ 32768). Documented in
// docs/tiff-tags.md.
const (
	TagWSIImageType    uint16 = 65080 // ASCII; emitted on every IFD
	TagWSILevelIndex   uint16 = 65081 // LONG;  pyramid IFDs only
	TagWSILevelCount   uint16 = 65082 // LONG;  pyramid IFDs only
	TagWSISourceFormat uint16 = 65083 // ASCII; L0 only
	TagWSIToolsVersion uint16 = 65084 // ASCII; L0 only
)

// WSIImageType canonical values. Lower-case to match opentile-go's existing
// AssociatedImage.Kind() values.
const (
	WSIImageTypePyramid     = "pyramid"
	WSIImageTypeLabel       = "label"
	WSIImageTypeMacro       = "macro"
	WSIImageTypeOverview    = "overview"
	WSIImageTypeThumbnail   = "thumbnail"
	WSIImageTypeProbability = "probability"
	WSIImageTypeMap         = "map"
	WSIImageTypeAssociated  = "associated"
)

var validWSIImageTypes = map[string]bool{
	WSIImageTypePyramid:     true,
	WSIImageTypeLabel:       true,
	WSIImageTypeMacro:       true,
	WSIImageTypeOverview:    true,
	WSIImageTypeThumbnail:   true,
	WSIImageTypeProbability: true,
	WSIImageTypeMap:         true,
	WSIImageTypeAssociated:  true,
}

// ValidateWSIImageType returns nil if v is one of the canonical
// WSIImageType values. Use this at the boundary where caller-supplied
// kind strings flow into LevelSpec.WSIImageType / AssociatedSpec.WSIImageType.
func ValidateWSIImageType(v string) error {
	if !validWSIImageTypes[v] {
		return fmt.Errorf("wsi: invalid WSIImageType %q (want one of %s)", v, validValuesString())
	}
	return nil
}

func validValuesString() string {
	return "pyramid|label|macro|overview|thumbnail|probability|map|associated"
}
```

- [ ] **Step 4: Run, verify pass**

```bash
go test ./internal/wsiwriter/ -run TestWSIImageType -race -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Rename and extend the doc**

```bash
git mv docs/compression-tags.md docs/tiff-tags.md
```

Then edit `docs/tiff-tags.md` to add a new section after the existing compression-tags content:

```markdown
## wsi-tools private TIFF tags

Tag values in the private range (≥ 32768) used by wsi-tools to make output
files self-describing. Future opentile-go releases can be taught to read these
tags as authoritative for WSI image classification.

| Tag | Name | Type | Where emitted | Purpose |
|---|---|---|---|---|
| 65080 | WSIImageType | ASCII | every IFD (pyramid + associated) | One of: `pyramid`, `label`, `macro`, `overview`, `thumbnail`, `probability`, `map`, `associated`. Authoritative for image classification. |
| 65081 | WSILevelIndex | LONG | pyramid IFDs only | 0-based pyramid level index (L0 = 0, L1 = 1, …). |
| 65082 | WSILevelCount | LONG | pyramid IFDs only | Total pyramid levels in this file. |
| 65083 | WSISourceFormat | ASCII | L0 only | The source format wsi-tools transcoded from (e.g. `svs`, `philips-tiff`, `ome-tiff`). |
| 65084 | WSIToolsVersion | ASCII | L0 only | The wsi-tools version that produced this file (e.g. `0.2.0`). |

## DICOM-WSI alignment

The WSIImageType vocabulary aligns with DICOM Whole Slide Imaging (PS3.3
Sup. 145), which uses VOLUME / LABEL / OVERVIEW / THUMBNAIL as standard
ImageType values for WSI files. We use lowercase + the additional values
`pyramid`, `macro`, `probability`, `map`, `associated` to match opentile-go's
existing AssociatedImage.Kind() vocabulary.
```

- [ ] **Step 6: Commit**

```bash
git add internal/wsiwriter/wsitags.go internal/wsiwriter/wsitags_test.go docs/tiff-tags.md docs/compression-tags.md
git commit -m "feat(wsiwriter): WSI tag block constants + tiff-tags.md rename"
```

(The `git mv` shows up in the commit as a delete + add; `git mv` records this as a rename in the diff.)

---

### Task C2: Writer options for Make/Model/Software/DateTime/Source/Version

**Files:**
- Modify: `internal/wsiwriter/tiff.go`

- [ ] **Step 1: Failing test (extend tiff_test.go)**

Append to `internal/wsiwriter/tiff_test.go`:

```go
import "time"

func TestWriterOptions_StandardMetadata(t *testing.T) {
	if _, err := exec.LookPath("tiffinfo"); err != nil {
		t.Skip("tiffinfo missing")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "with-md.tiff")
	when := time.Date(2026, 1, 15, 13, 14, 15, 0, time.UTC)

	w, err := Create(path,
		WithBigTIFF(true),
		WithMake("Hamamatsu"),
		WithModel("C9600"),
		WithSoftware("wsi-tools/0.2.0-dev"),
		WithDateTime(when),
		WithSourceFormat("philips-tiff"),
		WithToolsVersion("0.2.0-dev"),
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	level, _ := w.AddLevel(LevelSpec{
		ImageWidth: 8, ImageHeight: 8, TileWidth: 8, TileHeight: 8,
		Compression: CompressionNone, PhotometricInterpretation: 2,
	})
	level.WriteTile(0, 0, make([]byte, 8*8*3))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("tiffinfo", path).CombinedOutput()
	if err != nil {
		t.Fatalf("tiffinfo: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{"Hamamatsu", "C9600", "wsi-tools/0.2.0-dev", "2026:01:15 13:14:15"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in tiffinfo output, got:\n%s", want, got)
		}
	}
}
```

- [ ] **Step 2: Run, verify failure**

Compile error: `undefined: WithMake` etc.

- [ ] **Step 3: Add the options + emit logic in tiff.go**

Add fields to `writerConfig` and corresponding option functions:

```go
type writerConfig struct {
	bo               binary.ByteOrder
	bigtiff          bool
	imageDescription string

	// New v0.2 metadata fields:
	make_         string  // Trailing underscore avoids the Go keyword.
	model         string
	software      string
	dateTime      time.Time
	hasDateTime   bool
	sourceFormat  string  // emits TagWSISourceFormat on L0
	toolsVersion  string  // emits TagWSIToolsVersion on L0
}

func WithMake(s string) Option         { return func(c *writerConfig) { c.make_ = s } }
func WithModel(s string) Option        { return func(c *writerConfig) { c.model = s } }
func WithSoftware(s string) Option     { return func(c *writerConfig) { c.software = s } }
func WithDateTime(t time.Time) Option  { return func(c *writerConfig) { c.dateTime = t; c.hasDateTime = true } }
func WithSourceFormat(s string) Option { return func(c *writerConfig) { c.sourceFormat = s } }
func WithToolsVersion(s string) Option { return func(c *writerConfig) { c.toolsVersion = s } }
```

Mirror these into the `Writer` struct's stored config (Writer needs them at IFD-emit time).

In `buildTiledTags(entry, isL0)`, when `isL0 == true`, append additional tags from the writer's config:

- 271 Make (`makeASCIITag(271, w.make_)`) if non-empty
- 272 Model (`makeASCIITag(272, w.model)`) if non-empty
- 305 Software (`makeASCIITag(305, w.software)`) if non-empty
- 306 DateTime (`makeASCIITag(306, formatTIFFDateTime(w.dateTime))`) if `w.hasDateTime`
- TagWSISourceFormat (`makeASCIITag(TagWSISourceFormat, w.sourceFormat)`) if non-empty
- TagWSIToolsVersion (`makeASCIITag(TagWSIToolsVersion, w.toolsVersion)`) if non-empty

Helper:

```go
// formatTIFFDateTime formats t as TIFF 6.0's "YYYY:MM:DD HH:MM:SS".
func formatTIFFDateTime(t time.Time) string {
	return t.Format("2006:01:02 15:04:05")
}
```

- [ ] **Step 4: Run, verify pass**

```bash
go test ./internal/wsiwriter/ -race -count=1 -v
```

Expected: all wsiwriter tests pass including the new one.

- [ ] **Step 5: Commit**

```bash
git add internal/wsiwriter/tiff.go internal/wsiwriter/tiff_test.go
git commit -m "feat(wsiwriter): WithMake/Model/Software/DateTime/Source/Version options"
```

---

### Task C3: WSIImageType + WSILevelIndex/Count emission per IFD

**Files:**
- Modify: `internal/wsiwriter/tiff.go`

- [ ] **Step 1: Failing test**

Append to `tiff_test.go`:

```go
func TestWSIImageType_PyramidAndAssociated(t *testing.T) {
	if _, err := exec.LookPath("tiffinfo"); err != nil {
		t.Skip("tiffinfo missing")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "wsi-tags.tiff")

	w, _ := Create(path, WithBigTIFF(true))
	// Two pyramid levels; level count should land in TagWSILevelCount=2.
	for _, dims := range [][2]uint32{{16, 16}, {8, 8}} {
		l, _ := w.AddLevel(LevelSpec{
			ImageWidth: dims[0], ImageHeight: dims[1],
			TileWidth: 8, TileHeight: 8,
			Compression: CompressionNone, PhotometricInterpretation: 2,
			WSIImageType: WSIImageTypePyramid,
		})
		tx := (dims[0] + 7) / 8
		ty := (dims[1] + 7) / 8
		for y := uint32(0); y < ty; y++ {
			for x := uint32(0); x < tx; x++ {
				l.WriteTile(x, y, make([]byte, 8*8*3))
			}
		}
	}
	w.AddAssociated(AssociatedSpec{
		Kind:                      "label",
		Compressed:                make([]byte, 4*4*3),
		Width:                     4, Height: 4,
		Compression:               CompressionNone,
		PhotometricInterpretation: 2,
		NewSubfileType:            1,
		WSIImageType:              WSIImageTypeLabel,
	})
	w.Close()

	out, _ := exec.Command("tiffinfo", path).CombinedOutput()
	got := string(out)
	// tiffinfo prints private tags as "Tag 65080 ..."; check substring.
	if !strings.Contains(got, "65080") {
		t.Errorf("WSIImageType (tag 65080) not in tiffinfo output:\n%s", got)
	}
	if !strings.Contains(got, "65082") {
		t.Errorf("WSILevelCount (tag 65082) not in tiffinfo output:\n%s", got)
	}
}
```

- [ ] **Step 2: Run, verify failure**

Compile error (`LevelSpec.WSIImageType`, `AssociatedSpec.WSIImageType` don't exist).

- [ ] **Step 3: Add fields to LevelSpec and AssociatedSpec; emit at IFD build time**

In `tiff.go`, extend `LevelSpec`:

```go
type LevelSpec struct {
	// ... existing fields (ImageWidth, ImageHeight, TileWidth, TileHeight,
	// Compression, PhotometricInterpretation, JPEGTables,
	// JPEGAbbreviatedTiles, ICCProfile, ExtraTags, SamplesPerPixel,
	// NewSubfileType)

	// WSIImageType, when non-empty, emits the WSIImageType tag (65080)
	// with this value. For pyramid levels also emits WSILevelIndex (65081)
	// and WSILevelCount (65082) populated from the writer's level walk.
	WSIImageType string
}
```

Extend `AssociatedSpec` analogously (just `WSIImageType string`).

In `buildTiledTags(entry, isL0)`, when `entry.spec.WSIImageType != ""`:

```go
tags = append(tags, w.makeASCIITag(TagWSIImageType, entry.spec.WSIImageType))

// For pyramid IFDs, also emit level index + level count.
if entry.spec.WSIImageType == WSIImageTypePyramid {
	tags = append(tags, w.makeLongTag(TagWSILevelIndex, uint32(entry.pyramidLevelIndex)))
	tags = append(tags, w.makeLongTag(TagWSILevelCount, uint32(w.pyramidLevelCount)))
}
```

`entry.pyramidLevelIndex` is computed by walking `w.imgs` at Close time and counting pyramid entries (those whose `spec.WSIImageType == "pyramid"`). `w.pyramidLevelCount` is the total of that walk.

Stripped associated images (added via `AddAssociated`) need `WSIImageType` plumbed through. Update the eager-build path in `AddAssociated` to call the new helper that emits tag 65080 from `s.WSIImageType` if set.

- [ ] **Step 4: Run + iterate**

```bash
go test ./internal/wsiwriter/ -race -count=1 -v
```

Expected: PASS. Iterate on the build until tiffinfo reports both 65080 and 65082 in the output.

- [ ] **Step 5: Commit**

```bash
git add internal/wsiwriter/tiff.go internal/wsiwriter/tiff_test.go
git commit -m "feat(wsiwriter): WSIImageType + WSILevelIndex/Count emission"
```

---

## Batch D — jpegli codec wrapper (1 task)

### Task D1: jpegli codec wrapper (libjxl)

**Files:**
- Create: `internal/codec/jpegli/jpegli.go`
- Create: `internal/codec/jpegli/jpegli_test.go`

The structure mirrors `internal/codec/jpeg/jpeg.go` (v0.1) almost exactly. jpegli's libjpeg-API-compatible header (`<jpegli/encode.h>`) means we can copy the existing `wsi_encode` C helper, swap function name prefixes, and adjust the cgo pkg-config.

- [ ] **Step 1: Verify libjxl is installed locally and exposes jpegli**

```bash
brew install jpeg-xl
ls /opt/homebrew/include/jpegli/encode.h
pkg-config --cflags --libs libjxl_threads libjxl
```

Expected: header exists at the path; pkg-config returns -I/-L flags. If not, `brew install jpeg-xl` again, then escalate as BLOCKED if still missing.

- [ ] **Step 2: Failing test**

Create `internal/codec/jpegli/jpegli_test.go`:

```go
//go:build !nojpegli

package jpegli

import (
	"bytes"
	"testing"

	"github.com/cornish/wsi-tools/internal/codec"
	"github.com/cornish/wsi-tools/internal/decoder"
)

func TestJpegliEncoderRoundTrip(t *testing.T) {
	rgb := make([]byte, 256*256*3)
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			off := (y*256 + x) * 3
			rgb[off+0] = byte(x)
			rgb[off+1] = byte(y)
			rgb[off+2] = 128
		}
	}

	enc, err := (Factory{}).NewEncoder(codec.LevelGeometry{
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
		t.Errorf("TIFFCompressionTag: got %d, want 7 (jpegli output is standard JPEG)", enc.TIFFCompressionTag())
	}

	tables := enc.LevelHeader()
	if len(tables) == 0 {
		t.Fatal("LevelHeader: empty")
	}
	// Splice tables + tile, decode via libjpeg-turbo (which honors APP14 like our jpeg codec).
	spliced := spliceForDecode(tables, tile)
	if spliced == nil {
		t.Fatal("splice produced nil")
	}
	out, err := decoder.NewJPEG().DecodeTile(spliced, nil, 1, 1)
	if err != nil {
		t.Fatalf("DecodeTile: %v", err)
	}
	if got, want := len(out), 256*256*3; got != want {
		t.Errorf("decoded length: got %d, want %d", got, want)
	}
	off := (20*256 + 10) * 3
	gotR, gotG, gotB := int(out[off]), int(out[off+1]), int(out[off+2])
	if abs(gotR-10) > 8 || abs(gotG-20) > 8 || abs(gotB-128) > 8 {
		t.Errorf("pixel (10,20) drift: got R=%d G=%d B=%d", gotR, gotG, gotB)
	}
}

func spliceForDecode(tables, tile []byte) []byte {
	if !bytes.HasSuffix(tables, []byte{0xFF, 0xD9}) || !bytes.HasPrefix(tile, []byte{0xFF, 0xD8}) {
		return nil
	}
	out := make([]byte, 0, len(tables)+len(tile)-4)
	out = append(out, tables[:len(tables)-2]...)
	out = append(out, tile[2:]...)
	return out
}

func abs(x int) int { if x < 0 { return -x }; return x }
```

- [ ] **Step 3: Run, verify compile failure**

```bash
go test ./internal/codec/jpegli/ -count=1
```

Expected: compile error.

- [ ] **Step 4: Implement jpegli.go**

Take `internal/codec/jpeg/jpeg.go` as a template; copy its full content into `internal/codec/jpegli/jpegli.go`, then:

1. Change package: `package jpegli`.
2. Add the build-tag header: `//go:build !nojpegli` — at the top of the file BEFORE the package declaration (the cgo block must be inside the build tag's effect).
3. Update cgo pkg-config: `#cgo pkg-config: libjxl_threads libjxl`.
4. Update `#include <jpeglib.h>` → `#include <jpegli/encode.h>` and `#include <jpegli/decode.h>` if needed (jpegli ships its own headers).
5. Update the C helper function name (`wsi_encode` → `wsi_jpegli_encode`) and all `jpeg_*` C calls → `jpegli_*` (e.g., `jpeg_create_compress` → `jpegli_create_compress`, `jpeg_set_defaults` → `jpegli_set_defaults`, `jpeg_set_colorspace` → `jpegli_set_colorspace`, `jpeg_set_quality` → `jpegli_set_quality`, `jpeg_suppress_tables` → `jpegli_suppress_tables`, `jpeg_start_compress` → `jpegli_start_compress`, `jpeg_write_marker` → `jpegli_write_marker`, `jpeg_write_scanlines` → `jpegli_write_scanlines`, `jpeg_finish_compress` → `jpegli_finish_compress`, `jpeg_destroy_compress` → `jpegli_destroy_compress`, `jpeg_std_error` → `jpegli_std_error`, `jpeg_mem_dest` → `jpegli_mem_dest`).
6. **Keep** the JCS_RGB colorspace override (matches our v0.1 fix); jpegli's enum values are the same as libjpeg's.
7. **Keep** the Adobe APP14 marker write — jpegli supports the same marker mechanism.
8. Update `Factory.Name()` to return `"jpegli"`.
9. Optional: read `Quality.Knobs["distance"]` for an alternative Butteraugli-distance quality knob (jpegli supports `jpegli_set_distance(&cinfo, distance)` as an alternative to `jpegli_set_quality`).

If jpegli's C API surface differs from libjpeg's in some way that breaks the copy-paste pattern, read `/opt/homebrew/include/jpegli/encode.h` and adjust. Document any surprises in a comment.

The Encoder struct's `TIFFCompressionTag()` returns 7 — output IS standard JPEG. opentile-go reads jpegli output as JPEG with no special handling.

- [ ] **Step 5: Run, verify pass**

```bash
go test ./internal/codec/jpegli/ -race -count=1 -v
```

Expected: PASS. Pixel drift within ±8 per channel at q=85.

- [ ] **Step 6: Commit**

```bash
git add internal/codec/jpegli/
git commit -m "feat(codec/jpegli): libjxl/jpegli encoder (libjpeg-API-compatible)"
```

---

## Batch E — JPEG-XL codec wrapper (1 task)

### Task E1: JPEG-XL codec wrapper (libjxl JxlEncoder API)

**Files:**
- Create: `internal/codec/jpegxl/jpegxl.go`
- Create: `internal/codec/jpegxl/jpegxl_test.go`

JPEG-XL's encoder is more elaborate than libjpeg's — it's a true JPEG-XL codestream, not a JPEG variant. libjxl's `JxlEncoder` API has a setup/configure/add-frame/flush/close lifecycle.

- [ ] **Step 1: Verify libjxl exposes the JxlEncoder API**

```bash
ls /opt/homebrew/include/jxl/encode.h
grep -E "JxlEncoderCreate|JxlEncoderAddImageFrame|JxlEncoderFinish" /opt/homebrew/include/jxl/encode.h
```

Expected: file exists; the three symbol names appear.

- [ ] **Step 2: Failing test**

Create `internal/codec/jpegxl/jpegxl_test.go`:

```go
//go:build !nojxl

package jpegxl

import (
	"testing"

	"github.com/cornish/wsi-tools/internal/codec"
)

func TestJPEGXLEncoderRoundTrip(t *testing.T) {
	rgb := make([]byte, 256*256*3)
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			off := (y*256 + x) * 3
			rgb[off+0] = byte(x)
			rgb[off+1] = byte(y)
			rgb[off+2] = 128
		}
	}

	enc, err := (Factory{}).NewEncoder(codec.LevelGeometry{
		TileWidth: 256, TileHeight: 256,
		PixelFormat: codec.PixelFormatRGB8,
	}, codec.Quality{Knobs: map[string]string{"q": "85"}})
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	defer enc.Close()

	tile, err := enc.EncodeTile(rgb, 256, 256, nil)
	if err != nil {
		t.Fatalf("EncodeTile: %v", err)
	}
	if len(tile) == 0 {
		t.Fatal("encoded tile is empty")
	}
	if enc.TIFFCompressionTag() != 50002 {
		t.Errorf("TIFFCompressionTag: got %d, want 50002 (Adobe-allocated draft for JPEG-XL)", enc.TIFFCompressionTag())
	}

	// Decode round-trip via JxlDecoder. For v0.2.0 we don't have a JPEG-XL
	// decoder shipped (decoder package only handles JPEG + JP2K source bytes).
	// So this test asserts only structural validity: produces non-empty
	// codestream with the JPEG-XL signature.
	// JPEG-XL codestream signature: 0xFF 0x0A (or 0x00 0x00 0x00 0x0C ... 'J' 'X' 'L' ' ' for boxed).
	if len(tile) < 2 {
		t.Fatal("tile too short for signature check")
	}
	hasJxlSignature := (tile[0] == 0xFF && tile[1] == 0x0A) ||
		(len(tile) >= 12 && string(tile[4:8]) == "JXL ")
	if !hasJxlSignature {
		t.Errorf("tile bytes don't start with a recognised JPEG-XL signature: % X", tile[:min(16, len(tile))])
	}
}

func min(a, b int) int { if a < b { return a }; return b }
```

(Why no PSNR round-trip: v0.2.0 doesn't ship a JPEG-XL decoder. Encoder structural validity is all we can unit-test cheaply. Layer-2 integration tests verify visual fidelity by re-opening through opentile-go's generic-TIFF reader and decoding via libjxl during a separate integration test that pulls in a libjxl-decode helper.)

- [ ] **Step 3: Run, verify failure**

Compile error.

- [ ] **Step 4: Implement jpegxl.go**

```go
//go:build !nojxl

// Package jpegxl provides a libjxl-backed JPEG-XL encoder, registered with
// internal/codec under the name "jpegxl". One frame per tile; output is a
// JPEG-XL codestream (TIFF Compression=50002).
package jpegxl

/*
#cgo pkg-config: libjxl_threads libjxl
#include <stdlib.h>
#include <string.h>
#include <jxl/encode.h>
#include <jxl/thread_parallel_runner.h>
#include <jxl/types.h>

// wsi_jxl_encode encodes a single RGB888 frame as a JPEG-XL codestream.
// On success, *outbuf is a malloc'd buffer of *outsize bytes; caller frees.
// distance is the JPEG-XL distance parameter (1.0 = visually lossless;
// higher = more compression). 0.0 = lossless.
// Returns 0 on success, -1 on error.
static int wsi_jxl_encode(
    const unsigned char *rgb, int width, int height,
    float distance, int effort,
    unsigned char **outbuf, size_t *outsize)
{
    JxlEncoder *enc = JxlEncoderCreate(NULL);
    if (!enc) return -1;

    void *runner = JxlThreadParallelRunnerCreate(NULL,
        JxlThreadParallelRunnerDefaultNumWorkerThreads());
    if (JxlEncoderSetParallelRunner(enc, JxlThreadParallelRunner, runner) != JXL_ENC_SUCCESS) {
        JxlThreadParallelRunnerDestroy(runner);
        JxlEncoderDestroy(enc);
        return -1;
    }

    JxlBasicInfo info;
    JxlEncoderInitBasicInfo(&info);
    info.xsize = width;
    info.ysize = height;
    info.bits_per_sample = 8;
    info.num_color_channels = 3;
    info.alpha_bits = 0;
    info.uses_original_profile = JXL_FALSE; // allow XYB encoding for better quality

    if (JxlEncoderSetBasicInfo(enc, &info) != JXL_ENC_SUCCESS) goto fail;

    JxlColorEncoding color;
    JxlColorEncodingSetToSRGB(&color, JXL_FALSE);
    if (JxlEncoderSetColorEncoding(enc, &color) != JXL_ENC_SUCCESS) goto fail;

    JxlEncoderFrameSettings *frame_settings = JxlEncoderFrameSettingsCreate(enc, NULL);
    if (!frame_settings) goto fail;
    if (JxlEncoderSetFrameDistance(frame_settings, distance) != JXL_ENC_SUCCESS) goto fail;
    if (JxlEncoderFrameSettingsSetOption(frame_settings, JXL_ENC_FRAME_SETTING_EFFORT, effort) != JXL_ENC_SUCCESS) goto fail;
    if (distance == 0.0f) {
        if (JxlEncoderSetFrameLossless(frame_settings, JXL_TRUE) != JXL_ENC_SUCCESS) goto fail;
    }

    JxlPixelFormat pixel_format = {
        .num_channels = 3,
        .data_type = JXL_TYPE_UINT8,
        .endianness = JXL_NATIVE_ENDIAN,
        .align = 0,
    };

    if (JxlEncoderAddImageFrame(frame_settings, &pixel_format, rgb,
            (size_t)width * height * 3) != JXL_ENC_SUCCESS) goto fail;

    JxlEncoderCloseInput(enc);

    // Iteratively pull bytes from JxlEncoderProcessOutput.
    size_t cap = 65536;
    *outbuf = (unsigned char *)malloc(cap);
    if (!*outbuf) goto fail;
    *outsize = 0;

    for (;;) {
        unsigned char *next_out = *outbuf + *outsize;
        size_t avail_out = cap - *outsize;
        JxlEncoderStatus s = JxlEncoderProcessOutput(enc, &next_out, &avail_out);
        size_t written = (cap - *outsize) - avail_out;
        *outsize += written;
        if (s == JXL_ENC_SUCCESS) break;
        if (s != JXL_ENC_NEED_MORE_OUTPUT) goto fail;
        // Grow buffer.
        cap *= 2;
        unsigned char *grown = (unsigned char *)realloc(*outbuf, cap);
        if (!grown) goto fail;
        *outbuf = grown;
    }

    JxlThreadParallelRunnerDestroy(runner);
    JxlEncoderDestroy(enc);
    return 0;

fail:
    if (*outbuf) { free(*outbuf); *outbuf = NULL; *outsize = 0; }
    JxlThreadParallelRunnerDestroy(runner);
    JxlEncoderDestroy(enc);
    return -1;
}
*/
import "C"
import (
	"fmt"
	"runtime"
	"strconv"
	"unsafe"

	"github.com/cornish/wsi-tools/internal/codec"
	"github.com/cornish/wsi-tools/internal/wsiwriter"
)

func init() {
	codec.Register(Factory{})
}

type Factory struct{}

func (Factory) Name() string { return "jpegxl" }

func (Factory) NewEncoder(g codec.LevelGeometry, q codec.Quality) (codec.Encoder, error) {
	// Quality knob mapping: --quality 1..100 → distance.
	// libjxl recommends distance ≈ 1.0 for "visually lossless"; quality 100 maps
	// to distance 0.0 (true lossless), quality 90 → ~1.0, quality 50 → ~3.0.
	// Formula: distance = 15 * (1 - quality/100) ^ 1.5 (approximate, per libjxl docs).
	distance := float32(1.0)
	if v, ok := q.Knobs["q"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 100 {
			if n >= 100 {
				distance = 0.0
			} else {
				// Simple linear-ish mapping good enough for v0.2.0.
				// quality 100 → 0.0, quality 1 → 15.0.
				distance = float32(15.0 * (1.0 - float32(n)/100.0))
			}
		}
	}
	if v, ok := q.Knobs["distance"]; ok {
		if d, err := strconv.ParseFloat(v, 32); err == nil {
			distance = float32(d)
		}
	}
	effort := 7
	if v, ok := q.Knobs["effort"]; ok {
		if e, err := strconv.Atoi(v); err == nil && e >= 1 && e <= 9 {
			effort = e
		}
	}
	return &Encoder{distance: distance, effort: effort}, nil
}

type Encoder struct {
	distance float32
	effort   int
}

func (*Encoder) LevelHeader() []byte                  { return nil }
func (*Encoder) TIFFCompressionTag() uint16           { return wsiwriter.CompressionJPEGXL }
func (*Encoder) ExtraTIFFTags() []wsiwriter.TIFFTag   { return nil }
func (*Encoder) Close() error                         { return nil }

func (e *Encoder) EncodeTile(rgb []byte, w, h int, dst []byte) ([]byte, error) {
	var outBuf *C.uchar
	var outSize C.size_t

	rc := C.wsi_jxl_encode(
		(*C.uchar)(unsafe.Pointer(&rgb[0])),
		C.int(w), C.int(h),
		C.float(e.distance), C.int(e.effort),
		&outBuf, &outSize,
	)
	runtime.KeepAlive(rgb)

	if rc != 0 {
		return nil, fmt.Errorf("codec/jpegxl: wsi_jxl_encode returned %d", rc)
	}
	if outBuf == nil {
		return nil, fmt.Errorf("codec/jpegxl: nil output")
	}
	out := C.GoBytes(unsafe.Pointer(outBuf), C.int(outSize))
	C.free(unsafe.Pointer(outBuf))
	if dst != nil && cap(dst) >= len(out) {
		dst = dst[:len(out)]
		copy(dst, out)
		return dst, nil
	}
	return out, nil
}
```

- [ ] **Step 5: Run, verify pass**

```bash
go test ./internal/codec/jpegxl/ -race -count=1 -v
```

Expected: PASS — encoded bytes have the JPEG-XL signature.

- [ ] **Step 6: Commit**

```bash
git add internal/codec/jpegxl/
git commit -m "feat(codec/jpegxl): libjxl encoder (one frame per tile)"
```

---

## Batch F — AVIF codec wrapper (1 task)

### Task F1: AVIF codec wrapper (libavif)

**Files:**
- Create: `internal/codec/avif/avif.go`
- Create: `internal/codec/avif/avif_test.go`

- [ ] **Step 1: Verify libavif is installed**

```bash
brew install libavif
ls /opt/homebrew/include/avif/avif.h
pkg-config --cflags --libs libavif
```

- [ ] **Step 2: Failing test**

```go
//go:build !noavif

package avif

import (
	"testing"

	"github.com/cornish/wsi-tools/internal/codec"
)

func TestAVIFEncoderRoundTrip(t *testing.T) {
	rgb := make([]byte, 256*256*3)
	for i := range rgb {
		rgb[i] = byte(i)
	}

	enc, err := (Factory{}).NewEncoder(codec.LevelGeometry{
		TileWidth: 256, TileHeight: 256,
		PixelFormat: codec.PixelFormatRGB8,
	}, codec.Quality{Knobs: map[string]string{"q": "85"}})
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	defer enc.Close()

	tile, err := enc.EncodeTile(rgb, 256, 256, nil)
	if err != nil {
		t.Fatalf("EncodeTile: %v", err)
	}
	if enc.TIFFCompressionTag() != 60001 {
		t.Errorf("TIFFCompressionTag: got %d, want 60001", enc.TIFFCompressionTag())
	}
	// AVIF starts with 'ftyp' box at bytes 4-7.
	if len(tile) < 12 || string(tile[4:8]) != "ftyp" {
		t.Errorf("not a valid AVIF (no ftyp box): % X", tile[:min(16, len(tile))])
	}
}

func min(a, b int) int { if a < b { return a }; return b }
```

- [ ] **Step 3: Run, verify failure**

Compile error.

- [ ] **Step 4: Implement avif.go**

```go
//go:build !noavif

// Package avif provides a libavif-backed AVIF encoder, registered with
// internal/codec under the name "avif". One AVIF still per tile; output
// uses TIFF Compression=60001 (wsi-tools-private).
package avif

/*
#cgo pkg-config: libavif
#include <stdlib.h>
#include <string.h>
#include <avif/avif.h>

// wsi_avif_encode encodes width*height RGB888 pixels as one AVIF still.
// On success, *outbuf is malloc'd; caller frees.
// quality is 1..100 (libavif's quality knob, mapped to QP internally).
// speed is 0..10 (0 = slowest/best, 10 = fastest/worst).
// Returns 0 on success, -1 on error.
static int wsi_avif_encode(
    const unsigned char *rgb, int width, int height,
    int quality, int speed,
    unsigned char **outbuf, size_t *outsize)
{
    avifResult r;
    avifImage *image = avifImageCreate(width, height, 8, AVIF_PIXEL_FORMAT_YUV444);
    if (!image) return -1;

    avifRGBImage rgb_img;
    avifRGBImageSetDefaults(&rgb_img, image);
    rgb_img.format = AVIF_RGB_FORMAT_RGB;
    rgb_img.depth = 8;
    rgb_img.pixels = (uint8_t *)rgb;
    rgb_img.rowBytes = (uint32_t)(width * 3);

    r = avifImageRGBToYUV(image, &rgb_img);
    if (r != AVIF_RESULT_OK) {
        avifImageDestroy(image);
        return -1;
    }

    avifEncoder *encoder = avifEncoderCreate();
    if (!encoder) {
        avifImageDestroy(image);
        return -1;
    }
    encoder->quality = quality;
    encoder->qualityAlpha = quality;
    encoder->speed = speed;

    avifRWData output = AVIF_DATA_EMPTY;
    r = avifEncoderWrite(encoder, image, &output);
    avifEncoderDestroy(encoder);
    avifImageDestroy(image);

    if (r != AVIF_RESULT_OK) return -1;

    *outbuf = (unsigned char *)malloc(output.size);
    if (!*outbuf) {
        avifRWDataFree(&output);
        return -1;
    }
    memcpy(*outbuf, output.data, output.size);
    *outsize = output.size;
    avifRWDataFree(&output);
    return 0;
}
*/
import "C"
import (
	"fmt"
	"runtime"
	"strconv"
	"unsafe"

	"github.com/cornish/wsi-tools/internal/codec"
	"github.com/cornish/wsi-tools/internal/wsiwriter"
)

func init() {
	codec.Register(Factory{})
}

type Factory struct{}

func (Factory) Name() string { return "avif" }

func (Factory) NewEncoder(g codec.LevelGeometry, q codec.Quality) (codec.Encoder, error) {
	quality := 85
	if v, ok := q.Knobs["q"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 100 {
			quality = n
		}
	}
	speed := 6
	if v, ok := q.Knobs["speed"]; ok {
		if s, err := strconv.Atoi(v); err == nil && s >= 0 && s <= 10 {
			speed = s
		}
	}
	return &Encoder{quality: quality, speed: speed}, nil
}

type Encoder struct {
	quality int
	speed   int
}

func (*Encoder) LevelHeader() []byte                { return nil }
func (*Encoder) TIFFCompressionTag() uint16         { return wsiwriter.CompressionAVIF }
func (*Encoder) ExtraTIFFTags() []wsiwriter.TIFFTag { return nil }
func (*Encoder) Close() error                       { return nil }

func (e *Encoder) EncodeTile(rgb []byte, w, h int, dst []byte) ([]byte, error) {
	var outBuf *C.uchar
	var outSize C.size_t

	rc := C.wsi_avif_encode(
		(*C.uchar)(unsafe.Pointer(&rgb[0])),
		C.int(w), C.int(h),
		C.int(e.quality), C.int(e.speed),
		&outBuf, &outSize,
	)
	runtime.KeepAlive(rgb)

	if rc != 0 {
		return nil, fmt.Errorf("codec/avif: wsi_avif_encode returned %d", rc)
	}
	if outBuf == nil {
		return nil, fmt.Errorf("codec/avif: nil output")
	}
	out := C.GoBytes(unsafe.Pointer(outBuf), C.int(outSize))
	C.free(unsafe.Pointer(outBuf))
	if dst != nil && cap(dst) >= len(out) {
		dst = dst[:len(out)]
		copy(dst, out)
		return dst, nil
	}
	return out, nil
}
```

- [ ] **Step 5: Run, verify pass + commit**

```bash
go test ./internal/codec/avif/ -race -count=1 -v
git add internal/codec/avif/
git commit -m "feat(codec/avif): libavif encoder (one still per tile)"
```

---

## Batch G — WebP codec wrapper (1 task)

### Task G1: WebP codec wrapper (libwebp)

**Files:**
- Create: `internal/codec/webp/webp.go`
- Create: `internal/codec/webp/webp_test.go`

WebP has the smallest cgo surface — `WebPEncodeRGB` is a single call.

- [ ] **Step 1: Verify libwebp**

```bash
brew install webp
pkg-config --cflags --libs libwebp
```

- [ ] **Step 2: Failing test**

```go
//go:build !nowebp

package webp

import (
	"testing"

	"github.com/cornish/wsi-tools/internal/codec"
)

func TestWebPEncoderRoundTrip(t *testing.T) {
	rgb := make([]byte, 256*256*3)
	for i := range rgb {
		rgb[i] = byte(i)
	}

	enc, err := (Factory{}).NewEncoder(codec.LevelGeometry{
		TileWidth: 256, TileHeight: 256,
		PixelFormat: codec.PixelFormatRGB8,
	}, codec.Quality{Knobs: map[string]string{"q": "85"}})
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	defer enc.Close()

	tile, err := enc.EncodeTile(rgb, 256, 256, nil)
	if err != nil {
		t.Fatalf("EncodeTile: %v", err)
	}
	if enc.TIFFCompressionTag() != 50001 {
		t.Errorf("TIFFCompressionTag: got %d, want 50001", enc.TIFFCompressionTag())
	}
	// WebP RIFF signature: bytes 0-3 = "RIFF", bytes 8-11 = "WEBP".
	if len(tile) < 12 || string(tile[0:4]) != "RIFF" || string(tile[8:12]) != "WEBP" {
		t.Errorf("not a valid WebP RIFF: % X", tile[:min(16, len(tile))])
	}
}

func min(a, b int) int { if a < b { return a }; return b }
```

- [ ] **Step 3: Run, verify failure**

- [ ] **Step 4: Implement webp.go**

```go
//go:build !nowebp

// Package webp provides a libwebp-backed WebP encoder.
package webp

/*
#cgo pkg-config: libwebp
#include <stdlib.h>
#include <webp/encode.h>

// wsi_webp_encode encodes RGB888 pixels as WebP.
// quality 1..100 maps to WebPEncodeRGB's quality_factor (lossy mode).
// If lossless != 0, uses WebPEncodeLosslessRGB instead and ignores quality.
// On success, *outbuf is allocated by libwebp; caller calls WebPFree(*outbuf).
// We malloc a copy for the Go side and WebPFree the libwebp buffer.
static int wsi_webp_encode(
    const unsigned char *rgb, int width, int height,
    int quality, int lossless,
    unsigned char **outbuf, size_t *outsize)
{
    uint8_t *libwebp_out = NULL;
    size_t out_size = 0;
    if (lossless) {
        out_size = WebPEncodeLosslessRGB(rgb, width, height, width * 3, &libwebp_out);
    } else {
        out_size = WebPEncodeRGB(rgb, width, height, width * 3, (float)quality, &libwebp_out);
    }
    if (out_size == 0 || libwebp_out == NULL) return -1;
    *outbuf = libwebp_out; // caller must WebPFree
    *outsize = out_size;
    return 0;
}
*/
import "C"
import (
	"fmt"
	"runtime"
	"strconv"
	"unsafe"

	"github.com/cornish/wsi-tools/internal/codec"
	"github.com/cornish/wsi-tools/internal/wsiwriter"
)

func init() {
	codec.Register(Factory{})
}

type Factory struct{}

func (Factory) Name() string { return "webp" }

func (Factory) NewEncoder(g codec.LevelGeometry, q codec.Quality) (codec.Encoder, error) {
	quality := 85
	if v, ok := q.Knobs["q"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 100 {
			quality = n
		}
	}
	lossless := false
	if v, ok := q.Knobs["lossless"]; ok && v == "true" {
		lossless = true
	}
	return &Encoder{quality: quality, lossless: lossless}, nil
}

type Encoder struct {
	quality  int
	lossless bool
}

func (*Encoder) LevelHeader() []byte                { return nil }
func (*Encoder) TIFFCompressionTag() uint16         { return wsiwriter.CompressionWebP }
func (*Encoder) ExtraTIFFTags() []wsiwriter.TIFFTag { return nil }
func (*Encoder) Close() error                       { return nil }

func (e *Encoder) EncodeTile(rgb []byte, w, h int, dst []byte) ([]byte, error) {
	var outBuf *C.uchar
	var outSize C.size_t
	losslessFlag := 0
	if e.lossless {
		losslessFlag = 1
	}
	rc := C.wsi_webp_encode(
		(*C.uchar)(unsafe.Pointer(&rgb[0])),
		C.int(w), C.int(h),
		C.int(e.quality), C.int(losslessFlag),
		&outBuf, &outSize,
	)
	runtime.KeepAlive(rgb)
	if rc != 0 || outBuf == nil {
		return nil, fmt.Errorf("codec/webp: encode failed (rc=%d)", rc)
	}
	out := C.GoBytes(unsafe.Pointer(outBuf), C.int(outSize))
	C.WebPFree(unsafe.Pointer(outBuf))
	if dst != nil && cap(dst) >= len(out) {
		dst = dst[:len(out)]
		copy(dst, out)
		return dst, nil
	}
	return out, nil
}
```

- [ ] **Step 5: Run, verify pass + commit**

```bash
go test ./internal/codec/webp/ -race -count=1 -v
git add internal/codec/webp/
git commit -m "feat(codec/webp): libwebp encoder"
```

---

## Batch H — HTJ2K codec wrapper (1 task)

### Task H1: HTJ2K codec wrapper (OpenJPH)

**Files:**
- Create: `internal/codec/htj2k/htj2k.go`
- Create: `internal/codec/htj2k/htj2k_test.go`

OpenJPH is C++ with no Homebrew pkg-config file. We use explicit cgo flags.

- [ ] **Step 1: Verify OpenJPH installed and find headers**

```bash
brew install openjph
ls /opt/homebrew/include/openjph/ojph_*.h
ls /opt/homebrew/lib/libopenjph.*
```

Expected: header files like `ojph_arch.h`, `ojph_codestream.h`, `ojph_file.h`, `ojph_mem.h`, `ojph_params.h`, etc. Library `libopenjph.dylib`.

If OpenJPH isn't installed, escalate as BLOCKED — there's no apt/brew workaround, would need source build.

- [ ] **Step 2: Failing test**

```go
//go:build !nohtj2k

package htj2k

import (
	"testing"

	"github.com/cornish/wsi-tools/internal/codec"
)

func TestHTJ2KEncoderRoundTrip(t *testing.T) {
	rgb := make([]byte, 256*256*3)
	for i := range rgb {
		rgb[i] = byte(i)
	}
	enc, err := (Factory{}).NewEncoder(codec.LevelGeometry{
		TileWidth: 256, TileHeight: 256,
		PixelFormat: codec.PixelFormatRGB8,
	}, codec.Quality{Knobs: map[string]string{"q": "85"}})
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	defer enc.Close()
	tile, err := enc.EncodeTile(rgb, 256, 256, nil)
	if err != nil {
		t.Fatalf("EncodeTile: %v", err)
	}
	if enc.TIFFCompressionTag() != 60003 {
		t.Errorf("TIFFCompressionTag: got %d, want 60003", enc.TIFFCompressionTag())
	}
	// J2K codestream signature: SOC marker = 0xFF 0x4F.
	if len(tile) < 2 || tile[0] != 0xFF || tile[1] != 0x4F {
		t.Errorf("not a J2K codestream (no SOC marker): % X", tile[:min(8, len(tile))])
	}
}

func min(a, b int) int { if a < b { return a }; return b }
```

- [ ] **Step 3: Run, verify failure**

- [ ] **Step 4: Implement htj2k.go (with C++ shim)**

OpenJPH's API is C++. We wrap it in an `extern "C"` shim file. Two approaches:

(a) `htj2k.go` declares the cgo block calling C functions; we put the C++ implementation in a sibling `htj2k_shim.cpp`.

(b) Put the C++ in the cgo preamble using `#cgo CXXFLAGS:` — works if Go's cgo can compile inline C++.

Go's cgo supports C++ via `.cpp`/`.cc` files in the same package. Use approach (a).

Create `internal/codec/htj2k/shim.cpp`:

```cpp
// shim.cpp — C wrappers around OpenJPH for cgo.

#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include <ojph_arch.h>
#include <ojph_mem.h>
#include <ojph_params.h>
#include <ojph_codestream.h>
#include <ojph_file.h>

extern "C" {

// wsi_htj2k_encode encodes RGB888 (3 components, 8-bit each) as a
// HTJ2K J2K codestream (no JP2 boxing).
// On success, *outbuf is malloc'd; caller frees.
// quality is 1..100; mapped to a quantization step internally.
// Returns 0 on success, -1 on error.
int wsi_htj2k_encode(
    const unsigned char *rgb, int width, int height,
    int quality,
    unsigned char **outbuf, size_t *outsize)
{
    using namespace ojph;

    try {
        codestream cs;
        param_siz siz = cs.access_siz();
        siz.set_image_extent(point(width, height));
        siz.set_num_components(3);
        for (uint32_t c = 0; c < 3; ++c) {
            siz.set_component(c, point(1, 1), 8, false);
        }
        siz.set_image_offset(point(0, 0));
        siz.set_tile_size(size(width, height));
        siz.set_tile_offset(point(0, 0));

        param_cod cod = cs.access_cod();
        cod.set_num_decomposition(5);
        cod.set_block_dims(64, 64);
        cod.set_progression_order("RPCL");
        cod.set_color_transform(true);
        cod.set_reversible(quality >= 100);  // lossless when quality=100

        if (quality < 100) {
            // Quantization step: simple linear-ish from quality.
            // qstep = 0.001 * (101 - quality) / 100  (small qstep = high quality)
            float qstep = 0.001f * (float)(101 - quality) / 100.0f;
            cs.access_qcd().set_irrev_quant(qstep);
        }

        cs.set_planar(false);

        mem_outfile out;
        out.open();
        cs.write_headers(&out);

        auto *next_line = cs.exchange(NULL, 0);
        for (int y = 0; y < height; ++y) {
            for (int c = 0; c < 3; ++c) {
                int32_t *target = next_line->i32;
                const unsigned char *src = rgb + (y * width * 3) + c;
                for (int x = 0; x < width; ++x) {
                    target[x] = (int32_t)src[x * 3];
                }
                int next_comp = c;
                next_line = cs.exchange(next_line, next_comp);
            }
        }

        cs.flush();
        cs.close();

        size_t size = out.tell();
        *outbuf = (unsigned char *)malloc(size);
        if (!*outbuf) return -1;
        memcpy(*outbuf, out.get_data(), size);
        *outsize = size;
        return 0;
    } catch (...) {
        return -1;
    }
}

}
```

Create `internal/codec/htj2k/htj2k.go`:

```go
//go:build !nohtj2k

// Package htj2k provides an OpenJPH-backed High-Throughput JPEG 2000 encoder.
package htj2k

/*
#cgo CXXFLAGS: -I/opt/homebrew/include/openjph -std=c++17
#cgo LDFLAGS: -L/opt/homebrew/lib -lopenjph -lstdc++
#include <stdlib.h>

extern int wsi_htj2k_encode(
    const unsigned char *rgb, int width, int height,
    int quality,
    unsigned char **outbuf, size_t *outsize);
*/
import "C"
import (
	"fmt"
	"runtime"
	"strconv"
	"unsafe"

	"github.com/cornish/wsi-tools/internal/codec"
	"github.com/cornish/wsi-tools/internal/wsiwriter"
)

func init() {
	codec.Register(Factory{})
}

type Factory struct{}

func (Factory) Name() string { return "htj2k" }

func (Factory) NewEncoder(g codec.LevelGeometry, q codec.Quality) (codec.Encoder, error) {
	quality := 85
	if v, ok := q.Knobs["q"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 100 {
			quality = n
		}
	}
	return &Encoder{quality: quality}, nil
}

type Encoder struct {
	quality int
}

func (*Encoder) LevelHeader() []byte                { return nil }
func (*Encoder) TIFFCompressionTag() uint16         { return wsiwriter.CompressionHTJ2K }
func (*Encoder) ExtraTIFFTags() []wsiwriter.TIFFTag { return nil }
func (*Encoder) Close() error                       { return nil }

func (e *Encoder) EncodeTile(rgb []byte, w, h int, dst []byte) ([]byte, error) {
	var outBuf *C.uchar
	var outSize C.size_t
	rc := C.wsi_htj2k_encode(
		(*C.uchar)(unsafe.Pointer(&rgb[0])),
		C.int(w), C.int(h),
		C.int(e.quality),
		&outBuf, &outSize,
	)
	runtime.KeepAlive(rgb)
	if rc != 0 || outBuf == nil {
		return nil, fmt.Errorf("codec/htj2k: wsi_htj2k_encode failed (rc=%d)", rc)
	}
	out := C.GoBytes(unsafe.Pointer(outBuf), C.int(outSize))
	C.free(unsafe.Pointer(outBuf))
	if dst != nil && cap(dst) >= len(out) {
		dst = dst[:len(out)]
		copy(dst, out)
		return dst, nil
	}
	return out, nil
}
```

- [ ] **Step 5: Run, verify pass**

```bash
go test ./internal/codec/htj2k/ -race -count=1 -v
```

Expected: PASS — output starts with 0xFF 0x4F (J2K SOC). Iterate on the C++ shim if compilation fails or output isn't a valid codestream. Common causes:
- OpenJPH header layout differs between versions; check `ls /opt/homebrew/include/openjph/`.
- The `cs.exchange()` line-by-line interleaved-component pattern might need adjustment based on OpenJPH's expected planar/interleaved input. Read OpenJPH's `ojph_compress` example in its source tree to compare.

If the line-by-line exchange pattern doesn't compile, alternative: build a single `line_buf*` per-component in a planar pre-pass, then call `cs.write_planar()`. Reference OpenJPH's `tests/test_executables.cpp`.

- [ ] **Step 6: Commit**

```bash
git add internal/codec/htj2k/
git commit -m "feat(codec/htj2k): OpenJPH encoder (HTJ2K codestream per tile)"
```

---

## Batch I — transcode CLI wiring (2 tasks)

### Task I1: transcode subcommand + buildPipeline

**Files:**
- Create: `cmd/wsi-tools/transcode.go`
- Modify: `internal/codec/all/all.go`

- [ ] **Step 1: Update internal/codec/all/all.go**

Replace the existing content with:

```go
// Package all exists solely to import every codec subpackage so they register
// themselves with the codec registry on import. Application binaries
// (cmd/wsi-tools) blank-import this package once.
package all

import (
	_ "github.com/cornish/wsi-tools/internal/codec/avif"
	_ "github.com/cornish/wsi-tools/internal/codec/htj2k"
	_ "github.com/cornish/wsi-tools/internal/codec/jpeg"
	_ "github.com/cornish/wsi-tools/internal/codec/jpegli"
	_ "github.com/cornish/wsi-tools/internal/codec/jpegxl"
	_ "github.com/cornish/wsi-tools/internal/codec/webp"
)
```

- [ ] **Step 2: Implement cmd/wsi-tools/transcode.go**

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"time"

	opentile "github.com/cornish/opentile-go"
	"github.com/spf13/cobra"

	codec "github.com/cornish/wsi-tools/internal/codec"
	"github.com/cornish/wsi-tools/internal/decoder"
	"github.com/cornish/wsi-tools/internal/pipeline"
	"github.com/cornish/wsi-tools/internal/source"
	"github.com/cornish/wsi-tools/internal/wsiwriter"
)

var (
	tcOutput    string
	tcCodec     string
	tcQuality   int
	tcCodecOpts []string
	tcContainer string
	tcJobs      int
	tcBigTIFF   string
	tcForce     bool
)

var transcodeCmd = &cobra.Command{
	Use:   "transcode [flags] <input>",
	Short: "Re-encode the pyramid tiles in a different compression codec",
	Long: `Re-encode the pyramid tiles of a WSI in a different compression codec
while preserving the source's tile geometry and metadata. Associated images
(label, macro, thumbnail, overview) are passed through verbatim.

Output container defaults:
  --codec jpeg|jpegli on SVS source: SVS-shaped output (Aperio convention).
  Everything else: generic pyramidal TIFF with WSIImageType-tagged IFDs.

v0.2.0 supported source formats: SVS, Philips-TIFF, OME-TIFF (tiled), BIF, IFE,
generic-TIFF. NDPI and OME-OneFrame error cleanly.

Examples:

  # SVS to JPEG-XL (generic TIFF output, since JPEG-XL doesn't fit SVS).
  wsi-tools transcode --codec jpegxl -o slide-jxl.tiff slide.svs

  # SVS to jpegli (still SVS-shaped, smaller per-tile bytes).
  wsi-tools transcode --codec jpegli -o slide-jpegli.svs slide.svs

  # AVIF with faster encoder.
  wsi-tools transcode --codec avif --codec-opt avif.speed=8 -o out.tiff in.svs

  # Lossless WebP archival.
  wsi-tools transcode --codec webp --codec-opt webp.lossless=true -o out.tiff in.svs`,
	Args: cobra.ExactArgs(1),
	RunE: runTranscode,
}

func init() {
	transcodeCmd.Flags().StringVarP(&tcOutput, "output", "o", "", "output file path (required)")
	transcodeCmd.Flags().StringVar(&tcCodec, "codec", "", "target codec: jpeg|jpegli|jpegxl|avif|webp|htj2k")
	transcodeCmd.Flags().IntVar(&tcQuality, "quality", 85, "codec-agnostic quality 1..100")
	transcodeCmd.Flags().StringSliceVar(&tcCodecOpts, "codec-opt", nil, "codec-specific KEY=VAL (repeatable)")
	transcodeCmd.Flags().StringVar(&tcContainer, "container", "", "output container: svs|tiff (default depends on source + codec)")
	transcodeCmd.Flags().IntVar(&tcJobs, "jobs", runtime.NumCPU(), "worker goroutines")
	transcodeCmd.Flags().StringVar(&tcBigTIFF, "bigtiff", "auto", "auto|on|off")
	transcodeCmd.Flags().BoolVarP(&tcForce, "force", "f", false, "overwrite output if it exists")
	_ = transcodeCmd.MarkFlagRequired("output")
	_ = transcodeCmd.MarkFlagRequired("codec")
	rootCmd.AddCommand(transcodeCmd)
}

func runTranscode(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	input := args[0]

	if _, err := os.Stat(input); err != nil {
		return fmt.Errorf("input %s: %w", input, err)
	}
	if !tcForce {
		if _, err := os.Stat(tcOutput); err == nil {
			return fmt.Errorf("output %s already exists (use --force)", tcOutput)
		}
	}
	if tcQuality < 1 || tcQuality > 100 {
		return fmt.Errorf("--quality must be 1..100")
	}

	// Lookup the codec factory; fail early if unavailable.
	fac, err := codec.Lookup(tcCodec)
	if err != nil {
		return fmt.Errorf("--codec %q: %w", tcCodec, err)
	}

	// Open the source via the sanity-gated adapter.
	src, err := source.Open(input)
	if err != nil {
		if errors.Is(err, source.ErrUnsupportedFormat) {
			return fmt.Errorf("source format unsupported at v0.2.0: %w", err)
		}
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close()

	container := resolveContainer(src.Format(), tcCodec, tcContainer)
	bigtiffMode := resolveBigTIFF(tcBigTIFF, src)

	// Quality knobs from CLI flags + --codec-opt.
	knobs := map[string]string{"q": fmt.Sprintf("%d", tcQuality)}
	for _, opt := range tcCodecOpts {
		k, v, ok := strings.Cut(opt, "=")
		if !ok {
			return fmt.Errorf("--codec-opt %q: missing '='", opt)
		}
		// Strip the codec prefix when present (e.g. "jxl.distance=1.5" → "distance").
		if pfx := tcCodec + "."; strings.HasPrefix(k, pfx) {
			k = k[len(pfx):]
		} else if pfx2 := strings.SplitN(k, ".", 2); len(pfx2) == 2 {
			k = pfx2[1] // foreign-codec-prefixed; tolerate
		}
		knobs[k] = v
	}

	// Open the writer.
	wOpts := []wsiwriter.Option{
		wsiwriter.WithBigTIFF(bigtiffMode),
		wsiwriter.WithToolsVersion("0.2.0-dev"),
		wsiwriter.WithSourceFormat(src.Format()),
	}
	md := src.Metadata()
	if md.Make != "" {
		wOpts = append(wOpts, wsiwriter.WithMake(md.Make))
	}
	if md.Model != "" {
		wOpts = append(wOpts, wsiwriter.WithModel(md.Model))
	}
	if md.Software != "" {
		wOpts = append(wOpts, wsiwriter.WithSoftware(md.Software))
	}
	if !md.AcquisitionDateTime.IsZero() {
		wOpts = append(wOpts, wsiwriter.WithDateTime(md.AcquisitionDateTime))
	}
	if container == "svs" && src.Format() == string(opentile.FormatSVS) {
		// Re-emit the source's Aperio ImageDescription verbatim — output is
		// SVS-shaped, no metadata mutation.
		if desc := src.SourceImageDescription(); desc != "" {
			wOpts = append(wOpts, wsiwriter.WithImageDescription(desc))
		}
	} else {
		// Generic TIFF: assemble a wsi-tools provenance string.
		wOpts = append(wOpts, wsiwriter.WithImageDescription(buildProvenanceDesc(src, tcCodec, md)))
	}

	w, err := wsiwriter.Create(tcOutput, wOpts...)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}

	// Per-level transcode pipeline.
	if err := transcodePyramid(cmd.Context(), src, w, fac, knobs, tcJobs, container); err != nil {
		w.Close() // tmp removed by Close
		return err
	}

	// Pass through associated images.
	if err := writeAssociatedImages(src, w, container); err != nil {
		w.Close()
		return err
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}

	stat, _ := os.Stat(tcOutput)
	if stat != nil {
		slog.Info("transcode complete",
			"output", tcOutput,
			"size", formatSize(stat.Size()),
		)
		fmt.Printf("wrote %s (%s)\n", tcOutput, formatSize(stat.Size()))
	}
	return nil
}

func resolveContainer(srcFormat, codecName, override string) string {
	if override != "" {
		return override
	}
	// SVS source + jpeg/jpegli codec → SVS-shaped.
	if srcFormat == string(opentile.FormatSVS) {
		switch codecName {
		case "jpeg", "jpegli":
			return "svs"
		}
	}
	return "tiff"
}

func resolveBigTIFF(mode string, src source.Source) bool {
	switch mode {
	case "on":
		return true
	case "off":
		return false
	}
	// auto: predict output size; promote when > 2 GiB.
	var total int64
	for _, lvl := range src.Levels() {
		total += int64(lvl.Size().X) * int64(lvl.Size().Y) * 1 // ~1 bpp lossy estimate
	}
	return total > (2 << 30)
}

func transcodePyramid(ctx context.Context, src source.Source, w *wsiwriter.Writer, fac codec.EncoderFactory, knobs map[string]string, workers int, container string) error {
	for _, lvl := range src.Levels() {
		if err := transcodeLevel(ctx, lvl, w, fac, knobs, workers, container, len(src.Levels())); err != nil {
			return fmt.Errorf("level %d: %w", lvl.Index(), err)
		}
	}
	return nil
}

func transcodeLevel(ctx context.Context, lvl source.Level, w *wsiwriter.Writer, fac codec.EncoderFactory, knobs map[string]string, workers int, container string, totalLevels int) error {
	enc, err := fac.NewEncoder(codec.LevelGeometry{
		TileWidth: lvl.TileSize().X, TileHeight: lvl.TileSize().Y,
		PixelFormat: codec.PixelFormatRGB8,
	}, codec.Quality{Knobs: knobs})
	if err != nil {
		return err
	}
	defer enc.Close()

	spec := wsiwriter.LevelSpec{
		ImageWidth:                uint32(lvl.Size().X),
		ImageHeight:               uint32(lvl.Size().Y),
		TileWidth:                 uint32(lvl.TileSize().X),
		TileHeight:                uint32(lvl.TileSize().Y),
		Compression:               enc.TIFFCompressionTag(),
		PhotometricInterpretation: photometricFor(enc.TIFFCompressionTag()),
		JPEGTables:                enc.LevelHeader(),
		JPEGAbbreviatedTiles:      enc.TIFFCompressionTag() == wsiwriter.CompressionJPEG,
		NewSubfileType:            0,
		WSIImageType:              wsiwriter.WSIImageTypePyramid,
	}
	for _, t := range enc.ExtraTIFFTags() {
		spec.ExtraTags = append(spec.ExtraTags, t)
	}

	lh, err := w.AddLevel(spec)
	if err != nil {
		return err
	}

	dec := pickDecoder(lvl.Compression())
	if dec == nil {
		return fmt.Errorf("no decoder for source compression %s", lvl.Compression())
	}

	grid := lvl.Grid()
	tileBytes := lvl.TileSize().X * lvl.TileSize().Y * 3
	return pipeline.Run(ctx, pipeline.Config{
		Workers: workers,
		Source: func(ctx context.Context, emit func(pipeline.Tile) error) error {
			for ty := 0; ty < grid.Y; ty++ {
				for tx := 0; tx < grid.X; tx++ {
					b, err := lvl.Tile(tx, ty)
					if err != nil {
						return err
					}
					if err := emit(pipeline.Tile{Level: lvl.Index(), X: uint32(tx), Y: uint32(ty), Bytes: b}); err != nil {
						return err
					}
				}
			}
			return nil
		},
		Process: func(t pipeline.Tile) (pipeline.Tile, error) {
			rgb := make([]byte, tileBytes)
			rgbOut, err := dec.DecodeTile(t.Bytes, rgb, 1, 1)
			if err != nil {
				return pipeline.Tile{}, err
			}
			encoded, err := enc.EncodeTile(rgbOut, lvl.TileSize().X, lvl.TileSize().Y, nil)
			if err != nil {
				return pipeline.Tile{}, err
			}
			t.Bytes = encoded
			return t, nil
		},
		Sink: func(t pipeline.Tile) error {
			return lh.WriteTile(t.X, t.Y, t.Bytes)
		},
	})
}

func pickDecoder(c source.Compression) decoder.Decoder {
	switch c {
	case source.CompressionJPEG:
		return decoder.NewJPEG()
	case source.CompressionJPEG2000:
		return decoder.NewJPEG2000()
	}
	return nil
}

func photometricFor(compression uint16) uint16 {
	// Aperio convention: JPEG-in-TIFF → PhotometricInterpretation=2 (RGB)
	// because we use raw-RGB-storage JPEGs with APP14 marker. Other codecs
	// generally also accept PhotometricInterpretation=2 since the codec
	// stream carries its own colour model.
	return 2 // RGB
}

func writeAssociatedImages(src source.Source, w *wsiwriter.Writer, container string) error {
	for _, a := range src.Associated() {
		bs, err := a.Bytes()
		if err != nil {
			return fmt.Errorf("associated %s: %w", a.Kind(), err)
		}
		spec := wsiwriter.AssociatedSpec{
			Kind:                      a.Kind(),
			Compressed:                bs,
			Width:                     uint32(a.Size().X),
			Height:                    uint32(a.Size().Y),
			Compression:               mapCompressionForOutput(a.Compression()),
			PhotometricInterpretation: 2,
			NewSubfileType:            newSubfileTypeFor(container, a.Kind()),
			WSIImageType:              a.Kind(),
		}
		if err := w.AddAssociated(spec); err != nil {
			return fmt.Errorf("write associated %s: %w", a.Kind(), err)
		}
	}
	return nil
}

func mapCompressionForOutput(c source.Compression) uint16 {
	switch c {
	case source.CompressionJPEG:
		return wsiwriter.CompressionJPEG
	case source.CompressionLZW:
		return wsiwriter.CompressionLZW
	case source.CompressionJPEG2000:
		return wsiwriter.CompressionJPEG2000
	}
	return wsiwriter.CompressionNone
}

func newSubfileTypeFor(container, kind string) uint32 {
	if container == "svs" {
		switch kind {
		case "label":
			return 1
		case "macro", "overview":
			return 9
		}
	}
	return 1 // generic TIFF: any associated image is "reduced-resolution"
}

func buildProvenanceDesc(src source.Source, codecName string, md source.Metadata) string {
	var b strings.Builder
	fmt.Fprintf(&b, "wsi-tools/0.2.0-dev transcode source=%s codec=%s", src.Format(), codecName)
	if md.MPP > 0 {
		fmt.Fprintf(&b, " mpp=%v", md.MPP)
	}
	if md.Magnification > 0 {
		fmt.Fprintf(&b, " mag=%vx", md.Magnification)
	}
	if md.Make != "" || md.Model != "" {
		fmt.Fprintf(&b, " scanner=%q", strings.TrimSpace(md.Make+" "+md.Model))
	}
	if !md.AcquisitionDateTime.IsZero() {
		fmt.Fprintf(&b, " date=%s", md.AcquisitionDateTime.Format("2006-01-02"))
	}
	return b.String()
}

func formatSize(n int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.1f GB", float64(n)/GB)
	case n >= MB:
		return fmt.Sprintf("%.1f MB", float64(n)/MB)
	case n >= KB:
		return fmt.Sprintf("%.1f KB", float64(n)/KB)
	}
	return fmt.Sprintf("%d B", n)
}

// Suppress unused imports while iterating; remove during finalization.
var _ = image.Point{}
var _ = time.Now
```

Some of these helper functions assume CLI integration we'll polish in the test pass. Iterate.

- [ ] **Step 3: Build + smoke test on the small fixture**

```bash
go build -o /tmp/wsi-tools ./cmd/wsi-tools
/tmp/wsi-tools transcode --codec webp -o /tmp/out.tiff sample_files/svs/CMU-1-Small-Region.svs
tiffinfo /tmp/out.tiff | head -40
```

Expected: prints `wrote /tmp/out.tiff`. tiffinfo shows multiple TIFF Directories with `Compression Scheme: 50001` (WebP).

Iterate on bugs surfaced by the smoke test.

- [ ] **Step 4: Commit**

```bash
git add cmd/wsi-tools/transcode.go internal/codec/all/all.go
git commit -m "feat(cli): transcode subcommand wiring + streaming pipeline"
```

---

### Task I2: Doctor extension — codec library version reporting

**Files:**
- Modify: `cmd/wsi-tools/doctor.go`

- [ ] **Step 1: Replace doctor's Run with the extended version**

```go
package main

import (
	"fmt"
	"sort"

	codec "github.com/cornish/wsi-tools/internal/codec"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Report installed codec libraries + version info",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("wsi-tools", Version, "— codec / library health check.")
		fmt.Println()
		fmt.Println("Codecs:")
		names := codec.List()
		sort.Strings(names)
		for _, name := range names {
			fmt.Printf("  ✓ %s\n", name)
		}
		fmt.Println()
		fmt.Println("Source decoders:")
		fmt.Println("  ✓ jpeg      (libjpeg-turbo via internal/decoder)")
		fmt.Println("  ✓ jpeg2000  (openjpeg via internal/decoder)")
		fmt.Println()
		fmt.Println("Reader: opentile-go (see go.mod for version)")
		return nil
	},
}

func init() { rootCmd.AddCommand(doctorCmd) }
```

(v0.2 keeps doctor a thin stub — per-library version probing via cgo would require each codec subpackage to expose a `LibraryVersion()` method. That's a v0.3 polish.)

- [ ] **Step 2: Smoke test**

```bash
go build -o /tmp/wsi-tools ./cmd/wsi-tools
/tmp/wsi-tools doctor
```

Expected: lists `avif, htj2k, jpeg, jpegli, jpegxl, webp` under Codecs.

- [ ] **Step 3: Commit**

```bash
git add cmd/wsi-tools/doctor.go
git commit -m "feat(cli): doctor lists all 6 registered codecs"
```

---

## Batch J — Downsample IFD-ordering fix (2 tasks)

### Task J1: Add a regression test for downsample's associated-image Kind round-trip

**Files:**
- Modify: `tests/integration/downsample_test.go`

- [ ] **Step 1: Add the regression test**

Append a test that asserts opentile-go's re-read Kind matches what we wrote:

```go
func TestDownsample_AssociatedKindRoundTrip(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1.svs")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out.svs")
	bin := buildOnce(t)
	if b, err := exec.Command(bin, "downsample", "-o", out, src).CombinedOutput(); err != nil {
		t.Fatalf("downsample: %v\n%s", err, b)
	}

	srcTlr, _ := opentile.OpenFile(src)
	defer srcTlr.Close()
	outTlr, err := opentile.OpenFile(out)
	if err != nil {
		t.Fatalf("opentile.OpenFile(out): %v", err)
	}
	defer outTlr.Close()

	srcKinds := map[string]bool{}
	for _, a := range srcTlr.Associated() {
		srcKinds[a.Kind()] = true
	}
	outKinds := map[string]bool{}
	for _, a := range outTlr.Associated() {
		outKinds[a.Kind()] = true
	}
	for k := range srcKinds {
		if !outKinds[k] {
			t.Errorf("source had associated %q, output's missing it (kind round-trip broken)", k)
		}
	}
}
```

- [ ] **Step 2: Run, verify failure**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -run TestDownsample_AssociatedKindRoundTrip -count=1 -v
```

Expected: FAIL — opentile-go's SVS classifier mis-classifies trailing pages because we write `L0..LN, thumbnail, label, macro` (3 trailing) and the classifier's "last-2 trailing pages" rule drops one.

- [ ] **Step 3: Commit (test only, fix lands in J2)**

```bash
git add tests/integration/downsample_test.go
git commit -m "test(integration): regression test for downsample associated-image Kind round-trip"
```

---

### Task J2: Fix the IFD ordering — interleave thumbnail between L0 and L1+

**Files:**
- Modify: `cmd/wsi-tools/downsample.go`

- [ ] **Step 1: Refactor the level + associated write loop**

In `runDownsample` (or wherever `buildPyramid` + `AddAssociated` are called), change the ordering from:

```
build all pyramid levels  (L0, L1, L2, ...)
then write all associated images (label, macro, thumbnail, ...)
```

to:

```
build pyramid L0
write thumbnail (if present in source)
build pyramid L1, L2, ...
write label
write macro/overview
```

Implementation sketch:

```go
// After computing the output pyramid plan and finding the associated images:
srcAssoc := src.Associated()
var thumbnail, label, macro source.AssociatedImage
otherAssoc := []source.AssociatedImage{}
for _, a := range srcAssoc {
    switch a.Kind() {
    case "thumbnail":
        thumbnail = a
    case "label":
        label = a
    case "macro", "overview":
        macro = a
    default:
        otherAssoc = append(otherAssoc, a)
    }
}

// L0
writeLevel(0)
// thumbnail interleaved between L0 and L1
if thumbnail != nil {
    writeAssociated(thumbnail)
}
// L1+
for i := 1; i < len(levels); i++ {
    writeLevel(i)
}
// label, macro at end
if label != nil { writeAssociated(label) }
if macro != nil { writeAssociated(macro) }
// Anything else (probability/map) goes after.
for _, a := range otherAssoc {
    writeAssociated(a)
}
```

This is the Aperio ordering opentile-go's classifier expects.

- [ ] **Step 2: Run the regression test, verify pass**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -run TestDownsample_AssociatedKindRoundTrip -race -count=1 -v
```

Expected: PASS.

- [ ] **Step 3: Run the full integration suite — no regressions**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -race -count=1 -timeout 30m
```

- [ ] **Step 4: Commit**

```bash
git add cmd/wsi-tools/downsample.go
git commit -m "fix(downsample): match Aperio IFD ordering for associated-image Kind round-trip"
```

---

## Batch K — Integration tests (1 task)

### Task K1: transcode integration tests (per-codec + per-source-format)

**Files:**
- Create: `tests/integration/transcode_test.go`

- [ ] **Step 1: Write the test file**

```go
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

var v02Codecs = []string{"jpegli", "jpegxl", "avif", "webp", "htj2k"}

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
}

func TestTranscode_PerSourceFormat_ToJPEGXL(t *testing.T) {
	td := testdir(t)
	bin := buildOnce(t)

	cases := []struct {
		name     string
		path     string
		wantOK   bool
	}{
		{"svs", filepath.Join(td, "svs", "CMU-1-Small-Region.svs"), true},
		{"philips", filepath.Join(td, "philips-tiff", "Philips-1.tiff"), true},
		{"ome-tiled", filepath.Join(td, "ome-tiff", "Leica-1.ome.tiff"), true},
		{"bif", filepath.Join(td, "bif", "Ventana-1.bif"), true},
		{"ife", filepath.Join(td, "ife", "cervix_2x_jpeg.iris"), true},
		{"generic-tiff", filepath.Join(td, "generictiff", "CMU-1.tiff"), true},
		{"ndpi-rejection", filepath.Join(td, "ndpi", "CMU-1.ndpi"), false},
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
				if !strings.Contains(string(b), "format unsupported") && !strings.Contains(string(b), "ErrUnsupportedFormat") {
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
	cmd := exec.Command(bin, "transcode", "--codec", "webp", "-o", out, src)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("transcode of 4.8 GB BigTIFF failed: %v\n%s", err, b)
	}
	// If we got here without OOM, streaming works.
}
```

- [ ] **Step 2: Run**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -race -count=1 -timeout 60m -v
```

Expected: all pass. The BigTIFF test is the proof streaming works. If any per-source-format test fails because of a writer or codec quirk, debug per case.

- [ ] **Step 3: Commit**

```bash
git add tests/integration/transcode_test.go
git commit -m "test(integration): per-codec + per-source-format transcode sweep"
```

---

## Batch L — CI + viewer-compat (2 tasks)

### Task L1: Update CI workflow for new codec deps

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add codec installs**

Edit `.github/workflows/ci.yml` macOS step to install the new codec libraries:

```yaml
      - name: Install cgo dependencies
        run: |
          brew update
          brew install jpeg-turbo openjpeg pkg-config libtiff jpeg-xl libavif webp openjph
```

Edit Windows msys2 install list:

```yaml
          install: >-
            mingw-w64-x86_64-toolchain
            mingw-w64-x86_64-go
            mingw-w64-x86_64-libjpeg-turbo
            mingw-w64-x86_64-openjpeg2
            mingw-w64-x86_64-libjxl
            mingw-w64-x86_64-libavif
            mingw-w64-x86_64-libwebp
            mingw-w64-x86_64-pkgconf
```

If `mingw-w64-x86_64-openjph` doesn't exist on msys2, add a `-tags nohtj2k` to the Windows build step (don't try to compile against a missing OpenJPH):

```yaml
      - name: go build (Windows, htj2k disabled — openjph not packaged for msys2)
        run: |
          export PATH=/mingw64/bin:$PATH
          export PKG_CONFIG_PATH=/mingw64/lib/pkgconfig
          export CGO_ENABLED=1
          go build -tags nohtj2k ./...
```

- [ ] **Step 2: Verify locally that build tag works**

```bash
go build -tags nohtj2k ./...
go build ./...
```

Expected: both succeed; the slim build skips compiling htj2k.

- [ ] **Step 3: Commit + push to trigger CI**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: install jpeg-xl libavif webp openjph; htj2k disabled on Windows"
git push
```

Watch the CI run on GitHub. If openjph is in fact available on Homebrew, macOS should be green. If Windows fails because libjxl isn't packaged for msys2, also slim-build with `-tags nojxl nojpegli` on Windows.

---

### Task L2: Update viewer-compat checklist + README

**Files:**
- Modify: `docs/viewer-compat.md`
- Modify: `README.md`

- [ ] **Step 1: Update viewer-compat.md**

Add the v0.2 transcode section:

```markdown
## v0.2 — transcode tool

Manual checklist of (codec, viewer) pairs verified to load.

| Codec   | QuPath | openslide | Custom Viewer | OpenSeadragon |
|---------|--------|-----------|---------------|---------------|
| jpeg    | -      | -         | -             | -             |
| jpegli  | -      | -         | -             | -             |
| jpegxl  | -      | -         | -             | -             |
| avif    | -      | -         | -             | -             |
| webp    | -      | -         | -             | -             |
| htj2k   | -      | -         | -             | -             |
```

- [ ] **Step 2: Update README.md to mention transcode**

In `README.md`, add the transcode subcommand to the v0.2 "what's here" section and add a usage example:

```markdown
## v0.2 — what's here

- `wsi-tools downsample` — downsample a WSI by a power-of-2 factor (e.g. 40x → 20x).
- `wsi-tools transcode` — re-encode the pyramid in a different codec
  (jpegli, jpegxl, avif, webp, htj2k, jpeg). 6 source formats supported:
  SVS, Philips-TIFF, OME-TIFF (tiled), BIF, IFE, generic-TIFF.
- `wsi-tools doctor` — report installed codec libraries.
- `wsi-tools version` — print version + Go runtime info.

## Transcode usage

\`\`\`sh
# SVS to JPEG-XL (generic TIFF output)
wsi-tools transcode --codec jpegxl -o slide-jxl.tiff slide.svs

# SVS to jpegli (still SVS-shaped — smaller file, same viewer compatibility)
wsi-tools transcode --codec jpegli -o slide-jpegli.svs slide.svs

# AVIF with a faster encoder preset
wsi-tools transcode --codec avif --codec-opt avif.speed=8 -o out.tiff in.svs

# Lossless WebP for archival
wsi-tools transcode --codec webp --codec-opt webp.lossless=true -o out.tiff in.svs
\`\`\`
```

- [ ] **Step 3: Commit**

```bash
git add docs/viewer-compat.md README.md
git commit -m "docs: README transcode usage + viewer-compat v0.2 matrix"
```

---

## Batch M — Tag v0.2.0 release (1 task)

### Task M1: Final smoke test + tag v0.2.0

**Files:**
- (no source changes)

- [ ] **Step 1: Final regression check**

```bash
make test
make vet
make build
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -race -count=1 -timeout 60m
./bin/wsi-tools doctor
./bin/wsi-tools transcode --codec webp -o /tmp/v02-final.tiff sample_files/svs/CMU-1-Small-Region.svs
```

Expected: every step passes.

- [ ] **Step 2: Update CHANGELOG.md**

Add a v0.2.0 section to `CHANGELOG.md` with a full feature list mirroring the spec's "what's here" goal. Move the previous "Unreleased" content under v0.2.0 if any was accumulated.

- [ ] **Step 3: Commit CHANGELOG**

```bash
git add CHANGELOG.md
git commit -m "docs: CHANGELOG.md for v0.2.0"
```

- [ ] **Step 4: Merge feat branch into main + tag**

```bash
git checkout main
git merge --ff-only feat/v0.2-transcode
git tag -a v0.2.0 -m "wsi-tools v0.2.0 — transcode tool (jpegli, jpegxl, avif, webp, htj2k)"
git push origin main
git push origin v0.2.0
```

- [ ] **Step 5: Create GitHub Release**

```bash
gh release create v0.2.0 --repo cornish/wsi-tools --title "v0.2.0 — transcode tool" --notes-file <(extract v0.2.0 section from CHANGELOG.md)
```

(Or write the release notes inline — pull from CHANGELOG.md's v0.2.0 section.)

---

## Self-review checklist (executor: do this after Task M1)

1. **All tasks committed?** `git log --oneline | wc -l` — expect roughly 50 commits since the v0.1.0 tag.
2. **All tests pass?** `make test` exits 0; integration sweep passes including BigTIFF.
3. **`make vet` clean?**
4. **CI green on macOS + Windows?**
5. **`wsi-tools transcode` works end-to-end on all 6 source formats × at least one codec?** Spot-check by hand if integration tests didn't cover something.
6. **CHANGELOG.md accurate?** Lists all 5 codecs + sanity gate + WSI tags + downsample IFD fix.
7. **`docs/tiff-tags.md` documents the WSI tag block?**
8. **Visual verification in QuPath for at least jpegli + WebP?** (Other codecs require viewers that may not be installed locally.)
