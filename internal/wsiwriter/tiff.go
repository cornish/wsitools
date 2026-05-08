// Package wsiwriter writes WSI files in TIFF / BigTIFF / SVS shapes.
package wsiwriter

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"time"
)

// Writer writes a TIFF file. Construct via Create.
type Writer struct {
	path             string
	tmpPath          string
	f                *os.File
	bo               binary.ByteOrder
	bigtiff          bool
	wordSize         int // 4 for classic TIFF, 8 for BigTIFF
	imgs             []*imageEntry
	closed           bool
	imageDescription string // optional; emitted as tag 270 on L0 only

	// Standard TIFF metadata tags emitted on L0 when set.
	make_       string
	model       string
	software    string
	dateTime    time.Time
	hasDateTime bool

	// wsi-tools private metadata tags emitted on L0 when set.
	sourceFormat string // → TagWSISourceFormat
	toolsVersion string // → TagWSIToolsVersion

	// pyramidLevelCount is set in Close() after counting pyramid IFDs.
	pyramidLevelCount int
}

// Option is a functional option for Create.
type Option func(*writerConfig)

type writerConfig struct {
	bo               binary.ByteOrder
	bigtiff          bool
	imageDescription string

	// Standard TIFF metadata tags emitted on L0 when set.
	make_       string // trailing underscore avoids the Go keyword
	model       string
	software    string
	dateTime    time.Time
	hasDateTime bool

	// wsi-tools private metadata tags emitted on L0 when set.
	sourceFormat string // → TagWSISourceFormat
	toolsVersion string // → TagWSIToolsVersion
}

// WithByteOrder sets the byte order for the TIFF file (default: LittleEndian).
func WithByteOrder(bo binary.ByteOrder) Option { return func(c *writerConfig) { c.bo = bo } }

// WithBigTIFF enables BigTIFF mode (8-byte offsets, 0x002B magic).
func WithBigTIFF(b bool) Option { return func(c *writerConfig) { c.bigtiff = b } }

// WithImageDescription sets the TIFF ImageDescription tag (270) written on the
// first IFD (L0) only. Used by Aperio SVS to embed slide metadata.
func WithImageDescription(s string) Option {
	return func(c *writerConfig) { c.imageDescription = s }
}

// WithMake sets the TIFF Make tag (271) on the L0 IFD.
func WithMake(s string) Option { return func(c *writerConfig) { c.make_ = s } }

// WithModel sets the TIFF Model tag (272) on the L0 IFD.
func WithModel(s string) Option { return func(c *writerConfig) { c.model = s } }

// WithSoftware sets the TIFF Software tag (305) on the L0 IFD.
func WithSoftware(s string) Option { return func(c *writerConfig) { c.software = s } }

// WithDateTime sets the TIFF DateTime tag (306) on the L0 IFD, formatted as
// "YYYY:MM:DD HH:MM:SS" per TIFF 6.0.
func WithDateTime(t time.Time) Option {
	return func(c *writerConfig) { c.dateTime = t; c.hasDateTime = true }
}

// WithSourceFormat sets the wsi-tools private tag WSISourceFormat (65083) on
// the L0 IFD. The value should be the source format name (e.g. "svs",
// "philips-tiff", "ome-tiff").
func WithSourceFormat(s string) Option { return func(c *writerConfig) { c.sourceFormat = s } }

// WithToolsVersion sets the wsi-tools private tag WSIToolsVersion (65084) on
// the L0 IFD. The value should be the wsi-tools version string (e.g. "0.2.0").
func WithToolsVersion(s string) Option { return func(c *writerConfig) { c.toolsVersion = s } }

// imageEntry holds the data for one IFD to be written.
// If spec is non-nil the entry is tiled (AddLevel); if nil it is stripped (AddStrippedImage).
type imageEntry struct {
	tags         []ifdTag
	stripOffsets []uint32
	stripCounts  []uint32
	tileOffsets  []uint64  // populated by LevelHandle.WriteTile
	tileCounts   []uint64  // populated by LevelHandle.WriteTile
	spec         *LevelSpec // non-nil for tiled entries
}

