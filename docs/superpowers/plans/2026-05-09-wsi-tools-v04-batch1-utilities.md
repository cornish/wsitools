# wsi-tools v0.4 — batch 1 inspection utilities Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship four read-side CLI utilities — `info`, `dump-ifds`, `extract`, `hash` — analogs of openslide-tools and (slim) tiffinfo, plus a shared `internal/cliout` package for text/JSON dual rendering, plus `docs/roadmap.md` to track the full utilities roadmap durably.

**Architecture:** One subcommand per file under `cmd/wsi-tools/`, each registered via `init()` calling `rootCmd.AddCommand`. Every utility supports `--json` via `internal/cliout`'s shared flag binder + render dispatcher. `dump-ifds` adds a new `internal/source/ifdwalk.go` TIFF IFD walker (handles ClassicTIFF + BigTIFF + SubIFDs); the other utilities are pure orchestration over existing packages.

**Tech Stack:** Go 1.22+ • cobra • `internal/source` (opentile-go adapter) • `internal/decoder` (libjpeg-turbo + OpenJPEG) • Go stdlib `image/png`, `image/jpeg`, `crypto/sha256`, `golang.org/x/image/tiff` (LZW decode for label associated images).

---

## File structure

| Path | Action | Responsibility |
|---|---|---|
| `internal/cliout/cliout.go` | Create | `RegisterJSONFlag`, `Render`, `JSON` helpers |
| `internal/cliout/cliout_test.go` | Create | Unit tests for the three helpers |
| `cmd/wsi-tools/info.go` | Create | `info` subcommand + JSON struct + render closure |
| `cmd/wsi-tools/extract.go` | Create | `extract` subcommand |
| `cmd/wsi-tools/hash.go` | Create | `hash` subcommand (file + pixel modes) |
| `cmd/wsi-tools/dump_ifds.go` | Create | `dump-ifds` subcommand + classifier crossref |
| `internal/source/ifdwalk.go` | Create | TIFF IFD walker (ClassicTIFF + BigTIFF, main chain + SubIFDs) |
| `internal/source/ifdwalk_test.go` | Create | Walker unit tests against fixture pool |
| `tests/integration/info_test.go` | Create | `info` integration test (text + JSON) |
| `tests/integration/extract_test.go` | Create | `extract` integration test |
| `tests/integration/hash_test.go` | Create | `hash` integration test |
| `tests/integration/dump_ifds_test.go` | Create | `dump-ifds` integration test |
| `docs/roadmap.md` | Create | Full utilities roadmap (batch 1, 2, 3, larger items) |
| `CHANGELOG.md` | Modify | Add `[0.4.0]` section |
| `cmd/wsi-tools/version.go` | Modify | bump Version 0.4.0-dev → 0.4.0 (release task only) |

No new third-party dependencies. `golang.org/x/image v0.39.0` is already in the dep tree.

---

## Conventions for the executor

- Working directory: `/Users/cornish/GitHub/wsi-tools`
- Branch: `feat/v0.4-batch1-utilities` (already created from main; spec already committed at `2a0c0ec`)
- Sample fixtures: `WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files` for integration tests; the in-repo `sample_files/` symlink already points there.
- One commit per task unless a task explicitly says otherwise. Use the commit message verbatim from each task's last step.
- `make vet && make test` must pass at the end of every task.
- TDD discipline: write the failing test first, run to confirm fail, implement, run to confirm pass, commit. The test/run/implement/run/commit pattern is explicit in every task — follow it.
- Don't add `// removed: …` comments or rename unused vars to `_var` to silence the compiler. If something becomes unused, delete it.
- The integration sweep (~29 min) is NOT run at the end of every task — only at the release task. Per-utility integration tests run via `-run TestUtilityName` and finish in seconds.

---

## Task 1: `internal/cliout` package

**Files:**
- Create: `internal/cliout/cliout.go`
- Create: `internal/cliout/cliout_test.go`

Three helpers, used by all four subcommands. Total surface ~30 lines.

- [ ] **Step 1: Write the failing test**

Create `internal/cliout/cliout_test.go`:

```go
package cliout

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestJSON_RoundTrip(t *testing.T) {
	type sample struct {
		Foo string `json:"foo"`
		Bar int    `json:"bar"`
	}
	in := sample{Foo: "hello", Bar: 42}
	var buf bytes.Buffer
	if err := JSON(&buf, in); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var out sample
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, in)
	}
	// Indented output should contain newlines + spaces.
	if !strings.Contains(buf.String(), "\n  ") {
		t.Errorf("expected indented JSON, got: %q", buf.String())
	}
	// Trailing newline.
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("expected trailing newline")
	}
}

func TestRegisterJSONFlag_DefaultFalse(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	flag := RegisterJSONFlag(cmd)
	if *flag != false {
		t.Errorf("expected default false, got %v", *flag)
	}
	if f := cmd.Flag("json"); f == nil {
		t.Errorf("expected --json flag registered")
	}
}

func TestRender_TextMode(t *testing.T) {
	var buf bytes.Buffer
	humanCalled := false
	err := Render(false, &buf,
		func(w bytes.Buffer) error { return nil }, // wrong signature on purpose; will fix below
		nil)
	_ = err
	_ = humanCalled
}

func TestRender_JSONMode(t *testing.T) {
	type sample struct{ Foo string `json:"foo"` }
	var buf bytes.Buffer
	err := Render(true, &buf, func(w interface{}) error {
		t.Errorf("human closure should not run in JSON mode")
		return nil
	}, sample{Foo: "ok"})
	_ = err
	if !strings.Contains(buf.String(), `"foo": "ok"`) {
		t.Errorf("expected JSON output, got: %q", buf.String())
	}
}
```

(The two `TestRender_*` tests above use deliberately mismatched signatures so the test file fails to compile — that's the "test failure" we want before implementing. Step 3 fixes the signatures and the implementation.)

- [ ] **Step 2: Run test, verify it fails (compile error)**

```bash
go test ./internal/cliout/ -v
```

Expected: compile error — `cliout` package doesn't exist; types like `*bytes.Buffer` mismatched with `interface{}`.

- [ ] **Step 3: Implement `internal/cliout/cliout.go`**

```go
// Package cliout holds shared text/JSON dual-rendering helpers used by the
// wsi-tools read-side subcommands (info, dump-ifds, extract, hash).
package cliout

import (
	"encoding/json"
	"io"

	"github.com/spf13/cobra"
)

// RegisterJSONFlag binds --json on cmd and returns a pointer to read.
// Subcommands call this in init() and consume *flag in RunE.
func RegisterJSONFlag(cmd *cobra.Command) *bool {
	var jsonMode bool
	cmd.Flags().BoolVar(&jsonMode, "json", false,
		"emit JSON instead of human-readable text")
	return &jsonMode
}

// Render dispatches to human (text) or machine (JSON) based on jsonMode.
// human writes free-form text to w; machine is a JSON-encodable struct.
func Render(jsonMode bool, w io.Writer, human func(io.Writer) error, machine any) error {
	if jsonMode {
		return JSON(w, machine)
	}
	return human(w)
}

// JSON marshals v to indented JSON and writes to w with a trailing newline.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
```

- [ ] **Step 4: Replace the deliberately-broken Render tests in `cliout_test.go` with the correct ones**

Replace the two `TestRender_*` test functions with:

```go
func TestRender_TextMode(t *testing.T) {
	var buf bytes.Buffer
	humanCalled := false
	err := Render(false, &buf, func(w io.Writer) error {
		humanCalled = true
		w.Write([]byte("hello world"))
		return nil
	}, struct{}{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !humanCalled {
		t.Error("human closure was not invoked in text mode")
	}
	if !strings.Contains(buf.String(), "hello world") {
		t.Errorf("expected human text in output, got: %q", buf.String())
	}
}

func TestRender_JSONMode(t *testing.T) {
	type sample struct {
		Foo string `json:"foo"`
	}
	var buf bytes.Buffer
	err := Render(true, &buf, func(w io.Writer) error {
		t.Errorf("human closure should not run in JSON mode")
		return nil
	}, sample{Foo: "ok"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(buf.String(), `"foo": "ok"`) {
		t.Errorf("expected JSON output, got: %q", buf.String())
	}
}
```

Also add `"io"` to the imports block.

- [ ] **Step 5: Run tests, verify PASS**

```bash
go test ./internal/cliout/ -race -count=1 -v
```

Expected: all 4 tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cliout/
git commit -m "$(cat <<'EOF'
feat(cliout): shared text/JSON dual-rendering helpers

RegisterJSONFlag binds --json on a cobra command. Render dispatches
between a human-text closure and a JSON-encodable struct. JSON marshals
indented output with a trailing newline. Used by the four batch-1
inspection utilities (info / dump-ifds / extract / hash) to avoid
per-subcommand format-flag boilerplate.
EOF
)"
```

---

## Task 2: `wsi-tools info` subcommand

**Files:**
- Create: `cmd/wsi-tools/info.go`
- Create: `tests/integration/info_test.go`

Slide summary, openslide-show-properties analog. Pure read of `source.Source`.

- [ ] **Step 1: Write the failing integration test**

Create `tests/integration/info_test.go`:

```go
//go:build integration

