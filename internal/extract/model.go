package extract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"storywork/internal/modelchat"
	"storywork/internal/provider"
)

type Mode string

const (
	ModeStructure Mode = "structure"
)

var (
	ErrInvalidRequest  = errors.New("invalid extraction request")
	ErrInvalidResponse = errors.New("invalid extraction response")
)

var (
	generatedLocalIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)
	chunkIDPattern          = regexp.MustCompile(`^chk_[0-9a-f]{20}$`)
	importIDPattern         = regexp.MustCompile(`^imp_[0-9a-f]{20}$`)
)

type Chunk struct {
	ID         string
	ImportID   string
	SourcePath string
	StartLine  int
	EndLine    int
	Text       string
}

type CodexProposal struct {
	Kind        string   `json:"kind"`
	LocalID     string   `json:"local_id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Aliases     []string `json:"aliases"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
}

type ArcProposal struct {
	Kind    string `json:"kind"`
	LocalID string `json:"local_id"`
	Title   string `json:"title"`
}

type ChapterProposal struct {
	Kind          string `json:"kind"`
	LocalID       string `json:"local_id"`
	Title         string `json:"title"`
	ParentLocalID string `json:"parent_local_id"`
}

type SceneProposal struct {
	Kind          string `json:"kind"`
	LocalID       string `json:"local_id"`
	Title         string `json:"title"`
	ParentLocalID string `json:"parent_local_id"`
}

type Proposal struct {
	Kind    string
	Codex   *CodexProposal
	Arc     *ArcProposal
	Chapter *ChapterProposal
	Scene   *SceneProposal
}

type Request struct {
	Chunks    []Chunk
	Mode      Mode
	ProfileID string
	Model     string
}

type Result struct {
	Proposals []Proposal
	Provider  modelchat.ProviderIdentity
}

type Extractor interface {
	Extract(context.Context, Request) (Result, error)
}

type modeHandler interface {
	BuildPrompts(Request) (string, string)
	ParseResponse([]byte) ([]Proposal, error)
}

type structureModeHandler struct{}

var modeHandlers = map[Mode]modeHandler{
	ModeStructure: structureModeHandler{},
}

type profileResolver interface {
	Resolve(ctx context.Context, profileID string) (provider.ResolvedProfile, bool, error)
}

type RemoteExtractor struct {
	resolver profileResolver
	client   *http.Client
}

func NewRemoteExtractor(resolver profileResolver, client *http.Client) *RemoteExtractor {
	return &RemoteExtractor{resolver: resolver, client: modelchat.PrepareHTTPClient(client)}
}