// ifdTag represents one TIFF directory entry before encoding.
// value holds the raw bytes (in file byte order) for the tag value.
// For values larger than 4 bytes (classic TIFF inline limit), writeOutOfBandValues
// replaces value with a 4-byte offset pointer before IFD emission.
type ifdTag struct {
	tag   uint16
	typ   uint16 // TIFF type: 1=BYTE, 3=SHORT, 4=LONG, 16=LONG8 (BigTIFF only)
	count uint64 // number of values
	value []byte // raw bytes in file byte order; ≤4 bytes inline, or 4-byte pointer
}

// TIFF type constants.
const (
	tiffTypeASCII     = uint16(2)  // TIFF type 2: null-terminated ASCII string
	tiffTypeSHORT     = uint16(3)
	tiffTypeLONG      = uint16(4)
	tiffTypeUNDEFINED = uint16(7)  // TIFF type 7: arbitrary byte data (JPEGTables, ICCProfile)
	tiffTypeLONG8     = uint16(16) // BigTIFF-only: 8-byte unsigned integer (for tile/strip offsets)
)

// Create opens a new TIFF file for writing. The file is created at path+".tmp"
// and renamed to path on successful Close.
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
	wordSize := 4
	if cfg.bigtiff {
		wordSize = 8
	}
	w := &Writer{
		path:             path,
		tmpPath:          tmp,
		f:                f,
		bo:               cfg.bo,
		bigtiff:          cfg.bigtiff,
		wordSize:         wordSize,
		imageDescription: cfg.imageDescription,
		make_:            cfg.make_,
		model:            cfg.model,
		software:         cfg.software,
		dateTime:         cfg.dateTime,
		hasDateTime:      cfg.hasDateTime,
		sourceFormat:     cfg.sourceFormat,
		toolsVersion:     cfg.toolsVersion,
	}
	if err := w.writeHeader(); err != nil {
		f.Close()
		os.Remove(tmp)
		return nil, err
	}
	return w, nil
}

// StrippedSpec describes a single-strip image (uncompressed or already-encoded).
// Used for associated images and small thumbnail cases. For tiled pyramidal
// data, use AddLevel (Task 5).
type StrippedSpec struct {
	ImageWidth, ImageHeight   uint32
	Compression               uint16
	PhotometricInterpretation uint16
	StripBytes                []byte // raw pixel data (or pre-encoded if Compression != None)
}

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

	// SubfileType / NewSubfileType for pyramid level signalling. Aperio uses
	// NewSubfileType=0 for L0 and NewSubfileType=1 (reduced-resolution) for L1+.
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

// AssociatedSpec describes an associated image (label, macro, thumbnail, overview)
// to be stored as a stripped IFD. The caller provides already-encoded bytes.
type AssociatedSpec struct {
	Kind                      string // "label", "macro", "thumbnail", "overview" (informational only)
	Compressed                []byte // already-encoded bytes (or raw if Compression == CompressionNone)
	Width, Height             uint32
	Compression               uint16
	PhotometricInterpretation uint16
	NewSubfileType            uint32 // 1 for reduced-res, 9 for label, etc.
	ExtraTags                 []TIFFTag
}

// AddAssociated appends a stripped associated-image IFD to the TIFF file.
// The pixel/compressed bytes in s.Compressed are written immediately; the IFD
// is deferred until Close (consistent with AddLevel). Both CompressionNone and
// pre-encoded data (any other compression) are handled identically: bytes are
// written verbatim.
func (w *Writer) AddAssociated(s AssociatedSpec) error {
	if w.closed {
		return fmt.Errorf("wsiwriter: writer is closed")
	}

	// Write the strip data at the current file position.
	stripOffset, err := w.currentOffset()
	if err != nil {
		return fmt.Errorf("wsiwriter: get strip offset: %w", err)
	}
	if _, err := w.f.Write(s.Compressed); err != nil {
		return fmt.Errorf("wsiwriter: write associated strip data: %w", err)
	}
	stripCount := uint32(len(s.Compressed))

	entry := &imageEntry{
		stripOffsets: []uint32{uint32(stripOffset)},
		stripCounts:  []uint32{stripCount},
	}

	// BitsPerSample for RGB: [8,8,8].
	bps := []uint16{8, 8, 8}
	tags := []ifdTag{
		w.makeLongTag(256, s.Width),                       // ImageWidth
		w.makeLongTag(257, s.Height),                      // ImageLength
		w.makeShortsTag(258, bps),                         // BitsPerSample [8,8,8]
		w.makeShortTag(259, s.Compression),                // Compression
		w.makeShortTag(262, s.PhotometricInterpretation),  // PhotometricInterpretation
		w.makeLongTag(273, uint32(stripOffset)),            // StripOffsets
		w.makeShortTag(277, 3),                             // SamplesPerPixel
		w.makeLongTag(278, s.Height),                       // RowsPerStrip = Height (single strip)
		w.makeLongTag(279, stripCount),                     // StripByteCounts
		w.makeShortTag(284, 1),                             // PlanarConfiguration = chunky
	}

	// NewSubfileType (tag 254): emit when non-zero.
	if s.NewSubfileType != 0 {
		tags = append(tags, w.makeLongTag(254, s.NewSubfileType))
	}

	// Caller-supplied extra tags (emitted verbatim).
	for _, et := range s.ExtraTags {
		b := make([]byte, len(et.Value))
		copy(b, et.Value)
		tags = append(tags, ifdTag{
			tag:   et.Tag,
			typ:   et.Type,
			count: et.Count,
			value: b,
		})
	}

	entry.tags = tags
	w.imgs = append(w.imgs, entry)
	return nil
}

