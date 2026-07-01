package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	Version               = 1
	maxNameRunes          = 100
	maxModelRunes         = 200
	maxContextTokensLimit = 10_000_000
)

var (
	ErrInvalidProfile          = errors.New("invalid provider profile")
	ErrProfileStore            = errors.New("provider profile store failure")
	ErrProfileRevisionConflict = errors.New("provider profile revision conflict")
	ErrNoProfileChanges        = errors.New("provider profile save has no changes")
)

var (
	profileIDPattern     = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	credentialEnvPattern = regexp.MustCompile(`^STORYWORK_[A-Z][A-Z0-9_]{0,127}$`)
	revisionPattern      = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type Type string

const (
	TypeOpenAICompatible Type = "openai_compatible"
	TypeOllama           Type = "ollama"
)

type AuthType string

const (
	AuthTypeNone      AuthType = "none"
	AuthTypeBearerEnv AuthType = "bearer_env"
)

type Readiness string

const (
	ReadinessReady             Readiness = "ready"
	ReadinessMissingCredential Readiness = "missing_credential"
)

type ProfileReadiness string

const (
	ProfileReadinessReady             ProfileReadiness = "ready"
	ProfileReadinessMissingProfile    ProfileReadiness = "missing_profile"
	ProfileReadinessMissingCredential ProfileReadiness = "missing_credential"
)

type Profile struct {
	ID           string       `yaml:"id" json:"id"`
	Name         string       `yaml:"name" json:"name"`
	Type         Type         `yaml:"type" json:"type"`
	BaseURL      string       `yaml:"base_url" json:"base_url"`
	Auth         AuthConfig   `yaml:"auth" json:"auth"`
	Capabilities Capabilities `yaml:"capabilities" json:"capabilities"`
	Readiness    Readiness    `yaml:"-" json:"readiness,omitempty"`
}

type AuthConfig struct {
	Type          AuthType `yaml:"type" json:"type"`
	CredentialEnv string   `yaml:"credential_env" json:"credential_env"`
}

type Capabilities struct {
	Chat             bool `yaml:"chat" json:"chat"`
	Streaming        bool `yaml:"streaming" json:"streaming"`
	StructuredOutput bool `yaml:"structured_output" json:"structured_output"`
	MaxContextTokens int  `yaml:"max_context_tokens" json:"max_context_tokens"`
}

type Credential struct {
	Value string
}

type Broker interface {
	Resolve(ctx context.Context, credentialEnv string) (Credential, Readiness, error)
}

type EnvironmentBroker struct {
	LookupEnv func(string) (string, bool)
}

func (b EnvironmentBroker) Resolve(ctx context.Context, credentialEnv string) (Credential, Readiness, error) {
	select {
	case <-ctx.Done():
		return Credential{}, ReadinessMissingCredential, ctx.Err()
	default:
	}
	lookup := b.LookupEnv
	if lookup == nil {
		lookup = func(string) (string, bool) { return "", false }
	}
	value, ok := lookup(credentialEnv)
	if !ok || value == "" {
		return Credential{}, ReadinessMissingCredential, nil
	}
	return Credential{Value: value}, ReadinessReady, nil
}

func ComputeRevision(contents []byte) string {
	digest := sha256.Sum256(contents)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func ResolveConfigDir(override string, userConfigDir func() (string, error)) (string, error) {
	override = strings.TrimSpace(override)
	if override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf("STORYWORK_CONFIG_DIR must be absolute: %w", ErrInvalidProfile)
		}
		return filepath.Clean(override), nil
	}
	base, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "storywork"), nil
}

func ValidateProfiles(profiles []Profile) ([]Profile, []byte, string, error) {
	normalized := make([]Profile, 0, len(profiles))
	seen := map[string]struct{}{}
	for _, profile := range profiles {
		item, err := validateProfile(profile)
		if err != nil {
			return nil, nil, "", err
		}
		if _, exists := seen[item.ID]; exists {
			return nil, nil, "", fmt.Errorf("duplicate provider profile id %q: %w", item.ID, ErrInvalidProfile)
		}
		seen[item.ID] = struct{}{}
		normalized = append(normalized, item)
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Name != normalized[j].Name {
			return normalized[i].Name < normalized[j].Name
		}
		return normalized[i].ID < normalized[j].ID
	})
	canonical := marshalCanonical(normalized)
	return normalized, canonical, ComputeRevision(canonical), nil
}