func (e *RemoteExtractor) Extract(ctx context.Context, request Request) (Result, error) {
	if err := ValidateRequest(request); err != nil {
		return Result{}, err
	}
	if e.resolver == nil {
		return Result{}, modelchat.ErrProviderInvalid
	}
	resolved, found, err := e.resolver.Resolve(ctx, request.ProfileID)
	if err != nil {
		return Result{}, err
	}
	if !found || resolved.Readiness != provider.ReadinessReady || !resolved.Profile.Capabilities.Chat {
		return Result{}, modelchat.ErrProviderInvalid
	}
	handler := modeHandlers[request.Mode]
	systemPrompt, userPrompt := handler.BuildPrompts(request)
	chatResponse, err := modelchat.Complete(ctx, e.client, modelchat.Request{
		Profile: resolved,
		Model:   request.Model,
		Messages: []modelchat.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err != nil {
		return Result{}, err
	}
	proposals, err := handler.ParseResponse([]byte(chatResponse.Content))
	if err != nil {
		return Result{}, err
	}
	return Result{Proposals: proposals, Provider: chatResponse.Provider}, nil
}

func ValidateRequest(request Request) error {
	if _, supported := modeHandlers[request.Mode]; !supported {
		return fmt.Errorf("mode %q is unsupported: %w", request.Mode, ErrInvalidRequest)
	}
	if strings.TrimSpace(request.ProfileID) == "" || strings.TrimSpace(request.Model) == "" {
		return fmt.Errorf("profile and model are required: %w", ErrInvalidRequest)
	}
	if len(request.Chunks) == 0 || len(request.Chunks) > 50 {
		return fmt.Errorf("chunk count %d is invalid: %w", len(request.Chunks), ErrInvalidRequest)
	}
	totalBytes := 0
	importID := ""
	seenChunkIDs := make(map[string]struct{}, len(request.Chunks))
	for _, chunk := range request.Chunks {
		if !chunkIDPattern.MatchString(chunk.ID) || !importIDPattern.MatchString(chunk.ImportID) ||
			strings.TrimSpace(chunk.SourcePath) == "" || chunk.StartLine < 1 ||
			chunk.EndLine < chunk.StartLine || chunk.Text == "" || !utf8.ValidString(chunk.Text) {
			return fmt.Errorf("chunk metadata is invalid: %w", ErrInvalidRequest)
		}
		if _, exists := seenChunkIDs[chunk.ID]; exists {
			return fmt.Errorf("chunk %q is repeated: %w", chunk.ID, ErrInvalidRequest)
		}
		seenChunkIDs[chunk.ID] = struct{}{}
		if importID == "" {
			importID = chunk.ImportID
		} else if chunk.ImportID != importID {
			return fmt.Errorf("chunks belong to multiple imports: %w", ErrInvalidRequest)
		}
		totalBytes += len([]byte(chunk.Text))
	}
	if totalBytes > 200<<10 {
		return fmt.Errorf("chunk payload exceeds 200 KiB: %w", ErrInvalidRequest)
	}
	return nil
}

func BuildPrompts(request Request) (string, string) {
	return structureModeHandler{}.BuildPrompts(request)
}

func (structureModeHandler) BuildPrompts(request Request) (string, string) {
	var builder strings.Builder
	builder.WriteString("Return exactly one JSON object with shape {\"candidates\":[...]}. ")
	builder.WriteString("Do not include markdown fences, commentary, or trailing text. ")
	builder.WriteString("Each candidate must use one of these kinds: codex, arc, chapter, scene. ")
	builder.WriteString("Chapter parent_local_id must refer to an arc candidate local_id. ")
	builder.WriteString("Scene parent_local_id must refer to a chapter candidate local_id.\n\n")
	builder.WriteString("Exact candidate schemas: ")
	builder.WriteString(`{"kind":"codex","local_id":"...","type":"character|location|lore|custom","name":"...","aliases":[],"tags":[],"description":"..."}; `)
	builder.WriteString(`{"kind":"arc","local_id":"...","title":"..."}; `)
	builder.WriteString(`{"kind":"chapter","local_id":"...","title":"...","parent_local_id":"..."}; `)
	builder.WriteString(`{"kind":"scene","local_id":"...","title":"...","parent_local_id":"..."}.`)
	builder.WriteString(" Do not add fields.\n\n")
	builder.WriteString("Chunk inputs:\n")
	for _, chunk := range request.Chunks {
		builder.WriteString("- ")
		builder.WriteString(chunk.ID)
		builder.WriteString(" ")
		builder.WriteString(chunk.SourcePath)
		builder.WriteString(fmt.Sprintf(" lines %d-%d\n", chunk.StartLine, chunk.EndLine))
		builder.WriteString(chunk.Text)
		if !strings.HasSuffix(chunk.Text, "\n") {
			builder.WriteByte('\n')
		}
	}
	return "Extract structured story candidates from imported markdown notes.", builder.String()
}

func ParseResponse(body []byte) ([]Proposal, error) {
	return structureModeHandler{}.ParseResponse(body)
}

func (structureModeHandler) ParseResponse(body []byte) ([]Proposal, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || len(trimmed) > 1<<20 {
		return nil, ErrInvalidResponse
	}
	if bytes.HasPrefix(trimmed, []byte("```")) {
		return nil, ErrInvalidResponse
	}
	var envelope struct {
		Candidates []json.RawMessage `json:"candidates"`
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode response envelope: %w", ErrInvalidResponse)
	}
	if err := decoder.Decode(new(any)); err == nil || !errors.Is(err, io.EOF) {
		return nil, ErrInvalidResponse
	}
	if len(envelope.Candidates) == 0 || len(envelope.Candidates) > 200 {
		return nil, ErrInvalidResponse
	}
	proposals := make([]Proposal, 0, len(envelope.Candidates))
	seenLocalIDs := map[string]struct{}{}
	for _, raw := range envelope.Candidates {
		var header struct {
			Kind string `json:"kind"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			return nil, ErrInvalidResponse
		}
		proposal, err := parseProposal(header.Kind, raw)
		if err != nil {
			return nil, err
		}
		localID := proposalLocalID(proposal)
		if localID == "" {
			return nil, ErrInvalidResponse
		}
		if _, ok := seenLocalIDs[localID]; ok {
			return nil, ErrInvalidResponse
		}
		seenLocalIDs[localID] = struct{}{}
		proposals = append(proposals, proposal)
	}
	if err := validateProposals(proposals); err != nil {
		return nil, err
	}
	return proposals, nil
}

func validateProposals(proposals []Proposal) error {
	kinds := make(map[string]string, len(proposals))
	for _, proposal := range proposals {
		localID := proposalLocalID(proposal)
		if !generatedLocalIDPattern.MatchString(localID) {
			return ErrInvalidResponse
		}
		kinds[localID] = proposal.Kind
		switch proposal.Kind {
		case "codex":
			value := proposal.Codex
			if value.Kind != "codex" || strings.TrimSpace(value.Name) == "" || strings.TrimSpace(value.Type) == "" || strings.TrimSpace(value.Description) == "" {
				return ErrInvalidResponse
			}
		case "arc":
			if proposal.Arc.Kind != "arc" || strings.TrimSpace(proposal.Arc.Title) == "" {
				return ErrInvalidResponse
			}
		case "chapter":
			if proposal.Chapter.Kind != "chapter" || strings.TrimSpace(proposal.Chapter.Title) == "" || proposal.Chapter.ParentLocalID == localID {
				return ErrInvalidResponse
			}
		case "scene":
			if proposal.Scene.Kind != "scene" || strings.TrimSpace(proposal.Scene.Title) == "" || proposal.Scene.ParentLocalID == localID {
				return ErrInvalidResponse
			}
		}
	}
	for _, proposal := range proposals {
		switch proposal.Kind {
		case "chapter":
			if kinds[proposal.Chapter.ParentLocalID] != "arc" {
				return ErrInvalidResponse
			}
		case "scene":
			if kinds[proposal.Scene.ParentLocalID] != "chapter" {
				return ErrInvalidResponse
			}
		}
	}
	return nil
}

func parseProposal(kind string, raw json.RawMessage) (Proposal, error) {
	switch kind {
	case "codex":
		var proposal CodexProposal
		if err := strictDecode(raw, &proposal); err != nil {
			return Proposal{}, ErrInvalidResponse
		}
		return Proposal{Kind: kind, Codex: &proposal}, nil
	case "arc":
		var proposal ArcProposal
		if err := strictDecode(raw, &proposal); err != nil {
			return Proposal{}, ErrInvalidResponse
		}
		return Proposal{Kind: kind, Arc: &proposal}, nil
	case "chapter":
		var proposal ChapterProposal
		if err := strictDecode(raw, &proposal); err != nil {
			return Proposal{}, ErrInvalidResponse
		}
		return Proposal{Kind: kind, Chapter: &proposal}, nil
	case "scene":
		var proposal SceneProposal
		if err := strictDecode(raw, &proposal); err != nil {
			return Proposal{}, ErrInvalidResponse
		}
		return Proposal{Kind: kind, Scene: &proposal}, nil
	default:
		return Proposal{}, ErrInvalidResponse
	}
}

func strictDecode(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func proposalLocalID(proposal Proposal) string {
	switch proposal.Kind {
	case "codex":
		return proposal.Codex.LocalID
	case "arc":
		return proposal.Arc.LocalID
	case "chapter":
		return proposal.Chapter.LocalID
	case "scene":
		return proposal.Scene.LocalID
	default:
		return ""
	}
}