// LevelHandle accepts tile bytes for one level.
type LevelHandle struct {
	w      *Writer
	entry  *imageEntry
	tilesX uint32
	tilesY uint32
}

// AddLevel appends a tiled IFD to the TIFF file. Tile data is written by
// subsequent LevelHandle.WriteTile calls; the IFD is deferred until Close.
func (w *Writer) AddLevel(s LevelSpec) (*LevelHandle, error) {
	if w.closed {
		return nil, fmt.Errorf("wsiwriter: writer is closed")
	}
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
		spec:        &s,
	}
	w.imgs = append(w.imgs, entry)
	return &LevelHandle{w: w, entry: entry, tilesX: tilesX, tilesY: tilesY}, nil
}

// WriteTile writes compressed (or raw) tile bytes to the file and records the
// offset and byte count for later IFD emission. Tiles may be written in any order.
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

// AddStrippedImage appends a single-strip IFD to the TIFF file.
// Strip pixel data is written immediately; the IFD is deferred until Close.
func (w *Writer) AddStrippedImage(s StrippedSpec) error {
	if w.closed {
		return fmt.Errorf("wsiwriter: writer is closed")
	}

	// Write strip data at current file position.
	stripOffset, err := w.currentOffset()
	if err != nil {
		return fmt.Errorf("wsiwriter: get strip offset: %w", err)
	}
	if _, err := w.f.Write(s.StripBytes); err != nil {
		return fmt.Errorf("wsiwriter: write strip data: %w", err)
	}
	stripCount := uint32(len(s.StripBytes))

	entry := &imageEntry{
		stripOffsets: []uint32{uint32(stripOffset)},
		stripCounts:  []uint32{stripCount},
	}

	// Build IFD tags. Tags will be sorted by number in emitIFD.
	//
	// BitsPerSample for RGB: [8,8,8] = 3 SHORTs = 6 bytes — exceeds the 4-byte
	// classic-TIFF inline limit, so writeOutOfBandValues will write this
	// out-of-band and replace the value with a 4-byte file offset pointer.
	bps := []uint16{8, 8, 8}
	entry.tags = []ifdTag{
		w.makeLongTag(256, s.ImageWidth),                 // ImageWidth
		w.makeLongTag(257, s.ImageHeight),                // ImageLength
		w.makeShortsTag(258, bps),                        // BitsPerSample [8,8,8]
		w.makeShortTag(259, s.Compression),               // Compression
		w.makeShortTag(262, s.PhotometricInterpretation), // PhotometricInterpretation
		w.makeLongTag(273, uint32(stripOffset)),           // StripOffsets
		w.makeShortTag(277, 3),                            // SamplesPerPixel
		w.makeLongTag(278, s.ImageHeight),                 // RowsPerStrip = ImageHeight (single strip)
		w.makeLongTag(279, stripCount),                    // StripByteCounts
		w.makeShortTag(284, 1),                            // PlanarConfiguration = 1 (chunky/interleaved)
	}

	w.imgs = append(w.imgs, entry)
	return nil
}

