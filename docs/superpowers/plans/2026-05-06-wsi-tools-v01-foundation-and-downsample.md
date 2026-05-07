# wsi-tools v0.1 Foundation + Downsample Tool — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a working `wsi-tools downsample` CLI that converts a 40x SVS slide to a 20x SVS slide (or any power-of-2 factor), regenerating the entire pyramid from the new L0 and passing through associated images verbatim. Foundation pieces (`wsiwriter`, `pipeline`, `internal/decoder/jpeg`, `internal/codec/jpeg`, `internal/resample/area`, cobra shell) ship complete for reuse by the upcoming `transcode` plan.

**Architecture:** Bottom-up build. TIFF/SVS writer first (the load-bearing piece opentile-go can't give us), then the JPEG codec wrappers, then the pipeline, then the CLI. Every component is unit-tested with synthetic inputs before any integration test touches a real fixture. Real-fixture integration tests run last, gated by `WSI_TOOLS_TESTDIR`.

**Tech Stack:** Go 1.23+, `github.com/cornish/opentile-go` v0.10+ as the slide reader, libjpeg-turbo via cgo for JPEG encode/decode, OpenJPEG via cgo for JPEG-2000 source decode, `spf13/cobra` for the CLI, `vbauerster/mpb/v8` for the progress bar, `log/slog` from stdlib for structured logging.

**Spec:** `docs/superpowers/specs/2026-05-06-wsi-tools-v01-design.md`

---

## File structure

```
wsi-tools/
├── go.mod
├── go.sum
├── Makefile
├── README.md
├── LICENSE
├── CLAUDE.md
├── .gitignore                                   # already created
├── cmd/wsi-tools/
│   ├── main.go                                  # cobra root command + global flags
│   ├── downsample.go                            # `wsi-tools downsample` subcommand
│   ├── doctor.go                                # `wsi-tools doctor` subcommand
│   └── version.go                               # `wsi-tools version` subcommand
├── internal/
│   ├── wsiwriter/
│   │   ├── compression.go                       # TIFF Compression tag constants
│   │   ├── tiff.go                              # TIFF / BigTIFF IFD writer
│   │   ├── tiff_test.go
│   │   ├── svs.go                               # Aperio ImageDescription parse + emit
│   │   ├── svs_test.go
│   │   ├── jpegtables.go                        # tables-only JPEG synthesis
│   │   └── jpegtables_test.go
│   ├── codec/
│   │   ├── codec.go                             # Encoder / EncoderFactory / registry
│   │   ├── codec_test.go
│   │   ├── jpeg/
│   │   │   ├── jpeg.go                          # libjpeg-turbo Encoder (abbreviated + APP14)
│   │   │   └── jpeg_test.go
│   │   └── all/
│   │       └── all.go                           # umbrella registration
│   ├── decoder/
│   │   ├── decoder.go                           # Decoder interface + dispatch
│   │   ├── jpeg.go                              # libjpeg-turbo Decoder, 1/N fast-scale
│   │   ├── jpeg_test.go
│   │   ├── jpeg2000.go                          # OpenJPEG Decoder
│   │   └── jpeg2000_test.go
│   ├── resample/
│   │   ├── area.go                              # 2x2 area average (pure Go)
│   │   └── area_test.go
│   └── pipeline/
│       ├── pipeline.go                          # decode → process → encode worker pool
│       └── pipeline_test.go
├── docs/
│   ├── compression-tags.md                      # canonical codec → TIFF Compression value table
│   ├── viewer-compat.md                         # manual viewer-compat checklist (stub at v0.1)
│   └── superpowers/
│       ├── specs/2026-05-06-wsi-tools-v01-design.md   # already exists
│       └── plans/2026-05-06-wsi-tools-v01-foundation-and-downsample.md  # this file
├── tests/
│   ├── integration/
│   │   └── downsample_test.go                   # WSI_TOOLS_TESTDIR-gated SVS round-trip
│   └── bench/
│       └── downsample_bench_test.go             # not in CI
└── sample_files/                                # gitignored; soft-link to opentile-go's pool
```

---

## Conventions for the executor

1. **Always run tests with `-race -count=1`** unless a step says otherwise. The `count=1` defeats Go's test cache, which gives misleading "PASS" output during iterative work.
2. **`cgo` build tag dependencies live in `// #cgo pkg-config: …` directives.** Don't hand-roll `-L`/`-I` flags.
3. **One commit per task** (Step 5 of every task). Commit message format: `<type>: <one-line summary>` where `<type>` ∈ `feat`, `test`, `fix`, `chore`, `docs`, `refactor`. Body optional.
4. **Reference fixtures live in opentile-go's pool.** Soft-link before running integration tests:
   ```sh
   ln -s "$HOME/GitHub/opentile-go/sample_files" "$HOME/GitHub/wsi-tools/sample_files"
   ```
   Tests resolve this via `WSI_TOOLS_TESTDIR` (default: `./sample_files`).
5. **No guessing about TIFF byte layout or Aperio ImageDescription contents.** When unsure, read opentile-go's source (the canonical reader) or `formats/svs/` upstream Python opentile. The spec's "no guessing" rule applies here too.
6. **When a step says "verify with `tiffinfo`",** install it via `brew install libtiff` first if missing. `tiffinfo` is the ground-truth tool for "is this TIFF valid?"
7. **All cgo wrappers must compile with `CGO_ENABLED=1`. There is no nocgo path in this plan**; v0.1 is cgo-only. Document it in the README, do not engineer around it.

---

## Batch A — Bootstrap (3 tasks)

### Task 1: `go mod init` + minimum viable repo skeleton

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `LICENSE`
- Create: `README.md`
- Create: `CLAUDE.md`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/cornish/GitHub/wsi-tools
go mod init github.com/cornish/wsi-tools
```

Expected: creates `go.mod` declaring module path + Go 1.23 directive.

- [ ] **Step 2: Pull in opentile-go and cobra**

```bash
go get github.com/cornish/opentile-go@latest
go get github.com/spf13/cobra@latest
go get github.com/vbauerster/mpb/v8@latest
```

Expected: `go.mod` and `go.sum` are populated.

- [ ] **Step 3: Write Makefile**

Create `Makefile`:

```makefile
.PHONY: test vet cover bench install clean

GO ?= go
BIN = bin/wsi-tools

test:
	$(GO) test ./... -race -count=1

vet:
	$(GO) vet ./...

cover:
	$(GO) test ./... -race -count=1 -coverprofile=coverage.txt -covermode=atomic
	$(GO) tool cover -func=coverage.txt | tail -1

bench:
	$(GO) test ./tests/bench/... -bench=. -benchmem -run=^$$

install:
	$(GO) install ./cmd/wsi-tools

build:
	$(GO) build -o $(BIN) ./cmd/wsi-tools

clean:
	rm -rf bin/ coverage.txt
```

- [ ] **Step 4: Write LICENSE (Apache-2.0, matching opentile-go)**

Copy the standard Apache-2.0 license text into `LICENSE` with `Copyright 2026 Tom Cornish`. Use the canonical text from https://www.apache.org/licenses/LICENSE-2.0.txt.

- [ ] **Step 5: Write README.md skeleton**

Create `README.md`:

```markdown
# wsi-tools

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](./LICENSE)

A Swiss-army knife of utilities for whole-slide imaging (WSI) files used in digital pathology.

## v0.1 — what's here

- `wsi-tools downsample` — downsample a WSI by a power-of-2 factor (e.g. 40x → 20x). Regenerates the full pyramid from the new base; passes through associated images verbatim. SVS-only at v0.1.

## Future

- `wsi-tools transcode` — re-encode a WSI in alternative codecs (JPEG-XL, AVIF, WebP, jpegli, HEIF, JPEG-LS, JPEG-XR, HTJ2K, Basis Universal). v0.2.
- NDPI / Philips / OME-TIFF / BIF / IFE source support. v0.2+.
- GUI front-end. v0.4+.
- Absorption of `cornish/wsi-label-tools` (label remove / replace).

## Install

```sh
go install github.com/cornish/wsi-tools/cmd/wsi-tools@latest
```

## Build prerequisites

cgo dependencies (macOS via Homebrew):

```sh
brew install jpeg-turbo openjpeg
```

`pkg-config` resolves both at build time.

## Usage

```sh
# 40x SVS → 20x SVS
wsi-tools downsample -o slide-20x.svs slide-40x.svs

# Check installed codec libs
wsi-tools doctor
```

## License

Apache 2.0. See `LICENSE`.
```

- [ ] **Step 6: Write CLAUDE.md skeleton**

Create `CLAUDE.md`:

```markdown
# wsi-tools

Go-based utilities for whole-slide imaging (WSI) files. v0.1 ships a downsample CLI;
v0.2+ adds transcode + more source formats.

## Module path

`github.com/cornish/wsi-tools`

## Conventions

- Reader = `github.com/cornish/opentile-go` (consumed as a Go module dep, not forked).
- Writer = `internal/wsiwriter` (pure Go for TIFF structure; cgo only inside codec wrappers).
- Codecs = `internal/codec/<codec>/` subpackages, one per codec, registered via `init()`.
- Decoders = `internal/decoder/` (smaller surface — only what source slides need).
- Pipeline = `internal/pipeline` (worker-pool decode/process/encode).
- CLI = `cmd/wsi-tools/` using cobra.

## Test discipline

- `make test` runs with `-race -count=1`.
- Integration tests gated by `WSI_TOOLS_TESTDIR` env var (default `./sample_files`).
- `sample_files/` is gitignored; soft-link to opentile-go's pool:

  ```sh
  ln -s "$HOME/GitHub/opentile-go/sample_files" sample_files
  ```

## No guessing

When unsure about TIFF byte layout, Aperio ImageDescription, or any WSI quirk: read
the opentile-go reader source first; it's canonical. The spec rule from opentile-go's
CLAUDE.md applies here too — don't reason from first principles about WSI formats,
read the reference implementation.

## Spec + plans

Design docs live at `docs/superpowers/specs/`; implementation plans at
`docs/superpowers/plans/`.
```

- [ ] **Step 7: Verify it builds**

```bash
go build ./...
```

Expected: succeeds (no source files yet, no error).

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum Makefile LICENSE README.md CLAUDE.md
git commit -m "chore: bootstrap module, Makefile, README, license"
```

---

### Task 2: TIFF compression tag constants + docs

**Files:**
- Create: `internal/wsiwriter/compression.go`
- Create: `docs/compression-tags.md`

- [ ] **Step 1: Write `internal/wsiwriter/compression.go`**

```go
// Package wsiwriter writes WSI files in TIFF / BigTIFF / SVS shapes.
package wsiwriter

// TIFF Compression tag (TIFF tag 259) values used by wsi-tools. Standard values
// have ISO / Adobe / community allocations. Private values (≥ 32768) are
// wsi-tools-assigned for codecs without a recognized TIFF tag.
//
// The full canonical mapping lives at docs/compression-tags.md.
const (
	// Standard / community-allocated values.
	CompressionNone     uint16 = 1
	CompressionLZW      uint16 = 5
	CompressionJPEG     uint16 = 7     // also covers jpegli (output is standard JPEG)
	CompressionDeflate  uint16 = 8
	CompressionJPEG2000 uint16 = 33003 // Aperio JP2K (YCbCr); 33005 is the alt RGB form
	CompressionJPEGLS   uint16 = 34712 // ISO-allocated
	CompressionWebP     uint16 = 50001 // Adobe-allocated
	CompressionJPEGXL   uint16 = 50002 // Adobe-allocated (draft)

	// wsi-tools-private values (≥ 32768). Documented in docs/compression-tags.md.
	// These will only be readable by wsi-tools-aware viewers.
	CompressionAVIF    uint16 = 60001
	CompressionHEIF    uint16 = 60002
	CompressionHTJ2K   uint16 = 60003
	CompressionJPEGXR  uint16 = 60004
	CompressionBasisU  uint16 = 60005
)
```

- [ ] **Step 2: Write `docs/compression-tags.md`**

```markdown
# wsi-tools — TIFF Compression tag values

TIFF tag 259 (`Compression`) values used by wsi-tools when writing tiled pyramidal
TIFF / SVS output. Standard values come from ISO, Adobe, or community allocation;
private values (≥ 32768) are wsi-tools-assigned for codecs that lack a recognized
tag value.

## Standard / community-allocated

| Codec | Tag | Source | Notes |
|---|---|---|---|
| None / uncompressed | 1 | TIFF 6.0 | |
| LZW | 5 | TIFF 6.0 | Used by Aperio for label associated images. |
| JPEG | 7 | TIFF 6.0 (Tech Note 2) | "New-style" JPEG-in-TIFF; not "OJPEG" (6). |
| Deflate | 8 | TIFF 6.0 + community | Adobe-allocated. |
| JPEG 2000 (Aperio) | 33003 / 33005 | Aperio | 33003 = YCbCr; 33005 = RGB. Aperio-private. |
| JPEG-LS | 34712 | ISO/IEC 14495 | Standard. |
| WebP | 50001 | Adobe | Adobe-allocated; libtiff supports. |
| JPEG-XL | 50002 | Adobe (draft) | Allocated; spec finalising. |

## wsi-tools private (≥ 32768)

| Codec | Tag | Notes |
|---|---|---|
| AVIF | 60001 | No standard TIFF tag. |
| HEIF | 60002 | No standard TIFF tag. |
| HTJ2K | 60003 | Could overlap JP2K (33003/33005); private to disambiguate. |
| JPEG-XR | 60004 | Microsoft has historically used 22610 for HD Photo; we're not bound to that. |
| Basis Universal | 60005 | Wrapped in KTX2 inside the tile bytes. |

These private values are only readable by wsi-tools-aware viewers and decoders.
This is by design — the `transcode` tool's purpose is to feed test fixtures into
viewers that understand these codecs natively.

## Verification of standard values

Pinned against:
- `libtiff` `libtiff/tif_dir.h` (COMPRESSION_* constants).
- AwareSystems TIFF tag reference (https://www.awaresystems.be/imaging/tiff/tifftags/compression.html).

When opening any newly-written TIFF in `tiffinfo`, the Compression line should
match these values exactly.
```

- [ ] **Step 3: Verify `compression.go` builds**

```bash
go build ./internal/wsiwriter/
```

Expected: succeeds.

- [ ] **Step 4: Commit**

```bash
git add internal/wsiwriter/compression.go docs/compression-tags.md
git commit -m "feat: TIFF Compression tag constants + docs"
```

---

### Task 3: Soft-link sample fixtures + verify opentile-go can read them

**Files:**
- (no source changes; verifies environment)

- [ ] **Step 1: Create the symlink**

```bash
ln -s "$HOME/GitHub/opentile-go/sample_files" /Users/cornish/GitHub/wsi-tools/sample_files
ls -la /Users/cornish/GitHub/wsi-tools/sample_files/svs/
```

Expected: directory listing shows `CMU-1-Small-Region.svs`, `CMU-1.svs`, `JP2K-33003-1.svs`, etc.

- [ ] **Step 2: Write a throwaway probe to confirm opentile-go reads our local fixtures**

Create `cmd/probe/main.go` (will delete in Step 4):