package integration

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfo_HumanText(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out, err := exec.Command(bin, "info", src).CombinedOutput()
	if err != nil {
		t.Fatalf("info: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{
		"Format:  svs",
		"Levels:",
		"L0",
		"Associated images:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestInfo_JSON(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out, err := exec.Command(bin, "info", "--json", src).CombinedOutput()
	if err != nil {
		t.Fatalf("info --json: %v\n%s", err, out)
	}

	var got struct {
		Path      string `json:"path"`
		SizeBytes int64  `json:"size_bytes"`
		Format    string `json:"format"`
		Levels    []struct {
			Index       int    `json:"index"`
			Width       int    `json:"width"`
			Height      int    `json:"height"`
			Compression string `json:"compression"`
		} `json:"levels"`
		Associated []struct {
			Kind        string `json:"kind"`
			Compression string `json:"compression"`
		} `json:"associated_images"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if got.Format != "svs" {
		t.Errorf("Format = %q, want svs", got.Format)
	}
	if got.SizeBytes <= 0 {
		t.Errorf("SizeBytes = %d, want > 0", got.SizeBytes)
	}
	if len(got.Levels) == 0 {
		t.Errorf("no levels in JSON output")
	}
	if len(got.Associated) == 0 {
		t.Errorf("no associated images in JSON output")
	}
}
```

- [ ] **Step 2: Run, verify FAIL**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -count=1 -run TestInfo -v
```

Expected: FAIL — `unknown command "info"` (or build error if anything else is wrong first).

- [ ] **Step 3: Implement `cmd/wsi-tools/info.go`**

```go
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/cornish/wsi-tools/internal/cliout"
	"github.com/cornish/wsi-tools/internal/source"
)

var infoJSON *bool

var infoCmd = &cobra.Command{
	Use:   "info <file>",
	Short: "Print slide summary (format, levels, metadata, associated images)",
	Long: `Print a summary of a whole-slide image: file size, format,
scanner metadata (make/model/software/datetime/MPP/magnification),
pyramid levels (dimensions + tile size + compression per level), and
associated images (label/macro/thumbnail/overview).

Use --json to emit machine-readable JSON instead of human-readable text.`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func init() {
	infoJSON = cliout.RegisterJSONFlag(infoCmd)
	rootCmd.AddCommand(infoCmd)
}

type infoLevel struct {
	Index       int    `json:"index"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	TileWidth   int    `json:"tile_width"`
	TileHeight  int    `json:"tile_height"`
	Compression string `json:"compression"`
}

type infoAssoc struct {
	Kind        string `json:"kind"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Compression string `json:"compression"`
}

type infoMetadata struct {
	Make          string  `json:"make"`
	Model         string  `json:"model"`
	Software      string  `json:"software"`
	DateTime      string  `json:"datetime"`
	MPP           float64 `json:"mpp"`
	Magnification float64 `json:"magnification"`
}

type infoResult struct {
	Path       string       `json:"path"`
	SizeBytes  int64        `json:"size_bytes"`
	Format     string       `json:"format"`
	Metadata   infoMetadata `json:"metadata"`
	Levels     []infoLevel  `json:"levels"`
	Associated []infoAssoc  `json:"associated_images"`
}

func runInfo(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	path := args[0]

	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	src, err := source.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	md := src.Metadata()
	result := infoResult{
		Path:      path,
		SizeBytes: stat.Size(),
		Format:    src.Format(),
		Metadata: infoMetadata{
			Make:          md.Make,
			Model:         md.Model,
			Software:      md.Software,
			MPP:           md.MPP,
			Magnification: md.Magnification,
		},
	}
	if !md.AcquisitionDateTime.IsZero() {
		result.Metadata.DateTime = md.AcquisitionDateTime.Format(time.RFC3339)
	}
	for _, lvl := range src.Levels() {
		result.Levels = append(result.Levels, infoLevel{
			Index:       lvl.Index(),
			Width:       lvl.Size().X,
			Height:      lvl.Size().Y,
			TileWidth:   lvl.TileSize().X,
			TileHeight:  lvl.TileSize().Y,
			Compression: lvl.Compression().String(),
		})
	}
	for _, a := range src.Associated() {
		result.Associated = append(result.Associated, infoAssoc{
			Kind:        a.Kind(),
			Width:       a.Size().X,
			Height:      a.Size().Y,
			Compression: a.Compression().String(),
		})
	}

	return cliout.Render(*infoJSON, cmd.OutOrStdout(),
		func(w io.Writer) error { return renderInfoText(w, &result) },
		result)
}

func renderInfoText(w io.Writer, r *infoResult) error {
	fmt.Fprintf(w, "File:    %s (%s)\n", r.Path, formatBytes(r.SizeBytes))
	fmt.Fprintf(w, "Format:  %s\n", r.Format)
	if r.Metadata.Make != "" {
		fmt.Fprintf(w, "Make:    %s\n", r.Metadata.Make)
	}
	if r.Metadata.Model != "" {
		fmt.Fprintf(w, "Model:   %s\n", r.Metadata.Model)
	}
	if r.Metadata.Software != "" {
		fmt.Fprintf(w, "Software: %s\n", r.Metadata.Software)
	}
	if r.Metadata.DateTime != "" {
		fmt.Fprintf(w, "DateTime: %s\n", r.Metadata.DateTime)
	}
	if r.Metadata.MPP > 0 {
		fmt.Fprintf(w, "MPP:     %g\n", r.Metadata.MPP)
	}
	if r.Metadata.Magnification > 0 {
		fmt.Fprintf(w, "Magnification: %gx\n", r.Metadata.Magnification)
	}

	if len(r.Levels) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Levels:")
		for _, lvl := range r.Levels {
			fmt.Fprintf(w, "  L%d  %d × %d   tile %d×%d   %s\n",
				lvl.Index, lvl.Width, lvl.Height,
				lvl.TileWidth, lvl.TileHeight, lvl.Compression)
		}
	}
	if len(r.Associated) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Associated images:")
		for _, a := range r.Associated {
			fmt.Fprintf(w, "  %-10s %d × %d    %s\n",
				a.Kind, a.Width, a.Height, a.Compression)
		}
	}
	return nil
}
```

`formatBytes` is already defined in `cmd/wsi-tools/downsample.go:294` — same package, no import needed.

- [ ] **Step 4: Run tests, verify PASS**

```bash
make build
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -count=1 -run TestInfo -v
```

Expected: both sub-tests PASS.

- [ ] **Step 5: Run vet + unit suite**

```bash
make vet && make test
```

Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add cmd/wsi-tools/info.go tests/integration/info_test.go
git commit -m "$(cat <<'EOF'
feat(cli): info subcommand — slide summary (openslide-show-properties analog)

Prints file size, format, scanner metadata (make/model/software/
datetime/MPP/magnification), pyramid levels with dimensions+tile size+
compression, and associated images. --json emits a structured object
with the same fields. Pure read of source.Source; no new packages.
EOF
)"
```

---

## Task 3: `internal/source/ifdwalk.go` — TIFF IFD walker

**Files:**
- Create: `internal/source/ifdwalk.go`
- Create: `internal/source/ifdwalk_test.go`

Walks every IFD in a TIFF file (ClassicTIFF + BigTIFF, main chain + SubIFDs from tag 330) and returns per-IFD records with the tags `dump-ifds` needs.

This is the most code-heavy task. Read `internal/source/imagedesc.go` first — it already parses TIFF headers and walks the main IFD chain looking for tag 270; ifdwalk extends that pattern.

- [ ] **Step 1: Read existing TIFF parsing helpers**

```bash
cat internal/source/imagedesc.go
```

This shows the existing header parse + IFD walk pattern. ifdwalk follows the same shape but accumulates per-IFD records and walks SubIFDs.

- [ ] **Step 2: Write the failing test**

Create `internal/source/ifdwalk_test.go`:

```go
package source

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalkIFDs_SVS(t *testing.T) {
	testDir := os.Getenv("WSI_TOOLS_TESTDIR")
	if testDir == "" {
		t.Skip("WSI_TOOLS_TESTDIR not set")
	}
	path := filepath.Join(testDir, "svs", "CMU-1-Small-Region.svs")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	ifds, err := WalkIFDs(path)
	if err != nil {
		t.Fatalf("WalkIFDs: %v", err)
	}
	if len(ifds) < 4 {
		t.Errorf("expected >= 4 IFDs in CMU-1-Small-Region.svs, got %d", len(ifds))
	}
	first := ifds[0]
	if first.Width == 0 || first.Height == 0 {
		t.Errorf("IFD 0 dimensions zero: %+v", first)
	}
	// CMU-1-Small-Region.svs L0 is JPEG-compressed (TIFF compression tag 7).
	if first.Compression != 7 {
		t.Errorf("IFD 0 Compression = %d, want 7 (JPEG)", first.Compression)
	}
	// L0 should be tiled (TileWidth/TileHeight non-zero).
	if first.TileWidth == 0 || first.TileHeight == 0 {
		t.Errorf("IFD 0 should be tiled: %+v", first)
	}
}

func TestWalkIFDs_BigTIFF(t *testing.T) {
	testDir := os.Getenv("WSI_TOOLS_TESTDIR")
	if testDir == "" {
		t.Skip("WSI_TOOLS_TESTDIR not set")
	}
	// Use a BigTIFF fixture from the pool; pick one that exists.
	path := filepath.Join(testDir, "philips-tiff", "Philips-1.tiff")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	ifds, err := WalkIFDs(path)
	if err != nil {
		t.Fatalf("WalkIFDs: %v", err)
	}
	if len(ifds) == 0 {
		t.Fatalf("no IFDs walked")
	}
	for i, ifd := range ifds {
		if ifd.Width == 0 || ifd.Height == 0 {
			t.Errorf("IFD %d has zero dimensions: %+v", i, ifd)
		}
	}
}

func TestWalkIFDs_GenericTIFF(t *testing.T) {
	testDir := os.Getenv("WSI_TOOLS_TESTDIR")
	if testDir == "" {
		t.Skip("WSI_TOOLS_TESTDIR not set")
	}
	path := filepath.Join(testDir, "generic-tiff", "CMU-1.tiff")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	ifds, err := WalkIFDs(path)
	if err != nil {
		t.Fatalf("WalkIFDs: %v", err)
	}
	if len(ifds) == 0 {
		t.Fatalf("no IFDs walked")
	}
}
```

- [ ] **Step 3: Run, verify FAIL**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./internal/source/ -run TestWalkIFDs -v
```

Expected: compile error — `WalkIFDs` undefined.

- [ ] **Step 4: Implement `internal/source/ifdwalk.go`**

```go
package source

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// IFDRecord is one IFD's tags-of-interest, returned by WalkIFDs.
// Field defaults (zero for ints, "" for strings, nil for maps) indicate
// the tag was not present.
type IFDRecord struct {
	// Index is the 0-based position in walk order. Top-level IFDs are
	// numbered first in main-chain order; SubIFDs are appended in the
	// order they're discovered (after their parent).
	Index int

	// Offset is the IFD's byte offset in the file. Useful for ordering
	// by physical layout if the caller wants to.
	Offset int64

	// IsBigTIFF is true if the file uses BigTIFF format.
	IsBigTIFF bool

	// IsSubIFD is true for IFDs reached via tag 330 (SubIFDs) on a
	// parent IFD.
	IsSubIFD     bool
	ParentIndex  int // valid only when IsSubIFD; the parent's Index

	// Standard TIFF tags we extract for dump-ifds.
	Width            uint64 // tag 256
	Height           uint64 // tag 257
	TileWidth        uint64 // tag 322 (0 if not tiled)
	TileHeight       uint64 // tag 323 (0 if not tiled)
	Compression      uint64 // tag 259
	NewSubfileType   uint64 // tag 254
	ImageDescription string // tag 270 (truncated to 200 chars)

	// wsi-tools private tags 65080–65084.
	WSIImageType    string  // 65080
	WSILevelIndex   *uint64 // 65081 (pointer so we distinguish "absent" from 0)
	WSILevelCount   *uint64 // 65082
	WSISourceFormat string  // 65083
	WSIToolsVersion string  // 65084
}

// HasWSITags reports whether any of the wsi-tools private tags are present.
func (r *IFDRecord) HasWSITags() bool {
	return r.WSIImageType != "" || r.WSILevelIndex != nil ||
		r.WSILevelCount != nil || r.WSISourceFormat != "" ||
		r.WSIToolsVersion != ""
}

// WalkIFDs opens path and returns one IFDRecord per IFD found, in walk
// order: top-level chain first, with each IFD's SubIFDs (tag 330) appended
// immediately after the parent.
func WalkIFDs(path string) ([]IFDRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("ifdwalk: open %s: %w", path, err)
	}
	defer f.Close()

	hdr := make([]byte, 16)
	if _, err := io.ReadFull(f, hdr[:8]); err != nil {
		return nil, fmt.Errorf("ifdwalk: read header: %w", err)
	}
	var bo binary.ByteOrder
	switch string(hdr[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return nil, fmt.Errorf("ifdwalk: not a TIFF (bad byte order %q)", hdr[:2])
	}
	magic := bo.Uint16(hdr[2:4])
	var bigTIFF bool
	var firstIFDOff int64
	switch magic {
	case 42:
		bigTIFF = false
		firstIFDOff = int64(bo.Uint32(hdr[4:8]))
	case 43:
		bigTIFF = true
		// BigTIFF: bytes 4-5 = offset size (8), 6-7 = constant 0,
		//          8-15  = offset of first IFD.
		if _, err := io.ReadFull(f, hdr[8:16]); err != nil {
			return nil, fmt.Errorf("ifdwalk: read BigTIFF header: %w", err)
		}
		if bo.Uint16(hdr[4:6]) != 8 || bo.Uint16(hdr[6:8]) != 0 {
			return nil, fmt.Errorf("ifdwalk: malformed BigTIFF header")
		}
		firstIFDOff = int64(bo.Uint64(hdr[8:16]))
	default:
		return nil, fmt.Errorf("ifdwalk: bad TIFF magic %d", magic)
	}

	var out []IFDRecord
	// Walk the main chain, recording parents as we go so we can later
	// append their SubIFDs.
	type subPending struct {
		parentIndex int
		offsets     []int64
	}
	var pending []subPending

	off := firstIFDOff
	for off != 0 {
		rec, nextOff, subs, err := readIFD(f, bo, bigTIFF, off)
		if err != nil {
			return nil, err
		}
		rec.Index = len(out)
		rec.IsBigTIFF = bigTIFF
		out = append(out, rec)
		if len(subs) > 0 {
			pending = append(pending, subPending{
				parentIndex: rec.Index,
				offsets:     subs,
			})
		}
		off = nextOff
	}

	// Walk pending SubIFD chains, in the order their parents appeared.
	for _, p := range pending {
		for _, subOff := range p.offsets {
			// Each entry in the SubIFDs tag is itself the head of an
			// IFD chain (rare to have a chain, but follow it).
			cur := subOff
			for cur != 0 {
				rec, nextOff, _, err := readIFD(f, bo, bigTIFF, cur)
				if err != nil {
					return nil, err
				}
				rec.Index = len(out)
				rec.IsBigTIFF = bigTIFF
				rec.IsSubIFD = true
				rec.ParentIndex = p.parentIndex
				out = append(out, rec)
				cur = nextOff
			}
		}
	}

	return out, nil
}

// readIFD reads one IFD at offset off, populates an IFDRecord, and returns
// (record, nextIFDOffset, subIFDOffsets, err).
func readIFD(f *os.File, bo binary.ByteOrder, bigTIFF bool, off int64) (IFDRecord, int64, []int64, error) {
	var rec IFDRecord
	rec.Offset = off

	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return rec, 0, nil, fmt.Errorf("ifdwalk: seek IFD@%d: %w", off, err)
	}

	// Entry count: 2 bytes (Classic) or 8 bytes (BigTIFF).
	var nEntries uint64
	if bigTIFF {
		var b [8]byte
		if _, err := io.ReadFull(f, b[:]); err != nil {
			return rec, 0, nil, fmt.Errorf("ifdwalk: read entry count: %w", err)
		}
		nEntries = bo.Uint64(b[:])
	} else {
		var b [2]byte
		if _, err := io.ReadFull(f, b[:]); err != nil {
			return rec, 0, nil, fmt.Errorf("ifdwalk: read entry count: %w", err)
		}
		nEntries = uint64(bo.Uint16(b[:]))
	}

	entrySize := 12
	if bigTIFF {
		entrySize = 20
	}
	entriesBuf := make([]byte, int(nEntries)*entrySize)
	if _, err := io.ReadFull(f, entriesBuf); err != nil {
		return rec, 0, nil, fmt.Errorf("ifdwalk: read entries: %w", err)
	}

	var subIFDs []int64
	for i := uint64(0); i < nEntries; i++ {
		entry := entriesBuf[int(i)*entrySize : int(i+1)*entrySize]
		tag := bo.Uint16(entry[0:2])
		typ := bo.Uint16(entry[2:4])
		var count uint64
		var valueField []byte
		if bigTIFF {
			count = bo.Uint64(entry[4:12])
			valueField = entry[12:20]
		} else {
			count = uint64(bo.Uint32(entry[4:8]))
			valueField = entry[8:12]
		}

		readValue := func() ([]byte, error) {
			return readTagValue(f, bo, bigTIFF, typ, count, valueField)
		}

		switch tag {
		case 254: // NewSubfileType
			if v, err := readValue(); err == nil {
				rec.NewSubfileType = readUint(bo, typ, v)
			}
		case 256: // ImageWidth
			if v, err := readValue(); err == nil {
				rec.Width = readUint(bo, typ, v)
			}
		case 257: // ImageLength
			if v, err := readValue(); err == nil {
				rec.Height = readUint(bo, typ, v)
			}
		case 259: // Compression
			if v, err := readValue(); err == nil {
				rec.Compression = readUint(bo, typ, v)
			}
		case 270: // ImageDescription
			if v, err := readValue(); err == nil {
				s := string(v)
				if len(s) > 200 {
					s = s[:200]
				}
				// Strip trailing NUL.
				for len(s) > 0 && s[len(s)-1] == 0 {
					s = s[:len(s)-1]
				}
				rec.ImageDescription = s
			}
		case 322: // TileWidth
			if v, err := readValue(); err == nil {
				rec.TileWidth = readUint(bo, typ, v)
			}
		case 323: // TileLength
			if v, err := readValue(); err == nil {
				rec.TileHeight = readUint(bo, typ, v)
			}
		case 330: // SubIFDs
			if v, err := readValue(); err == nil {
				// One offset per count.
				step := 4
				if bigTIFF || typ == 16 {
					step = 8
				}
				for j := 0; j+step <= len(v); j += step {
					var off64 int64
					if step == 8 {
						off64 = int64(bo.Uint64(v[j : j+8]))
					} else {
						off64 = int64(bo.Uint32(v[j : j+4]))
					}
					if off64 > 0 {
						subIFDs = append(subIFDs, off64)
					}
				}
			}
		case 65080:
			if v, err := readValue(); err == nil {
				s := string(v)
				for len(s) > 0 && s[len(s)-1] == 0 {
					s = s[:len(s)-1]
				}
				rec.WSIImageType = s
			}
		case 65081:
			if v, err := readValue(); err == nil {
				u := readUint(bo, typ, v)
				rec.WSILevelIndex = &u
			}
		case 65082:
			if v, err := readValue(); err == nil {
				u := readUint(bo, typ, v)
				rec.WSILevelCount = &u
			}
		case 65083:
			if v, err := readValue(); err == nil {
				s := string(v)
				for len(s) > 0 && s[len(s)-1] == 0 {
					s = s[:len(s)-1]
				}
				rec.WSISourceFormat = s
			}
		case 65084:
			if v, err := readValue(); err == nil {
				s := string(v)
				for len(s) > 0 && s[len(s)-1] == 0 {
					s = s[:len(s)-1]
				}
				rec.WSIToolsVersion = s
			}
		}
	}

	// Read next-IFD offset (4 bytes Classic, 8 bytes BigTIFF).
	var nextOff int64
	if bigTIFF {
		var b [8]byte
		if _, err := io.ReadFull(f, b[:]); err != nil {
			return rec, 0, nil, fmt.Errorf("ifdwalk: read next-IFD offset: %w", err)
		}
		nextOff = int64(bo.Uint64(b[:]))
	} else {
		var b [4]byte
		if _, err := io.ReadFull(f, b[:]); err != nil {
			return rec, 0, nil, fmt.Errorf("ifdwalk: read next-IFD offset: %w", err)
		}
		nextOff = int64(bo.Uint32(b[:]))
	}

	return rec, nextOff, subIFDs, nil
}

// readTagValue returns the raw bytes for a TIFF tag's value, handling
// inline-vs-offset based on size. typ is the TIFF type; count is the
// number of elements; valueField is the 4-byte (Classic) or 8-byte
// (BigTIFF) inline value-or-offset slot.
func readTagValue(f *os.File, bo binary.ByteOrder, bigTIFF bool, typ uint16, count uint64, valueField []byte) ([]byte, error) {
	elemSize := tiffTypeSize(typ)
	if elemSize == 0 {
		return nil, fmt.Errorf("ifdwalk: unknown TIFF type %d", typ)
	}
	totalSize := count * uint64(elemSize)

	inlineCap := uint64(4)
	if bigTIFF {
		inlineCap = 8
	}
	if totalSize <= inlineCap {
		return valueField[:totalSize], nil
	}
	// Out-of-line: valueField holds an offset.
	var off int64
	if bigTIFF {
		off = int64(bo.Uint64(valueField))
	} else {
		off = int64(bo.Uint32(valueField))
	}
	cur, _ := f.Seek(0, io.SeekCurrent)
	defer f.Seek(cur, io.SeekStart)
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil, fmt.Errorf("ifdwalk: seek tag value@%d: %w", off, err)
	}
	out := make([]byte, totalSize)
	if _, err := io.ReadFull(f, out); err != nil {
		return nil, fmt.Errorf("ifdwalk: read tag value: %w", err)
	}
	return out, nil
}

