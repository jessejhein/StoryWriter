package action

// findings.go parses strict Chapter Review provider output.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"storywork/internal/agent"
)

var (
	ErrInvalidFindings = errors.New("invalid chapter review findings")
	findingsSceneID    = regexp.MustCompile(`^scn_[0-9a-f]{20}$`)
)

const (
	maxFindingsCount    = 20
	maxFindingTitle     = 200
	maxFindingExplBytes = 4000
)

// Finding is one validated editorial suggestion.
type Finding struct {
	Title            string   `json:"title"`
	Explanation      string   `json:"explanation"`
	SceneIDs         []string `json:"scene_ids"`
	FollowUpAgentIDs []string `json:"follow_up_agent_ids"`
}

// FindingsResponse is the validated provider output envelope.
type FindingsResponse struct {
	Findings []Finding `json:"findings"`
}

// ParseFindings validates one strict Chapter Review JSON response.
func ParseFindings(raw string, allowedFollowUps map[string]struct{}, allowedSceneIDs map[string]struct{}) (FindingsResponse, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		return FindingsResponse{}, fmt.Errorf("fenced findings response: %w", ErrInvalidFindings)
	}
	var envelope struct {
		Findings *[]Finding `json:"findings"`
	}
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	if err := decoder.Decode(&envelope); err != nil {
		return FindingsResponse{}, fmt.Errorf("findings JSON: %w: %w", err, ErrInvalidFindings)
	}
	if envelope.Findings == nil {
		return FindingsResponse{}, fmt.Errorf("findings must not be null: %w", ErrInvalidFindings)
	}
	response := FindingsResponse{Findings: *envelope.Findings}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return FindingsResponse{}, fmt.Errorf("trailing JSON: %w", ErrInvalidFindings)
		}
		return FindingsResponse{}, fmt.Errorf("findings JSON: %w: %w", err, ErrInvalidFindings)
	}
	if len(response.Findings) > maxFindingsCount {
		return FindingsResponse{}, fmt.Errorf("too many findings: %w", ErrInvalidFindings)
	}
	seenTitles := map[string]struct{}{}
	for _, finding := range response.Findings {
		if err := validateFinding(finding, allowedFollowUps, allowedSceneIDs, seenTitles); err != nil {
			return FindingsResponse{}, err
		}
	}
	return response, nil
}

func validateFinding(finding Finding, allowedFollowUps map[string]struct{}, allowedSceneIDs map[string]struct{}, seenTitles map[string]struct{}) error {
	if finding.Title == "" || utf8.RuneCountInString(finding.Title) > maxFindingTitle {
		return fmt.Errorf("invalid finding title: %w", ErrInvalidFindings)
	}
	if finding.Explanation == "" || len([]byte(finding.Explanation)) > maxFindingExplBytes {
		return fmt.Errorf("invalid finding explanation: %w", ErrInvalidFindings)
	}
	if len(finding.SceneIDs) < 1 || len(finding.SceneIDs) > 20 {
		return fmt.Errorf("invalid finding scene_ids length: %w", ErrInvalidFindings)
	}
	seenScenes := map[string]struct{}{}
	for _, sceneID := range finding.SceneIDs {
		if !findingsSceneID.MatchString(sceneID) {
			return fmt.Errorf("invalid scene id %q: %w", sceneID, ErrInvalidFindings)
		}
		if _, ok := allowedSceneIDs[sceneID]; !ok {
			return fmt.Errorf("unknown scene id %q: %w", sceneID, ErrInvalidFindings)
		}
		if _, ok := seenScenes[sceneID]; ok {
			return fmt.Errorf("duplicate scene id %q: %w", sceneID, ErrInvalidFindings)
		}
		seenScenes[sceneID] = struct{}{}
	}
	for _, followUp := range finding.FollowUpAgentIDs {
		if _, ok := allowedFollowUps[followUp]; !ok {
			return fmt.Errorf("unknown follow-up %q: %w", followUp, ErrInvalidFindings)
		}
	}
	if _, ok := seenTitles[finding.Title]; ok {
		return fmt.Errorf("duplicate finding title: %w", ErrInvalidFindings)
	}
	seenTitles[finding.Title] = struct{}{}
	return nil
}

func allowedFollowUpsFromAgent(agentDefinition agent.Agent) map[string]struct{} {
	allowed := map[string]struct{}{}
	for _, rule := range agentDefinition.FollowUps.OnAccept {
		allowed[rule.AgentID] = struct{}{}
	}
	return allowed
}