```go
package main

import (
	"fmt"
	"os"

	opentile "github.com/cornish/opentile-go"
	_ "github.com/cornish/opentile-go/formats/all"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: probe <slide.svs>")
		os.Exit(2)
	}
	t, err := opentile.OpenFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer t.Close()
	fmt.Printf("format: %s\n", t.Format())
	fmt.Printf("levels: %d\n", len(t.Levels()))
	fmt.Printf("associated: %d images\n", len(t.Associated()))
	for _, l := range t.Levels() {
		fmt.Printf("  level: %v size, %v tiles, compression %s\n", l.Size(), l.Grid(), l.Compression())
	}
}
```

```bash
go run ./cmd/probe sample_files/svs/CMU-1-Small-Region.svs
```

Expected: prints something like:

```
format: svs
levels: 3
associated: 3 images
  level: ... size, ... tiles, compression jpeg
  ...
```

- [ ] **Step 3: Confirm magnification + MPP are in metadata**

Extend `cmd/probe/main.go` to also print:

```go
import svs "github.com/cornish/opentile-go/formats/svs"
// ...
if md, ok := svs.MetadataOf(t); ok {
    fmt.Printf("MPP: %v\n", md.MPP)
    fmt.Printf("AppMag: %v\n", md.Magnification)
}
```

Re-run; verify MPP and AppMag print correctly. **Note the actual field names** that opentile-go's `svs.Metadata` exposes — these will get used in `internal/wsiwriter/svs.go` later. If the field is named differently than `Magnification` (e.g., `AppMag` directly), record it now.

- [ ] **Step 4: Delete the probe**

```bash
rm -rf cmd/probe
```

- [ ] **Step 5: Commit (just the symlink — already gitignored, so nothing to commit; this task confirms environment)**

No commit. Move on.

---

## Batch B — TIFF writer (6 tasks)

### Task 4: Write a failing test for the simplest possible TIFF (one uncompressed strip IFD)

**Files:**
- Create: `internal/wsiwriter/tiff.go` (skeleton)
- Create: `internal/wsiwriter/tiff_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/wsiwriter/tiff_test.go`:

```go
package wsiwriter

import (
	"os"
	"path/filepath"
	"testing"
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

	// Reload via stdlib TIFF reader to validate structure.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	// We re-decode to assert the file is a parseable TIFF; bytewise pixel
	// equality isn't the point here — structural validity is.
	// Use golang.org/x/image/tiff (already in go.sum if pulled by anything;
	// if not, the test will fail to compile and we'll go get it).
	// (See Step 3 for the import.)
}
```

Note: this test references `Create`, `AddStrippedImage`, `StrippedSpec` — none of which exist yet. That's deliberate; the test should fail to compile.

- [ ] **Step 2: Run the test, verify it fails to compile**

```bash
go test ./internal/wsiwriter/ -run TestWriteMinimalTIFF -count=1
```

Expected: compile error: `undefined: Create`, `undefined: StrippedSpec`, `undefined: AddStrippedImage`.

- [ ] **Step 3: Add `golang.org/x/image/tiff` for read-side validation**

```bash
go get golang.org/x/image/tiff
```

Update the test to actually validate structure:

```go
import (
	"image"
	"os"
	"path/filepath"
	"testing"

	xtiff "golang.org/x/image/tiff"
)

func TestWriteMinimalTIFF(t *testing.T) {
	// ... (same as above, then:)

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
```

- [ ] **Step 4: Write the minimal `tiff.go` to satisfy the test**

Create `internal/wsiwriter/tiff.go`. The minimum surface needed:

```go
package wsiwriter

import (
	"encoding/binary"
	"fmt"
	"os"
)

// Writer writes a TIFF file. Construct via Create.
type Writer struct {
	path     string
	tmpPath  string
	f        *os.File
	bo       binary.ByteOrder
	bigtiff  bool
	imgs     []*imageEntry  // one per top-level IFD
	closed   bool
}

type Option func(*writerConfig)

type writerConfig struct {
	bo      binary.ByteOrder
	bigtiff bool
}

func WithByteOrder(bo binary.ByteOrder) Option { return func(c *writerConfig) { c.bo = bo } }
func WithBigTIFF(b bool) Option                { return func(c *writerConfig) { c.bigtiff = b } }

// Create opens path for writing. The actual write goes to <path>.tmp until Close
// renames it atomically. If <path>.tmp already exists, it's overwritten.
func Create(path string, opts ...Option) (*Writer, error) {
	cfg := writerConfig{bo: binary.LittleEndian, bigtiff: false}
	for _, o := range opts {
		o(&cfg)
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return nil, fmt.Errorf("wsiwriter: create tmp: %w", err)
	}
	w := &Writer{path: path, tmpPath: tmp, f: f, bo: cfg.bo, bigtiff: cfg.bigtiff}
	if err := w.writeHeader(); err != nil {
		f.Close()
		os.Remove(tmp)
		return nil, err
	}
	return w, nil
}

// StrippedSpec describes a single-strip uncompressed (or LZW-compressed) image.
// Used for associated images and small "thumbnail" cases. For tiled pyramidal
// data, use AddLevel.
type StrippedSpec struct {
	ImageWidth, ImageHeight   uint32
	Compression               uint16
	PhotometricInterpretation uint16
	StripBytes                []byte // already-encoded if Compression != None
}

// AddStrippedImage adds a single-strip IFD to the file.
func (w *Writer) AddStrippedImage(s StrippedSpec) error {
	// Implementation:
	// 1. Append s.StripBytes to the file at the current offset.
	// 2. Record an imageEntry with the strip-offset/byte-count and the IFD tags.
	// 3. Actual IFD bytes get emitted in Close().
	// ...
	// (Full implementation: header writes the magic + endianness; AddStrippedImage
	// appends pixel/strip data immediately and remembers tag values + offsets;
	// Close walks the imgs slice and emits each IFD with proper next-IFD pointer
	// chaining, then back-patches the first-IFD offset in the header.)
	panic("TODO Task 4 Step 4: implement AddStrippedImage")
}

// Close finalizes IFD chain + StripOffsets/TileOffsets back-patching, then
// renames tmp to the final path.
func (w *Writer) Close() error {
	panic("TODO Task 4 Step 4: implement Close")
}

func (w *Writer) writeHeader() error {
	// Classic TIFF: 'II' or 'MM', 0x002A, then 4-byte first-IFD offset (back-patched).
	// BigTIFF: 'II' or 'MM', 0x002B, 0x0008, 0x0000, then 8-byte first-IFD offset.
	// We back-patch the first-IFD offset in Close; here we write a placeholder.
	// ...
	panic("TODO Task 4 Step 4: implement writeHeader")
}

type imageEntry struct {
	tags         []ifdTag
	stripOffsets []uint32 // for stripped images
	stripCounts  []uint32
	tileOffsets  []uint64 // for tiled images (BigTIFF widens these to uint64)
	tileCounts   []uint64
}

type ifdTag struct {
	tag    uint16
	typ    uint16 // TIFF type (1=BYTE, 3=SHORT, 4=LONG, 7=UNDEFINED, 16=LONG8 BigTIFF-only)
	count  uint64
	value  []byte // either inline (≤4 bytes classic, ≤8 BigTIFF) or external pointer
}
```

