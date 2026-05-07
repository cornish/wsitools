// Package wsiwriter writes WSI files in TIFF / BigTIFF / SVS shapes.
package wsiwriter

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

// Writer writes a TIFF file. Construct via Create.
type Writer struct {
	path    string
	tmpPath string
	f       *os.File
	bo      binary.ByteOrder
	bigtiff bool
	imgs    []*imageEntry
	closed  bool
}

// Option is a functional option for Create.
type Option func(*writerConfig)

type writerConfig struct {
	bo      binary.ByteOrder
	bigtiff bool
}

// WithByteOrder sets the byte order for the TIFF file (default: LittleEndian).
func WithByteOrder(bo binary.ByteOrder) Option { return func(c *writerConfig) { c.bo = bo } }

// WithBigTIFF enables BigTIFF mode (8-byte offsets, 0x002B magic).
func WithBigTIFF(b bool) Option { return func(c *writerConfig) { c.bigtiff = b } }

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
	tiffTypeSHORT     = uint16(3)
	tiffTypeLONG      = uint16(4)
	tiffTypeUNDEFINED = uint16(7) // TIFF type 7: arbitrary byte data (JPEGTables, ICCProfile)
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
	w := &Writer{path: path, tmpPath: tmp, f: f, bo: cfg.bo, bigtiff: cfg.bigtiff}
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
// and renames it to the final path. Tmp removal on error is Task 7's job.
func (w *Writer) Close() error {
	if w.closed {
		return fmt.Errorf("wsiwriter: already closed")
	}
	w.closed = true

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
			if err := w.buildTiledTags(entry); err != nil {
				w.f.Close()
				return fmt.Errorf("wsiwriter: build tiled tags for IFD %d: %w", i, err)
			}
		}

		// 1. Write out-of-band tag values (BitsPerSample etc.).
		if err := w.writeOutOfBandValues(entry); err != nil {
			w.f.Close()
			return fmt.Errorf("wsiwriter: out-of-band values for IFD %d: %w", i, err)
		}

		// 2. Record IFD start position.
		start, err := w.currentOffset()
		if err != nil {
			w.f.Close()
			return fmt.Errorf("wsiwriter: seek before IFD %d: %w", i, err)
		}
		ifdStarts[i] = start

		// 3. Back-patch previous IFD's next-IFD field.
		if i > 0 {
			if err := w.patchUint32(nextIFDPatchAt[i-1], uint32(start)); err != nil {
				w.f.Close()
				return fmt.Errorf("wsiwriter: patch next-IFD for IFD %d: %w", i-1, err)
			}
		}

		// 4. Emit this IFD; record where its next-IFD field is for future patching.
		patchAt, err := w.emitIFD(entry)
		if err != nil {
			w.f.Close()
			return fmt.Errorf("wsiwriter: emit IFD %d: %w", i, err)
		}
		nextIFDPatchAt[i] = patchAt
	}

	// Back-patch the first-IFD offset in the file header (classic TIFF: bytes 4-7).
	var firstIFDOffset uint32
	if len(ifdStarts) > 0 {
		firstIFDOffset = uint32(ifdStarts[0])
	}
	if err := w.patchUint32(4, firstIFDOffset); err != nil {
		w.f.Close()
		return fmt.Errorf("wsiwriter: patch first-IFD offset: %w", err)
	}

	if err := w.f.Close(); err != nil {
		return fmt.Errorf("wsiwriter: close tmp: %w", err)
	}
	if err := os.Rename(w.tmpPath, w.path); err != nil {
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

// writeOutOfBandValues writes tag values that exceed the 4-byte inline limit
// to the current end of file, replacing the tag's value field with a 4-byte
// offset pointer. Called for each imageEntry before emitting its IFD.
func (w *Writer) writeOutOfBandValues(entry *imageEntry) error {
	const inlineLimit = 4

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
			// Replace value bytes with a 4-byte offset pointer.
			ptr := make([]byte, 4)
			w.bo.PutUint32(ptr, uint32(off))
			t.value = ptr
		}
	}
	return nil
}

// emitIFD writes one IFD to the file: entry count (2B), N×12-byte sorted entries,
// next-IFD offset (4B, written as 0 placeholder). Returns the file offset of the
// next-IFD field so the caller can back-patch it for IFD chaining.
func (w *Writer) emitIFD(entry *imageEntry) (nextIFDPatchOffset int64, _ error) {
	// Sort entries by tag number (TIFF spec requirement, §2).
	tags := make([]ifdTag, len(entry.tags))
	copy(tags, entry.tags)
	sort.Slice(tags, func(i, j int) bool { return tags[i].tag < tags[j].tag })

	// Write entry count (2 bytes).
	if err := binary.Write(w.f, w.bo, uint16(len(tags))); err != nil {
		return 0, fmt.Errorf("write IFD entry count: %w", err)
	}

	// Write each 12-byte entry: tag(2) + type(2) + count(4) + value/offset(4).
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

	// Record the file offset of the next-IFD field before writing it.
	patchOffset, err := w.currentOffset()
	if err != nil {
		return 0, err
	}
	// Write next-IFD offset placeholder (0 = last IFD; caller patches if chaining).
	if err := binary.Write(w.f, w.bo, uint32(0)); err != nil {
		return 0, fmt.Errorf("write next-IFD offset: %w", err)
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

// makeUndefinedTag creates an ifdTag for an opaque byte blob (TIFF type UNDEFINED).
// Values longer than 4 bytes will be written out-of-band by writeOutOfBandValues.
func (w *Writer) makeUndefinedTag(tag uint16, data []byte) ifdTag {
	b := make([]byte, len(data))
	copy(b, data)
	return ifdTag{tag: tag, typ: tiffTypeUNDEFINED, count: uint64(len(data)), value: b}
}

// buildTiledTags constructs the IFD tag list for a tiled imageEntry.
// Must be called after all WriteTile calls have populated entry.tileOffsets
// and entry.tileCounts.
func (w *Writer) buildTiledTags(entry *imageEntry) error {
	s := entry.spec

	// Convert uint64 slices to uint32 for classic TIFF LONG arrays.
	// (BigTIFF LONG8 is Task 6's concern.)
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

	bps := []uint16{8, 8, 8}
	spp := s.SamplesPerPixel
	if spp == 0 {
		spp = 3
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
		w.makeLongsTag(324, offsets),                     // TileOffsets
		w.makeLongsTag(325, counts),                      // TileByteCounts
		w.makeShortTag(284, 1),                           // PlanarConfiguration = chunky
	}

	// Optional: NewSubfileType (tag 254), only emit when non-zero.
	if s.NewSubfileType != 0 {
		tags = append(tags, w.makeLongTag(254, s.NewSubfileType))
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