// Close finalises the TIFF: emits all IFDs with correct next-IFD chaining,
// back-patches the first-IFD offset into the header, closes the temp file,
// and renames it to the final path. On error, the .tmp file is removed.
// Close is idempotent: a second call returns nil.
func (w *Writer) Close() (err error) {
	if w.closed {
		return nil // idempotent close
	}
	w.closed = true

	defer func() {
		if err != nil {
			os.Remove(w.tmpPath)
		}
	}()

	// Emit IFDs in order. Strategy:
	//   1. Write any out-of-band tag values (e.g. BitsPerSample [8,8,8]).
	//   2. Record the file position where this IFD starts.
	//   3. Emit the IFD; emitIFD returns the file offset of the next-IFD field.
	//   4. Back-patch the previous IFD's next-IFD field with the current IFD's start.
	//
	// ifdStarts[i] = file offset where IFD[i] begins.
	// nextIFDPatchAt[i] = file offset of IFD[i]'s next-IFD pointer field.
	ifdStarts := make([]int64, len(w.imgs))
	nextIFDPatchAt := make([]int64, len(w.imgs))

	for i, entry := range w.imgs {
		// For tiled entries, build the IFD tag list now (after all WriteTile calls
		// have populated tileOffsets/tileCounts).
		if entry.spec != nil {
			if err = w.buildTiledTags(entry, i == 0); err != nil {
				w.f.Close()
				return fmt.Errorf("wsiwriter: build tiled tags for IFD %d: %w", i, err)
			}
		}

		// 1. Write out-of-band tag values (BitsPerSample etc.).
		if err = w.writeOutOfBandValues(entry); err != nil {
			w.f.Close()
			return fmt.Errorf("wsiwriter: out-of-band values for IFD %d: %w", i, err)
		}

		// 2. Record IFD start position.
		var start int64
		if start, err = w.currentOffset(); err != nil {
			w.f.Close()
			return fmt.Errorf("wsiwriter: seek before IFD %d: %w", i, err)
		}
		ifdStarts[i] = start

		// 3. Back-patch previous IFD's next-IFD field.
		if i > 0 {
			if err = w.patchOffset(nextIFDPatchAt[i-1], uint64(start)); err != nil {
				w.f.Close()
				return fmt.Errorf("wsiwriter: patch next-IFD for IFD %d: %w", i-1, err)
			}
		}

		// 4. Emit this IFD; record where its next-IFD field is for future patching.
		var patchAt int64
		if patchAt, err = w.emitIFD(entry); err != nil {
			w.f.Close()
			return fmt.Errorf("wsiwriter: emit IFD %d: %w", i, err)
		}
		nextIFDPatchAt[i] = patchAt
	}

	// Back-patch the first-IFD offset in the file header.
	//   Classic TIFF: bytes 4-7 (4-byte uint32).
	//   BigTIFF:      bytes 8-15 (8-byte uint64, after the 8-byte BigTIFF preamble).
	var firstIFDOffset uint64
	if len(ifdStarts) > 0 {
		firstIFDOffset = uint64(ifdStarts[0])
	}
	firstIFDPatchAt := int64(4)
	if w.bigtiff {
		firstIFDPatchAt = 8
	}
	if err = w.patchOffset(firstIFDPatchAt, firstIFDOffset); err != nil {
		w.f.Close()
		return fmt.Errorf("wsiwriter: patch first-IFD offset: %w", err)
	}

	if err = w.f.Close(); err != nil {
		return fmt.Errorf("wsiwriter: close tmp: %w", err)
	}
	if err = os.Rename(w.tmpPath, w.path); err != nil {
		return fmt.Errorf("wsiwriter: rename tmp to final: %w", err)
	}
	return nil
}

// patchUint32 seeks to the given absolute file offset, writes a uint32 in w.bo,
// then seeks back to the end of the file.
func (w *Writer) patchUint32(at int64, val uint32) error {
	end, err := w.currentOffset()
	if err != nil {
		return err
	}
	if _, err := w.f.Seek(at, 0); err != nil {
		return fmt.Errorf("seek to patch offset %d: %w", at, err)
	}
	if err := binary.Write(w.f, w.bo, val); err != nil {
		return fmt.Errorf("write patch value at %d: %w", at, err)
	}
	if _, err := w.f.Seek(end, 0); err != nil {
		return fmt.Errorf("seek back to end after patch: %w", err)
	}
	return nil
}

