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
	think := ollamaThinkProfile(request.Model)
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

	chatURL, err := ollamaChatURL(settings.NormalizedBaseURL())
	if err != nil {
		return ChatResult{}, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, fmt.Errorf("build ollama chat request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return ChatResult{}, fmt.Errorf("send ollama chat request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(response.Body)
		return ChatResult{}, fmt.Errorf("ollama chat failed: status=%s body=%s", response.Status, strings.TrimSpace(string(data)))
	}

	return readOllamaStream(response, onDelta)
}

func ollamaThinkProfile(model string) any {
	normalized := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(normalized, "qwen") && (strings.Contains(normalized, "3.5") || strings.Contains(normalized, "35")):
		return true
	case strings.Contains(normalized, "deepseek"):
		return true
	case strings.Contains(normalized, "gemma-4"),
		strings.Contains(normalized, "gemma4"):
		return true
	case strings.HasPrefix(normalized, "glm-"),
		strings.Contains(normalized, "glm-4"):
		return true
	default:
		return nil
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
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	var content strings.Builder
	var reasoning strings.Builder
	toolCalls := make([]domain.ToolCall, 0)
	finishReason := ""
	usage := domain.UsageStats{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var chunk ollamaStreamChunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return ChatResult{}, fmt.Errorf("decode ollama stream chunk: %w", err)
		}

		if chunk.Message.Thinking != "" {
			reasoning.WriteString(chunk.Message.Thinking)
			if onDelta != nil {
				if err := onDelta(Delta{Reasoning: chunk.Message.Thinking}); err != nil {
					return ChatResult{}, err
				}
			}
		}

		if chunk.Message.Content != "" {
			content.WriteString(chunk.Message.Content)
			if onDelta != nil {
				if err := onDelta(Delta{Content: chunk.Message.Content}); err != nil {
					return ChatResult{}, err
				}
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
	}
	if err := scanner.Err(); err != nil {
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