The full TIFF write logic — header layout, IFD entry encoding, offset back-patching, the precise byte order details for classic vs BigTIFF — must be implemented per Adobe TIFF 6.0 + the BigTIFF community spec (https://www.awaresystems.be/imaging/tiff/bigtiff.html). Reference points:
- TIFF 6.0 spec, Section 2 ("TIFF Structure").
- BigTIFF spec on AwareSystems for the 8-byte offset widening + 0x002B magic + extra header bytes.
- `golang.org/x/image/tiff/lzw_test.go` has minimal classic-TIFF write examples for sanity-check byte sequences (it doesn't write tiled, so we can't reuse the writer, but its byte layouts confirm header + simple IFD shape).

The implementation should:
1. In `Create`: open `path.tmp`, write the 8-byte header (or 16-byte BigTIFF header), placeholder for first-IFD offset.
2. In `AddStrippedImage`: append `s.StripBytes` to the file at the current offset, record the offset + length in `stripOffsets/stripCounts`. Build the `tags` slice with ImageWidth (256), ImageLength (257), BitsPerSample (258), Compression (259), PhotometricInterpretation (262), StripOffsets (273), SamplesPerPixel (277), RowsPerStrip (278), StripByteCounts (279).
3. In `Close`: for each `imageEntry`, emit the IFD (entry count u16/u64, then sorted-by-tag entries, then next-IFD-offset 4/8 bytes pointing to the next IFD or 0 for last), then back-patch the first-IFD offset into the header.

- [ ] **Step 5: Run the test, verify it passes**

```bash
go test ./internal/wsiwriter/ -run TestWriteMinimalTIFF -race -count=1 -v
```

Expected: PASS.

- [ ] **Step 6: Verify the output is a valid TIFF using `tiffinfo`**

Generate a sample to inspect:

```bash
go test ./internal/wsiwriter/ -run TestWriteMinimalTIFF -count=1 \
  -args -keep-output 2>/dev/null || true
```

(If the test doesn't keep output, hand-write a tiny `cmd/probe/main.go` that calls `Create` + `AddStrippedImage` and writes to a known path, run it, then `tiffinfo`.) Expected `tiffinfo` output:

```
TIFF Directory at offset ...
  Image Width: 8 Image Length: 8
  Bits/Sample: 8
  Compression Scheme: None
  Photometric Interpretation: RGB color
  ...
```

If `tiffinfo` reports errors (truncated IFD, bad offset, etc.), debug before moving on.

- [ ] **Step 7: Commit**

```bash
git add internal/wsiwriter/tiff.go internal/wsiwriter/tiff_test.go go.mod go.sum
git commit -m "feat(wsiwriter): write minimal classic TIFF with one stripped IFD"
```

---

### Task 5: Add tiled IFD support (TileWidth/TileLength/TileOffsets/TileByteCounts)

**Files:**
- Modify: `internal/wsiwriter/tiff.go`
- Modify: `internal/wsiwriter/tiff_test.go`

- [ ] **Step 1: Write the failing test for a tiled TIFF**

Append to `tiff_test.go`:

```go
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
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./internal/wsiwriter/ -run TestWriteTiledTIFF -count=1
```

Expected: compile error: `undefined: AddLevel`, `undefined: LevelSpec`.

- [ ] **Step 3: Implement AddLevel + LevelHandle.WriteTile**

Add to `tiff.go`:

```go
// LevelSpec describes one pyramid level. For Aperio SVS output, downstream
// callers will additionally set JPEGTables + JPEGAbbreviatedTiles for the
// JPEG-7 case; v0.1 of this writer doesn't validate those interactions, but
// records them faithfully into the IFD.
type LevelSpec struct {
	ImageWidth, ImageHeight   uint32
	TileWidth, TileHeight     uint32
	Compression               uint16
	PhotometricInterpretation uint16
	JPEGTables                []byte // tables-only JPEG (SOI + DQT + DHT + EOI), per-level
	JPEGAbbreviatedTiles      bool
	ICCProfile                []byte
	ExtraTags                 []TIFFTag

	// SamplesPerPixel defaults to 3 if zero.
	SamplesPerPixel uint16

	// SubfileType / NewSubfileType for pyramid level signalling.
	//
	// CORRECTION (discovered Task 11): opentile-go's SVS classifier
	// (formats/svs/series.go:classifyPages) walks pyramid levels as
	// "tiled AND NOT reduced". A pyramid level with NewSubfileType bit 0
	// set (reduced=true) is classified as a trailing associated image,
	// not a Baseline pyramid level. So all pyramid levels (L0 + L1+) must
	// use NewSubfileType=0; the "reduced" bit is reserved for label/macro
	// associated IFDs.
	NewSubfileType uint32
}

// TIFFTag is an opaque carrier for caller-supplied IFD entries (e.g.,
// codec-specific private tags).
type TIFFTag struct {
	Tag   uint16
	Type  uint16
	Count uint64
	Value []byte
}

// LevelHandle accepts tile bytes for one level. WriteTile is callable in any
// order; the implementation maintains TileOffsets/TileByteCounts indexed by
// row-major position.
type LevelHandle struct {
	w        *Writer
	entry    *imageEntry
	tilesX   uint32
	tilesY   uint32
}

func (w *Writer) AddLevel(s LevelSpec) (*LevelHandle, error) {
	if s.TileWidth == 0 || s.TileHeight == 0 {
		return nil, fmt.Errorf("wsiwriter: tile dimensions must be non-zero")
	}
	if s.SamplesPerPixel == 0 {
		s.SamplesPerPixel = 3
	}
	tilesX := (s.ImageWidth + s.TileWidth - 1) / s.TileWidth
	tilesY := (s.ImageHeight + s.TileHeight - 1) / s.TileHeight
	entry := &imageEntry{
		tileOffsets: make([]uint64, tilesX*tilesY),
		tileCounts:  make([]uint64, tilesX*tilesY),
		spec:        s, // store the spec so Close() can emit the right tags
	}
	w.imgs = append(w.imgs, entry)
	return &LevelHandle{w: w, entry: entry, tilesX: tilesX, tilesY: tilesY}, nil
}

func (h *LevelHandle) WriteTile(x, y uint32, compressed []byte) error {
	if x >= h.tilesX || y >= h.tilesY {
		return fmt.Errorf("wsiwriter: tile (%d,%d) out of grid (%d,%d)",
			x, y, h.tilesX, h.tilesY)
	}
	off, err := h.w.f.Seek(0, 1) // current offset
	if err != nil {
		return err
	}
	if _, err := h.w.f.Write(compressed); err != nil {
		return err
	}
	idx := y*h.tilesX + x
	h.entry.tileOffsets[idx] = uint64(off)
	h.entry.tileCounts[idx] = uint64(len(compressed))
	return nil
}
```

Update `imageEntry` to carry `spec LevelSpec` (or a discriminated `kind`/`stripped`/`tiled` shape — pick one). The `Close()` walker now needs to handle both `AddStrippedImage` results and `AddLevel` results, emitting the right TIFF tags for each:

- For tiled: TileWidth (322), TileLength (323), TileOffsets (324), TileByteCounts (325). NO StripOffsets / StripByteCounts.
- For stripped: as before.
- Common: ImageWidth (256), ImageLength (257), BitsPerSample (258), Compression (259), PhotometricInterpretation (262), SamplesPerPixel (277), NewSubfileType (254).
- If `JPEGTables` is non-empty: emit tag 347 (JPEGTables) with TYPE=UNDEFINED.
- If `ICCProfile` is non-empty: emit tag 34675 (ICCProfile) with TYPE=UNDEFINED.
- For each `ExtraTags` entry: emit verbatim.

Implement the IFD tag-sorting + tag-encoding logic such that all entries are emitted in ascending tag order (TIFF requires this).

- [ ] **Step 4: Run the test, verify pass**

```bash
go test ./internal/wsiwriter/ -run TestWriteTiledTIFF -race -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Run `tiffinfo` on the output to confirm tile geometry**

Hand-run a probe (or extend the test temporarily) to dump the file path; then:

```bash
tiffinfo /tmp/.../tiled.tiff
```

Expected output includes:

```
  Image Width: 16 Image Length: 16
  Tile Width: 8 Tile Length: 8
  ...
```

- [ ] **Step 6: Commit**

```bash
git add internal/wsiwriter/tiff.go internal/wsiwriter/tiff_test.go
git commit -m "feat(wsiwriter): tiled IFD with TileOffsets/TileByteCounts"
```

---

### Task 6: BigTIFF support (8-byte offsets, magic 0x002B)

**Files:**
- Modify: `internal/wsiwriter/tiff.go`
- Modify: `internal/wsiwriter/tiff_test.go`

- [ ] **Step 1: Failing test — write a BigTIFF, re-open, verify**

`golang.org/x/image/tiff` doesn't read BigTIFF, so we use opentile-go's `internal/tiff` parser indirectly via `opentile.OpenFile` — but that needs SVS-shape, which we don't have yet. Instead, validate via `tiffinfo` shelling out from the test:

```go
import (
	"os/exec"
	"strings"
)

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
```

- [ ] **Step 2: Run, verify failure**

```bash
go test ./internal/wsiwriter/ -run TestWriteBigTIFF -count=1
```

Expected: PASS-with-error or FAIL because the writer currently writes classic TIFF regardless of `WithBigTIFF`. Either way, file isn't a valid BigTIFF and `tiffinfo` won't parse it as one.

- [ ] **Step 3: Implement BigTIFF mode**

In `tiff.go`:
- `writeHeader`: classic TIFF writes `0x002A` magic + 4-byte placeholder offset; BigTIFF writes `0x002B` magic + `0x0008` (offset size) + `0x0000` (constant) + 8-byte placeholder offset.
- IFD entry count: classic = u16; BigTIFF = u64.
- IFD entry value size: classic = 12 bytes (tag u16 + type u16 + count u32 + value u32); BigTIFF = 20 bytes (tag u16 + type u16 + count u64 + value u64).
- Inline-vs-external value threshold: classic = 4 bytes; BigTIFF = 8 bytes.
- TileOffsets/StripOffsets type: classic = LONG (4); BigTIFF = LONG8 (16).
- Next-IFD offset: classic = 4 bytes; BigTIFF = 8 bytes.

Implement `bigtiffEnabled()` plumbing through `writeHeader`, IFD emit, and tag value sizing. **Do not** branch deep inside helpers; instead, parameterize the writer struct once with a `wordSize int` (4 or 8) and use that consistently.

- [ ] **Step 4: Run, verify pass**

```bash
go test ./internal/wsiwriter/ -run TestWriteBigTIFF -race -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Verify both classic + BigTIFF tests still pass**

```bash
go test ./internal/wsiwriter/ -race -count=1
```

Expected: all three tests PASS (`TestWriteMinimalTIFF`, `TestWriteTiledTIFF`, `TestWriteBigTIFF`).

- [ ] **Step 6: Commit**

```bash
git add internal/wsiwriter/tiff.go internal/wsiwriter/tiff_test.go
git commit -m "feat(wsiwriter): BigTIFF support (8-byte offsets, 0x002B magic)"
```

---

### Task 7: Atomic close (rename tmp → final on success, remove on error)

**Files:**
- Modify: `internal/wsiwriter/tiff.go`
- Modify: `internal/wsiwriter/tiff_test.go`

- [ ] **Step 1: Failing test for atomic semantics**

```go
func TestAtomicClose_RemovesTmpOnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.tiff")

	w, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// Force an error by closing the underlying file out from under the writer.
	w.f.Close()
	err = w.Close()
	if err == nil {
		t.Fatalf("Close should have failed after underlying file was closed")
	}
	// .tmp file should be gone; final file should not exist.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf(".tmp not removed: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("final path exists: %v", err)
	}
}

func TestAtomicClose_RenamesOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.tiff")
	w, _ := Create(path)
	// Add one level + tile so Close has something to flush.
	level, _ := w.AddLevel(LevelSpec{
		ImageWidth: 8, ImageHeight: 8,
		TileWidth: 8, TileHeight: 8,
		Compression: CompressionNone, PhotometricInterpretation: 2,
	})
	level.WriteTile(0, 0, make([]byte, 8*8*3))
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("final path missing: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf(".tmp still present: %v", err)
	}
}
```

- [ ] **Step 2: Run, verify failure (or partial pass)**

Likely the success path already works (rename in current `Close`); the error path probably doesn't.

- [ ] **Step 3: Implement Close error-path tmp removal**

Wrap `Close`:

```go
func (w *Writer) Close() (err error) {
	if w.closed {
		return nil
	}
	w.closed = true
	defer func() {
		if err != nil {
			os.Remove(w.tmpPath)
		}
	}()
	if err = w.flushIFDChain(); err != nil { // existing logic, factored out
		return err
	}
	if err = w.f.Close(); err != nil {
		return err
	}
	return os.Rename(w.tmpPath, w.path)
}
```

- [ ] **Step 4: Run, verify pass**

```bash
go test ./internal/wsiwriter/ -race -count=1 -v
```

Expected: all four tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wsiwriter/tiff.go internal/wsiwriter/tiff_test.go
git commit -m "feat(wsiwriter): atomic Close (tmp file removed on error)"
```

---

### Task 8: AddAssociated — verbatim associated-image passthrough

**Files:**
- Modify: `internal/wsiwriter/tiff.go`
- Modify: `internal/wsiwriter/tiff_test.go`

- [ ] **Step 1: Failing test**

```go
func TestAddAssociated(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "with-assoc.tiff")
	w, _ := Create(path, WithBigTIFF(true))
	// Main pyramid level (one tile so we have a real L0).
	level, _ := w.AddLevel(LevelSpec{
		ImageWidth: 8, ImageHeight: 8, TileWidth: 8, TileHeight: 8,
		Compression: CompressionNone, PhotometricInterpretation: 2,
	})
	level.WriteTile(0, 0, make([]byte, 8*8*3))

	// Synthetic "label" image: 4x4 RGB strip, all 0x55 bytes.
	labelStrip := make([]byte, 4*4*3)
	for i := range labelStrip {
		labelStrip[i] = 0x55
	}
	if err := w.AddAssociated(AssociatedSpec{
		Kind:                      "label",
		Compressed:                labelStrip,
		Width:                     4,
		Height:                    4,
		Compression:               CompressionNone,
		PhotometricInterpretation: 2,
		NewSubfileType:            1, // reduced-resolution
	}); err != nil {
		t.Fatalf("AddAssociated: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Re-open via tiffinfo, confirm two IFDs.
	out, _ := exec.Command("tiffinfo", path).CombinedOutput()
	got := string(out)
	if strings.Count(got, "TIFF Directory") < 2 {
		t.Errorf("expected ≥2 IFDs, got:\n%s", got)
	}
}
```

- [ ] **Step 2: Run, verify failure**

Compile error: `undefined: AddAssociated`, `undefined: AssociatedSpec`.

- [ ] **Step 3: Implement**

In `tiff.go`:

```go
type AssociatedSpec struct {
	Kind                      string // "label", "macro", "thumbnail", "overview"
	Compressed                []byte // already-encoded bytes from opentile-go's AssociatedImage.Bytes()
	Width, Height             uint32
	Compression               uint16
	PhotometricInterpretation uint16
	NewSubfileType            uint32 // typically 1 for reduced-res, 9 for label per Aperio
	ExtraTags                 []TIFFTag
}

func (w *Writer) AddAssociated(s AssociatedSpec) error {
	// Append s.Compressed at current offset, record offset+len in
	// stripOffsets[0]/stripCounts[0]. Add an imageEntry with stripped layout
	// + the spec's tags + NewSubfileType.
	//
	// Note: the Kind string isn't a TIFF tag — Aperio infers it from
	// (NewSubfileType, image dimensions, ImageDescription). For v0.1, we
	// don't write an ImageDescription on associated IFDs; opentile-go's
	// SVS reader uses NewSubfileType + dimensions + position-in-file
	// heuristics to classify (label is the LZW page; macro/thumbnail are
	// JPEG strip pages). v0.1's writer mimics this layout faithfully.
	// ...
}
```

The subtlety: Aperio's classification heuristic in opentile-go looks at the order + Compression tag of trailing IFDs. We'll get this exactly right in Task 11 (SVS-shape integration test); for now, just write the IFD with `NewSubfileType` and structurally valid strip data.

- [ ] **Step 4: Run, verify pass**

```bash
go test ./internal/wsiwriter/ -run TestAddAssociated -race -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wsiwriter/tiff.go internal/wsiwriter/tiff_test.go
git commit -m "feat(wsiwriter): AddAssociated verbatim passthrough for label/macro/thumbnail"
```

---

### Task 9: ImageDescription (TIFF tag 270) emission for L0

**Files:**
- Modify: `internal/wsiwriter/tiff.go`

- [ ] **Step 1: Failing test**

```go
func TestImageDescription(t *testing.T) {
	if _, err := exec.LookPath("tiffinfo"); err != nil {
		t.Skip("tiffinfo missing")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "with-desc.tiff")
	desc := "Aperio Image Library v12.0.15\r\n8x8 [...] |MPP = 1.0 |AppMag = 20"
	w, _ := Create(path, WithBigTIFF(true), WithImageDescription(desc))
	level, _ := w.AddLevel(LevelSpec{
		ImageWidth: 8, ImageHeight: 8, TileWidth: 8, TileHeight: 8,
		Compression: CompressionNone, PhotometricInterpretation: 2,
	})
	level.WriteTile(0, 0, make([]byte, 8*8*3))
	w.Close()

	out, _ := exec.Command("tiffinfo", path).CombinedOutput()
	got := string(out)
	if !strings.Contains(got, "AppMag = 20") {
		t.Errorf("ImageDescription not in output:\n%s", got)
	}
}
```

- [ ] **Step 2: Run, verify failure**

Compile error: `undefined: WithImageDescription`.

- [ ] **Step 3: Implement**

```go
func WithImageDescription(s string) Option {
	return func(c *writerConfig) { c.imageDescription = s }
}

// In Close(), the FIRST imageEntry (which is L0) gets ImageDescription tag 270
// emitted with the configured string + null terminator.
```

ImageDescription is per-IFD; subsequent pyramid levels get their own (typically a stripped form). Aperio's Python opentile reader requires the L0 description starts with `Aperio` to detect SVS — we'll plumb the full Aperio-shaped string in from `internal/wsiwriter/svs.go` (Task 10).

- [ ] **Step 4: Run, verify pass + commit**

```bash
go test ./internal/wsiwriter/ -race -count=1 -v
git add internal/wsiwriter/tiff.go internal/wsiwriter/tiff_test.go
git commit -m "feat(wsiwriter): emit ImageDescription tag 270 on L0"
```

---

## Batch C — Aperio SVS specifics (3 tasks)

### Task 10: Aperio ImageDescription parser + mutator

**Files:**
- Create: `internal/wsiwriter/svs.go`
- Create: `internal/wsiwriter/svs_test.go`

- [ ] **Step 1: Write failing tests**

Create `svs_test.go`:

```go
package wsiwriter

import (
	"strings"
	"testing"
)

const sampleDesc = `Aperio Image Library v12.0.15
46000x32914 [0,100 46000x32814] (240x240) JPEG/RGB Q=70|Aperio Image Library v12.0.15
46000x32914 -> 11500x8228 - |AppMag = 40|StripeWidth = 992|ScanScope ID = SS1234|Filename = test|Date = 03/12/19|Time = 13:14:15|MPP = 0.2497|Left = 25.691574|Top = 23.449873|LineCameraSkew = -0.000424|LineAreaXOffset = 0.019265|LineAreaYOffset = -0.000313|Focus Offset = 0.000000|ImageID = 1234|OriginalWidth = 46000|OriginalHeight = 32914|ICC Profile = ScanScope v1`

func TestParseImageDescription(t *testing.T) {
	d, err := ParseImageDescription(sampleDesc)
	if err != nil {
		t.Fatalf("ParseImageDescription: %v", err)
	}
	if d.AppMag != 40 {
		t.Errorf("AppMag: got %v, want 40", d.AppMag)
	}
	if d.MPP != 0.2497 {
		t.Errorf("MPP: got %v, want 0.2497", d.MPP)
	}
	if d.SoftwareLine != "Aperio Image Library v12.0.15" {
		t.Errorf("SoftwareLine: got %q", d.SoftwareLine)
	}
}

func TestMutateForDownsample_Factor2(t *testing.T) {
	d, _ := ParseImageDescription(sampleDesc)
	d.MutateForDownsample(2, 23000, 16457) // new W/H = source/2
	out := d.Encode()
	if !strings.Contains(out, "AppMag = 20") {
		t.Errorf("expected AppMag=20 in:\n%s", out)
	}
	if !strings.Contains(out, "MPP = 0.4994") {
		t.Errorf("expected MPP=0.4994 in:\n%s", out)
	}
	if !strings.Contains(out, "23000x16457") {
		t.Errorf("expected 23000x16457 in:\n%s", out)
	}
}
```

- [ ] **Step 2: Run, verify compile failure**

```bash
go test ./internal/wsiwriter/ -run TestParseImageDescription -count=1
```

Expected: `undefined: ParseImageDescription`.

- [ ] **Step 3: Implement svs.go**

Create `internal/wsiwriter/svs.go`:

```go
package wsiwriter

import (
	"fmt"
	"strconv"
	"strings"
)

// AperioDescription represents a parsed Aperio SVS ImageDescription tag (270).
// Format reference: opentile-go's formats/svs/metadata.go (the canonical reader).
//
// Wire format:
//   <SoftwareLine>\r\n<W>x<H> [...] <details>|key1 = value1|key2 = value2|...
//
// Parsing strategy: line 1 = software banner; everything after \n joined by
// pipes. The first pipe-separated chunk is the geometry+codec banner; subsequent
// chunks are key=value pairs.
type AperioDescription struct {
	SoftwareLine string             // e.g. "Aperio Image Library v12.0.15"
	GeometryLine string             // e.g. "46000x32914 [0,100 46000x32814] (240x240) JPEG/RGB Q=70"
	AppMag       float64            // mutated on downsample
	MPP          float64            // mutated on downsample
	Properties   map[string]string  // all key=value pairs verbatim, including AppMag/MPP textually
	PropertyOrder []string          // preserve original order for round-tripping
}

func ParseImageDescription(desc string) (*AperioDescription, error) {
	if !strings.HasPrefix(desc, "Aperio") {
		return nil, fmt.Errorf("wsiwriter: not an Aperio ImageDescription")
	}
	// Split at first \n (Aperio uses \r\n; allow either).
	desc = strings.ReplaceAll(desc, "\r\n", "\n")
	lines := strings.SplitN(desc, "\n", 2)
	if len(lines) < 2 {
		return nil, fmt.Errorf("wsiwriter: malformed Aperio ImageDescription (no second line)")
	}
	d := &AperioDescription{
		SoftwareLine: lines[0],
		Properties:   map[string]string{},
	}
	chunks := strings.Split(lines[1], "|")
	d.GeometryLine = chunks[0]
	for _, c := range chunks[1:] {
		eq := strings.Index(c, "=")
		if eq < 0 {
			continue // tolerate stray chunks
		}
		k := strings.TrimSpace(c[:eq])
		v := strings.TrimSpace(c[eq+1:])
		if _, dup := d.Properties[k]; !dup {
			d.PropertyOrder = append(d.PropertyOrder, k)
		}
		d.Properties[k] = v
	}
	if v, ok := d.Properties["AppMag"]; ok {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("wsiwriter: AppMag parse: %w", err)
		}
		d.AppMag = f
	}
	if v, ok := d.Properties["MPP"]; ok {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("wsiwriter: MPP parse: %w", err)
		}
		d.MPP = f
	}
	return d, nil
}

// MutateForDownsample updates AppMag, MPP, and the geometry line for a
// power-of-2 downsample factor. newW and newH are the L0 dimensions of the
// downsampled output (source dimensions / factor).
func (d *AperioDescription) MutateForDownsample(factor int, newW, newH uint32) {
	d.AppMag = d.AppMag / float64(factor)
	d.MPP = d.MPP * float64(factor)
	d.Properties["AppMag"] = formatFloat(d.AppMag)
	d.Properties["MPP"] = formatFloat(d.MPP)
	// Rewrite the geometry line's leading dimensions (e.g. "46000x32914").
	parts := strings.SplitN(d.GeometryLine, " ", 2)
	if strings.Contains(parts[0], "x") {
		newGeo := fmt.Sprintf("%dx%d", newW, newH)
		if len(parts) == 2 {
			d.GeometryLine = newGeo + " " + parts[1]
		} else {
			d.GeometryLine = newGeo
		}
	}
	// Update OriginalWidth/Height too if present.
	if _, ok := d.Properties["OriginalWidth"]; ok {
		d.Properties["OriginalWidth"] = fmt.Sprintf("%d", newW)
	}
	if _, ok := d.Properties["OriginalHeight"]; ok {
		d.Properties["OriginalHeight"] = fmt.Sprintf("%d", newH)
	}
}

// Encode reconstructs the Aperio ImageDescription string in wire format.
func (d *AperioDescription) Encode() string {
	var b strings.Builder
	b.WriteString(d.SoftwareLine)
	b.WriteString("\r\n")
	b.WriteString(d.GeometryLine)
	for _, k := range d.PropertyOrder {
		b.WriteString("|")
		b.WriteString(k)
		b.WriteString(" = ")
		b.WriteString(d.Properties[k])
	}
	return b.String()
}

// formatFloat formats a float without trailing zeros.
func formatFloat(f float64) string {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	return s
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/wsiwriter/ -run "TestParseImageDescription|TestMutateForDownsample" -race -count=1 -v
```

Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wsiwriter/svs.go internal/wsiwriter/svs_test.go
git commit -m "feat(wsiwriter): parse + mutate Aperio ImageDescription for downsample"
```

---

### Task 11: End-to-end synthetic SVS round-trip via opentile-go

**Files:**
- Modify: `internal/wsiwriter/tiff.go` (potentially small fixes)
- Modify: `internal/wsiwriter/svs_test.go`

- [ ] **Step 1: Failing test — write a synthetic 2-level Aperio SVS, re-open via opentile-go, assert structure**

Append to `svs_test.go`:

```go
import (
	opentile "github.com/cornish/opentile-go"
	_ "github.com/cornish/opentile-go/formats/all"
	svsfmt "github.com/cornish/opentile-go/formats/svs"
)

func TestSyntheticSVSRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "synth.svs")

	desc := `Aperio Image Library v12.0.15
512x512 [0,0 512x512] (256x256) JPEG/RGB Q=80|AppMag = 40|MPP = 0.25|Filename = synth`

	w, err := Create(path, WithBigTIFF(false), WithImageDescription(desc))
	if err != nil {
		t.Fatal(err)
	}
	// Two levels: 512x512 with 256x256 tiles (4 tiles), and 256x256 with 256x256
	// tiles (1 tile). Use raw uncompressed RGB so this test doesn't depend on
	// the JPEG codec wrapper (Task 13).
	mkLevel := func(w *Writer, W, H uint32, sub uint32) *LevelHandle {
		l, err := w.AddLevel(LevelSpec{
			ImageWidth: W, ImageHeight: H,
			TileWidth: 256, TileHeight: 256,
			Compression: CompressionNone, PhotometricInterpretation: 2,
			NewSubfileType: sub,
		})
		if err != nil {
			t.Fatal(err)
		}
		tx := (W + 255) / 256
		ty := (H + 255) / 256
		for y := uint32(0); y < ty; y++ {
			for x := uint32(0); x < tx; x++ {
				if err := l.WriteTile(x, y, make([]byte, 256*256*3)); err != nil {
					t.Fatal(err)
				}
			}
		}
		return l
	}
	mkLevel(w, 512, 512, 0) // L0
	mkLevel(w, 256, 256, 1) // L1 (reduced-res)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Re-open via opentile-go.
	tile, err := opentile.OpenFile(path)
	if err != nil {
		t.Fatalf("opentile.OpenFile: %v", err)
	}
	defer tile.Close()
	if got, want := tile.Format(), "svs"; got != want {
		t.Errorf("Format: got %q, want %q", got, want)
	}
	if got, want := len(tile.Levels()), 2; got != want {
		t.Errorf("Levels: got %d, want %d", got, want)
	}
	if md, ok := svsfmt.MetadataOf(tile); ok {
		if md.MPP == 0 {
			t.Errorf("MPP zero in re-read metadata")
		}
	} else {
		t.Errorf("svs.MetadataOf returned !ok")
	}
}
```

- [ ] **Step 2: Run, verify failure modes**

```bash
go test ./internal/wsiwriter/ -run TestSyntheticSVSRoundTrip -race -count=1 -v
```

Expected: opentile-go either fails to recognise as SVS, or recognises 0 levels, or panics on the IFD layout. Each failure mode is a writer bug to fix.

Common failure modes + fixes:
- `format: generic-tiff` (not `svs`): we forgot to write `ImageDescription` on the **first** IFD, or the description doesn't start with `Aperio`. Check `tiffinfo`.
- `Levels: 0`: the L1 IFD's `NewSubfileType` isn't being picked up correctly, or the IFDs aren't chained (`next IFD offset` is wrong).
- Compression mismatch: opentile-go's SVS reader might insist on `Compression=7` (JPEG); if our test uses `CompressionNone`, opentile-go could reject. **If so, switch the test to use real JPEG tile bytes once the JPEG encoder lands** — for now, accept that opentile-go SVS may require JPEG and either (a) skip this test until Task 13, or (b) write the test with `CompressionLZW`-stripped tiles where applicable. **Recommended: defer the round-trip test to after Task 13**, replacing this Step 1 test with a `CompressionNone` validation that just checks `tiffinfo` output for now.

If the test must defer until JPEG codec lands, use `tiffinfo` validation here:

```go
out, _ := exec.Command("tiffinfo", path).CombinedOutput()
got := string(out)
if !strings.Contains(got, "AppMag = 40") || strings.Count(got, "TIFF Directory") < 2 {
    t.Errorf("synthetic SVS structurally malformed:\n%s", got)
}
```

And add a `// TODO Task 13` comment noting the opentile-go round-trip will land then.

- [ ] **Step 3: Implement fixes** until the test passes (or defer the opentile-go assertion to Task 13).

- [ ] **Step 4: Verify pass**

```bash
go test ./internal/wsiwriter/ -race -count=1 -v
```

Expected: all wsiwriter tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wsiwriter/
git commit -m "feat(wsiwriter): synthetic SVS round-trip via opentile-go"
```

---

### Task 12: JPEGTables synthesis (extract DQT/DHT from a probe-encoded tile)

**Files:**
- Create: `internal/wsiwriter/jpegtables.go`
- Create: `internal/wsiwriter/jpegtables_test.go`

- [ ] **Step 1: Failing test**

```go
package wsiwriter

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestExtractJPEGTables(t *testing.T) {
	// Encode a tiny JPEG with stdlib (which produces a self-contained JPEG
	// with embedded DQT/DHT), then run our extractor.
	im := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			im.SetRGBA(x, y, color.RGBA{R: byte(x*32), G: byte(y*32), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, im, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}

	tables, err := ExtractJPEGTables(buf.Bytes())
	if err != nil {
		t.Fatalf("ExtractJPEGTables: %v", err)
	}
	// The extracted tables-only JPEG should:
	//   - Start with SOI (0xFF 0xD8).
	//   - Contain at least one DQT (0xFF 0xDB) and one DHT (0xFF 0xC4) marker.
	//   - End with EOI (0xFF 0xD9).
	//   - NOT contain SOS (0xFF 0xDA).
	if !bytes.HasPrefix(tables, []byte{0xFF, 0xD8}) {
		t.Errorf("missing SOI prefix")
	}
	if !bytes.Contains(tables, []byte{0xFF, 0xDB}) {
		t.Errorf("missing DQT")
	}
	if !bytes.Contains(tables, []byte{0xFF, 0xC4}) {
		t.Errorf("missing DHT")
	}
	if bytes.Contains(tables, []byte{0xFF, 0xDA}) {
		t.Errorf("tables-only JPEG should not contain SOS")
	}
	if !bytes.HasSuffix(tables, []byte{0xFF, 0xD9}) {
		t.Errorf("missing EOI suffix")
	}
}

func TestStripJPEGTables(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var buf bytes.Buffer
	jpeg.Encode(&buf, im, &jpeg.Options{Quality: 80})

	abbrev, err := StripJPEGTables(buf.Bytes())
	if err != nil {
		t.Fatalf("StripJPEGTables: %v", err)
	}
	// Abbreviated tile bytes: SOI + (everything from SOF onwards, no DQT/DHT) + EOI.
	if bytes.Contains(abbrev, []byte{0xFF, 0xDB}) {
		t.Errorf("abbreviated tile should not contain DQT")
	}
	if bytes.Contains(abbrev, []byte{0xFF, 0xC4}) {
		t.Errorf("abbreviated tile should not contain DHT")
	}
	if !bytes.Contains(abbrev, []byte{0xFF, 0xDA}) {
		t.Errorf("abbreviated tile should still contain SOS")
	}
}
```

- [ ] **Step 2: Run, verify failure**

Compile error: `undefined: ExtractJPEGTables`, `undefined: StripJPEGTables`.

- [ ] **Step 3: Implement `jpegtables.go`**

```go
package wsiwriter

import (
	"bytes"
	"fmt"
)

// JPEG marker constants. Two-byte sequences 0xFF, 0x?? define markers.
const (
	jpegSOI = 0xD8
	jpegEOI = 0xD9
	jpegDQT = 0xDB
	jpegDHT = 0xC4
	jpegDRI = 0xDD
	jpegSOS = 0xDA
	// SOF markers: 0xC0..0xCF excluding 0xC4 (DHT) and 0xC8 (reserved).
	jpegSOF0 = 0xC0
	jpegSOF1 = 0xC1
	jpegSOF2 = 0xC2
	jpegSOF3 = 0xC3
)

// ExtractJPEGTables walks a self-contained JPEG and returns a tables-only JPEG
// containing SOI + all DQT + all DHT + (optional DRI) + EOI markers, suitable
// for writing into TIFF tag 347 (JPEGTables).
//
// The tables-only JPEG must end before SOS — once SOS is reached, scan data
// follows and must be excluded.
func ExtractJPEGTables(jpg []byte) ([]byte, error) {
	if len(jpg) < 4 || jpg[0] != 0xFF || jpg[1] != jpegSOI {
		return nil, fmt.Errorf("wsiwriter: not a JPEG (no SOI)")
	}
	out := []byte{0xFF, jpegSOI}
	i := 2
	for i < len(jpg)-1 {
		if jpg[i] != 0xFF {
			i++
			continue
		}
		marker := jpg[i+1]
		if marker == 0xFF {
			i++ // padding fill byte
			continue
		}
		// Stand-alone markers without length: SOI, EOI, RST0..RST7 (0xD0..0xD7).
		if marker == jpegSOI || marker == jpegEOI || (marker >= 0xD0 && marker <= 0xD7) {
			i += 2
			continue
		}
		// SOS = end of header section; stop.
		if marker == jpegSOS {
			break
		}
		// All other markers carry a 2-byte big-endian length following the marker.
		if i+4 > len(jpg) {
			return nil, fmt.Errorf("wsiwriter: truncated JPEG marker length")
		}
		segLen := int(jpg[i+2])<<8 | int(jpg[i+3])
		segEnd := i + 2 + segLen
		if segEnd > len(jpg) {
			return nil, fmt.Errorf("wsiwriter: truncated JPEG segment")
		}
		if marker == jpegDQT || marker == jpegDHT || marker == jpegDRI {
			out = append(out, jpg[i:segEnd]...)
		}
		i = segEnd
	}
	out = append(out, 0xFF, jpegEOI)
	return out, nil
}

// StripJPEGTables walks a self-contained JPEG and returns a copy with all DQT
// and DHT markers removed. Result is the abbreviated-form tile bytes that pair
// with a JPEGTables tag of the same shared tables.
func StripJPEGTables(jpg []byte) ([]byte, error) {
	if len(jpg) < 4 || jpg[0] != 0xFF || jpg[1] != jpegSOI {
		return nil, fmt.Errorf("wsiwriter: not a JPEG")
	}
	var out bytes.Buffer
	out.Write([]byte{0xFF, jpegSOI})
	i := 2
	for i < len(jpg)-1 {
		if jpg[i] != 0xFF {
			i++
			continue
		}
		marker := jpg[i+1]
		if marker == 0xFF {
			i++
			continue
		}
		if marker == jpegSOI || marker == jpegEOI || (marker >= 0xD0 && marker <= 0xD7) {
			out.Write(jpg[i : i+2])
			i += 2
			continue
		}
		if marker == jpegSOS {
			// Copy SOS + everything to EOI verbatim (entropy-coded scan).
			out.Write(jpg[i:])
			return out.Bytes(), nil
		}
		segLen := int(jpg[i+2])<<8 | int(jpg[i+3])
		segEnd := i + 2 + segLen
		if marker != jpegDQT && marker != jpegDHT {
			out.Write(jpg[i:segEnd])
		}
		i = segEnd
	}
	return out.Bytes(), nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/wsiwriter/ -run "TestExtractJPEGTables|TestStripJPEGTables" -race -count=1 -v
```

Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wsiwriter/jpegtables.go internal/wsiwriter/jpegtables_test.go
git commit -m "feat(wsiwriter): JPEG tables extraction + tile abbreviation"
```

---

## Batch D — Codec interface + JPEG encoder (3 tasks)

### Task 13: Codec interface + registry

**Files:**
- Create: `internal/codec/codec.go`
- Create: `internal/codec/codec_test.go`

- [ ] **Step 1: Failing test**

```go
package codec

import (
	"errors"
	"testing"
)

func TestRegistryRegisterAndLookup(t *testing.T) {
	// Reset registry for test isolation.
	resetRegistryForTesting()

	fac := &fakeFactory{name: "fake"}
	Register(fac)

	got, err := Lookup("fake")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got != fac {
		t.Errorf("Lookup returned different factory")
	}
}

func TestLookupUnknown(t *testing.T) {
	resetRegistryForTesting()
	_, err := Lookup("nope")
	if !errors.Is(err, ErrUnknownCodec) {
		t.Errorf("err: got %v, want ErrUnknownCodec", err)
	}
}

type fakeFactory struct{ name string }

func (f *fakeFactory) Name() string { return f.name }
func (f *fakeFactory) NewEncoder(LevelGeometry, Quality) (Encoder, error) {
	return nil, errors.New("not implemented")
}
```

- [ ] **Step 2: Run, verify failure**

Compile error.

- [ ] **Step 3: Implement codec.go**

```go
// Package codec defines the Encoder + EncoderFactory interfaces and a registry
// that codec subpackages register themselves into via init(). Concrete codec
// implementations live in internal/codec/<codec>/ subpackages.
package codec

import (
	"errors"
	"fmt"
	"sync"

	"github.com/cornish/wsi-tools/internal/wsiwriter"
)

var (
	ErrUnknownCodec       = errors.New("codec: unknown codec name")
	ErrCodecUnavailable   = errors.New("codec: not built into this binary")
)

type PixelFormat int

const (
	PixelFormatRGB8 PixelFormat = iota
	PixelFormatRGBA8
	PixelFormatYCbCr420
)

type ColorSpace struct {
	Name      string // "sRGB", "DisplayP3", or empty for ICC-only.
	ICC       []byte // optional embedded profile.
}

type LevelGeometry struct {
	TileWidth, TileHeight int
	PixelFormat           PixelFormat
	ColorSpace            ColorSpace
}

type Quality struct {
	Knobs map[string]string
}

type Encoder interface {
	LevelHeader() []byte
	EncodeTile(rgb []byte, w, h int, dst []byte) ([]byte, error)
	TIFFCompressionTag() uint16
	ExtraTIFFTags() []wsiwriter.TIFFTag
	Close() error
}

type EncoderFactory interface {
	Name() string
	NewEncoder(LevelGeometry, Quality) (Encoder, error)
}

var (
	regMu sync.RWMutex
	reg   = map[string]EncoderFactory{}
)

func Register(f EncoderFactory) {
	regMu.Lock()
	defer regMu.Unlock()
	reg[f.Name()] = f
}

func Lookup(name string) (EncoderFactory, error) {
	regMu.RLock()
	defer regMu.RUnlock()
	f, ok := reg[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownCodec, name)
	}
	return f, nil
}

func List() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(reg))
	for k := range reg {
		out = append(out, k)
	}
	return out
}

// resetRegistryForTesting clears the registry. Test-only.
func resetRegistryForTesting() {
	regMu.Lock()
	defer regMu.Unlock()
	reg = map[string]EncoderFactory{}
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/codec/ -race -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/codec/codec.go internal/codec/codec_test.go
git commit -m "feat(codec): Encoder/EncoderFactory interfaces + registry"
```

---

### Task 14: JPEG codec wrapper (libjpeg-turbo, abbreviated mode + APP14)

**Files:**
- Create: `internal/codec/jpeg/jpeg.go`
- Create: `internal/codec/jpeg/jpeg_test.go`
- Create: `internal/codec/all/all.go`

- [ ] **Step 1: Failing test**

```go
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
	// Make a 256x256 RGB tile with a simple gradient.
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

	// Splice LevelHeader (tables-only JPEG) + tile (abbreviated) and decode.
	tables := enc.LevelHeader()
	if len(tables) == 0 {
		t.Fatal("LevelHeader: empty")
	}
	whole := append([]byte{}, tables...)
	whole = append(whole, tile...)
	// stdlib JPEG decoder requires self-contained JPEGs; splice tables into the
	// tile by stripping the tables' EOI and the tile's SOI.
	spliced := spliceForDecode(tables, tile)
	im, err := stdjpeg.Decode(bytes.NewReader(spliced))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if im.Bounds() != image.Rect(0, 0, 256, 256) {
		t.Errorf("bounds: %v", im.Bounds())
	}
	// Spot-check pixel (10, 20): we put R=10, G=20, B=128. Expect close after JPEG round-trip.
	c := im.At(10, 20)
	r, g, b, _ := c.RGBA()
	got := color.RGBA{R: byte(r >> 8), G: byte(g >> 8), B: byte(b >> 8)}
	if abs(int(got.R)-10) > 8 || abs(int(got.G)-20) > 8 || abs(int(got.B)-128) > 8 {
		t.Errorf("pixel (10,20) round-trip drift too large: got %v", got)
	}
}

// spliceForDecode joins a tables-only JPEG (SOI + DQT + DHT + EOI) with an
// abbreviated tile (SOI + APP14 + SOF + SOS + entropy + EOI) into a
// self-contained JPEG by dropping the tables' EOI and the tile's SOI.
func spliceForDecode(tables, tile []byte) []byte {
	if !bytes.HasSuffix(tables, []byte{0xFF, 0xD9}) {
		return nil
	}
	if !bytes.HasPrefix(tile, []byte{0xFF, 0xD8}) {
		return nil
	}
	out := make([]byte, 0, len(tables)+len(tile)-4)
	out = append(out, tables[:len(tables)-2]...) // tables minus EOI
	out = append(out, tile[2:]...)               // tile minus SOI
	return out
}

func abs(x int) int { if x < 0 { return -x }; return x }
```

- [ ] **Step 2: Run, verify failure**

Compile error: `undefined: Factory`.

- [ ] **Step 3: Implement the JPEG codec wrapper**

Create `internal/codec/jpeg/jpeg.go`. This is the first cgo-bound file. The key responsibilities:

1. Compile against libjpeg-turbo (`#cgo pkg-config: libturbojpeg`) — but `tjCompress2` doesn't support abbreviated mode, so we use the lower-level **libjpeg API** (`jpeg_compress_struct`, `jpeg_suppress_tables`, `jpeg_write_marker`). Add `#cgo pkg-config: libjpeg` (libjpeg-turbo provides both pc files).
2. On `NewEncoder`: configure a `jpeg_compress_struct` for `tile_w × tile_h`, RGB input, write to a memory destination. Set quality from `Quality.Knobs["q"]` (default 85).
3. **Compute tables**: encode a probe tile (e.g., 8x8 zeroes) self-contained, run our `wsiwriter.ExtractJPEGTables` on the result, store the tables-only bytes as `LevelHeader`.
4. On `EncodeTile`:
   - Reset the compress struct to "abbreviated" mode: `jpeg_suppress_tables(&cinfo, TRUE)`, then `jpeg_start_compress(&cinfo, FALSE)` — the `FALSE` is the critical "don't write tables" flag.
   - Use `jpeg_write_marker(&cinfo, JPEG_APP0+14, adobeAPP14Bytes)` to inject the Aperio APP14 marker. **The APP14 byte sequence must match opentile-go's `internal/jpeg.adobeAPP14`** — read that file from the local opentile-go checkout (`/Users/cornish/GitHub/opentile-go/internal/jpeg/`) and copy the byte literal exactly. Do not transcribe from memory.
   - Feed RGB scanlines via `jpeg_write_scanlines`.
   - `jpeg_finish_compress` returns the abbreviated tile bytes.
5. `TIFFCompressionTag()` returns 7. `ExtraTIFFTags()` returns nil. `Close()` calls `jpeg_destroy_compress`.

Implementation skeleton:

```go
package jpeg

/*
#cgo pkg-config: libjpeg
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <jpeglib.h>

// Memory destination manager. libjpeg's jpeg_mem_dest exists in libjpeg-turbo;
// some old libjpeg versions don't have it. We rely on libjpeg-turbo (declared
// in the brew formula).
*/
import "C"
import (
	"fmt"
	"strconv"
	"unsafe"

	"github.com/cornish/wsi-tools/internal/codec"
	"github.com/cornish/wsi-tools/internal/wsiwriter"
)

// adobeAPP14 is the marker bytes Aperio prepends to declare RGB-not-YCbCr
// colourspace. Mirrors opentile-go's internal/jpeg.adobeAPP14 verbatim.
//
// Source: github.com/cornish/opentile-go/internal/jpeg/jpeg.go
// Copy the byte literal from there at implementation time; do NOT transcribe.
var adobeAPP14 = []byte{
	// FILL IN FROM opentile-go AT IMPLEMENTATION TIME
}

func init() {
	codec.Register(Factory{})
}

type Factory struct{}

func (Factory) Name() string { return "jpeg" }

func (Factory) NewEncoder(g codec.LevelGeometry, q codec.Quality) (codec.Encoder, error) {
	quality := 85
	if v, ok := q.Knobs["q"]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			quality = n
		}
	}
	enc := &Encoder{quality: quality, geom: g}
	if err := enc.computeTables(); err != nil {
		return nil, err
	}
	return enc, nil
}

type Encoder struct {
	quality int
	geom    codec.LevelGeometry
	tables  []byte // LevelHeader (tables-only JPEG)
}

func (e *Encoder) LevelHeader() []byte { return e.tables }

func (e *Encoder) computeTables() error {
	// Encode a small probe tile in self-contained mode, extract DQT/DHT.
	// (The actual cgo encode call lives in encodeRaw, defined below.)
	probe := make([]byte, e.geom.TileWidth*e.geom.TileHeight*3)
	full, err := e.encodeRaw(probe, e.geom.TileWidth, e.geom.TileHeight, false /* abbrev */)
	if err != nil {
		return err
	}
	tables, err := wsiwriter.ExtractJPEGTables(full)
	if err != nil {
		return err
	}
	e.tables = tables
	return nil
}

func (e *Encoder) EncodeTile(rgb []byte, w, h int, dst []byte) ([]byte, error) {
	out, err := e.encodeRaw(rgb, w, h, true /* abbrev */)
	if err != nil {
		return nil, err
	}
	// dst handling: if caller passed a dst with len(dst) >= len(out), copy in.
	if dst != nil && cap(dst) >= len(out) {
		dst = dst[:len(out)]
		copy(dst, out)
		return dst, nil
	}
	return out, nil
}

func (e *Encoder) TIFFCompressionTag() uint16 {
	return wsiwriter.CompressionJPEG
}

func (e *Encoder) ExtraTIFFTags() []wsiwriter.TIFFTag {
	return nil
}

func (e *Encoder) Close() error { return nil }

// encodeRaw drives the cgo libjpeg encode loop. abbreviated=true selects
// jpeg_start_compress(&cinfo, FALSE) and jpeg_suppress_tables, and prepends
// the Aperio APP14 marker.
func (e *Encoder) encodeRaw(rgb []byte, w, h int, abbreviated bool) ([]byte, error) {
	// Implementation: standard libjpeg compress loop. Key cgo calls:
	//   jpeg_create_compress / jpeg_destroy_compress
	//   jpeg_mem_dest(&cinfo, &out_buf, &out_size)
	//   cinfo.image_width/height/components/in_color_space = JCS_RGB
	//   jpeg_set_defaults; jpeg_set_quality(&cinfo, quality, TRUE)
	//   if abbreviated: jpeg_suppress_tables(&cinfo, TRUE)
	//   jpeg_start_compress(&cinfo, !abbreviated)
	//   if abbreviated: jpeg_write_marker(&cinfo, JPEG_APP0+14, adobeAPP14, len)
	//   jpeg_write_scanlines in a loop
	//   jpeg_finish_compress
	// Returns the bytes from out_buf with C.GoBytes.
	return nil, fmt.Errorf("TODO Task 14: implement encodeRaw")
}

// Helper: pin a Go slice for cgo. unsafe.SliceData(rgb) passed to C must be
// kept alive until the libjpeg call returns; in practice, since the call is
// synchronous within encodeRaw, runtime.KeepAlive(rgb) at the end suffices.
var _ = unsafe.Pointer(nil) // silence import if not used; remove when implementing
```

Reference for the byte-exact APP14 marker: read it now.

- [ ] **Step 4: Read the APP14 bytes from opentile-go and paste verbatim**

```bash
grep -A 20 "adobeAPP14" /Users/cornish/GitHub/opentile-go/internal/jpeg/*.go | head -40
```

Copy the exact `[]byte{...}` literal into `adobeAPP14` in `jpeg.go`.

- [ ] **Step 5: Implement encodeRaw**

Standard libjpeg memory-destination compress loop with `jpeg_suppress_tables` + `jpeg_start_compress(&cinfo, FALSE)` + `jpeg_write_marker(JPEG_APP0+14, ...)` for abbreviated mode. Reference: `libjpeg.txt` in the libjpeg-turbo source tree, sections "Suspending data sources and destinations" and "Abbreviated datastreams and multiple images."

Implementation gotcha: APP14 must be written **after** `jpeg_start_compress` and **before** the first `jpeg_write_scanlines` call.

- [ ] **Step 6: Wire the umbrella registration package**

Create `internal/codec/all/all.go`:

```go
// Package all exists solely to import every codec subpackage so they register
// themselves with the codec registry on import. Application binaries (cmd/wsi-tools)
// blank-import this package once; they should not reach into individual codec
// subpackages.
package all

import (
	_ "github.com/cornish/wsi-tools/internal/codec/jpeg"
)
```

- [ ] **Step 7: Run the test**

```bash
go test ./internal/codec/jpeg/ -race -count=1 -v
```

Expected: PASS. PSNR drift on the spot-check pixel should be well within the ±8/255 tolerance.

- [ ] **Step 8: Commit**

```bash
git add internal/codec/jpeg/ internal/codec/all/
git commit -m "feat(codec/jpeg): libjpeg-turbo encoder with abbreviated tiles + APP14"
```

---

### Task 15: Replace synthetic SVS test with real JPEG-tiled output (re-enables opentile-go round-trip)

**Files:**
- Modify: `internal/wsiwriter/svs_test.go`

- [ ] **Step 1: Replace the deferred test from Task 11**

Replace `TestSyntheticSVSRoundTrip` to use the JPEG codec wrapper, and assert opentile-go reads it back successfully:

```go
import (
	jpegcodec "github.com/cornish/wsi-tools/internal/codec/jpeg"
	codec "github.com/cornish/wsi-tools/internal/codec"
)

func TestSyntheticSVSRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "synth.svs")
	desc := `Aperio Image Library v12.0.15
512x512 [0,0 512x512] (256x256) JPEG/RGB Q=80|AppMag = 40|MPP = 0.25|Filename = synth`

	// Build a JPEG encoder for 256x256 tiles.
	enc, err := jpegcodec.Factory{}.NewEncoder(codec.LevelGeometry{
		TileWidth: 256, TileHeight: 256, PixelFormat: codec.PixelFormatRGB8,
	}, codec.Quality{Knobs: map[string]string{"q": "80"}})
	if err != nil {
		t.Fatal(err)
	}
	defer enc.Close()
	tables := enc.LevelHeader()

	w, err := Create(path, WithBigTIFF(false), WithImageDescription(desc))
	if err != nil {
		t.Fatal(err)
	}

	// L0: 512x512, 4 tiles of 256x256.
	l0, _ := w.AddLevel(LevelSpec{
		ImageWidth: 512, ImageHeight: 512,
		TileWidth: 256, TileHeight: 256,
		Compression: CompressionJPEG, PhotometricInterpretation: 2 /* RGB */,
		JPEGTables: tables, JPEGAbbreviatedTiles: true,
		NewSubfileType: 0,
	})
	tile := make([]byte, 256*256*3)
	encoded, _ := enc.EncodeTile(tile, 256, 256, nil)
	for ty := uint32(0); ty < 2; ty++ {
		for tx := uint32(0); tx < 2; tx++ {
			l0.WriteTile(tx, ty, encoded)
		}
	}
	// L1: 256x256, 1 tile.
	l1, _ := w.AddLevel(LevelSpec{
		ImageWidth: 256, ImageHeight: 256,
		TileWidth: 256, TileHeight: 256,
		Compression: CompressionJPEG, PhotometricInterpretation: 2,
		JPEGTables: tables, JPEGAbbreviatedTiles: true,
		NewSubfileType: 1,
	})
	l1.WriteTile(0, 0, encoded)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	// Re-open via opentile-go.
	tlr, err := opentile.OpenFile(path)
	if err != nil {
		t.Fatalf("opentile.OpenFile: %v", err)
	}
	defer tlr.Close()
	if got, want := tlr.Format(), "svs"; got != want {
		t.Errorf("Format: got %q, want %q", got, want)
	}
	if got, want := len(tlr.Levels()), 2; got != want {
		t.Errorf("Levels: got %d, want %d", got, want)
	}
	// Decode L0 tile (0,0) by reading it back through opentile-go.
	got, err := tlr.Levels()[0].Tile(0, 0)
	if err != nil {
		t.Fatalf("Tile: %v", err)
	}
	if len(got) == 0 {
		t.Errorf("empty tile bytes")
	}
}
```

- [ ] **Step 2: Run; iterate on writer fixes until PASS**

```bash
go test ./internal/wsiwriter/ -run TestSyntheticSVSRoundTrip -race -count=1 -v
```

Expected: PASS. Common iteration fixes: emit JPEGTables (tag 347) for JPEG levels, set PhotometricInterpretation correctly (Aperio uses 2/RGB despite YCbCr being typical for JPEG-in-TIFF — APP14 marker disambiguates), correct NewSubfileType chain.

- [ ] **Step 3: Commit**

```bash
git add internal/wsiwriter/svs_test.go
git commit -m "test(wsiwriter): real JPEG-tiled SVS round-trips via opentile-go"
```

---

## Batch E — Decoder + resampler (3 tasks)

### Task 16: JPEG decoder wrapper with libjpeg-turbo + 1/N fast-scale

**Files:**
- Create: `internal/decoder/decoder.go`
- Create: `internal/decoder/jpeg.go`
- Create: `internal/decoder/jpeg_test.go`

- [ ] **Step 1: Failing test**

```go
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
```

- [ ] **Step 2: Run, verify failure**

- [ ] **Step 3: Implement decoder.go + jpeg.go**

Create `internal/decoder/decoder.go`:

```go
package decoder

type Decoder interface {
	// DecodeTile decodes compressed bytes into RGB888 packed bytes.
	// scaleNum/scaleDen selects libjpeg-turbo's in-decode fast scale where
	// available (1/1, 1/2, 1/4, 1/8). Other decoders ignore them.
	// dst is an optional output buffer; if cap(dst) is large enough, decode
	// in-place and return dst; otherwise return a fresh slice.
	DecodeTile(compressed []byte, dst []byte, scaleNum, scaleDen int) ([]byte, error)
}
```

Create `internal/decoder/jpeg.go`:

```go
package decoder

/*
#cgo pkg-config: libturbojpeg
#include <stdint.h>
#include <stdlib.h>
#include <turbojpeg.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type JPEG struct{}

func NewJPEG() *JPEG { return &JPEG{} }

func (*JPEG) DecodeTile(compressed []byte, dst []byte, scaleNum, scaleDen int) ([]byte, error) {
	handle := C.tjInitDecompress()
	if handle == nil {
		return nil, fmt.Errorf("decoder/jpeg: tjInitDecompress failed")
	}
	defer C.tjDestroy(handle)

	// Read JPEG header to get full-resolution dimensions.
	var width, height, subsamp, colorspace C.int
	if rc := C.tjDecompressHeader3(handle,
		(*C.uchar)(unsafe.Pointer(&compressed[0])),
		C.ulong(len(compressed)),
		&width, &height, &subsamp, &colorspace); rc != 0 {
		return nil, fmt.Errorf("decoder/jpeg: tjDecompressHeader3: %s", C.GoString(C.tjGetErrorStr2(handle)))
	}
	// Compute scaled output size. tjDecompress2 supports a fixed set of
	// scaling factors via tjGetScalingFactors; for 1/N where N ∈ {1,2,4,8},
	// the formula is ceil(W * num / den).
	outW := int((int(width)*scaleNum + scaleDen - 1) / scaleDen)
	outH := int((int(height)*scaleNum + scaleDen - 1) / scaleDen)
	need := outW * outH * 3
	if cap(dst) < need {
		dst = make([]byte, need)
	} else {
		dst = dst[:need]
	}
	// Note: tjDecompress2 doesn't take scale numerator/denominator directly;
	// instead, libjpeg-turbo applies the scaling factor matching outW/outH.
	// Pass desiredWidth=outW, desiredHeight=outH; turbojpeg picks the closest
	// supported factor and decodes to that size.
	if rc := C.tjDecompress2(handle,
		(*C.uchar)(unsafe.Pointer(&compressed[0])),
		C.ulong(len(compressed)),
		(*C.uchar)(unsafe.Pointer(&dst[0])),
		C.int(outW), 0 /* pitch=0 = packed */, C.int(outH),
		C.TJPF_RGB, 0); rc != 0 {
		return nil, fmt.Errorf("decoder/jpeg: tjDecompress2: %s", C.GoString(C.tjGetErrorStr2(handle)))
	}
	return dst, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/decoder/ -race -count=1 -v
```

Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/decoder/
git commit -m "feat(decoder/jpeg): libjpeg-turbo decoder with 1/N fast-scale"
```

---

### Task 17: 2x2 area-average resampler

**Files:**
- Create: `internal/resample/area.go`
- Create: `internal/resample/area_test.go`

- [ ] **Step 1: Failing test**

```go
package resample

import (
	"testing"
)

func TestAreaAverage2x2(t *testing.T) {
	// 4x4 source: each 2x2 block has known mean.
	src := []byte{
		// row 0
		10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120,
		// row 1
		10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120,
		// row 2
		130, 140, 150, 160, 170, 180, 190, 200, 210, 220, 230, 240,
		// row 3
		130, 140, 150, 160, 170, 180, 190, 200, 210, 220, 230, 240,
	}
	// Expected 2x2 output: each pixel is the average of the 2x2 block above.
	dst, err := Area2x2(src, 4, 4)
	if err != nil {
		t.Fatalf("Area2x2: %v", err)
	}
	want := []byte{
		// (0,0): avg of src(0,0),(0,1),(1,0),(1,1) per channel
		10, 20, 30,
		// (0,1): avg of src(0,2),(0,3),(1,2),(1,3)
		// All are (50,60,70,80,90,100,110,120) repeated -> mean (70, 80, 90)... let me think.
		// Actually each pixel is 3 bytes: src(0,2) = (50,60,70), src(0,3) = (80,90,100),
		//                                src(1,2) = (50,60,70), src(1,3) = (80,90,100)
		// Avg R = (50+80+50+80)/4 = 65
		// Avg G = (60+90+60+90)/4 = 75
		// Avg B = (70+100+70+100)/4 = 85
		// Wait — I packed the src wrong above. Re-pack with 3 bytes per pixel × 4 pixels per row.
	}
	_ = want
	if len(dst) != 2*2*3 {
		t.Errorf("dst len: got %d, want %d", len(dst), 2*2*3)
	}
}
```

(Rewrite the expected values precisely; this skeleton just checks length first. Use a smaller input — say 2x2 → 1x1 — for the value-correctness test where it's easy to verify by hand.)

```go
func TestAreaAverage_SimpleCorrectness(t *testing.T) {
	// 2x2 source, 1 pixel output.
	// Pixels: (10,20,30), (40,50,60), (70,80,90), (100,110,120)
	src := []byte{10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120}
	dst, err := Area2x2(src, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	// avg R = (10+40+70+100)/4 = 55
	// avg G = (20+50+80+110)/4 = 65
	// avg B = (30+60+90+120)/4 = 75
	if dst[0] != 55 || dst[1] != 65 || dst[2] != 75 {
		t.Errorf("got %v, want [55 65 75]", dst[:3])
	}
}
```

- [ ] **Step 2: Run, verify failure**

- [ ] **Step 3: Implement area.go**

```go
// Package resample provides image resampling primitives. v0.1 only ships
// 2x2 area-average; lanczos is a stub for v0.2.
package resample

import "fmt"

// Area2x2 produces an RGB888-packed image at half each dimension of src using
// 2x2 area averaging. src must be RGB888-packed and (srcW, srcH) must both be
// even.
func Area2x2(src []byte, srcW, srcH int) ([]byte, error) {
	if srcW%2 != 0 || srcH%2 != 0 {
		return nil, fmt.Errorf("resample: srcW and srcH must be even, got %dx%d", srcW, srcH)
	}
	if len(src) != srcW*srcH*3 {
		return nil, fmt.Errorf("resample: src length %d != %d*%d*3", len(src), srcW, srcH)
	}
	dstW := srcW / 2
	dstH := srcH / 2
	dst := make([]byte, dstW*dstH*3)
	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			sx := dx * 2
			sy := dy * 2
			i00 := (sy*srcW + sx) * 3
			i01 := (sy*srcW + sx + 1) * 3
			i10 := ((sy+1)*srcW + sx) * 3
			i11 := ((sy+1)*srcW + sx + 1) * 3
			di := (dy*dstW + dx) * 3
			for c := 0; c < 3; c++ {
				sum := uint(src[i00+c]) + uint(src[i01+c]) + uint(src[i10+c]) + uint(src[i11+c])
				dst[di+c] = byte((sum + 2) / 4) // round-to-nearest with +2 offset
			}
		}
	}
	return dst, nil
}
```

- [ ] **Step 4: Run**

```bash
go test ./internal/resample/ -race -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Add Lanczos stub**

Per the spec, `internal/resample/lanczos.go` ships at v0.1 as a stub returning `ErrNotImplemented`, with the real libvips wrapper deferred to v0.2. Create `internal/resample/lanczos.go`:

```go
package resample

import "errors"

// ErrNotImplemented is returned by Lanczos at v0.1. The real libvips-backed
// implementation lands in v0.2 alongside arbitrary-factor downsampling.
var ErrNotImplemented = errors.New("resample: lanczos not implemented at v0.1 (use Area2x2 or set --resampler area)")

// Lanczos is the v0.2 entry point for non-power-of-2 downsampling. v0.1 returns
// ErrNotImplemented; the CLI rejects non-power-of-2 factors before reaching here.
func Lanczos(src []byte, srcW, srcH, dstW, dstH int) ([]byte, error) {
	return nil, ErrNotImplemented
}
```

- [ ] **Step 6: Commit**

```bash
git add internal/resample/
git commit -m "feat(resample): 2x2 area-average + lanczos stub"
```

---

### Task 18: JPEG2000 decoder via OpenJPEG

**Files:**
- Create: `internal/decoder/jpeg2000.go`
- Create: `internal/decoder/jpeg2000_test.go`

- [ ] **Step 1: Test — round-trip a known JP2K-encoded SVS tile through the decoder**

Use opentile-go to extract a tile from `sample_files/svs/JP2K-33003-1.svs`, decode via our wrapper, assert dimensions:

```go
package decoder

import (
	"os"
	"path/filepath"
	"testing"

	opentile "github.com/cornish/opentile-go"
	_ "github.com/cornish/opentile-go/formats/all"
)

func TestJPEG2000Decoder(t *testing.T) {
	testdir := os.Getenv("WSI_TOOLS_TESTDIR")
	if testdir == "" {
		testdir = "../../sample_files"
	}
	src := filepath.Join(testdir, "svs", "JP2K-33003-1.svs")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	tlr, err := opentile.OpenFile(src)
	if err != nil {
		t.Fatal(err)
	}
	defer tlr.Close()
	l0 := tlr.Levels()[0]
	tile, err := l0.Tile(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	tw, th := l0.TileSize().X, l0.TileSize().Y

	d := NewJPEG2000()
	dst, err := d.DecodeTile(tile, nil, 1, 1)
	if err != nil {
		t.Fatalf("DecodeTile: %v", err)
	}
	if len(dst) != tw*th*3 {
		t.Errorf("decoded length: got %d, want %d", len(dst), tw*th*3)
	}
}
```

- [ ] **Step 2: Run, verify failure**

- [ ] **Step 3: Implement jpeg2000.go**

```go
package decoder

/*
#cgo pkg-config: libopenjp2
#include <stdint.h>
#include <stdlib.h>
#include <openjpeg-2.5/openjpeg.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

type JPEG2000 struct{}

func NewJPEG2000() *JPEG2000 { return &JPEG2000{} }

func (*JPEG2000) DecodeTile(compressed []byte, dst []byte, scaleNum, scaleDen int) ([]byte, error) {
	if scaleNum != scaleDen {
		return nil, fmt.Errorf("decoder/jpeg2000: fast-scale not supported")
	}
	// OpenJPEG decode via memory stream:
	//   opj_stream_create_buffer_stream / opj_create_decompress / opj_setup_decoder
	//   opj_read_header / opj_decode / opj_end_decompress
	// Then convert opj_image_t YCbCr planes to packed RGB888.
	// Reference: openjpeg/test_decompress.c in the OpenJPEG source.
	return nil, fmt.Errorf("TODO Task 18 Step 3: implement OpenJPEG decode loop")
}
```

Implementation reference: opentile-go's `formats/svs/` reads JP2K SVS tiles as raw passthrough — opentile-go itself has no decode side. The OpenJPEG memory-stream pattern is documented at https://github.com/uclouvain/openjpeg/blob/master/src/lib/openjp2/openjpeg.h (search for `opj_stream_create_buffer_stream`). For a working Go cgo example to crib from, look at `chai2010/webp` or any openjpeg cgo binding on GitHub.

The Aperio JP2K subtypes (33003 = YCbCr, 33005 = RGB) require care: when decoded, the result must be converted to packed RGB. OpenJPEG returns separate component arrays in `opj_image_t.comps[0..N]`; pack them into RGB888.

- [ ] **Step 4: Run, verify pass**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files go test ./internal/decoder/ -race -count=1 -v
```

Expected: PASS (skips JP2K test if fixture missing).

- [ ] **Step 5: Commit**

```bash
git add internal/decoder/jpeg2000.go internal/decoder/jpeg2000_test.go
git commit -m "feat(decoder/jpeg2000): OpenJPEG decoder for Aperio JP2K SVS sources"
```

---

## Batch F — Pipeline (2 tasks)

### Task 19: Pipeline worker pool with synthetic source/sink

**Files:**
- Create: `internal/pipeline/pipeline.go`
- Create: `internal/pipeline/pipeline_test.go`

- [ ] **Step 1: Failing test**

```go
package pipeline

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestPipelineHappyPath(t *testing.T) {
	// 100 synthetic tiles. Source emits monotonic indices; processor doubles
	// the value; sink sums everything.
	const N = 100

	var sourceCalls, sinkCalls atomic.Int64
	src := func(ctx context.Context, emit func(Tile) error) error {
		for i := 0; i < N; i++ {
			sourceCalls.Add(1)
			if err := emit(Tile{Level: 0, X: uint32(i), Y: 0, Bytes: []byte{byte(i)}}); err != nil {
				return err
			}
		}
		return nil
	}
	proc := func(t Tile) (Tile, error) {
		t.Bytes = []byte{t.Bytes[0] * 2}
		return t, nil
	}
	var sum atomic.Int64
	sink := func(t Tile) error {
		sinkCalls.Add(1)
		sum.Add(int64(t.Bytes[0]))
		return nil
	}

	if err := Run(context.Background(), Config{Workers: 4, Source: src, Process: proc, Sink: sink}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, want := sourceCalls.Load(), int64(N); got != want {
		t.Errorf("source calls: got %d, want %d", got, want)
	}
	if got, want := sinkCalls.Load(), int64(N); got != want {
		t.Errorf("sink calls: got %d, want %d", got, want)
	}
	// sum of (i*2) for i in [0..99] = 2 * (99*100/2) = 9900 (mod 256 since byte wraps).
	// Just check non-zero.
	if sum.Load() == 0 {
		t.Errorf("sum is zero")
	}
}

func TestPipelineErrorCancelsRun(t *testing.T) {
	wantErr := errors.New("boom")
	src := func(ctx context.Context, emit func(Tile) error) error {
		for i := 0; i < 100; i++ {
			if err := emit(Tile{X: uint32(i)}); err != nil {
				return err
			}
		}
		return nil
	}
	proc := func(t Tile) (Tile, error) {
		if t.X == 50 {
			return Tile{}, wantErr
		}
		return t, nil
	}
	sink := func(t Tile) error { return nil }
	err := Run(context.Background(), Config{Workers: 4, Source: src, Process: proc, Sink: sink})
	if !errors.Is(err, wantErr) {
		t.Errorf("err: got %v, want %v", err, wantErr)
	}
}
```

- [ ] **Step 2: Implement pipeline.go**

```go
// Package pipeline runs decode → process → encode tile workflows over a worker
// pool, with cancellation, backpressure, and atomic sink semantics.
package pipeline

import (
	"context"
	"sync"
)

// Tile is the unit of work flowing through the pipeline.
type Tile struct {
	Level int
	X, Y  uint32
	Bytes []byte
}

type SourceFn func(ctx context.Context, emit func(Tile) error) error
type ProcessFn func(Tile) (Tile, error)
type SinkFn func(Tile) error

type Config struct {
	Workers int
	Source  SourceFn
	Process ProcessFn
	Sink    SinkFn
}

// Run drives the pipeline. The Source goroutine emits tiles into a buffered
// channel of size Workers*2; Workers process goroutines transform tiles in
// parallel; a single Sink goroutine receives processed tiles serially. The
// first error from any goroutine cancels the context and Run returns that
// error after draining.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	in := make(chan Tile, cfg.Workers*2)
	out := make(chan Tile, cfg.Workers*2)

	var wg sync.WaitGroup

	// Source.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(in)
		err := cfg.Source(ctx, func(t Tile) error {
			select {
			case in <- t:
				return nil
			case <-ctx.Done():
				return context.Cause(ctx)
			}
		})
		if err != nil && context.Cause(ctx) == nil {
			cancel(err)
		}
	}()

	// Process workers.
	var workWG sync.WaitGroup
	workWG.Add(cfg.Workers)
	for i := 0; i < cfg.Workers; i++ {
		go func() {
			defer workWG.Done()
			for t := range in {
				out2, err := cfg.Process(t)
				if err != nil {
					cancel(err)
					return
				}
				select {
				case out <- out2:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		workWG.Wait()
		close(out)
	}()

	// Sink.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for t := range out {
			if err := cfg.Sink(t); err != nil {
				cancel(err)
				return
			}
		}
	}()

	wg.Wait()
	if err := context.Cause(ctx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}
```

- [ ] **Step 3: Run, verify pass**

```bash
go test ./internal/pipeline/ -race -count=1 -v
```

Expected: both PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/pipeline/
git commit -m "feat(pipeline): worker-pool decode/process/encode with cancellation"
```

---

## Batch G — CLI shell + downsample subcommand (5 tasks)

### Task 20: cobra root + version subcommand

**Files:**
- Create: `cmd/wsi-tools/main.go`
- Create: `cmd/wsi-tools/version.go`

- [ ] **Step 1: Implement main.go + version.go**

`main.go`:

```go
package main

import (
	"fmt"
	"os"

	_ "github.com/cornish/wsi-tools/internal/codec/all"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wsi-tools",
	Short: "Utilities for whole-slide imaging (WSI) files",
	Long: `wsi-tools — a Swiss-army knife for whole-slide imaging files used in digital pathology.

Run 'wsi-tools <command> --help' for command-specific flags and examples.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
```

`version.go`:

```go
package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

const Version = "0.1.0-dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print wsi-tools version + build info",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("wsi-tools %s\n", Version)
		fmt.Printf("go %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return nil
	},
}

func init() { rootCmd.AddCommand(versionCmd) }
```

- [ ] **Step 2: Build + smoke test**

```bash
go build -o /tmp/wsi-tools ./cmd/wsi-tools
/tmp/wsi-tools version
```

Expected: prints version + go info.

- [ ] **Step 3: Commit**

```bash
git add cmd/wsi-tools/main.go cmd/wsi-tools/version.go
git commit -m "feat(cli): cobra root + version subcommand"
```

---

### Task 21: doctor subcommand

**Files:**
- Create: `cmd/wsi-tools/doctor.go`

- [ ] **Step 1: Implement doctor.go**

```go
package main

import (
	"fmt"

	codec "github.com/cornish/wsi-tools/internal/codec"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Report installed codec libraries + version info",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("wsi-tools", Version, "— codec / library health check.")
		fmt.Println()
		fmt.Println("Registered codecs:")
		for _, name := range codec.List() {
			fmt.Printf("  %s\n", name)
		}
		fmt.Println()
		fmt.Println("Required libs (probed at link time, not runtime):")
		fmt.Println("  libjpeg-turbo")
		fmt.Println("  libopenjp2")
		fmt.Println("  github.com/cornish/opentile-go")
		return nil
	},
}

func init() { rootCmd.AddCommand(doctorCmd) }
```

(v0.1 doctor is intentionally a thin stub — it lists the registered codecs and required libs. Per-codec runtime probe with version detection lands in v0.2 alongside the rest of the codec wrappers, where the matrix gets meaningful.)

- [ ] **Step 2: Smoke test**

```bash
go build -o /tmp/wsi-tools ./cmd/wsi-tools
/tmp/wsi-tools doctor
```

Expected: prints `Registered codecs: \n   jpeg`.

- [ ] **Step 3: Commit**

```bash
git add cmd/wsi-tools/doctor.go
git commit -m "feat(cli): doctor subcommand listing registered codecs"
```

---

### Task 22: downsample subcommand wiring (the main flow)

**Files:**
- Create: `cmd/wsi-tools/downsample.go`

- [ ] **Step 1: Implement downsample.go**

This is the heart of the tool. It orchestrates opentile-go → decode → resample → encode → wsiwriter through the pipeline.

```go
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"

	opentile "github.com/cornish/opentile-go"
	_ "github.com/cornish/opentile-go/formats/all"
	svsfmt "github.com/cornish/opentile-go/formats/svs"

	codec "github.com/cornish/wsi-tools/internal/codec"
	"github.com/cornish/wsi-tools/internal/decoder"
	"github.com/cornish/wsi-tools/internal/pipeline"
	"github.com/cornish/wsi-tools/internal/resample"
	"github.com/cornish/wsi-tools/internal/wsiwriter"
	"github.com/spf13/cobra"
)

var (
	dsOutput   string
	dsFactor   int
	dsTargetMag int
	dsQuality  int
	dsJobs     int
	dsForce    bool
)

var downsampleCmd = &cobra.Command{
	Use:   "downsample [flags] <input>",
	Short: "Downsample a WSI by a power-of-2 factor",
	Long: `Downsample a WSI by an integer power-of-2 factor (default 2 = 40x → 20x).
Regenerates the entire pyramid from the new L0; passes through associated
images (label, macro, thumbnail, overview) verbatim.

v0.1 supports SVS sources only.

Examples:

  # 40x → 20x SVS (defaults)
  wsi-tools downsample -o slide-20x.svs slide-40x.svs

  # 40x → 10x at higher quality, 8 workers
  wsi-tools downsample --factor 4 --quality 95 --jobs 8 -o out.svs in.svs`,
	Args: cobra.ExactArgs(1),
	RunE: runDownsample,
}

func init() {
	downsampleCmd.Flags().StringVarP(&dsOutput, "output", "o", "", "output file path (required)")
	downsampleCmd.Flags().IntVar(&dsFactor, "factor", 2, "downsample factor (must be a power of 2 ∈ {2,4,8,16})")
	downsampleCmd.Flags().IntVar(&dsTargetMag, "target-mag", 0, "alternative to --factor: derive factor from source AppMag")
	downsampleCmd.Flags().IntVar(&dsQuality, "quality", 90, "JPEG quality 1..100")
	downsampleCmd.Flags().IntVar(&dsJobs, "jobs", runtime.NumCPU(), "worker goroutines")
	downsampleCmd.Flags().BoolVarP(&dsForce, "force", "f", false, "overwrite output if it exists")
	downsampleCmd.MarkFlagRequired("output")
	rootCmd.AddCommand(downsampleCmd)
}

func runDownsample(cmd *cobra.Command, args []string) error {
	input := args[0]

	// 1. Validate inputs.
	if _, err := os.Stat(input); err != nil {
		return fmt.Errorf("input %s: %w", input, err)
	}
	if !dsForce {
		if _, err := os.Stat(dsOutput); err == nil {
			return fmt.Errorf("output %s already exists (use --force)", dsOutput)
		}
	}
	if dsQuality < 1 || dsQuality > 100 {
		return fmt.Errorf("quality must be 1..100")
	}
	if !isPowerOf2(dsFactor) || dsFactor < 2 || dsFactor > 16 {
		return fmt.Errorf("factor must be a power of 2 in {2,4,8,16}")
	}

	// 2. Open source via opentile-go.
	src, err := opentile.OpenFile(input)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer src.Close()
	if src.Format() != "svs" {
		return fmt.Errorf("v0.1 supports SVS only, got %q", src.Format())
	}

	// 3. Resolve --target-mag if set.
	if dsTargetMag != 0 {
		md, ok := svsfmt.MetadataOf(src)
		if !ok {
			return fmt.Errorf("--target-mag set but source has no SVS metadata")
		}
		// Derive factor = round(srcMag / targetMag).
		srcMag := md.AppMag // adjust per actual field name from Task 3 probe
		f := int(math.Round(srcMag / float64(dsTargetMag)))
		if !isPowerOf2(f) {
			return fmt.Errorf("derived factor %d (= %g/%d) is not a power of 2", f, srcMag, dsTargetMag)
		}
		dsFactor = f
	}

	// 4. Read + mutate source ImageDescription.
	//
	// IMPORTANT — access pattern needs verification at implementation time:
	// opentile-go's Tiler interface (per docs/formats/svs.md) does NOT expose
	// the raw ImageDescription string directly. Access paths to evaluate:
	//   (a) svs.MetadataOf(src) returns *svs.Metadata with MPP, SoftwareLine,
	//       Filename — but probably not the full raw string. Check the field
	//       set by reading internal/wsiwriter/svs.go's ParseImageDescription
	//       requirements: we need the FULL pipe-separated key=value list.
	//   (b) Read the source TIFF's L0 IFD tag 270 directly via a small helper
	//       that re-opens the file with golang.org/x/image/tiff or by using
	//       opentile-go's internal/tiff parser if exported.
	//   (c) Add a Tiler.ImageDescription() (or per-format equivalent) method
	//       upstream in opentile-go and consume it here.
	// Recommendation: try (a) first; if svs.Metadata exposes the raw string
	// (e.g., as a Raw or Description field), use it. Otherwise implement (b)
	// inline as a helper in cmd/wsi-tools/downsample.go: open input as a
	// TIFF, read L0 IFD tag 270, return the string. Don't add a method to
	// opentile-go in v0.1 of wsi-tools — that's a cross-repo change.
	srcDesc, err := readSourceImageDescription(input) // helper to be implemented per recommendation above
	if err != nil {
		return fmt.Errorf("read source ImageDescription: %w", err)
	}
	d, err := wsiwriter.ParseImageDescription(srcDesc)
	if err != nil {
		return fmt.Errorf("parse ImageDescription: %w", err)
	}
	srcL0 := src.Levels()[0]
	srcW, srcH := uint32(srcL0.Size().X), uint32(srcL0.Size().Y)
	newW := srcW / uint32(dsFactor)
	newH := srcH / uint32(dsFactor)
	d.MutateForDownsample(dsFactor, newW, newH)

	// 5. Open writer.
	bigtiff := srcL0.Size().X*srcL0.Size().Y*3 > 2<<30
	w, err := wsiwriter.Create(dsOutput,
		wsiwriter.WithBigTIFF(bigtiff),
		wsiwriter.WithImageDescription(d.Encode()))
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}

	// 6. For each source level, build a corresponding output level.
	//    L0 = downsample-factor of source L0.
	//    L1+ = downsample-factor of previous output level (so output ratio
	//          chain == source ratio chain).
	//    For v0.1 simplicity, we re-decode source L0 once per output level
	//    rather than caching, since the worker pool is the bottleneck.
	//    Optimisation can come later.
	if err := buildPyramid(cmd.Context(), src, w, dsFactor, dsQuality, dsJobs); err != nil {
		w.Close() // tmp removed by Close
		return err
	}

	// 7. Pass-through associated images.
	for _, a := range src.Associated() {
		bs, err := a.Bytes()
		if err != nil {
			return fmt.Errorf("associated %s: %w", a.Kind(), err)
		}
		// NewSubfileType: Aperio convention pinned at implementation time.
		// For v0.1, use 1 for all (reduced-res); fixture-based test will
		// flag if specific Kinds need different values.
		if err := w.AddAssociated(wsiwriter.AssociatedSpec{
			Kind:           a.Kind(),
			Compressed:     bs,
			Width:          uint32(a.Size().X),
			Height:         uint32(a.Size().Y),
			Compression:    mapAssociatedCompression(a.Compression()),
			NewSubfileType: 1,
		}); err != nil {
			return fmt.Errorf("add associated %s: %w", a.Kind(), err)
		}
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	fmt.Printf("wrote %s\n", dsOutput)
	return nil
}

func isPowerOf2(n int) bool {
	return n > 0 && (n&(n-1)) == 0
}

// buildPyramid decodes source L0, downsamples to produce output L0, then
// downsamples each subsequent output level from the previous one. For each
// output level, tiles flow through the pipeline.
func buildPyramid(ctx context.Context, src opentile.Tiler, w *wsiwriter.Writer, factor, quality, workers int) error {
	// Pseudocode at the level of Task 22 — actual impl gets fleshed out
	// alongside Task 23 (integration test). The shape is:
	//
	// for outLvl := range src.Levels():
	//     if outLvl == 0:
	//         srcLvl = 0
	//         decodeFrom = "source"
	//     else:
	//         decodeFrom = "previous output level (memory-resident raster)"
	//
	//     enc = jpegFactory.NewEncoder(...)
	//     levelHandle, _ = w.AddLevel(LevelSpec{
	//         ImageWidth: srcW/factor^outLvl, ImageHeight: srcH/factor^outLvl,
	//         TileWidth: 256, TileHeight: 256,
	//         Compression: CompressionJPEG, JPEGTables: enc.LevelHeader(),
	//         JPEGAbbreviatedTiles: true, NewSubfileType: 0, // ALL levels — see Task 11 lesson.
	//     })
	//
	//     pipeline.Run(ctx, pipeline.Config{
	//         Workers: workers,
	//         Source: emit each (tx,ty) coord with its raw decoded RGB tile,
	//         Process: enc.EncodeTile(rgb, tw, th, dst),
	//         Sink: levelHandle.WriteTile(tx, ty, encoded),
	//     })
	//
	// L1+ generation needs the L0 raster materialised. For v0.1, hold L0
	// fully in memory (RAM ceiling: a 20x slide is ~50K x 50K x 3 = 7.5 GB
	// max, which is tight but typically fits on a dev machine). v0.2 adds
	// streaming or disk-backed level chaining.
	return fmt.Errorf("TODO Task 22 Step 1: implement buildPyramid")
}

func mapAssociatedCompression(c opentile.Compression) uint16 {
	switch c {
	case opentile.CompressionJPEG:
		return wsiwriter.CompressionJPEG
	case opentile.CompressionLZW:
		return wsiwriter.CompressionLZW
	default:
		return wsiwriter.CompressionNone
	}
}

// Suppress unused imports while implementation is stubbed.
var _ = decoder.NewJPEG
var _ = resample.Area2x2
var _ = pipeline.Run
var _ = filepath.Join
var _ codec.Encoder
```

The `buildPyramid` function is the most important piece of the tool. Its v0.1 implementation:

1. Pyramid is built top-down (L0 first), each level fully materialised in memory before writing.
2. For L0: decode each source-L0 tile via `decoder.NewJPEG().DecodeTile(src, dst, 1, factor)` (using libjpeg-turbo's 1/N fast scale where applicable; falls back to full decode + `resample.Area2x2` otherwise). The decoded RGB is placed at the appropriate position in an in-memory L0 raster.
3. Once L0 raster is fully materialised, run a second pipeline pass over it: split into 256x256 tiles, encode each via the JPEG encoder, write via `levelHandle.WriteTile`.
4. For L1+: treat the previous output level's raster as the "source", apply 2x2 area average to produce the next level's raster, then encode + write tiles.

The exact code is too long to inline fully; commit it incrementally in Tasks 22a, 22b if needed (sub-tasks). For this plan's purposes, the unit of work is: "implement `buildPyramid` such that the integration test in Task 23 passes."

- [ ] **Step 2: Build**

```bash
go build ./cmd/wsi-tools/
```

Expected: builds successfully (with `buildPyramid` returning the TODO error). This step is just ensuring the rest of the file compiles around the stub.

- [ ] **Step 3: Implement buildPyramid (the real work)**

Per the pseudocode above. The implementation strategy:

- `srcL0Raster := decodeFullLevel(src.Levels()[0], decoder.NewJPEG(), factor /* fast-scale */)` returns `[]byte` of size `(srcW/factor) × (srcH/factor) × 3`.
- For each output level:
  - Build `LevelSpec` with the right dims + JPEGTables.
  - Call `w.AddLevel(spec)` → `*LevelHandle`.
  - Run pipeline that:
    - **Source**: emits one tile-coord per (tx, ty); the source func extracts the corresponding 256x256 raster slice from the in-memory level raster.
    - **Process**: encodes via the JPEG encoder.
    - **Sink**: `levelHandle.WriteTile(tx, ty, encoded)`.
  - For the next level, `levelRaster = resample.Area2x2(levelRaster, levelW, levelH)`; `levelW /= 2; levelH /= 2`.

`decodeFullLevel` is a helper in `cmd/wsi-tools/downsample.go` (or hoist to `internal/pipeline/decode_level.go` if it grows):

```go
func decodeFullLevel(level opentile.Level, dec decoder.Decoder, fastScale int) ([]byte, error) {
    tw, th := level.TileSize().X, level.TileSize().Y
    grid := level.Grid()
    outW := level.Size().X / fastScale
    outH := level.Size().Y / fastScale
    raster := make([]byte, outW*outH*3)
    // Iterate every tile, decode at 1/fastScale, copy into raster at the
    // right offset.
    for ty := 0; ty < grid.Y; ty++ {
        for tx := 0; tx < grid.X; tx++ {
            compressed, err := level.Tile(tx, ty)
            if err != nil { return nil, err }
            decoded, err := dec.DecodeTile(compressed, nil, 1, fastScale)
            if err != nil { return nil, err }
            // Copy decoded into raster at (tx*tw/fastScale, ty*th/fastScale).
            // Handle edge tiles where tw/fastScale doesn't tile-align.
            ...
        }
    }
    return raster, nil
}
```

- [ ] **Step 4: Build + smoke test against a real fixture**

```bash
go build -o /tmp/wsi-tools ./cmd/wsi-tools
/tmp/wsi-tools downsample -o /tmp/out.svs /Users/cornish/GitHub/wsi-tools/sample_files/svs/CMU-1-Small-Region.svs
```

Expected: prints `wrote /tmp/out.svs` and exits 0; `/tmp/out.svs` exists.

- [ ] **Step 5: Re-open output via opentile-go to confirm structural validity**

Use a one-shot probe (or `tiffinfo`):

```bash
tiffinfo /tmp/out.svs | head -40
```

Expected: shows multiple TIFF Directories, Compression: JPEG (old-style), AppMag = 20.

- [ ] **Step 6: Commit**

```bash
git add cmd/wsi-tools/downsample.go
git commit -m "feat(cli): downsample subcommand wiring + buildPyramid"
```

---

### Task 23: Integration test — downsample CMU-1-Small-Region.svs end-to-end

**Files:**
- Create: `tests/integration/downsample_test.go`

- [ ] **Step 1: Write the test**

```go
//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	opentile "github.com/cornish/opentile-go"
	_ "github.com/cornish/opentile-go/formats/all"
	svsfmt "github.com/cornish/opentile-go/formats/svs"
)

