package gitstore

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalidCommitMessage = errors.New("invalid commit message")
	runIDPattern            = regexp.MustCompile(`^run_[0-9a-f]{20}$`)
	sceneIDPattern          = regexp.MustCompile(`^scn_[0-9a-f]{20}$`)
	chapterIDPattern        = regexp.MustCompile(`^ch_[0-9a-f]{20}$`)
)

// CommitMessage is validated Git commit text with optional Storywork trailers.
type CommitMessage struct {
	Subject     string
	OperationID string
	TriggeredBy string
	DependsOn   string
	Scope       string
}

// FormatCommitMessage renders a subject and optional trailers in deterministic order.
func FormatCommitMessage(message CommitMessage) (string, error) {
	subject := strings.TrimSpace(message.Subject)
	if subject == "" || strings.Contains(subject, "\n") || strings.Contains(subject, "\r") {
		return "", fmt.Errorf("subject is invalid: %w", ErrInvalidCommitMessage)
	}
	if message.OperationID == "" && message.TriggeredBy == "" && message.DependsOn == "" && message.Scope == "" {
		return subject, nil
	}
	if err := validateRunID("operation id", message.OperationID); err != nil {
		return "", err
	}
	if err := validateScope(message.Scope); err != nil {
		return "", err
	}
	if message.DependsOn != "" && message.TriggeredBy == "" {
		return "", fmt.Errorf("depends_on requires triggered_by: %w", ErrInvalidCommitMessage)
	}
	if message.TriggeredBy != "" {
		if err := validateRunID("triggered_by", message.TriggeredBy); err != nil {
			return "", err
		}
		if message.TriggeredBy == message.OperationID {
			return "", fmt.Errorf("triggered_by cannot equal operation id: %w", ErrInvalidCommitMessage)
		}
	}
	if message.DependsOn != "" {
		if err := validateRunID("depends_on", message.DependsOn); err != nil {
			return "", err
		}
		if message.DependsOn == message.OperationID {
			return "", fmt.Errorf("depends_on cannot equal operation id: %w", ErrInvalidCommitMessage)
		}
	}

	lines := []string{subject, ""}
	lines = append(lines,
		"Storywork-Operation-ID: "+message.OperationID,
	)
	if message.TriggeredBy != "" {
		lines = append(lines, "Storywork-Triggered-By: "+message.TriggeredBy)
	}
	if message.DependsOn != "" {
		lines = append(lines, "Storywork-Depends-On: "+message.DependsOn)
	}
	lines = append(lines, "Storywork-Scope: "+message.Scope)
	return strings.Join(lines, "\n") + "\n", nil
}

func validateRunID(field, value string) error {
	if !runIDPattern.MatchString(value) {
		return fmt.Errorf("%s %q is invalid: %w", field, value, ErrInvalidCommitMessage)
	}
	return nil
}

func validateScope(scope string) error {
	parts := strings.SplitN(scope, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("scope %q is invalid: %w", scope, ErrInvalidCommitMessage)
	}
	switch parts[0] {
	case "selection", "scene":
		if !sceneIDPattern.MatchString(parts[1]) {
			return fmt.Errorf("scope %q is invalid: %w", scope, ErrInvalidCommitMessage)
		}
	case "chapter_review":
		if !chapterIDPattern.MatchString(parts[1]) {
			return fmt.Errorf("scope %q is invalid: %w", scope, ErrInvalidCommitMessage)
		}
	default:
		return fmt.Errorf("scope %q is invalid: %w", scope, ErrInvalidCommitMessage)
	}
	return nil
}