// patchUint64 seeks to the given absolute file offset, writes a uint64 in w.bo,
// then seeks back to the end of the file.
func (w *Writer) patchUint64(at int64, val uint64) error {
	end, err := w.currentOffset()
	if err != nil {
		return err
	}
	if _, err := w.f.Seek(at, 0); err != nil {
		return fmt.Errorf("seek to patch offset %d: %w", at, err)
	}
	if err := binary.Write(w.f, w.bo, val); err != nil {
		return fmt.Errorf("write patch value at %d: %w", at, err)
	}
	if _, err := w.f.Seek(end, 0); err != nil {
		return fmt.Errorf("seek back to end after patch: %w", err)
	}
	return nil
}

// patchOffset writes a wordSize-wide value at the given file offset: u32 for
// classic TIFF, u64 for BigTIFF. Restores the write position afterward.
func (w *Writer) patchOffset(at int64, val uint64) error {
	if w.wordSize == 8 {
		return w.patchUint64(at, val)
	}
	return w.patchUint32(at, uint32(val))
}

// writeHeader writes the 8-byte classic TIFF header (or 16-byte BigTIFF header).
// The first-IFD offset is written as placeholder 0 and back-patched in Close.
func (w *Writer) writeHeader() error {
	var boMark [2]byte
	if w.bo == binary.LittleEndian {
		boMark = [2]byte{'I', 'I'}
	} else {
		boMark = [2]byte{'M', 'M'}
	}
	if _, err := w.f.Write(boMark[:]); err != nil {
		return fmt.Errorf("wsiwriter: write byte order mark: %w", err)
	}

	if w.bigtiff {
		// BigTIFF header (16 bytes total):
		//   magic (2B) = 0x002B
		//   offset bytesize (2B) = 8
		//   constant (2B) = 0
		//   first-IFD offset (8B) = placeholder 0
		if err := binary.Write(w.f, w.bo, uint16(0x002B)); err != nil {
			return fmt.Errorf("wsiwriter: write BigTIFF magic: %w", err)
		}
		if err := binary.Write(w.f, w.bo, uint16(8)); err != nil {
			return fmt.Errorf("wsiwriter: write BigTIFF offset bytesize: %w", err)
		}
		if err := binary.Write(w.f, w.bo, uint16(0)); err != nil {
			return fmt.Errorf("wsiwriter: write BigTIFF constant: %w", err)
		}
		if err := binary.Write(w.f, w.bo, uint64(0)); err != nil {
			return fmt.Errorf("wsiwriter: write BigTIFF IFD placeholder: %w", err)
		}
	} else {
		// Classic TIFF header (8 bytes total):
		//   magic (2B) = 42
		//   first-IFD offset (4B) = placeholder 0
		if err := binary.Write(w.f, w.bo, uint16(42)); err != nil {
			return fmt.Errorf("wsiwriter: write classic TIFF magic: %w", err)
		}
		if err := binary.Write(w.f, w.bo, uint32(0)); err != nil {
			return fmt.Errorf("wsiwriter: write first-IFD placeholder: %w", err)
		}
	}
	return nil
}

// writeOutOfBandValues writes tag values that exceed the inline limit
// to the current end of file, replacing the tag's value field with an offset
// pointer. The inline limit is w.wordSize bytes (4 for classic, 8 for BigTIFF).
// Called for each imageEntry before emitting its IFD.
func (w *Writer) writeOutOfBandValues(entry *imageEntry) error {
	inlineLimit := w.wordSize

	for i := range entry.tags {
		t := &entry.tags[i]
		if len(t.value) > inlineLimit {
			off, err := w.currentOffset()
			if err != nil {
				return err
			}
			if _, err := w.f.Write(t.value); err != nil {
				return fmt.Errorf("tag %d out-of-band write: %w", t.tag, err)
			}
			// Replace value bytes with a wordSize-byte offset pointer.
			ptr := make([]byte, w.wordSize)
			if w.wordSize == 8 {
				w.bo.PutUint64(ptr, uint64(off))
			} else {
				w.bo.PutUint32(ptr, uint32(off))
			}
			t.value = ptr
		}
	}
	return nil
}