func tiffTypeSize(typ uint16) int {
	// TIFF types: 1=BYTE, 2=ASCII, 3=SHORT, 4=LONG, 5=RATIONAL, 6=SBYTE,
	// 7=UNDEFINED, 8=SSHORT, 9=SLONG, 10=SRATIONAL, 11=FLOAT, 12=DOUBLE,
	// 13=IFD, 16=LONG8, 17=SLONG8, 18=IFD8.
	switch typ {
	case 1, 2, 6, 7:
		return 1
	case 3, 8:
		return 2
	case 4, 9, 11, 13:
		return 4
	case 5, 10, 12, 16, 17, 18:
		return 8
	}
	return 0
}

// readUint extracts a single uint64 from a tag value buffer for a uint-ish
// TIFF type (BYTE/SHORT/LONG/LONG8). Returns 0 if the buffer is empty or
// the type isn't a uint variant.
func readUint(bo binary.ByteOrder, typ uint16, v []byte) uint64 {
	switch typ {
	case 1: // BYTE
		if len(v) >= 1 {
			return uint64(v[0])
		}
	case 3: // SHORT
		if len(v) >= 2 {
			return uint64(bo.Uint16(v[:2]))
		}
	case 4, 13: // LONG, IFD
		if len(v) >= 4 {
			return uint64(bo.Uint32(v[:4]))
		}
	case 16, 18: // LONG8, IFD8
		if len(v) >= 8 {
			return bo.Uint64(v[:8])
		}
	}
	return 0
}
```

- [ ] **Step 5: Run tests, verify PASS**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./internal/source/ -run TestWalkIFDs -race -count=1 -v
```

