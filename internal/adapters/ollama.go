package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
)

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func NormalizeOllamaBaseURL(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = "http://localhost:11434/v1"
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse Ollama base URL: %w", err)
	}
	if parsed.Scheme == "" {
		return "", fmt.Errorf("ollama base URL must include http:// or https://")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("ollama base URL must include a host")
	}

	cleanPath := strings.TrimRight(parsed.Path, "/")
	switch cleanPath {
	case "", "/", "/api", "/v1":
		parsed.Path = "/v1"
	default:
		parsed.Path = cleanPath
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func OllamaTagsURL(baseURL string) (string, error) {
	normalized, err := NormalizeOllamaBaseURL(baseURL)
	if err != nil {
		return "", err
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("parse Ollama base URL: %w", err)
	}
	parsed.Path = path.Join("/", strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/v1"), "api", "tags")
	return parsed.String(), nil
}

func ListOllamaModels(ctx context.Context, baseURL string) ([]string, string, error) {
	normalized, err := NormalizeOllamaBaseURL(baseURL)
	if err != nil {
		return nil, "", err
	}
	tagsURL, err := OllamaTagsURL(normalized)
	if err != nil {
		return nil, "", err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build Ollama model request: %w", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("connect to Ollama: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, "", fmt.Errorf("ollama model list failed: status=%s", response.Status)
	}

	var payload ollamaTagsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, "", fmt.Errorf("decode Ollama model list: %w", err)
	}

	models := make([]string, 0, len(payload.Models))
	for _, model := range payload.Models {
		name := strings.TrimSpace(model.Name)
		if name == "" {
			continue
		}
		models = append(models, name)
	}
	sort.Strings(models)
	return models, normalized, nil
}