func testdir(t *testing.T) string {
	d := os.Getenv("WSI_TOOLS_TESTDIR")
	if d == "" {
		d = "../../sample_files"
	}
	if _, err := os.Stat(d); err != nil {
		t.Skipf("WSI_TOOLS_TESTDIR=%s not accessible: %v", d, err)
	}
	return d
}

func TestDownsample40to20_CMU1SmallRegion(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out.svs")

	// Build wsi-tools fresh into the test's tmp dir.
	bin := filepath.Join(t.TempDir(), "wsi-tools")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/wsi-tools")
	cmd.Dir = "../.."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build wsi-tools: %v\n%s", err, out)
	}

	cmd = exec.Command(bin, "downsample", "-o", out, src)
	if outBytes, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("downsample: %v\n%s", err, outBytes)
	}

	srcTlr, _ := opentile.OpenFile(src)
	defer srcTlr.Close()
	outTlr, err := opentile.OpenFile(out)
	if err != nil {
		t.Fatalf("opentile.OpenFile(out): %v", err)
	}
	defer outTlr.Close()

	if outTlr.Format() != "svs" {
		t.Errorf("output format: got %q, want svs", outTlr.Format())
	}
	if got, want := len(outTlr.Levels()), len(srcTlr.Levels()); got != want {
		t.Errorf("level count: got %d, want %d", got, want)
	}

	// L0 dimensions = source L0 / 2.
	srcL0, outL0 := srcTlr.Levels()[0], outTlr.Levels()[0]
	if outL0.Size().X != srcL0.Size().X/2 || outL0.Size().Y != srcL0.Size().Y/2 {
		t.Errorf("L0 size: got %v, want %v", outL0.Size(), srcL0.Size())
	}

	// Magnification halved.
	if md, ok := svsfmt.MetadataOf(outTlr); ok {
		if got, want := md.AppMag, 20.0; got != want {
			t.Errorf("AppMag: got %v, want %v", got, want)
		}
	}

	// Associated images byte-equal to source.
	srcAssoc := srcTlr.Associated()
	outAssoc := outTlr.Associated()
	if len(srcAssoc) != len(outAssoc) {
		t.Errorf("associated count: got %d, want %d", len(outAssoc), len(srcAssoc))
	}
	// Spot-check first one for byte equality.
	if len(srcAssoc) > 0 && len(outAssoc) > 0 {
		srcBytes, _ := srcAssoc[0].Bytes()
		outBytes, _ := outAssoc[0].Bytes()
		if !bytesEqual(srcBytes, outBytes) {
			t.Errorf("associated[0] bytes differ — pass-through broken")
		}
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
```

- [ ] **Step 2: Run**

```bash
cd /Users/cornish/GitHub/wsi-tools
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -race -count=1 -v
```

Expected: PASS. Iteration is expected — the first run will likely surface bugs in `buildPyramid`, the JPEG-tables wiring, the associated-image NewSubfileType, or the ImageDescription mutation. Fix each, re-run.

- [ ] **Step 3: Commit**

```bash
git add tests/integration/
git commit -m "test(integration): downsample CMU-1-Small-Region.svs end-to-end"
```

---

### Task 24: Run the integration sweep across opentile-go's full SVS fixture set

**Files:**
- Modify: `tests/integration/downsample_test.go`

- [ ] **Step 1: Add a table-driven test that loops every SVS fixture**

```go
func TestDownsampleSVSFixtures(t *testing.T) {
	td := testdir(t)
	matches, _ := filepath.Glob(filepath.Join(td, "svs", "*.svs"))
	if len(matches) == 0 {
		t.Skipf("no SVS fixtures in %s", td)
	}
	bin := buildOnce(t)
	for _, src := range matches {
		src := src
		t.Run(filepath.Base(src), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out.svs")
			cmd := exec.Command(bin, "downsample", "-o", out, src)
			if b, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("downsample: %v\n%s", err, b)
			}
			tlr, err := opentile.OpenFile(out)
			if err != nil {
				t.Fatalf("re-open: %v", err)
			}
			tlr.Close()
		})
	}
}

