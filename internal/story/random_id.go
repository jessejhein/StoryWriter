package story

// random_id.go implements the production cryptographic ID generator.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// RandomIDGenerator creates opaque stable IDs using crypto/rand.
type RandomIDGenerator struct{}

// NewRandomIDGenerator creates the production ID generator.
func NewRandomIDGenerator() *RandomIDGenerator {
	return &RandomIDGenerator{}
}

// Next returns a new ID for the requested node kind.
func (g *RandomIDGenerator) Next(kind NodeKind) (string, error) {
	prefix := ""
	switch kind {
	case NodeKindArc:
		prefix = "arc_"
	case NodeKindChapter:
		prefix = "ch_"
	case NodeKindScene:
		prefix = "scn_"
	case NodeKindCharacter:
		prefix = "char_"
	case NodeKindLocation:
		prefix = "loc_"
	case NodeKindLore:
		prefix = "lore_"
	case NodeKindCustom:
		prefix = "custom_"
	case NodeKindProgression:
		prefix = "prog_"
	default:
		return "", fmt.Errorf("unknown node kind %q", kind)
	}

	var raw [10]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate %s ID: %w", kind, err)
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}
