package storyfile

// codex_store.go implements canonical Codex entry and progression file loading and serialization.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"storywork/internal/codex"
)

// LoadCodexEntries reads, validates, and sorts every canonical Codex entry in the project.
func (s *Store) LoadCodexEntries(_ context.Context, projectPath string) ([]codex.Entry, error) {
	var entries []codex.Entry
	for _, entryType := range []codex.EntryType{codex.TypeCharacter, codex.TypeLocation, codex.TypeLore, codex.TypeCustom} {
		directory, err := codex.DirectoryForType(entryType)
		if err != nil {
			return nil, err
		}
		pattern := filepath.Join(projectPath, "codex", directory, "*.yaml")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		sort.Strings(matches)
		for _, match := range matches {
			relative := filepath.ToSlash(strings.TrimPrefix(match, projectPath+string(os.PathSeparator)))
			contents, err := s.readFile(match)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", relative, maskPathError(err))
			}
			entry, err := parseCodexEntry(relative, contents)
			if err != nil {
				return nil, err
			}
			entries = append(entries, entry)
		}
	}
	codex.SortEntries(entries)
	return entries, nil
}

// LoadCodexEntry reads and validates one canonical Codex entry by stable ID.
func (s *Store) LoadCodexEntry(_ context.Context, projectPath, entryID string) (codex.Entry, error) {
	relativePath, err := codexEntryPath(entryID)
	if err != nil {
		return codex.Entry{}, err
	}
	contents, err := s.readFile(filepath.Join(projectPath, filepath.FromSlash(relativePath)))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return codex.Entry{}, fmt.Errorf("entry %q: %w", entryID, codex.ErrEntryNotFound)
		}
		return codex.Entry{}, fmt.Errorf("read %s: %w", relativePath, maskPathError(err))
	}
	return parseCodexEntry(relativePath, contents)
}

// LoadProgressions reads one canonical progression document or returns an empty logical document when none exists.
func (s *Store) LoadProgressions(_ context.Context, projectPath, entryID string) (codex.ProgressionDocument, error) {
	if err := codex.ValidateEntryID(entryID); err != nil {
		return codex.ProgressionDocument{}, err
	}
	relativePath := filepath.ToSlash(filepath.Join("progressions", entryID+".yaml"))
	contents, err := s.readFile(filepath.Join(projectPath, "progressions", entryID+".yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return codex.ProgressionDocument{
				Version:      codex.Version,
				EntryID:      entryID,
				Progressions: []codex.Progression{},
				Revision:     nil,
			}, nil
		}
		return codex.ProgressionDocument{}, fmt.Errorf("read %s: %w", relativePath, maskPathError(err))
	}
	return parseProgressionDocument(relativePath, contents)
}

// MarshalCodexEntry encodes one canonical Codex entry file.
func (s *Store) MarshalCodexEntry(entry codex.Entry) ([]byte, error) {
	normalized, err := codex.NormalizeEntry(entry)
	if err != nil {
		return nil, err
	}
	if normalized.ID == "" {
		return nil, fmt.Errorf("entry ID is required: %w", codex.ErrInvalidID)
	}
	var buffer strings.Builder
	buffer.WriteString("version: 1\n")
	buffer.WriteString("id: ")
	buffer.WriteString(yamlScalar(normalized.ID))
	buffer.WriteString("\n")
	buffer.WriteString("type: ")
	buffer.WriteString(string(normalized.Type))
	buffer.WriteString("\n")
	buffer.WriteString("name: ")
	buffer.WriteString(yamlScalar(normalized.Name))
	buffer.WriteString("\n")
	if len(normalized.Aliases) == 0 {
		buffer.WriteString("aliases: []\n")
	} else {
		buffer.WriteString("aliases:\n")
		for _, alias := range normalized.Aliases {
			buffer.WriteString("  - ")
			buffer.WriteString(yamlScalar(alias))
			buffer.WriteString("\n")
		}
	}
	if len(normalized.Tags) == 0 {
		buffer.WriteString("tags: []\n")
	} else {
		buffer.WriteString("tags:\n")
		for _, tag := range normalized.Tags {
			buffer.WriteString("  - ")
			buffer.WriteString(yamlScalar(tag))
			buffer.WriteString("\n")
		}
	}
	buffer.WriteString("description: ")
	buffer.WriteString(yamlScalar(normalized.Description))
	buffer.WriteString("\n")
	if len(normalized.Metadata) == 0 {
		buffer.WriteString("metadata: {}\n")
	} else {
		buffer.WriteString("metadata:\n")
		keys := sortedMetadataKeys(normalized.Metadata)
		for _, key := range keys {
			buffer.WriteString("  ")
			buffer.WriteString(yamlScalar(key))
			buffer.WriteString(": ")
			buffer.WriteString(yamlScalar(normalized.Metadata[key]))
			buffer.WriteString("\n")
		}
	}
	return []byte(buffer.String()), nil
}

