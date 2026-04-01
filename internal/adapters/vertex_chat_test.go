package adapters

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func TestBuildVertexRequestMapsMessagesAndTools(t *testing.T) {
	t.Parallel()

	request, err := buildVertexRequest(ChatRequest{
		Model: "gemini-2.5-pro",
		Messages: []Message{
			{Role: "system", Content: "system prompt"},
			{Role: "user", Content: "user prompt"},
			{
				Role:    "assistant",
				Content: "assistant draft",
				ToolCalls: []ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: ToolFunction{
						Name:      "ot",
						Arguments: `{"op":"read","path":"README.md"}`,
					},
				}},
			},
			{Role: "tool", Name: "ot", ToolCallID: "call_1", Content: `{"result":"done"}`},
		},
		Tools: []ToolDefinition{
			newTool("ot", "Run OT.", map[string]any{
				"type": "object",
			}),
		},
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	if request.SystemInstruction == nil || len(request.SystemInstruction.Parts) != 1 || request.SystemInstruction.Parts[0].Text != "system prompt" {
		t.Fatalf("unexpected system instruction: %+v", request.SystemInstruction)
	}
	if len(request.Contents) != 3 {
		t.Fatalf("unexpected contents: %+v", request.Contents)
	}
	if request.Contents[1].Role != "model" || request.Contents[1].Parts[1].FunctionCall == nil {
		t.Fatalf("expected assistant tool call mapping, got %+v", request.Contents[1])
	}
	if request.Contents[2].Role != "user" || request.Contents[2].Parts[0].FunctionResponse == nil {
		t.Fatalf("expected tool response mapping, got %+v", request.Contents[2])
	}
	if len(request.Tools) != 1 || len(request.Tools[0].FunctionDeclarations) != 1 || request.Tools[0].FunctionDeclarations[0].Name != "ot" {
		t.Fatalf("unexpected tools payload: %+v", request.Tools)
	}
}

func TestVertexClientStreamsTextToolCallsAndUsage(t *testing.T) {
	t.Setenv("GOOGLE_API_KEY", "google-key")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/publishers/google/models/gemini-2.5-pro:streamGenerateContent" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("key") != "google-key" {
			t.Fatalf("unexpected API key query: %s", r.URL.RawQuery)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if !strings.Contains(string(body), `"functionDeclarations"`) {
			t.Fatalf("expected function declarations in request body: %s", string(body))
		}

		_, _ = io.WriteString(w, "{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hello \"}]}}]}\n")
		_, _ = io.WriteString(w, "{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"world\"},{\"functionCall\":{\"name\":\"ot\",\"args\":{\"op\":\"read\",\"path\":\"README.md\"}}}],\"role\":\"model\"},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":6,\"totalTokenCount\":16}}\n")
	}))
	defer server.Close()

	client := NewVertexClient()
	result, err := client.Chat(context.Background(), domain.ProviderSettings{
		BaseURL:   server.URL + "/v1",
		Model:     "gemini-2.5-pro",
		APIKeyEnv: "GOOGLE_API_KEY",
	}, ChatRequest{
		Model: "gemini-2.5-pro",
		Messages: []Message{
			{Role: "system", Content: "system prompt"},
			{Role: "user", Content: "hello"},
		},
		Tools: []ToolDefinition{
			newTool("ot", "Run OT.", map[string]any{"type": "object"}),
		},
	}, nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if result.Content != "hello world" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Name != "ot" || result.ToolCalls[0].Arguments != `{"op":"read","path":"README.md"}` {
		t.Fatalf("unexpected tool calls: %+v", result.ToolCalls)
	}
	if result.Usage.TotalTokens != 16 || result.Usage.PromptTokens != 10 || result.Usage.CompletionTokens != 6 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}
}