// emitIFD writes one IFD to the file. For classic TIFF: entry count (2B),
// N×12-byte sorted entries, next-IFD offset (4B). For BigTIFF: entry count
// (8B), N×20-byte sorted entries, next-IFD offset (8B). Returns the file
// offset of the next-IFD field so the caller can back-patch it for IFD chaining.
func (w *Writer) emitIFD(entry *imageEntry) (nextIFDPatchOffset int64, _ error) {
	// Sort entries by tag number (TIFF spec requirement, §2).
	tags := make([]ifdTag, len(entry.tags))
	copy(tags, entry.tags)
	sort.Slice(tags, func(i, j int) bool { return tags[i].tag < tags[j].tag })

	if w.bigtiff {
		// BigTIFF entry count: 8 bytes (uint64).
		if err := binary.Write(w.f, w.bo, uint64(len(tags))); err != nil {
			return 0, fmt.Errorf("write IFD entry count: %w", err)
		}
		// BigTIFF entry: tag(2) + type(2) + count(8) + value/offset(8) = 20 bytes.
		for _, t := range tags {
			if err := binary.Write(w.f, w.bo, t.tag); err != nil {
				return 0, fmt.Errorf("write tag %d field: %w", t.tag, err)
			}
			if err := binary.Write(w.f, w.bo, t.typ); err != nil {
				return 0, fmt.Errorf("write tag %d type: %w", t.tag, err)
			}
			if err := binary.Write(w.f, w.bo, t.count); err != nil {
				return 0, fmt.Errorf("write tag %d count: %w", t.tag, err)
			}
			// Value/offset field (8 bytes). t.value is ≤8 bytes at this point
			// (out-of-band values have been resolved to 8-byte pointers).
			// Inline values are left-justified; copy fills from the start,
			// leaving trailing zeros — correct for both LE and BE.
			var padded [8]byte
			copy(padded[:], t.value)
			if _, err := w.f.Write(padded[:]); err != nil {
				return 0, fmt.Errorf("write tag %d value: %w", t.tag, err)
			}
		}
	} else {
		// Classic TIFF entry count: 2 bytes (uint16).
		if err := binary.Write(w.f, w.bo, uint16(len(tags))); err != nil {
			return 0, fmt.Errorf("write IFD entry count: %w", err)
		}
		// Classic entry: tag(2) + type(2) + count(4) + value/offset(4) = 12 bytes.
		for _, t := range tags {
			if err := binary.Write(w.f, w.bo, t.tag); err != nil {
				return 0, fmt.Errorf("write tag %d field: %w", t.tag, err)
			}
			if err := binary.Write(w.f, w.bo, t.typ); err != nil {
				return 0, fmt.Errorf("write tag %d type: %w", t.tag, err)
			}
			if err := binary.Write(w.f, w.bo, uint32(t.count)); err != nil {
				return 0, fmt.Errorf("write tag %d count: %w", t.tag, err)
			}
			// Value/offset field (4 bytes). t.value is ≤4 bytes at this point
			// (out-of-band values have been resolved to 4-byte pointers).
			// TIFF spec: inline values are left-justified in the 4-byte field
			// (i.e. stored at the lowest address). copy fills from the start,
			// leaving trailing zeros — correct for both LE and BE.
			var padded [4]byte
			copy(padded[:], t.value)
			if _, err := w.f.Write(padded[:]); err != nil {
				return 0, fmt.Errorf("write tag %d value: %w", t.tag, err)
			}
		}
	}

	// Record the file offset of the next-IFD field before writing it.
	patchOffset, err := w.currentOffset()
	if err != nil {
		return 0, err
	}
	// Write next-IFD offset placeholder (0 = last IFD; caller patches if chaining).
	// Classic: 4-byte uint32; BigTIFF: 8-byte uint64.
	if w.bigtiff {
		if err := binary.Write(w.f, w.bo, uint64(0)); err != nil {
			return 0, fmt.Errorf("write next-IFD offset: %w", err)
		}
	} else {
		if err := binary.Write(w.f, w.bo, uint32(0)); err != nil {
			return 0, fmt.Errorf("write next-IFD offset: %w", err)
		}
	}

	return patchOffset, nil
}

