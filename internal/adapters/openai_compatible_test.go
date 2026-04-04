package adapters

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

type openAICompatibleClientTestCase struct {
	name            string
	provider        domain.Provider
	endpointSuffix  string
	apiKey          string
	wantPath        string
	wantQueryKey    string
	wantQueryValue  string
	wantHeaderName  string
	wantHeaderValue string
	expectModelBody bool
	expectStreamOpt bool
	expectStreamInc bool
}

var openAICompatibleClientCases = []openAICompatibleClientTestCase{
	{
		name:            "gemini",
		provider:        domain.ProviderGemini,
		endpointSuffix:  "/v1beta/openai",
		apiKey:          "gemini-key",
		wantPath:        "/v1beta/openai/chat/completions",
		wantHeaderName:  "Authorization",
		wantHeaderValue: "Bearer gemini-key",
		expectModelBody: true,
		expectStreamOpt: true,
	},
	{
		name:            "bedrock",
		provider:        domain.ProviderBedrock,
		endpointSuffix:  "/v1",
		apiKey:          "bedrock-key",
		wantPath:        "/v1/chat/completions",
		wantHeaderName:  "Authorization",
		wantHeaderValue: "Bearer bedrock-key",
		expectModelBody: true,
		expectStreamOpt: true,
	},
	{
		name:            "claude",
		provider:        domain.ProviderClaude,
		endpointSuffix:  "/v1",
		apiKey:          "claude-key",
		wantPath:        "/v1/chat/completions",
		wantHeaderName:  "Authorization",
		wantHeaderValue: "Bearer claude-key",
		expectModelBody: true,
		expectStreamOpt: true,
	},
	{
		name:            "chatgpt",
		provider:        domain.ProviderChatGPT,
		endpointSuffix:  "/v1",
		apiKey:          "openai-key",
		wantPath:        "/v1/chat/completions",
		wantHeaderName:  "Authorization",
		wantHeaderValue: "Bearer openai-key",
		expectModelBody: true,
		expectStreamOpt: true,
	},
	{
		name:            "azure",
		provider:        domain.ProviderAzure,
		apiKey:          "azure-key",
		wantPath:        "/openai/deployments/deployment-a/chat/completions",
		wantQueryKey:    "api-version",
		wantQueryValue:  "2024-10-21",
		wantHeaderName:  "api-key",
		wantHeaderValue: "azure-key",
		expectModelBody: false,
		expectStreamOpt: true,
	},
	{
		name:            "vllm",
		provider:        domain.ProviderVLLM,
		endpointSuffix:  "/v1",
		apiKey:          "vllm-key",
		wantPath:        "/v1/chat/completions",
		wantHeaderName:  "Authorization",
		wantHeaderValue: "Bearer vllm-key",
		expectModelBody: true,
		expectStreamInc: true,
	},
}

func TestOpenAICompatibleClientsUseProviderSpecificAuthAndURL(t *testing.T) {
	for _, test := range openAICompatibleClientCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			server := newOpenAICompatibleTestServer(t, test)
			defer server.Close()
			result, err := runOpenAICompatibleClientTest(server, test)
			if err != nil {
				t.Fatalf("chat: %v", err)
			}
			if result.Content != "hello" || result.Usage.TotalTokens != 3 {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func newOpenAICompatibleTestServer(t *testing.T, test openAICompatibleClientTestCase) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertOpenAICompatibleRequest(t, r, test)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n")
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
}

func runOpenAICompatibleClientTest(server *httptest.Server, test openAICompatibleClientTestCase) (ChatResult, error) {
	endpoint := server.URL + test.endpointSuffix
	if test.provider == domain.ProviderAzure {
		endpoint = server.URL
	}
	client, err := NewClient(test.provider)
	if err != nil {
		return ChatResult{}, err
	}
	return client.Chat(context.Background(), domain.ProviderSettings{
		Endpoint: endpoint,
		Model:    "deployment-a",
		APIKey:   test.apiKey,
	}, ChatRequest{
		Model:    "deployment-a",
		Messages: []Message{{Role: "user", Content: "hello"}},
	}, nil)
}

func assertOpenAICompatibleRequest(t *testing.T, r *http.Request, test openAICompatibleClientTestCase) {
	t.Helper()
	if r.URL.Path != test.wantPath {
		t.Fatalf("unexpected path: %s", r.URL.Path)
	}
	if test.wantQueryKey != "" && r.URL.Query().Get(test.wantQueryKey) != test.wantQueryValue {
		t.Fatalf("unexpected query %s: %s", test.wantQueryKey, r.URL.Query().Get(test.wantQueryKey))
	}
	if got := r.Header.Get(test.wantHeaderName); got != test.wantHeaderValue {
		t.Fatalf("unexpected header %s: %q", test.wantHeaderName, got)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if _, ok := payload["model"]; ok != test.expectModelBody {
		t.Fatalf("unexpected model presence in request body: %+v", payload)
	}
	if _, ok := payload["stream_options"]; ok != test.expectStreamOpt {
		t.Fatalf("unexpected stream_options presence in request body: %+v", payload)
	}
	if _, ok := payload["stream_include_usage"]; ok != test.expectStreamInc {
		t.Fatalf("unexpected stream_include_usage presence in request body: %+v", payload)
	}
}

func TestBuildOpenAICompatibleRequestAppliesVLLMReasoningProfiles(t *testing.T) {
	getenv = func(name string) string {
		switch name {
		case "ORCH_VLLM_CHAT_TEMPLATE_QWEN35":
			return "{{ qwen35_template }}"
		case "ORCH_VLLM_CHAT_TEMPLATE_DEEPSEEK":
			return "{{ deepseek_template }}"
		default:
			return ""
		}
	}
	defer func() { getenv = os.Getenv }()

	request, err := buildOpenAICompatibleRequest(domain.ProviderVLLM, domain.ProviderSettings{Reasoning: "false"}, ChatRequest{
		Model:    "Qwen3.5-Coder",
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if request.ChatTemplate != "{{ qwen35_template }}" {
		t.Fatalf("unexpected chat template: %q", request.ChatTemplate)
	}
	if got := request.ChatTemplateKwargs["enable_thinking"]; got != false {
		t.Fatalf("unexpected vLLM reasoning kwargs: %+v", request.ChatTemplateKwargs)
	}
}

func TestBuildOpenAICompatibleRequestAppliesOpenAIReasoningEffort(t *testing.T) {
	request, err := buildOpenAICompatibleRequest(domain.ProviderChatGPT, domain.ProviderSettings{Reasoning: "xhigh"}, ChatRequest{
		Model:    "gpt-5.3-codex",
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if request.ReasoningEffort != "xhigh" {
		t.Fatalf("unexpected reasoning effort: %q", request.ReasoningEffort)
	}
}

func TestReadOpenAICompatibleStreamReadsReasoningFallbackFields(t *testing.T) {
	newResponse := func(body string) *http.Response {
		return &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	}

	payload := "data: {\"choices\":[{\"delta\":{\"reasoning\":\"hidden\",\"content\":\"ok\"}}]}\n\n" +
		"data: [DONE]\n\n"

	result, err := readOpenAICompatibleStream(newResponse(payload), nil)
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if result.Reasoning != "hidden" {
		t.Fatalf("expected reasoning fallback, got %q", result.Reasoning)
	}
	if result.Content != "ok" {
		t.Fatalf("expected content, got %q", result.Content)
	}
}
