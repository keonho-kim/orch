package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"orch/domain"
)

func TestOpenAICompatibleClientStreamsContentAndToolCalls(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"exec\",\"arguments\":\"{\\\"command\\\":\\\"ot\\\",\\\"args\\\":[\\\"read\\\",\\\"--path\\\",\\\"README.md\\\"]}\"}}],\"content\":\"\"},\"finish_reason\":\"tool_calls\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOllamaClient()
	result, err := client.Chat(context.Background(), domain.ProviderSettings{
		BaseURL: server.URL + "/v1",
		Model:   "test-model",
	}, ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "hello"}},
	}, nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if result.Content != "hello world" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("unexpected tool calls: %+v", result.ToolCalls)
	}
	if result.ToolCalls[0].Name != "exec" {
		t.Fatalf("unexpected tool call name: %s", result.ToolCalls[0].Name)
	}
}

func TestOpenAICompatibleClientStreamsReasoning(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"thinking \"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := NewOllamaClient()
	result, err := client.Chat(context.Background(), domain.ProviderSettings{
		BaseURL: server.URL + "/v1",
		Model:   "test-model",
	}, ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: "user", Content: "hello"}},
	}, nil)
	if err != nil {
		t.Fatalf("chat: %v", err)
	}

	if result.Reasoning != "thinking " {
		t.Fatalf("unexpected reasoning: %q", result.Reasoning)
	}
	if result.Content != "answer" {
		t.Fatalf("unexpected content: %q", result.Content)
	}
}

func TestToolCatalogDescriptionsMatchOTContract(t *testing.T) {
	t.Parallel()

	react := ToolCatalog(domain.RunModeReact)[0].Function.Description
	if !strings.Contains(react, "ot read --path <path>") {
		t.Fatalf("expected react tool description to mention path inspection, got %q", react)
	}
	if !strings.Contains(react, "`rg` or `find`") {
		t.Fatalf("expected react tool description to mention rg/find, got %q", react)
	}

	plan := ToolCatalog(domain.RunModePlan)[0].Function.Description
	if !strings.Contains(plan, "ot read --path <path>") {
		t.Fatalf("expected plan tool description to mention path inspection, got %q", plan)
	}
	if strings.Contains(plan, "ot read --path <file>") {
		t.Fatalf("did not expect legacy file-only wording, got %q", plan)
	}
}