// currentOffset returns the current file write position (end of written data).
func (w *Writer) currentOffset() (int64, error) {
	return w.f.Seek(0, 1) // 1 = io.SeekCurrent
}

// --- Tag construction helpers (Writer methods so they have access to w.bo) ---

// makeShortTag creates an ifdTag for a single SHORT (uint16) value.
func (w *Writer) makeShortTag(tag uint16, val uint16) ifdTag {
	b := make([]byte, 2)
	w.bo.PutUint16(b, val)
	return ifdTag{tag: tag, typ: tiffTypeSHORT, count: 1, value: b}
}

// makeShortsTag creates an ifdTag for a slice of SHORT (uint16) values.
// If len(vals)*2 > 4, the value will be written out-of-band by writeOutOfBandValues.
func (w *Writer) makeShortsTag(tag uint16, vals []uint16) ifdTag {
	b := make([]byte, 2*len(vals))
	for i, v := range vals {
		w.bo.PutUint16(b[2*i:], v)
	}
	return ifdTag{tag: tag, typ: tiffTypeSHORT, count: uint64(len(vals)), value: b}
}

// makeLongTag creates an ifdTag for a single LONG (uint32) value.
func (w *Writer) makeLongTag(tag uint16, val uint32) ifdTag {
	b := make([]byte, 4)
	w.bo.PutUint32(b, val)
	return ifdTag{tag: tag, typ: tiffTypeLONG, count: 1, value: b}
}

// makeLongsTag creates an ifdTag for a slice of LONG (uint32) values.
// If len(vals)*4 > 4, the value will be written out-of-band by writeOutOfBandValues.
func (w *Writer) makeLongsTag(tag uint16, vals []uint32) ifdTag {
	b := make([]byte, 4*len(vals))
	for i, v := range vals {
		w.bo.PutUint32(b[4*i:], v)
	}
	return ifdTag{tag: tag, typ: tiffTypeLONG, count: uint64(len(vals)), value: b}
}

// makeLong8sTag creates an ifdTag for a slice of LONG8 (uint64) values.
// LONG8 (type code 16) is a BigTIFF-only type for 8-byte unsigned integers.
// Used for TileOffsets/TileByteCounts in BigTIFF mode so each value can
// address beyond 4 GiB. Values longer than 8 bytes will be written out-of-band
// by writeOutOfBandValues.
func (w *Writer) makeLong8sTag(tag uint16, vals []uint64) ifdTag {
	b := make([]byte, 8*len(vals))
	for i, v := range vals {
		w.bo.PutUint64(b[8*i:], v)
	}
	return ifdTag{tag: tag, typ: tiffTypeLONG8, count: uint64(len(vals)), value: b}
}

// makeUndefinedTag creates an ifdTag for an opaque byte blob (TIFF type UNDEFINED).
// Values longer than 4 bytes will be written out-of-band by writeOutOfBandValues.
func (w *Writer) makeUndefinedTag(tag uint16, data []byte) ifdTag {
	b := make([]byte, len(data))
	copy(b, data)
	return ifdTag{tag: tag, typ: tiffTypeUNDEFINED, count: uint64(len(data)), value: b}
}

// makeASCIITag creates an ifdTag for a null-terminated ASCII string (TIFF type 2).
// TIFF spec: count includes the null terminator; values longer than the inline
// limit will be written out-of-band by writeOutOfBandValues.
func (w *Writer) makeASCIITag(tag uint16, s string) ifdTag {
	b := []byte(s + "\x00")
	return ifdTag{tag: tag, typ: tiffTypeASCII, count: uint64(len(b)), value: b}
}

// formatTIFFDateTime formats t as TIFF 6.0's "YYYY:MM:DD HH:MM:SS" datetime.
func formatTIFFDateTime(t time.Time) string {
	return t.Format("2006:01:02 15:04:05")
}

