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
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"exec\",\"arguments\":\"{\\\"command\\\":\\\"ot\\\",\\\"args\\\":[\\\"read\\\",\\\"--path\\\",\\\"README.md\\\"]}\"}}],\"content\":\"\"},\"finish_reason\":\"tool_calls\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":6,\"total_tokens\":16}}\n\n")
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
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].Name != "exec" {
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

func TestToolCatalogDescriptionsMatchOTContract(t *testing.T) {
	t.Parallel()

	react := ToolCatalog(domain.RunModeReact)[0].Function.Description
	if !strings.Contains(react, "ot read --path <path>") {
		t.Fatalf("expected react tool description to mention path inspection, got %q", react)
	}
	if !strings.Contains(react, "`ot list [--path <path>]`") {
		t.Fatalf("expected react tool description to mention ot list, got %q", react)
	}
	if !strings.Contains(react, "`ot search [--path <path>] [--name <glob>] [--content <pattern>]`") {
		t.Fatalf("expected react tool description to mention ot search, got %q", react)
	}
	if !strings.Contains(react, "`ot subagent --prompt <task>`") {
		t.Fatalf("expected react tool description to mention ot subagent, got %q", react)
	}
	if !strings.Contains(react, "`ot pointer --value <ot-pointer>`") {
		t.Fatalf("expected react tool description to mention ot pointer, got %q", react)
	}
	if !strings.Contains(react, "`rg` or `find` directly only") {
		t.Fatalf("expected react tool description to keep rg/find fallback wording, got %q", react)
	}

	plan := ToolCatalog(domain.RunModePlan)[0].Function.Description
	if !strings.Contains(plan, "ot read --path <path>") {
		t.Fatalf("expected plan tool description to mention path inspection, got %q", plan)
	}
	if !strings.Contains(plan, "ot list [--path <path>]") {
		t.Fatalf("expected plan tool description to mention ot list, got %q", plan)
	}
	if !strings.Contains(plan, "ot search [--path <path>] [--name <glob>] [--content <pattern>]") {
		t.Fatalf("expected plan tool description to mention ot search, got %q", plan)
	}
	if strings.Contains(plan, "ot subagent") {
		t.Fatalf("did not expect plan tool description to mention ot subagent, got %q", plan)
	}
}

func TestToolSummaryStaysModeAwareAndConcise(t *testing.T) {
	t.Parallel()

	react := ToolSummary(domain.RunModeReact)
	if !strings.Contains(react, "- exec: Run one allowed CLI-style command.") {
		t.Fatalf("expected react tool summary to contain concise exec description, got %q", react)
	}

	plan := ToolSummary(domain.RunModePlan)
	if !strings.Contains(plan, "- exec: Run one plan-mode command.") {
		t.Fatalf("expected plan tool summary to contain concise plan description, got %q", plan)
	}
}
