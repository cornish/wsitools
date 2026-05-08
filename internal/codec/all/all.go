// Package all exists solely to import every codec subpackage so they register
// themselves with the codec registry on import. Application binaries
// (cmd/wsi-tools) blank-import this package once.
package all

import (
	_ "github.com/cornish/wsi-tools/internal/codec/avif"
	_ "github.com/cornish/wsi-tools/internal/codec/htj2k"
	_ "github.com/cornish/wsi-tools/internal/codec/jpeg"
	_ "github.com/cornish/wsi-tools/internal/codec/jpegxl"
	_ "github.com/cornish/wsi-tools/internal/codec/webp"
)
