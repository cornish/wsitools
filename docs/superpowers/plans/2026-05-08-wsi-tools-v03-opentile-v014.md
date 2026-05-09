# wsi-tools v0.3 — opentile-go v0.14 migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bump opentile-go from v0.12.0 to v0.14.0, adopt opentile-go's
allocation-free `TileInto` API on the streaming hot path with a per-level
`sync.Pool`, extend `internal/source.Compression` to cover v0.14's three
new enum values (WebP / JPEGXL / HTJ2K), and replace `tests/integration/transcode_test.go`'s tiffinfo shell-out with a proper opentile-go round-trip
for the four novel-codec outputs.

**Architecture:** Forward-looking upgrade. Both opentile-go v0.13 (added
`TilePrefix` / `TileBodyInto` / `SpliceJPEGTile` to `opentile.Level`) and
v0.14 (added 3 new `Compression` constants, 5 new TIFF compression tag
mappings in the generic-TIFF parser, and a wsi-tools `ImageDescription`
parser) are purely additive — no breaking changes to absorb. We deliberately
do NOT adopt the splice-prefix family (no use case for transcode, which
fully decodes every tile). We DO adopt `TileInto` and `TileMaxSize`,
plumbing a `Release func()` field through `pipeline.Tile` so the worker
can return the per-tile buffer to a per-level pool right after decode.

**Tech Stack:** Go 1.22+ • cgo • [opentile-go v0.14](https://github.com/cornish/opentile-go) • libjpeg-turbo • libjxl • libavif • libwebp • OpenJPH

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `go.mod`, `go.sum` | modify | bump opentile-go to v0.14.0 |
| `internal/source/source.go` | modify | extend `Compression` enum (3 values + 3 String cases); replace `Tile()` with `TileMaxSize()` + `TileInto()` on the `Level` interface |
| `internal/source/opentile.go` | modify | adapter: delegate `TileMaxSize` / `TileInto` to opentile-go; map 3 new compression values; drop `Tile()` |
| `internal/source/source_test.go` | create | unit tests for `Compression.String()` covering the three new values |
| `internal/source/opentile_test.go` | modify | unit test for `mapOpentileCompression` covering WebP/JPEGXL/HTJ2K; replace any `Tile()` use with `TileInto` |
| `internal/pipeline/pipeline.go` | modify | add `Release func()` field to `Tile`; document semantics |
| `internal/pipeline/pipeline_test.go` | modify | add a unit test asserting Process can invoke `t.Release()`, demonstrating the lifecycle |
| `cmd/wsi-tools/transcode.go` | modify | adopt `TileInto` with per-level `sync.Pool`; wire `Release` on emitted tiles; call `t.Release()` between decode and encode in the worker |
| `cmd/wsi-tools/downsample.go` | modify | hoist a single tile-sized buffer outside the source-tile loop; switch to `TileInto` |
| `tests/integration/transcode_test.go` | modify | replace tiffinfo structural validation with opentile-go round-trip for the 4 novel-codec cases |
| `CHANGELOG.md` | modify | add `[0.3.0]` section; drop "opentile-go novel-codec decoders" line from v0.2.0 deferred list |

No new packages. No file moves. No deletions.

---

## Conventions for the executor

- **Don't reason about WSI byte layout from first principles.** When unsure
  about TIFF / Aperio / opentile-go behaviour, read the reference source.
  This is project policy from `CLAUDE.md`.
- **Frequent commits.** One commit per task unless a step explicitly says
  otherwise.
- **No vet / test skips.** `make vet && make test` must pass at the end of
  every task.
- **Working directory:** `/Users/cornish/GitHub/wsi-tools`
- **Branch:** `feat/v0.3-opentile-v014` (already created from `main`).
- **Sample files:** `WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files`
  for integration tests. The `sample_files/` symlink in this repo points
  there per `CLAUDE.md`.

---

## Task 1: Bump opentile-go to v0.14.0

**Files:**
- Modify: `go.mod`, `go.sum`

The dependency upgrade. Both v0.13 and v0.14 are additive — `make vet` and
`make test` should pass with zero source code changes after this single
upgrade. If they don't, stop and read the `opentile-go` CHANGELOG before
continuing.

- [ ] **Step 1: Run go get to fetch v0.14.0**

```bash
go get github.com/cornish/opentile-go@v0.14.0
```

Expected: `go.mod` updated to `github.com/cornish/opentile-go v0.14.0`,
`go.sum` updated with the new hash.

- [ ] **Step 2: Verify the version landed**

```bash
grep "opentile-go " go.mod
```

Expected output:

```
	github.com/cornish/opentile-go v0.14.0
```

(May be marked `// indirect` until we reference any new symbol; that's
fine for now.)

- [ ] **Step 3: Run vet**

```bash
make vet
```

Expected: clean output, no errors.

- [ ] **Step 4: Run the full unit test suite**

```bash
make test
```

Expected: all tests PASS (exit 0). v0.13 and v0.14 are additive — there is
no v0.12 method or constant we depend on that has been removed or renamed.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum
git commit -m "$(cat <<'EOF'
deps: bump opentile-go v0.12.0 -> v0.14.0

Upstream v0.13 added TilePrefix/TileBodyInto/SpliceJPEGTile to
opentile.Level (additive). v0.14 added 3 Compression constants
(CompressionWebP, CompressionJPEGXL, CompressionHTJ2K), 5 TIFF
compression tag mappings, and a wsi-tools ImageDescription parser
(also additive). No source code changes required; subsequent
tasks claim the new capabilities.
EOF
)"
```

---

## Task 2: Extend `source.Compression` enum with WebP / JPEGXL / HTJ2K

**Files:**
- Create: `internal/source/source_test.go`
- Modify: `internal/source/source.go:60-90` (the enum constants and `String()` method)
- Modify: `internal/source/opentile.go:141-159` (the `mapOpentileCompression` switch)
- Modify: `internal/source/opentile_test.go` (or create — check first)

opentile-go v0.14 added three `Compression` enum values for the codecs
we already write but couldn't read back: `CompressionWebP`,
`CompressionJPEGXL`, `CompressionHTJ2K`. AVIF reuses the existing
`CompressionAVIF` constant added in v0.8 — no change needed for AVIF.

- [ ] **Step 1: Write the failing tests**

Open `internal/source/source_test.go` (create if missing — the file may not
exist yet; if it doesn't, also include `package source` at the top):

```go
package source