// MarshalProgressions encodes one canonical progression document.
func (s *Store) MarshalProgressions(document codex.ProgressionDocument) ([]byte, error) {
	if err := codex.ValidateEntryID(document.EntryID); err != nil {
		return nil, err
	}
	sceneIDs := make(map[string]struct{}, len(document.Progressions))
	for _, progression := range document.Progressions {
		sceneIDs[progression.Anchor.ID] = struct{}{}
	}
	normalized, err := codex.NormalizeProgressions(document.EntryID, document.Progressions, sceneIDs)
	if err != nil {
		return nil, err
	}
	var buffer strings.Builder
	buffer.WriteString("version: 1\n")
	buffer.WriteString("entry_id: ")
	buffer.WriteString(yamlScalar(document.EntryID))
	buffer.WriteString("\n")
	if len(normalized) == 0 {
		buffer.WriteString("progressions: []\n")
		return []byte(buffer.String()), nil
	}
	buffer.WriteString("progressions:\n")
	for _, progression := range normalized {
		buffer.WriteString("  -")
		if progression.ID != "" {
			buffer.WriteString(" id: ")
			buffer.WriteString(yamlScalar(progression.ID))
			buffer.WriteString("\n")
		} else {
			buffer.WriteString("\n")
		}
		buffer.WriteString("    anchor:\n")
		buffer.WriteString("      type: scene\n")
		buffer.WriteString("      id: ")
		buffer.WriteString(yamlScalar(progression.Anchor.ID))
		buffer.WriteString("\n")
		buffer.WriteString("      timing: ")
		buffer.WriteString(progression.Anchor.Timing)
		buffer.WriteString("\n")
		buffer.WriteString("    changes:\n")
		if progression.Changes.Description != nil {
			buffer.WriteString("      description: ")
			buffer.WriteString(yamlScalar(*progression.Changes.Description))
			buffer.WriteString("\n")
		}
		if len(progression.Changes.Metadata) == 0 {
			if progression.Changes.Description == nil {
				buffer.WriteString("      metadata: {}\n")
			}
		} else {
			buffer.WriteString("      metadata:\n")
			for _, key := range sortedMetadataKeys(progression.Changes.Metadata) {
				buffer.WriteString("        ")
				buffer.WriteString(yamlScalar(key))
				buffer.WriteString(": ")
				buffer.WriteString(yamlScalar(progression.Changes.Metadata[key]))
				buffer.WriteString("\n")
			}
		}
	}
	return []byte(buffer.String()), nil
}

