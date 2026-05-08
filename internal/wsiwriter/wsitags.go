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

// WSIImageType canonical values. Lowercase to match opentile-go's existing
// AssociatedImage.Kind() vocabulary.
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
// WSIImageType values. Use at the boundary where caller-supplied kind
// strings flow into LevelSpec.WSIImageType / AssociatedSpec.WSIImageType.
func ValidateWSIImageType(v string) error {
	if !validWSIImageTypes[v] {
		return fmt.Errorf("wsi: invalid WSIImageType %q (want one of pyramid|label|macro|overview|thumbnail|probability|map|associated)", v)
	}
	return nil
}
