package action

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type RandomIDGenerator struct{}

func NewRandomIDGenerator() *RandomIDGenerator {
	return &RandomIDGenerator{}
}

func (g *RandomIDGenerator) Next() (string, error) {
	var raw [10]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate run ID: %w", err)
	}
	return "run_" + hex.EncodeToString(raw[:]), nil
}