func parseCodexEntry(path string, contents []byte) (codex.Entry, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(contents, &root); err != nil {
		return codex.Entry{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return codex.Entry{}, fmt.Errorf("decode %s: top-level document must be a mapping", path)
	}
	document := root.Content[0]
	fields, err := mappingFields(path, document, "version", "id", "type", "name", "aliases", "tags", "description", "metadata")
	if err != nil {
		return codex.Entry{}, err
	}
	var version int
	if err := fields["version"].Decode(&version); err != nil {
		return codex.Entry{}, fmt.Errorf("decode %s version: %w", path, err)
	}
	if version != codex.Version {
		return codex.Entry{}, fmt.Errorf("%s has unsupported version %d", path, version)
	}
	var entry codex.Entry
	entry.Version = version
	if err := fields["id"].Decode(&entry.ID); err != nil {
		return codex.Entry{}, fmt.Errorf("decode %s id: %w", path, err)
	}
	var entryType string
	if err := fields["type"].Decode(&entryType); err != nil {
		return codex.Entry{}, fmt.Errorf("decode %s type: %w", path, err)
	}
	entry.Type = codex.EntryType(entryType)
	if err := fields["name"].Decode(&entry.Name); err != nil {
		return codex.Entry{}, fmt.Errorf("decode %s name: %w", path, err)
	}
	aliases, err := decodeStringSequence(path, "aliases", fields["aliases"])
	if err != nil {
		return codex.Entry{}, err
	}
	tags, err := decodeStringSequence(path, "tags", fields["tags"])
	if err != nil {
		return codex.Entry{}, err
	}
	description, err := decodeStringScalar(path, "description", fields["description"])
	if err != nil {
		return codex.Entry{}, err
	}
	metadata, err := decodeStringMap(path, "metadata", fields["metadata"])
	if err != nil {
		return codex.Entry{}, err
	}
	entry.Aliases = aliases
	entry.Tags = tags
	entry.Description = description
	entry.Metadata = metadata
	normalized, err := codex.NormalizeEntry(entry)
	if err != nil {
		return codex.Entry{}, fmt.Errorf("decode %s: %w", path, err)
	}
	relativePath, err := codexEntryPath(normalized.ID)
	if err != nil {
		return codex.Entry{}, err
	}
	if relativePath != path {
		return codex.Entry{}, fmt.Errorf("%s does not match canonical path %s", path, relativePath)
	}
	normalized.Revision = codex.ComputeRevision(contents)
	normalized.Canonical = append([]byte(nil), contents...)
	return normalized, nil
}

func parseProgressionDocument(path string, contents []byte) (codex.ProgressionDocument, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(contents, &root); err != nil {
		return codex.ProgressionDocument{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return codex.ProgressionDocument{}, fmt.Errorf("decode %s: top-level document must be a mapping", path)
	}
	fields, err := mappingFields(path, root.Content[0], "version", "entry_id", "progressions")
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	var version int
	if err := fields["version"].Decode(&version); err != nil {
		return codex.ProgressionDocument{}, fmt.Errorf("decode %s version: %w", path, err)
	}
	if version != codex.Version {
		return codex.ProgressionDocument{}, fmt.Errorf("%s has unsupported version %d", path, version)
	}
	var entryID string
	if err := fields["entry_id"].Decode(&entryID); err != nil {
		return codex.ProgressionDocument{}, fmt.Errorf("decode %s entry_id: %w", path, err)
	}
	progressions, err := decodeProgressions(path, fields["progressions"])
	if err != nil {
		return codex.ProgressionDocument{}, err
	}
	normalized, err := codex.NormalizeStoredProgressions(entryID, progressions)
	if err != nil {
		return codex.ProgressionDocument{}, fmt.Errorf("decode %s: %w", path, err)
	}
	expectedPath := filepath.ToSlash(filepath.Join("progressions", entryID+".yaml"))
	if path != expectedPath {
		return codex.ProgressionDocument{}, fmt.Errorf("%s does not match canonical path %s", path, expectedPath)
	}
	revision := codex.ComputeRevision(contents)
	return codex.ProgressionDocument{
		Version:      codex.Version,
		EntryID:      entryID,
		Progressions: normalized,
		Revision:     &revision,
		Canonical:    append([]byte(nil), contents...),
	}, nil
}

func mappingFields(path string, node *yaml.Node, keys ...string) (map[string]*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("decode %s: expected mapping node", path)
	}
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
	}
	fields := make(map[string]*yaml.Node, len(keys))
	for index := 0; index < len(node.Content); index += 2 {
		keyNode := node.Content[index]
		valueNode := node.Content[index+1]
		if _, ok := allowed[keyNode.Value]; !ok {
			return nil, fmt.Errorf("decode %s: field %s not found", path, keyNode.Value)
		}
		if _, exists := fields[keyNode.Value]; exists {
			return nil, fmt.Errorf("decode %s: duplicate field %q", path, keyNode.Value)
		}
		fields[keyNode.Value] = valueNode
	}
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			return nil, fmt.Errorf("decode %s: missing field %q", path, key)
		}
	}
	return fields, nil
}

