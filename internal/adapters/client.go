package adapters

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"orch/domain"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments,omitempty"`
}

type ToolDefinition struct {
	Type     string             `json:"type"`
	Function ToolDefinitionBody `json:"function"`
}

type ToolDefinitionBody struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type ChatRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
}

type ChatResult struct {
	Content      string
	Reasoning    string
	ToolCalls    []domain.ToolCall
	FinishReason string
}

type Delta struct {
	Content   string
	Reasoning string
}

type DeltaHandler func(Delta) error

type Client interface {
	Provider() domain.Provider
	Chat(ctx context.Context, settings domain.ProviderSettings, request ChatRequest, onDelta DeltaHandler) (ChatResult, error)
}

func NewOllamaClient() Client {
	return openAICompatibleClient{provider: domain.ProviderOllama}
}

func NewVLLMClient() Client {
	return openAICompatibleClient{provider: domain.ProviderVLLM}
}

type openAICompatibleClient struct {
	provider domain.Provider
}

func (c openAICompatibleClient) Provider() domain.Provider {
	return c.provider
}

func (c openAICompatibleClient) Chat(ctx context.Context, settings domain.ProviderSettings, request ChatRequest, onDelta DeltaHandler) (ChatResult, error) {
	request.Stream = true

	body, err := json.Marshal(request)
	if err != nil {
		return ChatResult{}, fmt.Errorf("marshal chat request: %w", err)
	}

	url := strings.TrimRight(settings.NormalizedBaseURL(), "/") + "/chat/completions"
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, fmt.Errorf("build chat request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	if c.provider == domain.ProviderVLLM && strings.TrimSpace(settings.APIKeyEnv) != "" {
		if apiKey := strings.TrimSpace(os.Getenv(settings.APIKeyEnv)); apiKey != "" {
			httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
		}
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

	return readStream(response, onDelta)
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
}

func readStream(response *http.Response, onDelta DeltaHandler) (ChatResult, error) {
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	var content strings.Builder
	var reasoning strings.Builder
	toolCalls := map[int]domain.ToolCall{}
	finishReason := ""

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
	}, nil
}

func ToolCatalog(mode domain.RunMode) []ToolDefinition {
	description := "Run one allowed CLI-style command. Prefer `ot read --path <path>` for file and directory inspection. Use `rg` or `find` for search and discovery. Use `bash tools/<script>.sh ...` only for custom scripts not covered by the curated OT commands. Use direct toolchain commands only when needed for build, test, or task execution."
	if mode == domain.RunModePlan {
		description = "Run one plan-mode command. Only `cd <path>` and `ot read --path <path>` are allowed."
	}
	return []ToolDefinition{
		newTool("exec", description, map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string"},
				"args": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"cwd":         map[string]any{"type": "string"},
				"timeout_sec": map[string]any{"type": "integer"},
				"stdin":       map[string]any{"type": "string"},
			},
			"required": []string{"command"},
		}),
	}
}

func newTool(name string, description string, parameters map[string]any) ToolDefinition {
	return ToolDefinition{
		Type: "function",
		Function: ToolDefinitionBody{
			Name:        name,
			Description: description,
			Parameters:  parameters,
		},
	}
}

func ToAssistantMessage(result ChatResult) Message {
	toolCalls := make([]ToolCall, 0, len(result.ToolCalls))
	for _, call := range result.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			ID:   call.ID,
			Type: "function",
			Function: ToolFunction{
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		})
	}

	return Message{
		Role:      "assistant",
		Content:   result.Content,
		ToolCalls: toolCalls,
	}
}

func ToToolMessage(result domain.ToolResult) Message {
	return Message{
		Role:       "tool",
		Name:       result.Name,
		ToolCallID: result.ToolCallID,
		Content:    result.Content,
	}
}
