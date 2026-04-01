package adapters

import (
	"context"
	"fmt"
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
	return newOpenAICompatibleClient(domain.ProviderVLLM)
}

func NewGeminiClient() Client {
	return newOpenAICompatibleClient(domain.ProviderGemini)
}

func NewVertexClient() Client {
	return vertexClient{}
}

func NewBedrockClient() Client {
	return newOpenAICompatibleClient(domain.ProviderBedrock)
}

func NewClaudeClient() Client {
	return newOpenAICompatibleClient(domain.ProviderClaude)
}

func NewAzureClient() Client {
	return newOpenAICompatibleClient(domain.ProviderAzure)
}

func NewChatGPTClient() Client {
	return newOpenAICompatibleClient(domain.ProviderChatGPT)
}

func ToolCatalog(mode domain.RunMode, role domain.AgentRole) []ToolDefinition {
	allowedOps := []string{"context", "task_list", "task_get", "read", "list", "search"}
	description := "Run one OT operation. Allowed operations in this mode are context, task_list, task_get, read, list, and search."
	if mode != domain.RunModePlan {
		if role == domain.AgentRoleWorker {
			allowedOps = []string{"context", "task_list", "task_get", "read", "list", "search", "write", "patch", "check", "complete", "fail"}
			description = "Run one worker OT operation. Allowed operations are context, task_list, task_get, read, list, search, write, patch, check, complete, and fail."
		} else {
			allowedOps = []string{"context", "task_list", "task_get", "delegate", "read", "list", "search"}
			description = "Run one gateway OT operation. Allowed operations are context, task_list, task_get, delegate, read, list, and search."
		}
	}

	return []ToolDefinition{
		newTool("ot", description, map[string]any{
			"type": "object",
			"properties": map[string]any{
				"op": map[string]any{
					"type": "string",
					"enum": allowedOps,
				},
				"path":            map[string]any{"type": "string"},
				"start_line":      map[string]any{"type": "integer"},
				"end_line":        map[string]any{"type": "integer"},
				"name_pattern":    map[string]any{"type": "string"},
				"content_pattern": map[string]any{"type": "string"},
				"content":         map[string]any{"type": "string"},
				"patch":           map[string]any{"type": "string"},
				"check": map[string]any{
					"type": "string",
					"enum": []string{"go_test", "go_vet", "golangci_lint"},
				},
				"task_id":           map[string]any{"type": "string"},
				"task_title":        map[string]any{"type": "string"},
				"task_contract":     map[string]any{"type": "string"},
				"message":           map[string]any{"type": "string"},
				"wait":              map[string]any{"type": "boolean"},
				"status_filter":     map[string]any{"type": "string"},
				"summary":           map[string]any{"type": "string"},
				"changed_paths":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"checks_run":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"evidence_pointers": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"followups":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"error_kind":        map[string]any{"type": "string"},
			},
			"required": []string{"op"},
		}),
	}
}

func ToolSummary(mode domain.RunMode, role domain.AgentRole) string {
	tools := ToolCatalog(mode, role)
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