Expected: all 3 sub-tests PASS.

- [ ] **Step 6: Run full source unit suite**

```bash
go test ./internal/source/ -race -count=1
```

Expected: all PASS (no regressions).

- [ ] **Step 7: Commit**

```bash
git add internal/source/ifdwalk.go internal/source/ifdwalk_test.go
git commit -m "$(cat <<'EOF'
feat(source): TIFF IFD walker for dump-ifds

WalkIFDs walks every IFD in a TIFF file (ClassicTIFF + BigTIFF, main
chain + SubIFDs from tag 330) and returns per-IFD records with the tags
dump-ifds needs: ImageWidth/Length, Compression, NewSubfileType, tile
size, ImageDescription excerpt, and the wsi-tools private tags
65080–65084.

Tested against SVS, Philips-TIFF, and generic-TIFF fixtures.
EOF
)"
```

---

## Task 4: `wsi-tools dump-ifds` subcommand

**Files:**
- Create: `cmd/wsi-tools/dump_ifds.go`
- Create: `tests/integration/dump_ifds_test.go`

Format-aware per-IFD layout dump using `WalkIFDs` from T3 + cross-reference against `source.Source` for classification.

- [ ] **Step 1: Write the failing integration test**

Create `tests/integration/dump_ifds_test.go`:

```go
//go:build integration

package integration

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDumpIFDs_HumanText(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out, err := exec.Command(bin, "dump-ifds", src).CombinedOutput()
	if err != nil {
		t.Fatalf("dump-ifds: %v\n%s", err, out)
	}
	got := string(out)
	for _, want := range []string{"IFD 0", "pyramid", "SubfileType="} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output:\n%s", want, got)
		}
	}
}

func TestDumpIFDs_JSON(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out, err := exec.Command(bin, "dump-ifds", "--json", src).CombinedOutput()
	if err != nil {
		t.Fatalf("dump-ifds --json: %v\n%s", err, out)
	}
	var got struct {
		Path   string `json:"path"`
		Format string `json:"format"`
		IFDs   []struct {
			Index           int    `json:"index"`
			Kind            string `json:"kind"`
			Width           uint64 `json:"width"`
			Height          uint64 `json:"height"`
			Compression     uint64 `json:"compression_tag"`
			CompressionName string `json:"compression"`
		} `json:"ifds"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if got.Format != "svs" {
		t.Errorf("Format = %q, want svs", got.Format)
	}
	if len(got.IFDs) < 4 {
		t.Errorf("expected >= 4 IFDs, got %d", len(got.IFDs))
	}
	// At least one IFD should be classified as a pyramid level.
	foundPyramid := false
	for _, ifd := range got.IFDs {
		if ifd.Kind == "pyramid" {
			foundPyramid = true
			break
		}
	}
	if !foundPyramid {
		t.Errorf("expected at least one IFD classified as pyramid")
	}
}
```

- [ ] **Step 2: Run, verify FAIL**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -count=1 -run TestDumpIFDs -v
```

