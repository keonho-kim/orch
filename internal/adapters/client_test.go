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

func TestVLLMClientStreamsContentToolCallsAndUsage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello \"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"world\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"ot\",\"arguments\":\"{\\\"op\\\":\\\"read\\\",\\\"path\\\":\\\"README.md\\\"}\"}}],\"content\":\"\"},\"finish_reason\":\"tool_calls\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":6,\"total_tokens\":16}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewVLLMClient()
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
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Name != "ot" {
		t.Fatalf("unexpected tool calls: %+v", result.ToolCalls)
	}
	if result.Usage.TotalTokens != 16 || result.Usage.PromptTokens != 10 || result.Usage.CompletionTokens != 6 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}
}

func TestOllamaClientStreamsReasoningAndUsage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = io.WriteString(w, "{\"message\":{\"thinking\":\"thinking \",\"content\":\"answer\"},\"done\":false}\n")
		_, _ = io.WriteString(w, "{\"done\":true,\"done_reason\":\"stop\",\"prompt_eval_count\":11,\"eval_count\":7}\n")
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
	if result.Usage.TotalTokens != 18 || result.Usage.PromptTokens != 11 || result.Usage.CompletionTokens != 7 {
		t.Fatalf("unexpected usage: %+v", result.Usage)
	}
}

func TestToolCatalogIsRoleAwareAndOTOnly(t *testing.T) {
	t.Parallel()

	gateway := ToolCatalog(domain.RunModeReact, domain.AgentRoleGateway)
	if len(gateway) != 1 || gateway[0].Function.Name != "ot" {
		t.Fatalf("expected single ot tool, got %+v", gateway)
	}
	description := gateway[0].Function.Description
	if !strings.Contains(description, "gateway OT operation") {
		t.Fatalf("expected gateway description, got %q", description)
	}

	worker := ToolCatalog(domain.RunModeReact, domain.AgentRoleWorker)
	if !strings.Contains(worker[0].Function.Description, "worker OT operation") {
		t.Fatalf("expected worker description, got %q", worker[0].Function.Description)
	}

	plan := ToolCatalog(domain.RunModePlan, domain.AgentRoleGateway)
	if !strings.Contains(plan[0].Function.Description, "context, task_list, task_get, read, list, and search") {
		t.Fatalf("expected plan read-only description, got %q", plan[0].Function.Description)
	}
}

func TestToolSummaryStaysRoleAwareAndConcise(t *testing.T) {
	t.Parallel()

	gateway := ToolSummary(domain.RunModeReact, domain.AgentRoleGateway)
	if !strings.Contains(gateway, "- ot: Run one gateway OT operation.") {
		t.Fatalf("expected gateway summary, got %q", gateway)
	}

	worker := ToolSummary(domain.RunModeReact, domain.AgentRoleWorker)
	if !strings.Contains(worker, "- ot: Run one worker OT operation.") {
		t.Fatalf("expected worker summary, got %q", worker)
	}
}
