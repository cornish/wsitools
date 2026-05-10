package source

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ReadSourceImageDescription reads TIFF tag 270 (ImageDescription, ASCII)
// from IFD 0 of a TIFF / BigTIFF file. opentile-go's Tiler does not expose the
// raw description verbatim; this helper is a minimal one-shot parser used by
// the downsample CLI (to mutate AppMag/MPP) and the transcode CLI (to either
// passthrough or build a wsitools provenance string).
//
// Returns ("", error) for non-TIFF inputs or files missing tag 270 — the
// caller decides whether to surface or silence the error. For non-TIFF
// sources (e.g., IFE), callers should silence the error and treat "" as a
// "no source description available" sentinel.
func ReadSourceImageDescription(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var hdr [16]byte
	if _, err := io.ReadFull(f, hdr[:8]); err != nil {
		return "", fmt.Errorf("read header: %w", err)
	}
	var bo binary.ByteOrder
	switch {
	case hdr[0] == 'I' && hdr[1] == 'I':
		bo = binary.LittleEndian
	case hdr[0] == 'M' && hdr[1] == 'M':
		bo = binary.BigEndian
	default:
		return "", fmt.Errorf("not a TIFF file (bad byte-order mark %x %x)", hdr[0], hdr[1])
	}
	magic := bo.Uint16(hdr[2:4])
	var ifdOffset int64
	bigtiff := false
	switch magic {
	case 42:
		ifdOffset = int64(bo.Uint32(hdr[4:8]))
	case 0x002B:
		bigtiff = true
		// BigTIFF header: bytes 4-5 = offset bytesize (always 8),
		// bytes 6-7 = constant 0, bytes 8-15 = first-IFD offset.
		if _, err := io.ReadFull(f, hdr[8:16]); err != nil {
			return "", fmt.Errorf("read BigTIFF header tail: %w", err)
		}
		offsetSize := bo.Uint16(hdr[4:6])
		if offsetSize != 8 {
			return "", fmt.Errorf("BigTIFF offset bytesize != 8: got %d", offsetSize)
		}
		ifdOffset = int64(bo.Uint64(hdr[8:16]))
	default:
		return "", fmt.Errorf("unknown TIFF magic 0x%x", magic)
	}

	if _, err := f.Seek(ifdOffset, 0); err != nil {
		return "", fmt.Errorf("seek IFD: %w", err)
	}

	var entryCount uint64
	if bigtiff {
		var nb [8]byte
		if _, err := io.ReadFull(f, nb[:]); err != nil {
			return "", fmt.Errorf("read entry count: %w", err)
		}
		entryCount = bo.Uint64(nb[:])
	} else {
		var nb [2]byte
		if _, err := io.ReadFull(f, nb[:]); err != nil {
			return "", fmt.Errorf("read entry count: %w", err)
		}
		entryCount = uint64(bo.Uint16(nb[:]))
	}

	entrySize := 12
	if bigtiff {
		entrySize = 20
	}
	entries := make([]byte, int(entryCount)*entrySize)
	if _, err := io.ReadFull(f, entries); err != nil {
		return "", fmt.Errorf("read IFD entries: %w", err)
	}

	for i := uint64(0); i < entryCount; i++ {
		e := entries[int(i)*entrySize : (int(i)+1)*entrySize]
		tag := bo.Uint16(e[0:2])
		typ := bo.Uint16(e[2:4])
		if tag != 270 || typ != 2 {
			continue
		}
		var count uint64
		var valueField []byte
		if bigtiff {
			count = bo.Uint64(e[4:12])
			valueField = e[12:20]
		} else {
			count = uint64(bo.Uint32(e[4:8]))
			valueField = e[8:12]
		}
		inlineLimit := 4
		if bigtiff {
			inlineLimit = 8
		}
		var raw []byte
		if int(count) <= inlineLimit {
			raw = valueField[:count]
		} else {
			var off int64
			if bigtiff {
				off = int64(bo.Uint64(valueField))
			} else {
				off = int64(bo.Uint32(valueField))
			}
			if _, err := f.Seek(off, 0); err != nil {
				return "", fmt.Errorf("seek ImageDescription value: %w", err)
			}
			raw = make([]byte, count)
			if _, err := io.ReadFull(f, raw); err != nil {
				return "", fmt.Errorf("read ImageDescription value: %w", err)
			}
		}
		// Strip trailing NULs (count includes the null terminator).
		for len(raw) > 0 && raw[len(raw)-1] == 0 {
			raw = raw[:len(raw)-1]
		}
		return string(raw), nil
	}
	return "", fmt.Errorf("ImageDescription (tag 270) not found in IFD 0")
}
