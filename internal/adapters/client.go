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

	"github.com/keonho-kim/orch/domain"
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
	Model              string           `json:"model"`
	Messages           []Message        `json:"messages"`
	Tools              []ToolDefinition `json:"tools,omitempty"`
	Stream             bool             `json:"stream"`
	StreamIncludeUsage bool             `json:"stream_include_usage,omitempty"`
}

type ChatResult struct {
	Content      string
	Reasoning    string
	ToolCalls    []domain.ToolCall
	FinishReason string
	Usage        domain.UsageStats
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
	return ollamaClient{}
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
	if c.provider == domain.ProviderVLLM {
		request.StreamIncludeUsage = true
	}

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

	return readOpenAICompatibleStream(response, onDelta)
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

func ToolCatalog(mode domain.RunMode) []ToolDefinition {
	description := "Run one allowed CLI-style command. Prefer `ot read --path <path>` for file content and quick directory inspection, `ot list [--path <path>]` for long listings, and `ot search [--path <path>] [--name <glob>] [--content <pattern>]` for curated search. Use `ot pointer --value <ot-pointer>` to read current-session transcript lines referenced from compact summaries or chatHistory. Use `ot subagent --prompt <task>` when a bounded child run should execute in a separate child session. Use `rg` or `find` directly only when task execution needs behavior outside the curated OT commands. Use `bash tools/<script>.sh ...` only for custom scripts not covered by the curated OT commands. Use direct toolchain commands only when needed for build, test, or task execution."
	if mode == domain.RunModePlan {
		description = "Run one plan-mode command. Only `cd <path>`, `ot read --path <path>`, `ot list [--path <path>]`, and `ot search [--path <path>] [--name <glob>] [--content <pattern>]` are allowed."
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

func ToolSummary(mode domain.RunMode) string {
	tools := ToolCatalog(mode)
	if len(tools) == 0 {
		return "(none)"
	}

	lines := make([]string, 0, len(tools))
	for _, tool := range tools {
		description := firstSentence(tool.Function.Description)
		if description == "" {
			description = strings.TrimSpace(tool.Function.Description)
		}
		if description == "" {
			description = "No description."
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", tool.Function.Name, description))
	}
	return strings.Join(lines, "\n")
}

func firstSentence(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if index := strings.Index(value, ". "); index >= 0 {
		return strings.TrimSpace(value[:index+1])
	}
	if index := strings.IndexByte(value, '\n'); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return value
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