Expected: FAIL — `unknown command "dump-ifds"`.

- [ ] **Step 3: Implement `cmd/wsi-tools/dump_ifds.go`**

```go
package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/cornish/wsi-tools/internal/cliout"
	"github.com/cornish/wsi-tools/internal/source"
)

var dumpIFDsJSON *bool

var dumpIFDsCmd = &cobra.Command{
	Use:   "dump-ifds <file>",
	Short: "Format-aware per-IFD layout dump (slim tiffinfo analog)",
	Long: `Dump every IFD in a TIFF-shaped WSI file in file order, annotated
with wsi-tools' format-aware classification (pyramid L0/L1/.../label/
macro/thumbnail/overview/probability/map). For each IFD: dimensions,
tile size (if tiled), compression, and SubfileType. Plus a separate
WSI-tags section listing wsi-tools' private tags 65080–65084 if present.

Not a full tiffinfo replacement — does not dump every TIFF tag. A future
--raw flag will expand this.

Use --json to emit machine-readable JSON instead of human-readable text.`,
	Args: cobra.ExactArgs(1),
	RunE: runDumpIFDs,
}

func init() {
	dumpIFDsJSON = cliout.RegisterJSONFlag(dumpIFDsCmd)
	rootCmd.AddCommand(dumpIFDsCmd)
}

type dumpIFDEntry struct {
	Index           int     `json:"index"`
	Kind            string  `json:"kind"`        // "pyramid", "label", "macro", "thumbnail", "overview", "probability", "map", "(unclassified)"
	LevelIndex      *int    `json:"level_index,omitempty"`
	Width           uint64  `json:"width"`
	Height          uint64  `json:"height"`
	TileWidth       uint64  `json:"tile_width"`
	TileHeight      uint64  `json:"tile_height"`
	Compression     uint64  `json:"compression_tag"`
	CompressionName string  `json:"compression"`
	SubfileType     uint64  `json:"subfile_type"`
	IsSubIFD        bool    `json:"is_subifd,omitempty"`
	WSITags         *wsiTag `json:"wsi_tags,omitempty"`
}

type wsiTag struct {
	WSIImageType    string  `json:"WSIImageType,omitempty"`
	WSILevelIndex   *uint64 `json:"WSILevelIndex,omitempty"`
	WSILevelCount   *uint64 `json:"WSILevelCount,omitempty"`
	WSISourceFormat string  `json:"WSISourceFormat,omitempty"`
	WSIToolsVersion string  `json:"WSIToolsVersion,omitempty"`
}

type dumpIFDsResult struct {
	Path   string         `json:"path"`
	Format string         `json:"format"`
	IFDs   []dumpIFDEntry `json:"ifds"`
}

func runDumpIFDs(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	path := args[0]

	ifds, err := source.WalkIFDs(path)
	if err != nil {
		return fmt.Errorf("walk IFDs: %w", err)
	}

	src, err := source.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	classifier := buildIFDClassifier(src)

	result := dumpIFDsResult{
		Path:   path,
		Format: src.Format(),
	}
	for _, ifd := range ifds {
		entry := dumpIFDEntry{
			Index:           ifd.Index,
			Width:           ifd.Width,
			Height:          ifd.Height,
			TileWidth:       ifd.TileWidth,
			TileHeight:      ifd.TileHeight,
			Compression:     ifd.Compression,
			CompressionName: tiffCompressionName(ifd.Compression),
			SubfileType:     ifd.NewSubfileType,
			IsSubIFD:        ifd.IsSubIFD,
		}
		entry.Kind, entry.LevelIndex = classifier(ifd)
		if ifd.HasWSITags() {
			entry.WSITags = &wsiTag{
				WSIImageType:    ifd.WSIImageType,
				WSILevelIndex:   ifd.WSILevelIndex,
				WSILevelCount:   ifd.WSILevelCount,
				WSISourceFormat: ifd.WSISourceFormat,
				WSIToolsVersion: ifd.WSIToolsVersion,
			}
		}
		result.IFDs = append(result.IFDs, entry)
	}

	return cliout.Render(*dumpIFDsJSON, cmd.OutOrStdout(),
		func(w io.Writer) error { return renderDumpIFDsText(w, &result) },
		result)
}

// buildIFDClassifier returns a function that, given an IFDRecord, returns
// (kind, levelIndex). It crossrefs against source.Source's Levels() and
// Associated() by matching (width, height, compression-string) tuples.
func buildIFDClassifier(src source.Source) func(source.IFDRecord) (string, *int) {
	type key struct {
		w, h uint64
		comp string
	}
	type val struct {
		kind  string
		level *int
	}
	m := map[key]val{}
	for _, lvl := range src.Levels() {
		k := key{
			w:    uint64(lvl.Size().X),
			h:    uint64(lvl.Size().Y),
			comp: lvl.Compression().String(),
		}
		idx := lvl.Index()
		m[k] = val{kind: "pyramid", level: &idx}
	}
	for _, a := range src.Associated() {
		k := key{
			w:    uint64(a.Size().X),
			h:    uint64(a.Size().Y),
			comp: a.Compression().String(),
		}
		// Don't overwrite a level mapping if dimensions+comp collide
		// (extremely rare, but be safe).
		if _, ok := m[k]; !ok {
			m[k] = val{kind: a.Kind()}
		}
	}
	return func(ifd source.IFDRecord) (string, *int) {
		comp := tiffCompressionName(ifd.Compression)
		k := key{w: ifd.Width, h: ifd.Height, comp: comp}
		if v, ok := m[k]; ok {
			return v.kind, v.level
		}
		return "(unclassified)", nil
	}
}

// tiffCompressionName maps the TIFF compression tag (259) value to a
// human-readable name matching opentile-go's Compression.String() output
// for the codes opentile-go knows about; falls back to "tag-N" otherwise.
func tiffCompressionName(tag uint64) string {
	switch tag {
	case 1:
		return "none"
	case 5:
		return "lzw"
	case 7:
		return "jpeg"
	case 8, 32946:
		return "deflate"
	case 33003, 33005, 34712:
		return "jpeg2000"
	case 50001:
		return "webp"
	case 50002:
		return "jpegxl"
	case 60001:
		return "avif"
	case 60003:
		return "htj2k"
	}
	return fmt.Sprintf("tag-%d", tag)
}

func renderDumpIFDsText(w io.Writer, r *dumpIFDsResult) error {
	for _, ifd := range r.IFDs {
		label := ifd.Kind
		if ifd.LevelIndex != nil {
			label = fmt.Sprintf("pyramid L%d", *ifd.LevelIndex)
		}
		tile := ""
		if ifd.TileWidth > 0 && ifd.TileHeight > 0 {
			tile = fmt.Sprintf("  tile %d×%d", ifd.TileWidth, ifd.TileHeight)
		}
		fmt.Fprintf(w, "IFD %d  %-12s  %d × %d%s   %s   SubfileType=%d\n",
			ifd.Index, label, ifd.Width, ifd.Height, tile,
			ifd.CompressionName, ifd.SubfileType)
	}

	// WSI tags section: collate all WSI-tag-bearing IFDs.
	var hasWSITags bool
	for _, ifd := range r.IFDs {
		if ifd.WSITags != nil {
			hasWSITags = true
			break
		}
	}
	if hasWSITags {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "WSI tags (private 65080–65084):")
		for _, ifd := range r.IFDs {
			if ifd.WSITags == nil {
				continue
			}
			t := ifd.WSITags
			fmt.Fprintf(w, "  IFD %d:", ifd.Index)
			if t.WSIImageType != "" {
				fmt.Fprintf(w, " WSIImageType=%s", t.WSIImageType)
			}
			if t.WSILevelIndex != nil {
				fmt.Fprintf(w, " WSILevelIndex=%d", *t.WSILevelIndex)
			}
			if t.WSILevelCount != nil {
				fmt.Fprintf(w, " WSILevelCount=%d", *t.WSILevelCount)
			}
			if t.WSISourceFormat != "" {
				fmt.Fprintf(w, " WSISourceFormat=%s", t.WSISourceFormat)
			}
			if t.WSIToolsVersion != "" {
				fmt.Fprintf(w, " WSIToolsVersion=%s", t.WSIToolsVersion)
			}
			fmt.Fprintln(w)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run tests, verify PASS**

```bash
make build
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -count=1 -run TestDumpIFDs -v
```

Expected: both sub-tests PASS.

- [ ] **Step 5: Run vet + unit suite**

```bash
make vet && make test
```

Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add cmd/wsi-tools/dump_ifds.go tests/integration/dump_ifds_test.go
git commit -m "$(cat <<'EOF'
feat(cli): dump-ifds subcommand — slim tiffinfo analog with format-aware classification

Walks every IFD in file order via source.WalkIFDs, cross-references each
IFD against opentile-go's classification (pyramid L0/L1/.../label/macro/
thumbnail/overview/probability/map) by matching (width, height,
compression) tuples. Emits the wsi-tools private tags (65080–65084)
when present.

Not a full tiffinfo replacement — does not dump every TIFF tag.
A --raw expansion is reserved for batch 2.
EOF
)"
```

