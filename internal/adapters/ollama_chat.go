package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

type ollamaClient struct{}

type ollamaChatRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
	Think    any              `json:"think,omitempty"`
}

type ollamaStreamChunk struct {
	Message struct {
		Role      string `json:"role"`
		Content   string `json:"content"`
		Thinking  string `json:"thinking"`
		ToolCalls []struct {
			Function struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"message"`
	Done            bool   `json:"done"`
	DoneReason      string `json:"done_reason"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
}

func (ollamaClient) Provider() domain.Provider {
	return domain.ProviderOllama
}

func (ollamaClient) Chat(ctx context.Context, settings domain.ProviderSettings, request ChatRequest, onDelta DeltaHandler) (ChatResult, error) {
	request.Stream = true
	think, err := ollamaThinkProfile(request.Model, settings.Reasoning)
	if err != nil {
		return ChatResult{}, err
	}
	body, err := json.Marshal(ollamaChatRequest{
		Model:    request.Model,
		Messages: request.Messages,
		Tools:    request.Tools,
		Stream:   true,
		Think:    think,
	})
	if err != nil {
		return ChatResult{}, fmt.Errorf("marshal ollama chat request: %w", err)
	}

	chatURL, err := ollamaChatURL(settings.NormalizedEndpoint())
	if err != nil {
		return ChatResult{}, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, fmt.Errorf("build ollama chat request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if apiKey := strings.TrimSpace(settings.APIKey); apiKey != "" {
		httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	}

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return ChatResult{}, fmt.Errorf("send ollama chat request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(response.Body)
		return ChatResult{}, fmt.Errorf("ollama chat failed: status=%s body=%s", response.Status, strings.TrimSpace(string(data)))
	}

	return readOllamaStream(response, onDelta)
}

func ollamaThinkProfile(model string, reasoning string) (any, error) {
	switch strings.ToLower(strings.TrimSpace(reasoning)) {
	case "":
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(reasoning)), nil
	case "xhigh":
		return nil, fmt.Errorf("ollama does not support xhigh reasoning")
	default:
		return nil, fmt.Errorf("unsupported Ollama reasoning value %q", reasoning)
	}

	normalized := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(normalized, "qwen") && (strings.Contains(normalized, "3.5") || strings.Contains(normalized, "35")):
		return true, nil
	case strings.Contains(normalized, "deepseek"):
		return true, nil
	case strings.Contains(normalized, "gemma-4"),
		strings.Contains(normalized, "gemma4"):
		return true, nil
	case strings.HasPrefix(normalized, "glm-"),
		strings.Contains(normalized, "glm-4"):
		return true, nil
	default:
		return nil, nil
	}
}

func ollamaChatURL(baseURL string) (string, error) {
	normalized, err := NormalizeOllamaBaseURL(baseURL)
	if err != nil {
		return "", err
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", fmt.Errorf("parse Ollama base URL: %w", err)
	}
	parsed.Path = path.Join("/", strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/v1"), "api", "chat")
	return parsed.String(), nil
}

func readOllamaStream(response *http.Response, onDelta DeltaHandler) (ChatResult, error) {
	var content strings.Builder
	var reasoning strings.Builder
	toolCalls := make([]domain.ToolCall, 0)
	finishReason := ""
	usage := domain.UsageStats{}

	err := scanStreamLines(response.Body, streamScanOptions{}, func(line string) error {
		var chunk ollamaStreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return fmt.Errorf("decode ollama stream chunk: %w", err)
		}

		if chunk.Message.Thinking != "" {
			reasoning.WriteString(chunk.Message.Thinking)
			if err := emitDelta(onDelta, Delta{Reasoning: chunk.Message.Thinking}); err != nil {
				return err
			}
		}

		if chunk.Message.Content != "" {
			content.WriteString(chunk.Message.Content)
			if err := emitDelta(onDelta, Delta{Content: chunk.Message.Content}); err != nil {
				return err
			}
		}

		for index, toolCall := range chunk.Message.ToolCalls {
			arguments := "{}"
			if len(toolCall.Function.Arguments) > 0 {
				arguments = string(toolCall.Function.Arguments)
			}
			toolCalls = append(toolCalls, domain.ToolCall{
				ID:        fmt.Sprintf("call_%d", index+1),
				Name:      toolCall.Function.Name,
				Arguments: arguments,
			})
		}

		if chunk.Done {
			finishReason = chunk.DoneReason
			usage = domain.UsageStats{
				PromptTokens:     chunk.PromptEvalCount,
				CompletionTokens: chunk.EvalCount,
				TotalTokens:      chunk.PromptEvalCount + chunk.EvalCount,
			}
		}
		return nil
	})
	if err != nil {
		return ChatResult{}, fmt.Errorf("read ollama stream: %w", err)
	}

	return ChatResult{
		Content:      content.String(),
		Reasoning:    reasoning.String(),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}