// buildTiledTags constructs the IFD tag list for a tiled imageEntry.
// Must be called after all WriteTile calls have populated entry.tileOffsets
// and entry.tileCounts. isL0 indicates whether this is the first (L0) IFD,
// which may receive extra tags such as ImageDescription.
func (w *Writer) buildTiledTags(entry *imageEntry, isL0 bool) error {
	s := entry.spec

	bps := []uint16{8, 8, 8}
	spp := s.SamplesPerPixel
	if spp == 0 {
		spp = 3
	}

	// Build TileOffsets and TileByteCounts tags.
	// BigTIFF mode: use LONG8 (type 16, 8 bytes/value) so offsets can address >4 GiB.
	// Classic mode: use LONG (type 4, 4 bytes/value) with range validation.
	var offsetTag, countTag ifdTag
	if w.bigtiff {
		offsetTag = w.makeLong8sTag(324, entry.tileOffsets) // TileOffsets  (LONG8)
		countTag = w.makeLong8sTag(325, entry.tileCounts)   // TileByteCounts (LONG8)
	} else {
		offsets := make([]uint32, len(entry.tileOffsets))
		for i, v := range entry.tileOffsets {
			if v > 0xFFFFFFFF {
				return fmt.Errorf("wsiwriter: tile offset %d exceeds 4 GiB — use BigTIFF", v)
			}
			offsets[i] = uint32(v)
		}
		counts := make([]uint32, len(entry.tileCounts))
		for i, v := range entry.tileCounts {
			if v > 0xFFFFFFFF {
				return fmt.Errorf("wsiwriter: tile byte count %d exceeds 4 GiB — use BigTIFF", v)
			}
			counts[i] = uint32(v)
		}
		offsetTag = w.makeLongsTag(324, offsets) // TileOffsets  (LONG)
		countTag = w.makeLongsTag(325, counts)   // TileByteCounts (LONG)
	}

	tags := []ifdTag{
		w.makeLongTag(256, s.ImageWidth),                 // ImageWidth
		w.makeLongTag(257, s.ImageHeight),                // ImageLength
		w.makeShortsTag(258, bps),                        // BitsPerSample [8,8,8]
		w.makeShortTag(259, s.Compression),               // Compression
		w.makeShortTag(262, s.PhotometricInterpretation), // PhotometricInterpretation
		w.makeShortTag(277, spp),                         // SamplesPerPixel
		w.makeLongTag(322, s.TileWidth),                  // TileWidth
		w.makeLongTag(323, s.TileHeight),                 // TileLength
		offsetTag,                                        // TileOffsets
		countTag,                                         // TileByteCounts
		w.makeShortTag(284, 1),                           // PlanarConfiguration = chunky
	}

	// Optional: NewSubfileType (tag 254), only emit when non-zero.
	if s.NewSubfileType != 0 {
		tags = append(tags, w.makeLongTag(254, s.NewSubfileType))
	}

	// Optional: ImageDescription (tag 270, ASCII) — emitted on L0 only.
	if isL0 && w.imageDescription != "" {
		tags = append(tags, w.makeASCIITag(270, w.imageDescription))
	}

	// Optional standard metadata tags — emitted on L0 only.
	if isL0 {
		if w.make_ != "" {
			tags = append(tags, w.makeASCIITag(271, w.make_))   // Make
		}
		if w.model != "" {
			tags = append(tags, w.makeASCIITag(272, w.model))   // Model
		}
		if w.software != "" {
			tags = append(tags, w.makeASCIITag(305, w.software)) // Software
		}
		if w.hasDateTime {
			tags = append(tags, w.makeASCIITag(306, formatTIFFDateTime(w.dateTime))) // DateTime
		}
		if w.sourceFormat != "" {
			tags = append(tags, w.makeASCIITag(TagWSISourceFormat, w.sourceFormat))
		}
		if w.toolsVersion != "" {
			tags = append(tags, w.makeASCIITag(TagWSIToolsVersion, w.toolsVersion))
		}
	}

	// Optional: JPEGTables (tag 347, type UNDEFINED).
	if len(s.JPEGTables) > 0 {
		tags = append(tags, w.makeUndefinedTag(347, s.JPEGTables))
	}

	// Optional: ICCProfile (tag 34675, type UNDEFINED).
	if len(s.ICCProfile) > 0 {
		tags = append(tags, w.makeUndefinedTag(34675, s.ICCProfile))
	}

	// Caller-supplied extra tags (emitted verbatim).
	for _, et := range s.ExtraTags {
		b := make([]byte, len(et.Value))
		copy(b, et.Value)
		tags = append(tags, ifdTag{
			tag:   et.Tag,
			typ:   et.Type,
			count: et.Count,
			value: b,
		})
	}

	entry.tags = tags
	return nil
}
