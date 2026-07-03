package gitstore

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidPromotionMessage = errors.New("invalid promotion commit message")
	experimentIDPattern        = regexp.MustCompile(`^brn_[0-9a-f]{20}$`)
)

// PromotionMessage is validated provenance for one promotion commit.
type PromotionMessage struct {
	ExperimentID string
	SourceCommit string
	BaseCommit   string
}

// FormatPromotionMessage renders the exact promotion subject and trailers.
func FormatPromotionMessage(message PromotionMessage) (string, error) {
	if !experimentIDPattern.MatchString(message.ExperimentID) {
		return "", fmt.Errorf("experiment id %q: %w", message.ExperimentID, ErrInvalidPromotionMessage)
	}
	if err := validateCommitID(message.SourceCommit); err != nil {
		return "", fmt.Errorf("source commit: %w", ErrInvalidPromotionMessage)
	}
	if err := validateCommitID(message.BaseCommit); err != nil {
		return "", fmt.Errorf("base commit: %w", ErrInvalidPromotionMessage)
	}
	subject := "Promote what-if " + message.ExperimentID
	if strings.Contains(subject, "\n") || strings.Contains(subject, "\r") {
		return "", fmt.Errorf("subject is invalid: %w", ErrInvalidPromotionMessage)
	}
	lines := []string{
		subject,
		"",
		"Storywork-Experiment-ID: " + message.ExperimentID,
		"Storywork-Source-Commit: " + message.SourceCommit,
		"Storywork-Base-Commit: " + message.BaseCommit,
	}
	return strings.Join(lines, "\n") + "\n", nil
}
