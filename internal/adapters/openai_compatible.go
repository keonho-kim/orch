package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

type openAICompatibleClient struct {
	provider domain.Provider
}

type openAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAICompatibleRequest struct {
	Model              string               `json:"model,omitempty"`
	Messages           []Message            `json:"messages"`
	Tools              []ToolDefinition     `json:"tools,omitempty"`
	Stream             bool                 `json:"stream"`
	StreamIncludeUsage bool                 `json:"stream_include_usage,omitempty"`
	StreamOptions      *openAIStreamOptions `json:"stream_options,omitempty"`
	ChatTemplate       string               `json:"chat_template,omitempty"`
	ChatTemplateKwargs map[string]any       `json:"chat_template_kwargs,omitempty"`
}

type streamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamEnvelope struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage streamUsage `json:"usage"`
}

func newOpenAICompatibleClient(provider domain.Provider) Client {
	return openAICompatibleClient{provider: provider}
}

func (c openAICompatibleClient) Provider() domain.Provider {
	return c.provider
}

func (c openAICompatibleClient) Chat(ctx context.Context, settings domain.ProviderSettings, request ChatRequest, onDelta DeltaHandler) (ChatResult, error) {
	payload, err := buildOpenAICompatibleRequest(c.provider, request)
	if err != nil {
		return ChatResult{}, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResult{}, fmt.Errorf("marshal chat request: %w", err)
	}

	chatURL, err := openAICompatibleChatURL(c.provider, settings, request.Model)
	if err != nil {
		return ChatResult{}, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, fmt.Errorf("build chat request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if err := applyOpenAICompatibleAuth(httpRequest, c.provider, settings); err != nil {
		return ChatResult{}, err
	}

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return ChatResult{}, fmt.Errorf("send chat request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(response.Body)
		return ChatResult{}, fmt.Errorf("chat request failed: status=%s body=%s", response.Status, strings.TrimSpace(string(data)))
	}

	return readOpenAICompatibleStream(response, onDelta)
}

func buildOpenAICompatibleRequest(provider domain.Provider, request ChatRequest) (openAICompatibleRequest, error) {
	profile := modelProfile(provider, request.Model)
	payload := openAICompatibleRequest{
		Messages: request.Messages,
		Tools:    request.Tools,
		Stream:   true,
	}

	switch provider {
	case domain.ProviderAzure:
		payload.StreamOptions = &openAIStreamOptions{IncludeUsage: true}
	case domain.ProviderVLLM:
		payload.Model = request.Model
		payload.StreamIncludeUsage = true
		if profile.ChatTemplate != "" {
			payload.ChatTemplate = profile.ChatTemplate
		}
		if len(profile.ChatTemplateKwargs) > 0 {
			payload.ChatTemplateKwargs = profile.ChatTemplateKwargs
		}
	default:
		payload.Model = request.Model
		payload.StreamOptions = &openAIStreamOptions{IncludeUsage: true}
	}

	return payload, nil
}

type openAIModelProfile struct {
	ChatTemplate       string
	ChatTemplateKwargs map[string]any
}

func modelProfile(provider domain.Provider, model string) openAIModelProfile {
	if provider != domain.ProviderVLLM {
		return openAIModelProfile{}
	}

	normalized := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(normalized, "qwen") && (strings.Contains(normalized, "3.5") || strings.Contains(normalized, "35")):
		return openAIModelProfile{
			ChatTemplate:       strings.TrimSpace(os.Getenv("ORCH_VLLM_CHAT_TEMPLATE_QWEN35")),
			ChatTemplateKwargs: map[string]any{"enable_thinking": true},
		}
	case strings.Contains(normalized, "deepseek"):
		return openAIModelProfile{
			ChatTemplate:       strings.TrimSpace(os.Getenv("ORCH_VLLM_CHAT_TEMPLATE_DEEPSEEK")),
			ChatTemplateKwargs: map[string]any{"thinking": true},
		}
	case strings.Contains(normalized, "gemma-4"),
		strings.Contains(normalized, "gemma4"):
		return openAIModelProfile{
			ChatTemplate: strings.TrimSpace(os.Getenv("ORCH_VLLM_CHAT_TEMPLATE_GEMMA4")),
		}
	case strings.HasPrefix(normalized, "glm-"),
		strings.Contains(normalized, "glm-4"):
		return openAIModelProfile{
			ChatTemplate: strings.TrimSpace(os.Getenv("ORCH_VLLM_CHAT_TEMPLATE_GLM")),
		}
	default:
		return openAIModelProfile{}
	}
}

