package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"storywork/internal/provider"
)

type HTTPGenerator struct {
	client  *http.Client
	path    string
	decoder func([]byte) (string, error)
	body    func(GenerateRequest) ([]byte, error)
}

func (g *HTTPGenerator) Generate(ctx context.Context, request GenerateRequest, resolved provider.ResolvedProfile) (GenerateResponse, error) {
	body, err := g.body(request)
	if err != nil {
		return GenerateResponse{}, ErrProviderInvalid
	}
	if len(body) > 6<<20 {
		return GenerateResponse{}, ErrProviderRejected
	}
	targetURL, err := joinProviderURL(resolved.BaseURL, g.path)
	if err != nil {
		return GenerateResponse{}, ErrProviderInvalid
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return GenerateResponse{}, ErrProviderInvalid
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if resolved.Auth.Type == provider.AuthTypeBearerEnv {
		httpRequest.Header.Set("Authorization", "Bearer "+resolved.Credential.Value)
	}
	response, err := g.client.Do(httpRequest)
	if err != nil {
		return GenerateResponse{}, ErrProviderOffline
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusServiceUnavailable {
		return GenerateResponse{}, ErrProviderOffline
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return GenerateResponse{}, ErrProviderRejected
	}
	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if contentType != "" {
		mediaType, _, err := mime.ParseMediaType(contentType)
		if err != nil || mediaType != "application/json" {
			return GenerateResponse{}, ErrProviderRejected
		}
	}
	responseBody, err := readBoundedBody(response.Body, 2<<20)
	if err != nil {
		return GenerateResponse{}, ErrProviderRejected
	}
	content, err := g.decoder(responseBody)
	if err != nil {
		return GenerateResponse{}, ErrProviderRejected
	}
	content, err = NormalizeGeneratedReplacement(content)
	if err != nil {
		return GenerateResponse{}, ErrProviderRejected
	}
	return GenerateResponse{Replacement: content, Provider: ProviderIdentity{ProfileID: resolved.ID, Type: resolved.Type, Model: request.Style.Model}}, nil
}

func userPrompt(request GenerateRequest) string {
	return "Task: " + request.Agent.Description + "\n\nRewrite only the selected text. Return replacement text only. Do not add commentary or Markdown fences.\n\nSelected text:\n" + request.Packet.SelectedText
}

func joinProviderURL(baseURL, suffix string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parsed.Path = path.Clean(strings.TrimSuffix(parsed.Path, "/") + suffix)
	return parsed.String(), nil
}

func decodeStrictJSON(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if err == nil {
			return io.ErrUnexpectedEOF
		}
		return err
	}
	return nil
}

func readBoundedBody(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, ErrProviderRejected
	}
	return body, nil
}
