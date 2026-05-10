// Package wsiwriter writes WSI files in TIFF / BigTIFF / SVS shapes.
package wsiwriter

// TIFF Compression tag (TIFF tag 259) values used by wsitools. Standard values
// have ISO / Adobe / community allocations. Private values (≥ 32768) are
// wsitools-assigned for codecs without a recognized TIFF tag.
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

	// wsitools-private values (≥ 32768). Documented in docs/compression-tags.md.
	// These will only be readable by wsitools-aware viewers.
	CompressionAVIF   uint16 = 60001
	CompressionHEIF   uint16 = 60002
	CompressionHTJ2K  uint16 = 60003
	CompressionJPEGXR uint16 = 60004
	CompressionBasisU uint16 = 60005
)