func buildOnce(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(os.TempDir(), "wsi-tools-it")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/wsi-tools")
	cmd.Dir = "../.."
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, b)
	}
	return bin
}
```

- [ ] **Step 2: Run**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -race -count=1 -v -timeout 30m
```

Expected: every fixture passes, OR specific fixtures flag bugs (e.g., the JP2K SVS `JP2K-33003-1.svs` exercises the OpenJPEG decoder + area-average path). Fix, re-run.

- [ ] **Step 3: Commit**

```bash
git add tests/integration/downsample_test.go
git commit -m "test(integration): sweep all SVS fixtures through downsample"
```

---

## Batch H — Polish + ship (3 tasks)

### Task 25: Progress bar + structured logging

**Files:**
- Modify: `cmd/wsi-tools/downsample.go`
- Modify: `cmd/wsi-tools/main.go`

- [ ] **Step 1: Add global flags + slog setup**

In `main.go`, add `--quiet`, `--verbose`, `--log-level`, `--log-format` as persistent flags on `rootCmd`. Configure `slog.Default()` accordingly.

In `downsample.go`, instrument `buildPyramid`:

- Compute `totalTiles = sum across levels of tilesX*tilesY`.
- Wrap with a `mpb.Progress` bar (one per level, or one rolled-up bar for the whole job — pick one).
- Suppress when stdout isn't a TTY or `--quiet`.
- Emit per-level summary on `--verbose`.