import "testing"

func TestCompressionString_NewValues(t *testing.T) {
	cases := []struct {
		c    Compression
		want string
	}{
		{CompressionWebP, "webp"},
		{CompressionJPEGXL, "jpegxl"},
		{CompressionHTJ2K, "htj2k"},
	}
	for _, tc := range cases {
		if got := tc.c.String(); got != tc.want {
			t.Errorf("Compression(%d).String() = %q, want %q", tc.c, got, tc.want)
		}
	}
}
```

Then check whether `internal/source/opentile_test.go` exists and has a
`mapOpentileCompression` test. If it does, append; if not, create with:

```go
package source

import (
	"testing"

	opentile "github.com/cornish/opentile-go"
)

func TestMapOpentileCompression_NovelCodecs(t *testing.T) {
	cases := []struct {
		in   opentile.Compression
		want Compression
	}{
		{opentile.CompressionWebP, CompressionWebP},
		{opentile.CompressionJPEGXL, CompressionJPEGXL},
		{opentile.CompressionHTJ2K, CompressionHTJ2K},
	}
	for _, tc := range cases {
		if got := mapOpentileCompression(tc.in); got != tc.want {
			t.Errorf("mapOpentileCompression(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
```

- [ ] **Step 2: Run the new tests, verify they fail to compile**

```bash
go test ./internal/source/ -run 'TestCompressionString_NewValues|TestMapOpentileCompression_NovelCodecs' -v
```

Expected: compile error — `undefined: CompressionWebP` etc. (Tests can't
even compile until the constants exist.)

- [ ] **Step 3: Add the new enum values + String() cases**

In `internal/source/source.go`, replace the existing const block and
`String()` method:

```go
const (
	CompressionUnknown Compression = iota
	CompressionJPEG
	CompressionJPEG2000
	CompressionLZW
	CompressionDeflate
	CompressionNone
	CompressionAVIF
	CompressionIrisProprietary
	CompressionWebP
	CompressionJPEGXL
	CompressionHTJ2K
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
	case CompressionWebP:
		return "webp"
	case CompressionJPEGXL:
		return "jpegxl"
	case CompressionHTJ2K:
		return "htj2k"
	}
	return "unknown"
}
```

- [ ] **Step 4: Add the three new switch cases to mapOpentileCompression**

In `internal/source/opentile.go`, extend the existing switch:

```go
func mapOpentileCompression(c opentile.Compression) Compression {
	switch c {
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
	case opentile.CompressionAVIF:
		return CompressionAVIF
	case opentile.CompressionIRIS:
		return CompressionIrisProprietary
	case opentile.CompressionWebP:
		return CompressionWebP
	case opentile.CompressionJPEGXL:
		return CompressionJPEGXL
	case opentile.CompressionHTJ2K:
		return CompressionHTJ2K
	}
	return CompressionUnknown
}
```

- [ ] **Step 5: Run the new tests, verify PASS**

```bash
go test ./internal/source/ -run 'TestCompressionString_NewValues|TestMapOpentileCompression_NovelCodecs' -v
```

Expected: both PASS.

- [ ] **Step 6: Run the full source test suite**

```bash
go test ./internal/source/ -race -count=1
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/source/source.go internal/source/opentile.go internal/source/source_test.go internal/source/opentile_test.go
git commit -m "$(cat <<'EOF'
feat(source): extend Compression enum with WebP, JPEGXL, HTJ2K

Mirrors opentile-go v0.14's three new Compression constants. AVIF
already mapped via the existing CompressionAVIF (v0.8). Iris-proprietary
remains the only opentile-go compression we recognise but cannot decode.
EOF
)"
```

---

## Task 3: Replace `Tile()` with `TileInto` + `TileMaxSize` on `source.Level`

**Files:**
- Modify: `internal/source/source.go:39-47` (the `Level` interface)
- Modify: `internal/source/opentile.go:101-119` (the `opentileLevel` adapter)
- Modify: `internal/source/opentile_test.go` (add a method-presence test)
- Modify: `cmd/wsi-tools/transcode.go:264-296` (the producer closure inside `transcodeOneLevel`)
- Modify: `cmd/wsi-tools/downsample.go:514-525` (the source-tile loop)

This task swaps the interface surface in one shot:

1. Add `TileMaxSize() int` and `TileInto(x, y, dst) (int, error)` to
   `source.Level`.
2. Drop `Tile(x, y) ([]byte, error)`.
3. Update the two callers in `cmd/wsi-tools/` to use the new methods —
   **with naive per-call allocation for now** (e.g.,
   `buf := make([]byte, lvl.TileMaxSize())` inside the loop). The pool
   wiring lands in Task 4.

The naive path keeps Task 3's diff narrow and means the buffer-pool
plumbing in Task 4 has a clean before/after to test.

- [ ] **Step 1: Write the failing test**

Append to `internal/source/opentile_test.go`:

```go
import (
	"path/filepath"
	"os"
)

// TestLevel_TileInto_RoundTrip opens a known SVS fixture and verifies
// that TileInto fills a buffer with the same bytes the underlying
// opentile.Level returns.
func TestLevel_TileInto_RoundTrip(t *testing.T) {
	testDir := os.Getenv("WSI_TOOLS_TESTDIR")
	if testDir == "" {
		t.Skip("WSI_TOOLS_TESTDIR not set")
	}
	path := filepath.Join(testDir, "svs", "CMU-1-Small-Region.svs")
	src, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer src.Close()

	lvl := src.Levels()[0]
	max := lvl.TileMaxSize()
	if max <= 0 {
		t.Fatalf("TileMaxSize() = %d, want > 0", max)
	}
	buf := make([]byte, max)
	n, err := lvl.TileInto(0, 0, buf)
	if err != nil {
		t.Fatalf("TileInto: %v", err)
	}
	if n <= 0 || n > max {
		t.Fatalf("TileInto returned n=%d (max=%d)", n, max)
	}
}
```

- [ ] **Step 2: Run, verify FAIL (compile error)**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./internal/source/ -run TestLevel_TileInto_RoundTrip -v
```

Expected: compile error — `lvl.TileMaxSize undefined` / `lvl.TileInto undefined`.

- [ ] **Step 3: Update the `Level` interface**

In `internal/source/source.go`, replace the `Level` block:

```go
// Level is one pyramid level.
type Level interface {
	Index() int
	Size() image.Point     // image dimensions in pixels
	TileSize() image.Point // tile dimensions; preserved verbatim on output
	Grid() image.Point     // tilesX × tilesY
	Compression() Compression

	// TileMaxSize returns an upper bound on any tile's compressed-byte
	// length on this level — sized for sync.Pool buffers.
	TileMaxSize() int

	// TileInto writes the raw compressed tile bytes at (x, y) into dst
	// and returns the number of bytes written. dst must have len >=
	// TileMaxSize(); shorter buffers return io.ErrShortBuffer. The
	// returned slice (dst[:n]) is the canonical byte form for the
	// transcode/downsample decoder pipeline.
	TileInto(x, y int, dst []byte) (int, error)
}
```

- [ ] **Step 4: Update the adapter**

In `internal/source/opentile.go`, replace the `Tile()` method on
`opentileLevel` with the two new methods:

```go
func (l *opentileLevel) TileMaxSize() int { return l.lvl.TileMaxSize() }

func (l *opentileLevel) TileInto(x, y int, dst []byte) (int, error) {
	return l.lvl.TileInto(x, y, dst)
}
```

(Delete the old `func (l *opentileLevel) Tile(x, y int) ([]byte, error)` line.)

- [ ] **Step 5: Update the transcode producer**

In `cmd/wsi-tools/transcode.go`, replace the `Source` closure inside
`transcodeOneLevel` (around line 264-279) — keep the surrounding
`pipeline.Run` call structure exactly as is; only the closure changes.
Naive per-call allocation, no pool yet:

```go
		Source: func(ctx context.Context, emit func(pipeline.Tile) error) error {
			maxTileBytes := lvl.TileMaxSize()
			for ty := 0; ty < grid.Y; ty++ {
				for tx := 0; tx < grid.X; tx++ {
					buf := make([]byte, maxTileBytes)
					n, err := lvl.TileInto(tx, ty, buf)
					if err != nil {
						return err
					}
					if err := emit(pipeline.Tile{
						Level: lvl.Index(),
						X:     uint32(tx),
						Y:     uint32(ty),
						Bytes: buf[:n],
					}); err != nil {
						return err
					}
				}
			}
			return nil
		},
```

- [ ] **Step 6: Update the downsample source-tile read**

In `cmd/wsi-tools/downsample.go`, locate the `compressed, err := srcL0.Tile(tx, ty)`
line (~521) and the surrounding loop. Hoist a single buffer above the
loop, replacing `Tile()` with `TileInto()`. The full block looks like:

```go
	tileBuf := make([]byte, srcL0.TileMaxSize())

	for ty := 0; ty < srcGrid.H; ty++ {
		for tx := 0; tx < srcGrid.W; tx++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			n, err := srcL0.TileInto(tx, ty, tileBuf)
			if err != nil {
				return fmt.Errorf("read source tile (%d,%d): %w", tx, ty, err)
			}
			compressed := tileBuf[:n]
			// ... existing decode + accumulate logic, unchanged ...
```

The `compressed` variable name is preserved so the rest of the loop body
stays unchanged.

- [ ] **Step 7: Run vet + build**

```bash
make vet
make build
```

Expected: clean. (Linker warning `ignoring duplicate libraries: '-lc++', '-lturbojpeg'` is benign and pre-existing.)

- [ ] **Step 8: Run the new round-trip test**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./internal/source/ -run TestLevel_TileInto_RoundTrip -v
```

Expected: PASS.

- [ ] **Step 9: Run full test suite**

```bash
make test
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -race -count=1 -timeout 30m
```

Expected: all PASS. The integration suite proves end-to-end transcode +
downsample still work after the interface swap.

- [ ] **Step 10: Commit**

```bash
git add internal/source/source.go internal/source/opentile.go internal/source/opentile_test.go cmd/wsi-tools/transcode.go cmd/wsi-tools/downsample.go
git commit -m "$(cat <<'EOF'
refactor(source): replace Level.Tile with TileInto + TileMaxSize

Mirrors opentile-go's allocation-free hot path. Callers (transcode
producer, downsample source loop) updated to allocate or hoist
buffers themselves. No pool wiring yet — naive per-call allocation
in the transcode producer keeps this diff narrow; pool lands in
the next task.

Downsample loop hoists a single tile-sized buffer above the loop
since the loop is sequential.
EOF
)"
```

---

## Task 4: Buffer pool in transcode + `Release` plumbing

**Files:**
- Modify: `internal/pipeline/pipeline.go` (add `Release func()` field to `Tile`, document semantics)
- Modify: `internal/pipeline/pipeline_test.go` (test that Process can invoke `Release`)
- Modify: `cmd/wsi-tools/transcode.go` (wire `sync.Pool`, set `Release`, invoke after decode)

`pipeline.Tile` gains an optional `Release func()` field. The transcode
producer now pulls `*[]byte` from a per-level `sync.Pool` (sized at
`lvl.TileMaxSize()`), sets `Release: func() { pool.Put(bufp) }` on the
emitted tile, and the worker invokes `t.Release()` between decode and
encode. `Release` is nil-safe: existing producers (downsample's caller
flow doesn't use the pipeline; future callers may opt out) can leave it
nil and the pipeline behaves identically to today.

- [ ] **Step 1: Write the failing test**

In `internal/pipeline/pipeline_test.go`, append:

```go
import "sync/atomic"