func decodeStringSequence(path, field string, node *yaml.Node) ([]string, error) {
	if node.Kind == yaml.SequenceNode && len(node.Content) == 0 {
		return []string{}, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("decode %s %s: expected sequence", path, field)
	}
	result := make([]string, 0, len(node.Content))
	for _, child := range node.Content {
		var value string
		if err := child.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode %s %s: %w", path, field, err)
		}
		result = append(result, value)
	}
	return result, nil
}

func decodeStringScalar(path, field string, node *yaml.Node) (string, error) {
	var value string
	if err := node.Decode(&value); err != nil {
		return "", fmt.Errorf("decode %s %s: %w", path, field, err)
	}
	return value, nil
}

func decodeStringMap(path, field string, node *yaml.Node) (map[string]string, error) {
	if node.Kind == yaml.MappingNode {
		values := make(map[string]string, len(node.Content)/2)
		for index := 0; index < len(node.Content); index += 2 {
			keyNode := node.Content[index]
			valueNode := node.Content[index+1]
			if _, exists := values[keyNode.Value]; exists {
				return nil, fmt.Errorf("decode %s %s: duplicate key %q", path, field, keyNode.Value)
			}
			var value string
			if err := valueNode.Decode(&value); err != nil {
				return nil, fmt.Errorf("decode %s %s value %q: %w", path, field, keyNode.Value, err)
			}
			values[keyNode.Value] = value
		}
		return values, nil
	}
	return nil, fmt.Errorf("decode %s %s: expected mapping", path, field)
}

func decodeProgressions(path string, node *yaml.Node) ([]codex.Progression, error) {
	if node.Kind == yaml.SequenceNode {
		progressions := make([]codex.Progression, 0, len(node.Content))
		for index, child := range node.Content {
			fields, err := optionalMappingFields(fmt.Sprintf("%s progressions[%d]", path, index), child, "anchor", "changes", "id")
			if err != nil {
				return nil, err
			}
			if _, ok := fields["anchor"]; !ok {
				return nil, fmt.Errorf("decode %s progressions[%d]: missing field %q", path, index, "anchor")
			}
			if _, ok := fields["changes"]; !ok {
				return nil, fmt.Errorf("decode %s progressions[%d]: missing field %q", path, index, "changes")
			}
			progressions = append(progressions, codex.Progression{})
			if idNode, ok := fields["id"]; ok {
				if err := idNode.Decode(&progressions[index].ID); err != nil {
					return nil, fmt.Errorf("decode %s progressions[%d] id: %w", path, index, err)
				}
			}
			anchorFields, err := mappingFields(fmt.Sprintf("%s progressions[%d].anchor", path, index), fields["anchor"], "type", "id", "timing")
			if err != nil {
				return nil, err
			}
			if err := anchorFields["type"].Decode(&progressions[index].Anchor.Type); err != nil {
				return nil, fmt.Errorf("decode %s progressions[%d] anchor type: %w", path, index, err)
			}
			if err := anchorFields["id"].Decode(&progressions[index].Anchor.ID); err != nil {
				return nil, fmt.Errorf("decode %s progressions[%d] anchor id: %w", path, index, err)
			}
			if err := anchorFields["timing"].Decode(&progressions[index].Anchor.Timing); err != nil {
				return nil, fmt.Errorf("decode %s progressions[%d] anchor timing: %w", path, index, err)
			}
			changeFields, err := optionalMappingFields(fmt.Sprintf("%s progressions[%d].changes", path, index), fields["changes"], "description", "metadata")
			if err != nil {
				return nil, err
			}
			if descriptionNode, ok := changeFields["description"]; ok {
				description, err := decodeStringScalar(path, fmt.Sprintf("progressions[%d].changes.description", index), descriptionNode)
				if err != nil {
					return nil, err
				}
				progressions[index].Changes.Description = &description
			}
			if metadataNode, ok := changeFields["metadata"]; ok {
				metadata, err := decodeStringMap(path, fmt.Sprintf("progressions[%d].changes.metadata", index), metadataNode)
				if err != nil {
					return nil, err
				}
				progressions[index].Changes.Metadata = metadata
			}
		}
		return progressions, nil
	}
	return nil, fmt.Errorf("decode %s progressions: expected sequence", path)
}