- [ ] **Step 2: Smoke test**

```bash
/tmp/wsi-tools downsample -o /tmp/out.svs $HOME/GitHub/opentile-go/sample_files/svs/CMU-1-Small-Region.svs
```

Expected: progress bar updates; final line "wrote /tmp/out.svs".

- [ ] **Step 3: Commit**

```bash
git add cmd/wsi-tools/main.go cmd/wsi-tools/downsample.go
git commit -m "feat(cli): progress bar + structured slog logging"
```

---

### Task 26: SIGINT handling — drain + remove tmp

**Files:**
- Modify: `cmd/wsi-tools/main.go`

- [ ] **Step 1: Implement signal handling**

In `main.go`'s `main()`:

```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()
rootCmd.SetContext(ctx)
if err := rootCmd.ExecuteContext(ctx); err != nil {
    if errors.Is(err, context.Canceled) {
        fmt.Fprintln(os.Stderr, "interrupted")
        os.Exit(130)
    }
    fmt.Fprintln(os.Stderr, "error:", err)
    os.Exit(1)
}
```

- [ ] **Step 2: Manual smoke test (optional — hard to automate)**

Run a long downsample, Ctrl-C mid-flight, confirm `<output>.tmp` is removed.

- [ ] **Step 3: Commit**

```bash
git add cmd/wsi-tools/main.go
git commit -m "feat(cli): SIGINT/SIGTERM cancel, clean tmp on interrupt"
```