func ValidateRevision(revision string) error {
	if !revisionPattern.MatchString(revision) {
		return fmt.Errorf("revision %q is invalid: %w", revision, ErrInvalidProfile)
	}
	return nil
}

func validateProfile(profile Profile) (Profile, error) {
	profile.ID = strings.TrimSpace(profile.ID)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.BaseURL = strings.TrimSpace(profile.BaseURL)
	if !profileIDPattern.MatchString(profile.ID) {
		return Profile{}, fmt.Errorf("profile id %q is invalid: %w", profile.ID, ErrInvalidProfile)
	}
	if err := validateName(profile.Name); err != nil {
		return Profile{}, err
	}
	normalizedURL, parsedURL, err := normalizeBaseURL(profile.BaseURL)
	if err != nil {
		return Profile{}, err
	}
	profile.BaseURL = normalizedURL
	switch profile.Type {
	case TypeOpenAICompatible, TypeOllama:
	default:
		return Profile{}, fmt.Errorf("profile %q type %q is invalid: %w", profile.ID, profile.Type, ErrInvalidProfile)
	}
	if err := validateAuth(profile, parsedURL); err != nil {
		return Profile{}, err
	}
	if !profile.Capabilities.Chat && profile.Type == TypeOllama {
		// allowed by contract for compatibility reporting
	}
	if profile.Capabilities.MaxContextTokens < 1 || profile.Capabilities.MaxContextTokens > maxContextTokensLimit {
		return Profile{}, fmt.Errorf("profile %q max_context_tokens %d is invalid: %w", profile.ID, profile.Capabilities.MaxContextTokens, ErrInvalidProfile)
	}
	return profile, nil
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name is required: %w", ErrInvalidProfile)
	}
	if !utf8.ValidString(name) {
		return fmt.Errorf("profile name is invalid UTF-8: %w", ErrInvalidProfile)
	}
	if utf8.RuneCountInString(name) > maxNameRunes {
		return fmt.Errorf("profile name exceeds %d runes: %w", maxNameRunes, ErrInvalidProfile)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return fmt.Errorf("profile name contains control characters: %w", ErrInvalidProfile)
		}
	}
	return nil
}

func normalizeBaseURL(raw string) (string, *url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", nil, fmt.Errorf("base_url %q is invalid: %w", raw, ErrInvalidProfile)
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return "", nil, fmt.Errorf("base_url %q must be absolute with a host: %w", raw, ErrInvalidProfile)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", nil, fmt.Errorf("base_url %q scheme must be http or https: %w", raw, ErrInvalidProfile)
	}
	if parsed.User != nil {
		return "", nil, fmt.Errorf("base_url %q must not contain user info: %w", raw, ErrInvalidProfile)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", nil, fmt.Errorf("base_url %q must not contain query or fragment: %w", raw, ErrInvalidProfile)
	}
	if parsed.Path != "/" {
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
		if parsed.Path == "" {
			parsed.Path = "/"
		}
	}
	normalized := parsed.String()
	if strings.HasSuffix(normalized, "/") && parsed.Path == "/" {
		normalized = strings.TrimSuffix(normalized, "/")
	}
	if strings.HasSuffix(normalized, "/") {
		return "", nil, fmt.Errorf("base_url %q must not end with a trailing slash: %w", raw, ErrInvalidProfile)
	}
	return normalized, parsed, nil
}

