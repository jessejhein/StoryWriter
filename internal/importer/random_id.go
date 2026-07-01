package importer

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type RandomIDGenerator struct{}

func NewRandomIDGenerator() *RandomIDGenerator {
	return &RandomIDGenerator{}
}

func (g *RandomIDGenerator) NextImportID() (string, error) {
	return nextPrefixedID("imp_")
}

func (g *RandomIDGenerator) NextCandidateID() (string, error) {
	return nextPrefixedID("cand_")
}

func nextPrefixedID(prefix string) (string, error) {
	var raw [10]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate %s identifier: %w", prefix, err)
	}
	return prefix + hex.EncodeToString(raw[:]), nil
}