---

### Task 27: README + viewer-compat doc + final polish

**Files:**
- Modify: `README.md`
- Create: `docs/viewer-compat.md`

- [ ] **Step 1: Flesh out README**

Add usage examples for `downsample`, `doctor`, `version`. Document the cgo deps + brew install lines. Document the `WSI_TOOLS_TESTDIR` env var and the soft-link convention.

- [ ] **Step 2: Create docs/viewer-compat.md**

```markdown
# wsi-tools — viewer compatibility checklist

Manual checklist of (output codec, viewer) pairs that have been verified to load.
Not in CI; run by hand and update this file when you confirm a pair works.

## v0.1 — downsample tool

| Codec | Viewer | Verified? | Notes |
|---|---|---|---|
| JPEG (Aperio SVS) | QuPath | — | |
| JPEG (Aperio SVS) | openslide-bin | — | |
| JPEG (Aperio SVS) | OpenSeadragon (via DZI/IIIF) | — | |

## v0.2 — transcode tool

(populated when the transcode plan ships)
```

- [ ] **Step 3: Commit**

```bash
git add README.md docs/viewer-compat.md
git commit -m "docs: README usage + viewer-compat checklist"
```

---

### Task 28: Tag v0.1.0 release

**Files:**
- (no source changes; tag only)

- [ ] **Step 1: Final smoke test**

```bash
make test
make vet
make build
./bin/wsi-tools downsample -o /tmp/v01-final.svs $HOME/GitHub/opentile-go/sample_files/svs/CMU-1-Small-Region.svs
./bin/wsi-tools doctor
./bin/wsi-tools version
```

Expected: every command works.

- [ ] **Step 2: Tag**

```bash
git tag -a v0.1.0 -m "wsi-tools v0.1.0 — downsample tool"
```

- [ ] **Step 3: (Optional) push if there's a remote**

If the repo has a GitHub remote configured: `git push --tags`. Otherwise, the local tag is enough.

---

## Self-review checklist (executor: do this after Task 28)

1. **All tasks committed?** `git log --oneline | wc -l` should be ~28.
2. **All tests pass?** `make test` exits 0.
3. **`make vet` clean?** Yes.
4. **README accurate?** Build + run + downsample example matches reality.
5. **`wsi-tools doctor` honest?** Doesn't claim codecs it doesn't have.
6. **Atomic write verified?** Manual: `Ctrl-C` mid-downsample leaves no `.tmp` file.
7. **CMU-1.svs full-walk works?** It's 177 MB; should complete in single-digit minutes on a dev laptop.