// TestTile_ReleaseInvokedByProcess proves the canonical lifecycle: the
// producer attaches Release; Process invokes it; the pipeline itself
// never invokes Release (it is opaque to the field). After the run,
// the call count equals the number of emitted tiles.
func TestTile_ReleaseInvokedByProcess(t *testing.T) {
	const N = 17
	var released atomic.Int64

	cfg := Config{
		Workers: 4,
		Source: func(ctx context.Context, emit func(Tile) error) error {
			for i := 0; i < N; i++ {
				if err := emit(Tile{
					Level:   0,
					X:       uint32(i),
					Y:       0,
					Bytes:   []byte{byte(i)},
					Release: func() { released.Add(1) },
				}); err != nil {
					return err
				}
			}
			return nil
		},
		Process: func(t Tile) (Tile, error) {
			if t.Release != nil {
				t.Release()
				t.Release = nil
			}
			t.Bytes = []byte{0xFF}
			return t, nil
		},
		Sink: func(t Tile) error { return nil },
	}
	if err := Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := released.Load(); got != N {
		t.Errorf("Release invoked %d times, want %d", got, N)
	}
}
```

(`context` and `sync/atomic` imports may need to be added to the test file's import block.)

- [ ] **Step 2: Run, verify FAIL (compile error)**

```bash
go test ./internal/pipeline/ -run TestTile_ReleaseInvokedByProcess -v
```

Expected: compile error — `unknown field Release in struct literal`.

- [ ] **Step 3: Add Release to pipeline.Tile**

In `internal/pipeline/pipeline.go`:

```go
// Tile is the unit of work flowing through the pipeline.
type Tile struct {
	Level int
	X, Y  uint32
	Bytes []byte

	// Release, if non-nil, is invoked by the consumer (typically inside
	// ProcessFn between decode and encode) to return Bytes' underlying
	// buffer to its source pool. The pipeline itself never invokes
	// Release; ownership and lifetime are entirely caller-managed.
	// Nil is safe.
	Release func()
}
```

- [ ] **Step 4: Run the test, verify PASS**

```bash
go test ./internal/pipeline/ -run TestTile_ReleaseInvokedByProcess -race -v
```

Expected: PASS.

- [ ] **Step 5: Wire sync.Pool + Release in transcode**

In `cmd/wsi-tools/transcode.go::transcodeOneLevel`, replace the Task 3
naive `Source` closure and the existing `Process` closure with the
pooled versions. Imports: ensure `sync` is imported in the file.

```go
	maxTileBytes := lvl.TileMaxSize()
	pool := &sync.Pool{
		New: func() any {
			b := make([]byte, maxTileBytes)
			return &b
		},
	}

	return pipeline.Run(ctx, pipeline.Config{
		Workers: workers,
		Source: func(ctx context.Context, emit func(pipeline.Tile) error) error {
			for ty := 0; ty < grid.Y; ty++ {
				for tx := 0; tx < grid.X; tx++ {
					bufp := pool.Get().(*[]byte)
					n, err := lvl.TileInto(tx, ty, *bufp)
					if err != nil {
						pool.Put(bufp)
						return err
					}
					t := pipeline.Tile{
						Level:   lvl.Index(),
						X:       uint32(tx),
						Y:       uint32(ty),
						Bytes:   (*bufp)[:n],
						Release: func() { pool.Put(bufp) },
					}
					if err := emit(t); err != nil {
						pool.Put(bufp)
						return err
					}
				}
			}
			return nil
		},
		Process: func(t pipeline.Tile) (pipeline.Tile, error) {
			rgb := make([]byte, tileBytes)
			rgbOut, err := dec.DecodeTile(t.Bytes, rgb, 1, 1)
			if t.Release != nil {
				t.Release()
				t.Release = nil
			}
			if err != nil {
				return pipeline.Tile{}, err
			}
			encoded, err := enc.EncodeTile(rgbOut, tileW, tileH, nil)
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
```

Two invariants to keep straight while implementing:
- The `bufp` captured in the `Release` closure is the **pointer** returned
  from `pool.Get`, not the slice. This is intentional — `sync.Pool` of
  `*[]byte` (rather than `[]byte`) avoids a Go runtime perf cliff where
  the GC can't keep slice headers stable in pools.
- On producer-side error (`TileInto` fails or `emit` returns an error),
  the buffer is `Put` back before returning. On normal flow, the worker's
  `t.Release()` does it.

- [ ] **Step 6: Run the integration sweep with -race**

```bash
make build
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -race -count=1 -timeout 30m
```

Expected: all PASS, no race detector warnings. The race detector is the
canary for any pool-ownership bug.

- [ ] **Step 7: Commit**

```bash
git add internal/pipeline/pipeline.go internal/pipeline/pipeline_test.go cmd/wsi-tools/transcode.go
git commit -m "$(cat <<'EOF'
feat(transcode): per-level sync.Pool of tile buffers via Tile.Release

Adds an optional Release func() field to pipeline.Tile so producers
can hand back the per-tile buffer once the worker is done with it.
Transcode producer now pulls *[]byte from a per-level sync.Pool
sized at lvl.TileMaxSize(), and the worker invokes t.Release()
between decode and encode. Race detector clean.

Cancellation-path leak (buffers in flight when ctx is canceled) is
bounded at Workers*4 buffers per level and accepted: the pool falls
out of scope when the level returns and is GC'd shortly after.
EOF
)"
```

---

## Task 5: Round-trip integration test for the four novel codecs

**Files:**
- Modify: `tests/integration/transcode_test.go`

The current test shells out to `tiffinfo` for the WebP / JPEGXL / AVIF /
HTJ2K cases because opentile-go v0.12 couldn't recognise their
compression tags. v0.14 fixes that. This task replaces the tiffinfo path
with a proper opentile-go round-trip that asserts (a) the output opens,
(b) the format classifier picks `FormatGenericTIFF`, (c) `Compression()`
matches the codec's expected enum value, and (d) `Metadata` round-trips
via the v0.14 wsi-tools ImageDescription parser.

The JPEG-codec case stays as-is — it produces SVS-shaped output that
already round-trips through opentile-go's SVS reader.

- [ ] **Step 1: Read the current test to find the per-codec table**

```bash
grep -n "tiffinfo\|expectedCompression\|webp\|jpegxl\|avif\|htj2k" tests/integration/transcode_test.go
```

Note the line numbers of the existing per-codec table and the tiffinfo
shell-out helper. The exact shape of the test will determine whether
this is a small surgical edit or a larger restructure. Both are fine —
keep the JPEG case unchanged.

- [ ] **Step 2: Add the round-trip helper**

At the bottom of `tests/integration/transcode_test.go` (or in a
sensible spot near the existing per-codec helpers), add:

```go
// validateNovelCodecOutput re-opens a transcoded output via opentile-go
// (v0.14+) and asserts compression-tag recognition + ImageDescription
// metadata round-trip. The 4 novel codecs (WebP, JPEGXL, AVIF, HTJ2K)
// produce generic-TIFF outputs that v0.14 can parse but does not
// decode — assertions stay at the metadata layer, not the tile-pixel
// layer.
func validateNovelCodecOutput(t *testing.T, outPath string, wantCompression opentile.Compression, wantTileSize opentile.Size, wantMag float64) {
	t.Helper()
	tlr, err := opentile.OpenFile(outPath)
	if err != nil {
		t.Fatalf("opentile.OpenFile(%s): %v", outPath, err)
	}
	defer tlr.Close()

	if got := tlr.Format(); got != opentile.FormatGenericTIFF {
		t.Errorf("Format() = %v, want %v", got, opentile.FormatGenericTIFF)
	}
	levels := tlr.Levels()
	if len(levels) == 0 {
		t.Fatalf("no levels in %s", outPath)
	}
	if got := levels[0].Compression(); got != wantCompression {
		t.Errorf("L0 Compression() = %v, want %v", got, wantCompression)
	}
	if got := levels[0].TileSize(); got != wantTileSize {
		t.Errorf("L0 TileSize() = %v, want %v", got, wantTileSize)
	}

	md := tlr.Metadata()
	if md.Magnification != wantMag {
		t.Errorf("Metadata.Magnification = %v, want %v", md.Magnification, wantMag)
	}
	if md.AcquisitionDateTime.IsZero() {
		t.Errorf("Metadata.AcquisitionDateTime is zero — wsi-tools ImageDescription parser failed?")
	}
}
```

(Add `opentile "github.com/cornish/opentile-go"` to the imports if not
already present.)

- [ ] **Step 3: Wire the helper into the per-codec table for the 4 novel codecs**

Find the existing per-codec test loop. For the WebP / JPEGXL / AVIF /
HTJ2K cases, replace the `tiffinfo` invocation with a call to
`validateNovelCodecOutput`. Expected compression for each:

| codec name | wantCompression |
|---|---|
| `webp` | `opentile.CompressionWebP` |
| `jpegxl` | `opentile.CompressionJPEGXL` |
| `avif` | `opentile.CompressionAVIF` |
| `htj2k` | `opentile.CompressionHTJ2K` |

The source fixture is `CMU-1-Small-Region.svs`. Read its known properties
once — `wantTileSize` and `wantMag` come from the source. The simplest
form: open the source via opentile-go before the test loop, capture
`Levels()[0].TileSize()` and `Metadata().Magnification`, then pass these
into each `validateNovelCodecOutput` call.

If the existing test structure makes this awkward, factor a small
fixture-properties struct at the top of the test function rather than
forcing the existing structure to bend.

- [ ] **Step 4: Remove the tiffinfo helper if it's no longer referenced**

If the JPEG-codec case still uses tiffinfo, leave it. If not (i.e., all 4
remaining tiffinfo callsites were the novel-codec ones we just replaced),
delete the `tiffinfoCompressionFor` helper (or whatever the exact name is)
and its usages. Keep `make vet` clean.

- [ ] **Step 5: Run the integration sweep**

```bash
make build
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -race -count=1 -timeout 30m -v -run Transcode
```

Expected: every sub-test for the per-codec sweep PASSes, including the
4 novel codecs now using opentile-go.

- [ ] **Step 6: Commit**

```bash
git add tests/integration/transcode_test.go
git commit -m "$(cat <<'EOF'
test(integration): opentile-go round-trip for novel-codec transcode outputs

Replaces the tiffinfo structural shell-out for the 4 novel codecs
(WebP, JPEGXL, AVIF, HTJ2K) with a proper opentile-go OpenFile
round-trip that asserts FormatGenericTIFF classification, the
correct Compression() enum value, source TileSize preserved, and
a non-zero Metadata.AcquisitionDateTime (proving the v0.14
wsi-tools ImageDescription parser saw our output).

The JPEG-codec case stays as-is — its SVS-shaped output already
round-trips through opentile-go's SVS reader.
EOF
)"
```

---

## Task 6: CHANGELOG v0.3.0 entry + drop v0.2 deferred line

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add the v0.3.0 section**

At the top of `CHANGELOG.md`, just under the `## [Unreleased]` heading
and above `## [0.2.0]`, insert:

```markdown
## [0.3.0] — 2026-05-08

opentile-go v0.14 alignment. Bumps the upstream dep from v0.12 to
v0.14, claims the new capabilities that bump unlocks (novel-codec
recognition + wsi-tools ImageDescription parsing on re-read), and
migrates the streaming transcode hot path to opentile-go's
allocation-free `TileInto` API with a per-level `sync.Pool` of
tile-sized buffers.

### Added

- **`internal/source.Compression`** gains three new values —
  `CompressionWebP`, `CompressionJPEGXL`, `CompressionHTJ2K` —
  matching opentile-go v0.14's new enum values. AVIF was already
  mapped via the v0.8 `CompressionAVIF` constant.
- **`pipeline.Tile.Release func()`** — optional buffer-pool callback,
  invoked by the consumer between decode and encode. Nil-safe; the
  pipeline package itself stays opaque to the field.
- **opentile-go round-trip integration test for the 4 novel codecs**
  — replaces the prior `tiffinfo` shell-out with assertions on
  `Format()`, `Compression()`, `TileSize()`, and
  `Metadata.AcquisitionDateTime`.

### Changed

- **opentile-go bumped from v0.12 → v0.14.** Both v0.13 (additive
  splice-prefix family on `Level`) and v0.14 (additive Compression
  values + wsi-tools ImageDescription parser) are non-breaking.
- **`internal/source.Level` interface** — `Tile()` removed; replaced
  by `TileMaxSize()` and `TileInto(x, y int, dst []byte) (int, error)`.
  `internal/` is private API; no external callers affected. The
  transcode producer now uses a per-level `sync.Pool`; the downsample
  source loop hoists a single tile-sized buffer above the loop.
- **`cmd/wsi-tools/version.go::Version`** bumped to `0.3.0-dev`. The
  literal `"wsi-tools/0.2.0-dev"` strings in the transcode provenance
  builder and the `WithToolsVersion` writer option are now derived
  from `Version`, not hardcoded.

### Not adopted (intentional)

- `opentile.Level.TilePrefix` / `TileBodyInto` / `SpliceJPEGTile` are
  bandwidth-deduplication helpers for client-server byte-passthrough
  scenarios. wsi-tools' transcode pipeline fully decodes every tile,
  so the splice family offers no benefit. The decision is reversible
  if a future feature (e.g., a streaming HTTP tile server) needs it.
```

- [ ] **Step 2: Drop the "opentile-go novel-codec decoders" line from v0.2.0**

Find this paragraph in the `[0.2.0]` section's "Deferred to v0.2.x or later":

```markdown
- **opentile-go decoders for JXL / AVIF / WebP / HTJ2K compression tags**: opentile-go v0.12's generic-TIFF reader currently accepts only JPEG / JP2K / LZW / Deflate / None compression values. Files written with our private codes (50001/50002/60001/60003) are structurally valid TIFF but require either an opentile-go release that adds these decoders or a viewer with its own codec libraries.
```

Replace with:

```markdown
- ~~**opentile-go decoders for JXL / AVIF / WebP / HTJ2K compression tags**~~: **landed in v0.3.0** via opentile-go v0.14's new `Compression` enum values + generic-TIFF tag mappings. opentile-go does not decode the tile bytes (byte-passthrough contract — consumers bring their own codec libraries) but recognises the compression tags and parses the wsi-tools `ImageDescription`.
```

- [ ] **Step 3: Verify the file is clean Markdown**

```bash
grep -n '^## ' CHANGELOG.md | head
```

Expected: `## [Unreleased]` first, then `## [0.3.0]`, then `## [0.2.0]`,
then `## [0.1.0]`.

- [ ] **Step 4: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: CHANGELOG.md for v0.3.0"
```

---

## Task 7: Final smoke test + tag v0.3.0

**Files:**
- (no source changes)

- [ ] **Step 1: Final regression check**

```bash
make vet
make build
make test
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -race -count=1 -timeout 60m
./bin/wsi-tools doctor
./bin/wsi-tools version
./bin/wsi-tools transcode --codec webp -o /tmp/v03-final.tiff sample_files/svs/CMU-1-Small-Region.svs
```

Expected: every step passes. `wsi-tools version` should print
`wsi-tools 0.3.0-dev` (the `-dev` suffix is fine pre-tag; the tag itself
is the source-of-truth release marker).

- [ ] **Step 2: Bump Version literal to 0.3.0 (drop -dev) on the release commit**

In `cmd/wsi-tools/version.go`:

```go
const Version = "0.3.0"
```

```bash
git add cmd/wsi-tools/version.go
git commit -m "release: bump Version to 0.3.0"
```

- [ ] **Step 3: Merge feat branch into main + tag**

```bash
git checkout main
git merge --ff-only feat/v0.3-opentile-v014
git tag -a v0.3.0 -m "wsi-tools v0.3.0 — opentile-go v0.14 migration + TileInto adoption"
git push origin main
git push origin v0.3.0
```

Stop after `git tag` to confirm with the user before pushing.

- [ ] **Step 4: Create GitHub Release**

Extract the v0.3.0 section from CHANGELOG.md into a release notes file:

```bash
awk '/^## \[0\.3\.0\]/{flag=1; next} /^## \[/{flag=0} flag' CHANGELOG.md > /tmp/v0.3.0-release-notes.md
gh release create v0.3.0 --title "v0.3.0 — opentile-go v0.14 migration" --notes-file /tmp/v0.3.0-release-notes.md
```

- [ ] **Step 5: Bump Version back to 0.4.0-dev on main**

```bash
# In cmd/wsi-tools/version.go: const Version = "0.4.0-dev"
git add cmd/wsi-tools/version.go
git commit -m "post-release: bump Version to 0.4.0-dev"
git push origin main
```

---

## Self-review checklist (executor: do this after Task 7)

1. **All tasks committed?** `git log --oneline v0.2.0..HEAD` — expect ~7
   commits (one per task, plus the version bump and post-release bump).
2. **All tests pass?** `make test` exits 0; integration sweep passes
   including BigTIFF.
3. **`make vet` clean?**
4. **CI green on macOS + Windows?** (Windows still runs `-tags nohtj2k`.)
5. **`./bin/wsi-tools doctor`** lists the same 5 codecs as v0.2.
6. **`./bin/wsi-tools transcode --codec <each>` produces a file that
   opens via `opentile.OpenFile()` and reports the right `Format()` /
   `Compression()`?** Spot-check by hand for at least one codec if
   integration tests didn't already cover it.
7. **`./bin/wsi-tools version`** prints `wsi-tools 0.3.0` (tagged build) or
   `0.4.0-dev` (post-release main).
8. **CHANGELOG accurate?** v0.3.0 section lists the bump, the new enum
   values, the `pipeline.Tile.Release` field, the source-interface
   change, the round-trip test work, and the version-string fix.
9. **The "opentile-go novel-codec decoders" line in v0.2.0's deferred
   list is struck through with a forward reference to v0.3.0.**