---

## Task 5: `wsi-tools extract` subcommand

**Files:**
- Create: `cmd/wsi-tools/extract.go`
- Create: `tests/integration/extract_test.go`

Save an associated image as PNG or JPEG.

- [ ] **Step 1: Write the failing integration test**

Create `tests/integration/extract_test.go`:

```go
//go:build integration

package integration

import (
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExtract_LabelAsPNG(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out := filepath.Join(t.TempDir(), "label.png")
	cmd := exec.Command(bin, "extract", "--kind", "label", "-o", out, src)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("extract: %v\n%s", err, b)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	if img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
		t.Errorf("decoded PNG has zero dimensions: %v", img.Bounds())
	}
}

func TestExtract_MacroAsJPEG_PassThrough(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out := filepath.Join(t.TempDir(), "macro.jpg")
	cmd := exec.Command(bin, "extract", "--kind", "macro", "-o", out, "--format", "jpeg", src)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("extract: %v\n%s", err, b)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		t.Errorf("output is not a JPEG (missing SOI marker): %x", data[:4])
	}
}

func TestExtract_UnknownKind_Errors(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out := filepath.Join(t.TempDir(), "x.png")
	cmd := exec.Command(bin, "extract", "--kind", "nonexistent", "-o", out, src)
	if b, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("expected error for unknown --kind, got success:\n%s", b)
	}
}
```

- [ ] **Step 2: Run, verify FAIL**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -count=1 -run TestExtract -v
```

Expected: FAIL — `unknown command "extract"`.

- [ ] **Step 3: Implement `cmd/wsi-tools/extract.go`**

```go
package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"strings"

	"github.com/spf13/cobra"
	xtiff "golang.org/x/image/tiff"

	"github.com/cornish/wsi-tools/internal/decoder"
	"github.com/cornish/wsi-tools/internal/source"
)

var (
	extractKind   string
	extractOutput string
	extractFormat string
)

var extractCmd = &cobra.Command{
	Use:   "extract <file>",
	Short: "Save an associated image (label/macro/thumbnail/overview) as PNG or JPEG",
	Long: `Save an associated image embedded in a WSI as a standalone PNG or JPEG file.

Available associated-image kinds depend on the source format and the slide:
typically label, macro, thumbnail, overview. Run 'wsi-tools info <file>'
to list which kinds the slide carries.

For --format jpeg, when the source associated image is already JPEG-compressed,
the original bytes are passed through verbatim (no decode/re-encode loss).
For --format png, the image is decoded to RGB and re-encoded as PNG.`,
	Args: cobra.ExactArgs(1),
	RunE: runExtract,
}

func init() {
	extractCmd.Flags().StringVar(&extractKind, "kind", "", "associated-image kind (label|macro|thumbnail|overview)")
	extractCmd.Flags().StringVarP(&extractOutput, "output", "o", "", "output file path")
	extractCmd.Flags().StringVar(&extractFormat, "format", "png", "output format: png|jpeg")
	_ = extractCmd.MarkFlagRequired("kind")
	_ = extractCmd.MarkFlagRequired("output")
	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	path := args[0]

	if extractFormat != "png" && extractFormat != "jpeg" {
		return fmt.Errorf("--format must be png or jpeg, got %q", extractFormat)
	}

	src, err := source.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()

	var match source.AssociatedImage
	var available []string
	for _, a := range src.Associated() {
		available = append(available, a.Kind())
		if a.Kind() == extractKind {
			match = a
		}
	}
	if match == nil {
		return fmt.Errorf("no associated image with kind %q (available: %s)",
			extractKind, strings.Join(available, ", "))
	}

	bytesIn, err := match.Bytes()
	if err != nil {
		return fmt.Errorf("read associated %s: %w", extractKind, err)
	}
	srcComp := match.Compression()

	// JPEG byte-pass-through path: source is JPEG and user wants JPEG.
	if extractFormat == "jpeg" && srcComp == source.CompressionJPEG {
		if err := os.WriteFile(extractOutput, bytesIn, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", extractOutput, err)
		}
		fmt.Printf("wrote %s (%s)\n", extractOutput, formatBytes(int64(len(bytesIn))))
		return nil
	}

	// Decode → re-encode path.
	img, err := decodeAssociated(bytesIn, srcComp, match.Size().X, match.Size().Y)
	if err != nil {
		return err
	}

	out, err := os.Create(extractOutput)
	if err != nil {
		return fmt.Errorf("create %s: %w", extractOutput, err)
	}
	defer out.Close()
	switch extractFormat {
	case "png":
		if err := png.Encode(out, img); err != nil {
			return fmt.Errorf("encode png: %w", err)
		}
	case "jpeg":
		if err := jpeg.Encode(out, img, &jpeg.Options{Quality: 90}); err != nil {
			return fmt.Errorf("encode jpeg: %w", err)
		}
	}
	stat, _ := os.Stat(extractOutput)
	if stat != nil {
		fmt.Printf("wrote %s (%s)\n", extractOutput, formatBytes(stat.Size()))
	}
	return nil
}

func decodeAssociated(b []byte, comp source.Compression, w, h int) (image.Image, error) {
	switch comp {
	case source.CompressionJPEG:
		dec := decoder.NewJPEG()
		rgb := make([]byte, w*h*3)
		out, err := dec.DecodeTile(b, rgb, 1, 1)
		if err != nil {
			return nil, fmt.Errorf("decode jpeg: %w", err)
		}
		return rgbToImage(out, w, h), nil
	case source.CompressionJPEG2000:
		dec := decoder.NewJPEG2000()
		rgb := make([]byte, w*h*3)
		out, err := dec.DecodeTile(b, rgb, 1, 1)
		if err != nil {
			return nil, fmt.Errorf("decode jpeg2000: %w", err)
		}
		return rgbToImage(out, w, h), nil
	case source.CompressionLZW, source.CompressionDeflate, source.CompressionNone:
		// Wrap as a tiny single-IFD TIFF and use x/image/tiff to decode.
		return xtiff.Decode(bytes.NewReader(b))
	}
	return nil, fmt.Errorf("source compression %s is not decodable for extract", comp)
}

func rgbToImage(rgb []byte, w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			si := (y*w + x) * 3
			di := y*img.Stride + x*4
			img.Pix[di+0] = rgb[si+0]
			img.Pix[di+1] = rgb[si+1]
			img.Pix[di+2] = rgb[si+2]
			img.Pix[di+3] = 0xFF
		}
	}
	return img
}
```

Note: the LZW/Deflate/None branch wraps the bytes through `golang.org/x/image/tiff`. Some associated-image bytes (e.g., LZW labels in SVS) are stored as raw stripped TIFF data — `xtiff.Decode` handles that when the bytes already include a TIFF wrapper. If a fixture surfaces a case where the bytes are raw codec output without the wrapper, the implementation will need adjustment (rewrap as a single-IFD TIFF). Verify against the SVS label fixture.

- [ ] **Step 4: Run tests, verify PASS**

```bash
make build
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -count=1 -run TestExtract -v
```

Expected: 3 sub-tests PASS. If `TestExtract_LabelAsPNG` fails because the SVS label bytes don't have a TIFF wrapper, escalate via DONE_WITH_CONCERNS — we'll need to either pre-wrap in the wsiwriter or use a different decode path.

- [ ] **Step 5: Run vet + unit suite**

```bash
make vet && make test
```

- [ ] **Step 6: Commit**

```bash
git add cmd/wsi-tools/extract.go tests/integration/extract_test.go
git commit -m "$(cat <<'EOF'
feat(cli): extract subcommand — save associated image as PNG or JPEG

Reads source.Source's Associated() bytes, decodes via internal/decoder
(jpeg/jpeg2000) or golang.org/x/image/tiff (lzw/deflate/none), and
re-encodes via Go stdlib image/png or image/jpeg at quality 90. When
--format jpeg and the source is already JPEG, bytes pass through
verbatim (no decode/re-encode loss).
EOF
)"
```

---

## Task 6: `wsi-tools hash` subcommand

**Files:**
- Create: `cmd/wsi-tools/hash.go`
- Create: `tests/integration/hash_test.go`

Content hash, file or pixel mode.

- [ ] **Step 1: Write the failing integration test**

Create `tests/integration/hash_test.go`:

```go
//go:build integration

