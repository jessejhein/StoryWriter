package branch

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// RandomIDGenerator creates production experiment IDs.
type RandomIDGenerator struct{}

// NewRandomIDGenerator creates the production experiment ID generator.
func NewRandomIDGenerator() *RandomIDGenerator {
	return &RandomIDGenerator{}
}

// NextExperimentID returns brn_ plus 20 lowercase hex characters.
func (g *RandomIDGenerator) NextExperimentID() (ExperimentID, error) {
	var raw [10]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate experiment id: %w", err)
	}
	return ValidateExperimentID(experimentIDPrefix + hex.EncodeToString(raw[:]))
}
