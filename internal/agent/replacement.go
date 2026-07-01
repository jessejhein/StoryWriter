package agent

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizeGeneratedReplacement validates provider text and normalizes line endings.
func NormalizeGeneratedReplacement(replacement string) (string, error) {
	if !utf8.ValidString(replacement) {
		return "", fmt.Errorf("replacement is invalid UTF-8: %w", ErrProviderInvalid)
	}
	replacement = strings.ReplaceAll(replacement, "\r\n", "\n")
	replacement = strings.ReplaceAll(replacement, "\r", "\n")
	if len(replacement) > 5<<20 {
		return "", fmt.Errorf("replacement exceeds 5 MiB: %w", ErrProviderRejected)
	}
	if strings.TrimFunc(replacement, unicode.IsSpace) == "" {
		return "", fmt.Errorf("replacement is empty: %w", ErrProviderRejected)
	}
	if strings.ContainsRune(replacement, '\x00') {
		return "", fmt.Errorf("replacement contains NUL: %w", ErrProviderRejected)
	}
	return replacement, nil
}