package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHash_FileMode(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out, err := exec.Command(bin, "hash", src).CombinedOutput()
	if err != nil {
		t.Fatalf("hash: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	// Compute expected file hash directly.
	want := computeFileHash(t, src)
	if !strings.HasPrefix(got, "sha256:") {
		t.Errorf("expected sha256: prefix, got: %q", got)
	}
	if !strings.Contains(got, want) {
		t.Errorf("expected hash %s in output, got: %q", want, got)
	}
}

func TestHash_PixelMode_Deterministic(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out1, err := exec.Command(bin, "hash", "--mode", "pixel", src).CombinedOutput()
	if err != nil {
		t.Fatalf("hash --mode pixel: %v\n%s", err, out1)
	}
	out2, err := exec.Command(bin, "hash", "--mode", "pixel", src).CombinedOutput()
	if err != nil {
		t.Fatalf("hash --mode pixel (run 2): %v\n%s", err, out2)
	}
	if string(out1) != string(out2) {
		t.Errorf("pixel hash not deterministic across runs:\nrun 1: %s\nrun 2: %s",
			out1, out2)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(out1)), "sha256-pixel:") {
		t.Errorf("expected sha256-pixel: prefix, got: %q", out1)
	}
}

func TestHash_JSON(t *testing.T) {
	td := testdir(t)
	src := filepath.Join(td, "svs", "CMU-1-Small-Region.svs")
	bin := buildOnce(t)

	out, err := exec.Command(bin, "hash", "--json", src).CombinedOutput()
	if err != nil {
		t.Fatalf("hash --json: %v\n%s", err, out)
	}
	var got struct {
		Algorithm string `json:"algorithm"`
		Mode      string `json:"mode"`
		Hex       string `json:"hex"`
		Path      string `json:"path"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if got.Algorithm != "sha256" || got.Mode != "file" {
		t.Errorf("got %+v, want algorithm=sha256 mode=file", got)
	}
	if len(got.Hex) != 64 {
		t.Errorf("hex length = %d, want 64", len(got.Hex))
	}
}

func computeFileHash(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("hash file: %v", err)
	}
	return hex.EncodeToString(h.Sum(nil))
}
```

- [ ] **Step 2: Run, verify FAIL**

```bash
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -count=1 -run TestHash -v
```

Expected: FAIL — `unknown command "hash"`.

- [ ] **Step 3: Implement `cmd/wsi-tools/hash.go`**

```go
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/cornish/wsi-tools/internal/cliout"
	"github.com/cornish/wsi-tools/internal/decoder"
	"github.com/cornish/wsi-tools/internal/source"
)

var (
	hashMode string
	hashJSON *bool
)

var hashCmd = &cobra.Command{
	Use:   "hash <file>",
	Short: "Content hash (file or pixel mode) — openslide-quickhash1 analog",
	Long: `Compute a SHA-256 hash of a slide file.

--mode file (default): SHA-256 of the file bytes — equivalent to
sha256sum. Cheap and works for every format.

--mode pixel: SHA-256 of L0 tiles decoded to RGB in raster order.
Stable across re-encode at the same nominal quality. Errors cleanly if
the L0 compression isn't decodable. NOT byte-for-byte compatible with
openslide's quickhash1 algorithm.

The output prefix (sha256: vs sha256-pixel:) names the algorithm so any
future algorithm change can use a different prefix.`,
	Args: cobra.ExactArgs(1),
	RunE: runHash,
}

func init() {
	hashCmd.Flags().StringVar(&hashMode, "mode", "file", "hash mode: file|pixel")
	hashJSON = cliout.RegisterJSONFlag(hashCmd)
	rootCmd.AddCommand(hashCmd)
}

type hashResult struct {
	Algorithm string `json:"algorithm"`
	Mode      string `json:"mode"`
	Hex       string `json:"hex"`
	Path      string `json:"path"`
}

func runHash(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	path := args[0]

	switch hashMode {
	case "file":
		hex, err := hashFile(path)
		if err != nil {
			return err
		}
		return emitHash(cmd, "sha256", "file", hex, path)
	case "pixel":
		hex, err := hashL0Pixels(path)
		if err != nil {
			return err
		}
		return emitHash(cmd, "sha256", "pixel", hex, path)
	}
	return fmt.Errorf("--mode must be file or pixel, got %q", hashMode)
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashL0Pixels(path string) (string, error) {
	src, err := source.Open(path)
	if err != nil {
		return "", err
	}
	defer src.Close()
	levels := src.Levels()
	if len(levels) == 0 {
		return "", fmt.Errorf("no levels in %s", path)
	}
	l0 := levels[0]
	dec := pickDecoderForCompression(l0.Compression())
	if dec == nil {
		return "", fmt.Errorf("L0 compression %s is not decodable for pixel hash",
			l0.Compression())
	}

	tileBuf := make([]byte, l0.TileMaxSize())
	rgbBuf := make([]byte, l0.TileSize().X*l0.TileSize().Y*3)
	h := sha256.New()
	grid := l0.Grid()
	for ty := 0; ty < grid.Y; ty++ {
		for tx := 0; tx < grid.X; tx++ {
			n, err := l0.TileInto(tx, ty, tileBuf)
			if err != nil {
				return "", fmt.Errorf("read tile (%d,%d): %w", tx, ty, err)
			}
			out, err := dec.DecodeTile(tileBuf[:n], rgbBuf, 1, 1)
			if err != nil {
				return "", fmt.Errorf("decode tile (%d,%d): %w", tx, ty, err)
			}
			h.Write(out)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func pickDecoderForCompression(c source.Compression) decoder.Decoder {
	switch c {
	case source.CompressionJPEG:
		return decoder.NewJPEG()
	case source.CompressionJPEG2000:
		return decoder.NewJPEG2000()
	}
	return nil
}

func emitHash(cmd *cobra.Command, algorithm, mode, hexStr, path string) error {
	r := hashResult{Algorithm: algorithm, Mode: mode, Hex: hexStr, Path: path}
	return cliout.Render(*hashJSON, cmd.OutOrStdout(),
		func(w io.Writer) error {
			prefix := "sha256"
			if mode == "pixel" {
				prefix = "sha256-pixel"
			}
			fmt.Fprintf(w, "%s:%s %s\n", prefix, hexStr, path)
			return nil
		}, r)
}
```

- [ ] **Step 4: Run tests, verify PASS**

```bash
make build
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -count=1 -run TestHash -v
```

Expected: 3 sub-tests PASS.

- [ ] **Step 5: Run vet + unit suite**

```bash
make vet && make test
```

- [ ] **Step 6: Commit**

```bash
git add cmd/wsi-tools/hash.go tests/integration/hash_test.go
git commit -m "$(cat <<'EOF'
feat(cli): hash subcommand — file or pixel content hash

--mode file (default): SHA-256 of file bytes (sha256sum equivalent).
--mode pixel: SHA-256 of L0 tiles decoded to RGB in raster order;
errors cleanly when L0 compression isn't decodable. The output prefix
(sha256: vs sha256-pixel:) names the algorithm so future algorithm
changes can use a different prefix.

Not byte-for-byte compatible with openslide-quickhash1 (which is
per-format and includes additional canonicalization).
EOF
)"
```

---

## Task 7: `docs/roadmap.md` + CHANGELOG v0.4.0 entry

**Files:**
- Create: `docs/roadmap.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Write `docs/roadmap.md`**

```markdown
# wsi-tools utilities roadmap

Tracks the full set of CLI utilities planned for wsi-tools, organised
into "shipped" and "planned" sections. The shipped section is updated
as releases land; the planned section is the source of truth for what's
queued, deferred, or under consideration.

## Shipped

### v0.1
- `downsample` — produce a lower-magnification SVS by an integer power-of-2 factor.
- `doctor` — list registered codecs + cgo deps.
- `version` — print version + Go runtime info.

### v0.2
- `transcode` — re-encode pyramid tiles in a different codec (jpeg, jpegxl, avif, webp, htj2k); 6 sane source formats; streaming pipeline.

### v0.3
- (no new utilities — opentile-go v0.14 migration milestone; novel-codec round-trip + sync.Pool + TileInto adoption).

### v0.4
- `info` — slide summary (openslide-show-properties analog).
- `dump-ifds` — format-aware per-IFD layout dump (slim tiffinfo analog).
- `extract` — save associated image (label/macro/thumbnail/overview) as PNG or JPEG.
- `hash` — content hash (file mode default; pixel mode opt-in).

## Planned

### Batch 2 — extends batch 1
- **`region`** — openslide-write-png analog: extract `--x --y --w --h --level` rectangle as PNG. Requires tile decode + stitching across boundaries.
- **`dump-tile`** — single tile's compressed bytes to file or stdout. Pure debug aid.
- **`dump-ifds --raw`** — full tiffinfo-style tag dump per IFD; expansion of v0.4's slim dump-ifds.

### Batch 3 — operations
- **`tagset`** — in-place TIFF tag edit (e.g. ImageDescription, Software). Useful for fixing one bad slide in a pool without full re-encode.
- **`inventory`** — walk a directory; dump CSV/JSON of slide metadata for pool-management UIs.
- **`verify`** — open every IFD, decode every tile, report errors. "fsck for WSI."
- **`diff`** — compare two slides (pixel diff, metadata diff, IFD ordering diff).

### Larger items
- **`dzsave`** — DeepZoom pyramid generator; OpenSeadragon-compatible tile tree. Analog of libvips `dzsave` and openslide-python `deepzoom_tile.py`.
- **`tile-server`** — HTTP DZI/IIIF tile server; analog of openslide-python `deepzoom_server.py`. Activates opentile-go v0.13's splice-prefix optimization (TilePrefix / TileBodyInto / SpliceJPEGTile).
- **`dicom-wsi`** — convert WSI to DICOM-WSI format. Analog of `wsi2dcm` (highdicom) and `wsidicomizer`.

## Codecs (write-side, separate from utilities)

### Deferred from v0.2
- `jpegli` — blocked on Homebrew libjxl shipping libjpegli OR build-from-source.
- `HEIF`, `JPEG-LS`, `JPEG-XR`, `Basis Universal` — queued.
- `jpeg2000` as a transcode-encoder target — decoder shipped; encoder wrapper queued.

## Source format support

### Deferred from v0.2
- Leica SCN — multi-image / multi-channel pipeline plumbing.

## Architectural

### Deferred from v0.2
- Streaming retrofit for `downsample` — currently materialises full L0 raster.

### Deferred from v0.3
- TilePrefix / TileBodyInto / SpliceJPEGTile adoption — only valuable if `tile-server` is built.

## Quality gates

### Deferred from v0.2
- Visual-fidelity tests via mini decoders — decode v0.2 codec outputs through matching codec library; pixel-compare against source.
```

- [ ] **Step 2: Add `[0.4.0]` entry to CHANGELOG.md**

In `CHANGELOG.md`, just under `## [Unreleased]` and above `## [0.3.1]`, insert:

```markdown
## [0.4.0] — 2026-05-09

Inspection-utilities milestone. Adds four read-side CLI utilities —
`info`, `dump-ifds`, `extract`, `hash` — analogs of openslide-tools and
slim tiffinfo. Plus a shared `internal/cliout` package for text/JSON
dual rendering, and a top-level `docs/roadmap.md` tracking the full
utilities roadmap.

### Added

- **`wsi-tools info <file>`** — slide summary: file size, format,
  scanner metadata (make/model/software/datetime/MPP/magnification),
  pyramid levels with dimensions+tile size+compression per level, and
  associated images. `--json` emits a structured object.
- **`wsi-tools dump-ifds <file>`** — format-aware per-IFD layout dump.
  Walks every IFD in file order (ClassicTIFF + BigTIFF, main chain +
  SubIFDs), cross-references each against opentile-go's classifier
  (pyramid L0/L1/.../label/macro/thumbnail/overview/probability/map),
  and reports any wsi-tools private tags (65080–65084) present. Not a
  full tiffinfo replacement — does not dump every TIFF tag. A `--raw`
  expansion is reserved for batch 2.
- **`wsi-tools extract --kind <k> -o <path> <file>`** — save an
  associated image (label/macro/thumbnail/overview) as PNG (default) or
  JPEG. When `--format jpeg` and source is already JPEG, bytes pass
  through verbatim. PNG path decodes via `internal/decoder` (jpeg or
  jpeg2000) or `golang.org/x/image/tiff` (lzw/deflate/none).
- **`wsi-tools hash <file>`** — content hash. `--mode file` (default):
  SHA-256 of file bytes, `sha256sum` equivalent. `--mode pixel`:
  SHA-256 of L0 tiles decoded to RGB in raster order, stable across
  re-encode. Output prefix names the algorithm (`sha256:` vs
  `sha256-pixel:`) so any future algorithm change can use a different
  prefix. Not byte-for-byte compatible with openslide-quickhash1.
- **`internal/cliout`** — shared text/JSON dual-rendering helpers:
  `RegisterJSONFlag`, `Render`, `JSON`. Used by all four batch-1
  utilities to avoid per-subcommand format-flag boilerplate.
- **`internal/source.WalkIFDs`** — TIFF IFD walker (ClassicTIFF +
  BigTIFF, main chain + SubIFDs from tag 330) returning per-IFD
  records with the tags `dump-ifds` needs.
- **`docs/roadmap.md`** — durable record of the full utilities roadmap
  (batch 1, batch 2, batch 3, and larger items: dzsave, tile-server,
  DICOM-WSI conversion).
```

- [ ] **Step 3: Verify CHANGELOG section ordering**

```bash
grep -n '^## ' CHANGELOG.md | head
```

Expected order: `[Unreleased]`, `[0.4.0]`, `[0.3.1]`, `[0.3.0]`, `[0.2.0]`, `[0.1.0]`.

- [ ] **Step 4: Commit**

```bash
git add docs/roadmap.md CHANGELOG.md
git commit -m "$(cat <<'EOF'
docs: add roadmap.md + CHANGELOG.md for v0.4.0

Roadmap tracks the full utilities roadmap (batch 1 shipped here,
batch 2/3, plus larger items: dzsave, tile-server, DICOM-WSI). Living
at docs/roadmap.md so it's discoverable to anyone browsing the repo.
EOF
)"
```

---

## Task 8: Final smoke test + tag v0.4.0

**Files:**
- Modify: `cmd/wsi-tools/version.go` (release commit then post-release commit)

- [ ] **Step 1: Final regression check**

```bash
make vet
make build
make test
WSI_TOOLS_TESTDIR=$HOME/GitHub/opentile-go/sample_files \
  go test ./tests/integration/ -tags integration -count=1 -timeout 60m
./bin/wsi-tools doctor
./bin/wsi-tools version
./bin/wsi-tools info sample_files/svs/CMU-1-Small-Region.svs
./bin/wsi-tools dump-ifds sample_files/svs/CMU-1-Small-Region.svs
./bin/wsi-tools hash sample_files/svs/CMU-1-Small-Region.svs
./bin/wsi-tools extract --kind label -o /tmp/v04-label.png sample_files/svs/CMU-1-Small-Region.svs
ls -la /tmp/v04-label.png && rm /tmp/v04-label.png
```

Expected: every step passes. `wsi-tools version` should print `wsi-tools 0.4.0-dev`.

(Note: the integration sweep timeout is 60m here, not 30m — see the project memory on integration-sweep timing.)

- [ ] **Step 2: Bump Version literal to 0.4.0**

In `cmd/wsi-tools/version.go`, change:

```go
const Version = "0.4.0-dev"
```

to:

```go
const Version = "0.4.0"
```

```bash
go build -o bin/wsi-tools ./cmd/wsi-tools
./bin/wsi-tools version
```

Expected: prints `wsi-tools 0.4.0`.

```bash
git add cmd/wsi-tools/version.go
git commit -m "release: bump Version to 0.4.0"
```

- [ ] **Step 3: Merge feat branch into main + tag (LOCAL ONLY — STOP for confirmation before push)**

```bash
git checkout main
git merge --ff-only feat/v0.4-batch1-utilities
git tag -a v0.4.0 -m "wsi-tools v0.4.0 — batch 1 inspection utilities (info, dump-ifds, extract, hash)"
git log --oneline -5
git tag -l v0.4.0 -n5
```

**STOP HERE and report state to the controller. The next step pushes to origin and creates a public GH release — confirm before proceeding.**

- [ ] **Step 4: Push origin/main + tag (irreversible — only after confirmation)**

```bash
git push origin main
git push origin v0.4.0
```

- [ ] **Step 5: Create GitHub Release**

```bash
awk '/^## \[0\.4\.0\]/{flag=1; next} /^## \[/{flag=0} flag' CHANGELOG.md > /tmp/v0.4.0-release-notes.md
gh release create v0.4.0 --title "v0.4.0 — batch 1 inspection utilities" --notes-file /tmp/v0.4.0-release-notes.md
```

- [ ] **Step 6: Bump Version back to 0.5.0-dev on main**

In `cmd/wsi-tools/version.go`:

```go
const Version = "0.5.0-dev"
```

```bash
git add cmd/wsi-tools/version.go
git commit -m "post-release: bump Version to 0.5.0-dev"
git push origin main
```

- [ ] **Step 7: Verify CI on the v0.4.0 tag**

```bash
gh run list --branch v0.4.0 --limit 1
```

Wait for the tag's CI run to complete (per project memory, always verify tag CI before declaring shipped). Check both jobs (build + test (macOS), build (Windows)) report success.

If Windows fails, investigate immediately — the codec/all build-tag pattern (per `internal/codec/all/all_<name>.go` files) should have the new utilities (`info`, `dump-ifds`, `extract`, `hash`) compile-clean since they don't import any optional codec directly. But verify rather than assume.

---

## Self-review checklist (executor: do this after Task 8)

1. **All 8 tasks committed?** `git log --oneline v0.3.1..HEAD` — expect ~10 commits (one per task plus the release/post-release version bumps).
2. **All tests pass?** `make test` exits 0; integration sweep passes.
3. **`make vet` clean?**
4. **CI green on macOS + Windows?** (Tag run, not just main.)
5. **Each new subcommand works end-to-end on at least one fixture?**
6. **`./bin/wsi-tools version`** prints `wsi-tools 0.4.0` (tagged build) or `0.5.0-dev` (post-release main).
7. **CHANGELOG accurate?** v0.4.0 section lists all four utilities + cliout + ifdwalk + roadmap.
8. **`docs/roadmap.md` lives at `docs/roadmap.md` (top-level), not under `docs/superpowers/`.**