func optionalMappingFields(path string, node *yaml.Node, keys ...string) (map[string]*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("decode %s: expected mapping", path)
	}
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
	}
	fields := make(map[string]*yaml.Node, len(keys))
	for index := 0; index < len(node.Content); index += 2 {
		keyNode := node.Content[index]
		valueNode := node.Content[index+1]
		if _, ok := allowed[keyNode.Value]; !ok {
			return nil, fmt.Errorf("decode %s: field %s not found", path, keyNode.Value)
		}
		if _, exists := fields[keyNode.Value]; exists {
			return nil, fmt.Errorf("decode %s: duplicate field %q", path, keyNode.Value)
		}
		fields[keyNode.Value] = valueNode
	}
	return fields, nil
}

func codexEntryPath(entryID string) (string, error) {
	entryType, err := codex.TypeForID(entryID)
	if err != nil {
		return "", err
	}
	directory, err := codex.DirectoryForType(entryType)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join("codex", directory, entryID+".yaml")), nil
}

func sortedMetadataKeys(metadata map[string]string) []string {
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// maskPathError strips absolute filesystem paths from OS-level errors so the
// active project root never leaks through HTTP responses. The contract forbids
// exposing filesystem paths outside the active project root; wrapping with a
// relative path keeps errors contextual without revealing host layout.
func maskPathError(err error) error {
	if err == nil {
		return nil
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return fmt.Errorf("%s: %v", filepath.Base(pathErr.Path), pathErr.Err)
	}
	return err
}

// yamlScalar formats a string as a YAML scalar using the minimum quoting
// required by the canonical contract: plain scalars for safe values (IDs,
// tags, metadata keys, simple names), and double-quoted scalars for values
// that contain characters YAML would otherwise misinterpret (newlines,
// leading indicators, quotes, trailing whitespace, or empty strings).
// Quoted scalars use Go-style double-quoting so embedded newlines serialize
// as the documented `\n` escape rather than as block scalars.
func yamlScalar(value string) string {
	if value == "" {
		return `""`
	}
	if needsYAMLQuoting(value) {
		return strconv.Quote(value)
	}
	return value
}

// needsYAMLQuoting reports whether a plain scalar would be ambiguous or
// invalid for the YAML emitter. This intentionally stays conservative so
// canonical bytes remain stable and diffable.
func needsYAMLQuoting(value string) bool {
	if value == "" {
		return true
	}
	// Quote if the value starts with a YAML indicator character or whitespace.
	switch value[0] {
	case '!', '&', '*', '-', '?', ':', '>', '|', '%', '@', '`', '"', '\'', '#', ',', '[', ']', '{', '}', ' ', '\t':
		return true
	}
	// Quote if the value ends with whitespace.
	if value[len(value)-1] == ' ' || value[len(value)-1] == '\t' {
		return true
	}
	// Quote if the value contains a newline, colon-space, hash-space, or any
	// control character. Newlines must serialize as the documented \n escape.
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch == '\n' || ch == '\t' || ch == '\r' || ch == 0 {
			return true
		}
		if ch == ':' && i+1 < len(value) && (value[i+1] == ' ' || value[i+1] == '\t') {
			return true
		}
		if ch == ' ' && i+1 < len(value) && value[i+1] == '#' {
			return true
		}
		if ch < 0x20 {
			return true
		}
	}
	// Quote reserved YAML keywords that would otherwise parse as booleans,
	// null, or numbers when the field is a string.
	switch value {
	case "null", "Null", "NULL", "~",
		"true", "True", "TRUE", "false", "False", "FALSE",
		"yes", "Yes", "YES", "no", "No", "NO",
		"on", "On", "ON", "off", "Off", "OFF":
		return true
	}
	return false
}