func validateAuth(profile Profile, parsedURL *url.URL) error {
	switch profile.Auth.Type {
	case AuthTypeNone:
		if profile.Auth.CredentialEnv != "" {
			return fmt.Errorf("profile %q none auth requires empty credential_env: %w", profile.ID, ErrInvalidProfile)
		}
	case AuthTypeBearerEnv:
		if profile.Type != TypeOpenAICompatible {
			return fmt.Errorf("profile %q bearer_env auth is unsupported for type %q: %w", profile.ID, profile.Type, ErrInvalidProfile)
		}
		if !credentialEnvPattern.MatchString(profile.Auth.CredentialEnv) {
			return fmt.Errorf("profile %q credential_env %q is invalid: %w", profile.ID, profile.Auth.CredentialEnv, ErrInvalidProfile)
		}
		if parsedURL.Scheme != "https" && !isLoopbackHost(parsedURL.Hostname()) {
			return fmt.Errorf("profile %q bearer_env auth requires https or loopback http: %w", profile.ID, ErrInvalidProfile)
		}
	default:
		return fmt.Errorf("profile %q auth type %q is invalid: %w", profile.ID, profile.Auth.Type, ErrInvalidProfile)
	}
	if profile.Type == TypeOllama && profile.Auth.Type != AuthTypeNone {
		return fmt.Errorf("profile %q ollama auth must be none: %w", profile.ID, ErrInvalidProfile)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func marshalCanonical(profiles []Profile) []byte {
	var builder strings.Builder
	builder.WriteString("version: 1\n")
	builder.WriteString("profiles:\n")
	for _, profile := range profiles {
		builder.WriteString("  - id: ")
		builder.WriteString(profile.ID)
		builder.WriteString("\n")
		builder.WriteString("    name: ")
		builder.WriteString(yamlScalar(profile.Name))
		builder.WriteString("\n")
		builder.WriteString("    type: ")
		builder.WriteString(string(profile.Type))
		builder.WriteString("\n")
		builder.WriteString("    base_url: ")
		builder.WriteString(profile.BaseURL)
		builder.WriteString("\n")
		builder.WriteString("    auth:\n")
		builder.WriteString("      type: ")
		builder.WriteString(string(profile.Auth.Type))
		builder.WriteString("\n")
		builder.WriteString("      credential_env: ")
		builder.WriteString(yamlScalar(profile.Auth.CredentialEnv))
		builder.WriteString("\n")
		builder.WriteString("    capabilities:\n")
		builder.WriteString("      chat: ")
		builder.WriteString(boolString(profile.Capabilities.Chat))
		builder.WriteString("\n")
		builder.WriteString("      streaming: ")
		builder.WriteString(boolString(profile.Capabilities.Streaming))
		builder.WriteString("\n")
		builder.WriteString("      structured_output: ")
		builder.WriteString(boolString(profile.Capabilities.StructuredOutput))
		builder.WriteString("\n")
		builder.WriteString("      max_context_tokens: ")
		builder.WriteString(fmt.Sprintf("%d", profile.Capabilities.MaxContextTokens))
		builder.WriteString("\n")
	}
	return []byte(builder.String())
}

func yamlScalar(value string) string {
	if value == "" {
		return `""`
	}
	if strings.HasPrefix(value, " ") || strings.HasSuffix(value, " ") ||
		strings.Contains(value, ": ") || strings.Contains(value, " #") ||
		strings.ContainsAny(value, `[]{}#,>&*!|>'"%@\x60\\`) ||
		strings.EqualFold(value, "null") || strings.EqualFold(value, "true") || strings.EqualFold(value, "false") {
		return strconv.Quote(value)
	}
	return value
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func NormalizeReadiness(profile Profile, brokerReadiness Readiness) Profile {
	profile.Readiness = ReadinessReady
	if profile.Auth.Type == AuthTypeBearerEnv {
		profile.Readiness = brokerReadiness
	}
	return profile
}

func CompatibleReadiness(profile *Profile, readiness Readiness) ProfileReadiness {
	if profile == nil {
		return ProfileReadinessMissingProfile
	}
	if profile.Auth.Type == AuthTypeBearerEnv && readiness != ReadinessReady {
		return ProfileReadinessMissingCredential
	}
	return ProfileReadinessReady
}

func SortProfiles(profiles []Profile) {
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Name != profiles[j].Name {
			return profiles[i].Name < profiles[j].Name
		}
		return profiles[i].ID < profiles[j].ID
	})
}

func CloneProfiles(profiles []Profile) []Profile {
	return slices.Clone(profiles)
}