func openAICompatibleChatURL(provider domain.Provider, settings domain.ProviderSettings, model string) (string, error) {
	switch provider {
	case domain.ProviderAzure:
		baseURL := strings.TrimSpace(settings.BaseURL)
		if baseURL == "" {
			return "", fmt.Errorf("Azure base URL is required")
		}
		if strings.TrimSpace(model) == "" {
			return "", fmt.Errorf("Azure model is required")
		}

		parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
		if err != nil {
			return "", fmt.Errorf("parse Azure base URL: %w", err)
		}
		cleanPath := strings.TrimRight(parsed.Path, "/")
		cleanPath = strings.TrimSuffix(cleanPath, "/openai/v1")
		cleanPath = strings.TrimSuffix(cleanPath, "/openai")
		parsed.Path = strings.TrimRight(cleanPath, "/") + "/openai/deployments/" + strings.TrimSpace(model) + "/chat/completions"
		query := parsed.Query()
		query.Set("api-version", "2024-10-21")
		parsed.RawQuery = query.Encode()
		return parsed.String(), nil
	default:
		baseURL := settings.NormalizedBaseURL()
		if baseURL == "" {
			return "", fmt.Errorf("%s base URL is required", provider.DisplayName())
		}
		return strings.TrimRight(baseURL, "/") + "/chat/completions", nil
	}
}

func applyOpenAICompatibleAuth(request *http.Request, provider domain.Provider, settings domain.ProviderSettings) error {
	switch provider {
	case domain.ProviderAzure:
		apiKey, err := requiredAPIKey(settings, provider)
		if err != nil {
			return err
		}
		request.Header.Set("api-key", apiKey)
		return nil
	case domain.ProviderVLLM:
		apiKey, err := optionalAPIKey(settings)
		if err != nil {
			return err
		}
		if apiKey != "" {
			request.Header.Set("Authorization", "Bearer "+apiKey)
		}
		return nil
	default:
		apiKey, err := requiredAPIKey(settings, provider)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+apiKey)
		return nil
	}
}

func optionalAPIKey(settings domain.ProviderSettings) (string, error) {
	name := strings.TrimSpace(settings.APIKeyEnv)
	if name == "" {
		return "", nil
	}
	return strings.TrimSpace(os.Getenv(name)), nil
}

func requiredAPIKey(settings domain.ProviderSettings, provider domain.Provider) (string, error) {
	name := strings.TrimSpace(settings.APIKeyEnv)
	if name == "" {
		return "", fmt.Errorf("%s API key env is not configured", provider.DisplayName())
	}
	apiKey := strings.TrimSpace(os.Getenv(name))
	if apiKey == "" {
		return "", fmt.Errorf("%s API key env %s is not set", provider.DisplayName(), name)
	}
	return apiKey, nil
}

func readOpenAICompatibleStream(response *http.Response, onDelta DeltaHandler) (ChatResult, error) {
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	var content strings.Builder
	var reasoning strings.Builder
	toolCalls := map[int]domain.ToolCall{}
	finishReason := ""
	usage := domain.UsageStats{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var envelope streamEnvelope
		if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
			return ChatResult{}, fmt.Errorf("decode stream payload: %w", err)
		}

		if envelope.Usage.TotalTokens > 0 || envelope.Usage.PromptTokens > 0 || envelope.Usage.CompletionTokens > 0 {
			usage = domain.UsageStats{
				PromptTokens:     envelope.Usage.PromptTokens,
				CompletionTokens: envelope.Usage.CompletionTokens,
				TotalTokens:      envelope.Usage.TotalTokens,
			}
		}

		for _, choice := range envelope.Choices {
			reasoningChunk := choice.Delta.ReasoningContent
			if reasoningChunk == "" {
				reasoningChunk = choice.Delta.Reasoning
			}
			if reasoningChunk != "" {
				reasoning.WriteString(reasoningChunk)
				if onDelta != nil {
					if err := onDelta(Delta{Reasoning: reasoningChunk}); err != nil {
						return ChatResult{}, err
					}
				}
			}

			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				if onDelta != nil {
					if err := onDelta(Delta{Content: choice.Delta.Content}); err != nil {
						return ChatResult{}, err
					}
				}
			}

			for _, toolDelta := range choice.Delta.ToolCalls {
				call := toolCalls[toolDelta.Index]
				if call.ID == "" && toolDelta.ID != "" {
					call.ID = toolDelta.ID
				}
				if call.Name == "" && toolDelta.Function.Name != "" {
					call.Name = toolDelta.Function.Name
				}
				call.Arguments += toolDelta.Function.Arguments
				toolCalls[toolDelta.Index] = call
			}

			if choice.FinishReason != "" {
				finishReason = choice.FinishReason
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return ChatResult{}, fmt.Errorf("read stream: %w", err)
	}

	ordered := make([]domain.ToolCall, 0, len(toolCalls))
	for index := 0; index < len(toolCalls); index++ {
		if call, ok := toolCalls[index]; ok {
			ordered = append(ordered, call)
		}
	}

	return ChatResult{
		Content:      content.String(),
		Reasoning:    reasoning.String(),
		ToolCalls:    ordered,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}
