package adapters

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/keonho-kim/orch/domain"
)

func TestOpenAICompatibleClientsUseProviderSpecificAuthAndURL(t *testing.T) {
	tests := []struct {
		name            string
		client          func() Client
		provider        domain.Provider
		baseURLSuffix   string
		apiKeyEnv       string
		apiKeyValue     string
		wantPath        string
		wantQueryKey    string
		wantQueryValue  string
		wantHeaderName  string
		wantHeaderValue string
		expectModelBody bool
		expectStreamOpt bool
		expectStreamInc bool
	}{
		{
			name:            "gemini",
			client:          NewGeminiClient,
			provider:        domain.ProviderGemini,
			baseURLSuffix:   "/v1beta/openai",
			apiKeyEnv:       "GEMINI_API_KEY",
			apiKeyValue:     "gemini-key",
			wantPath:        "/v1beta/openai/chat/completions",
			wantHeaderName:  "Authorization",
			wantHeaderValue: "Bearer gemini-key",
			expectModelBody: true,
			expectStreamOpt: true,
		},
		{
			name:            "bedrock",
			client:          NewBedrockClient,
			provider:        domain.ProviderBedrock,
			baseURLSuffix:   "/v1",
			apiKeyEnv:       "AWS_BEARER_TOKEN_BEDROCK",
			apiKeyValue:     "bedrock-key",
			wantPath:        "/v1/chat/completions",
			wantHeaderName:  "Authorization",
			wantHeaderValue: "Bearer bedrock-key",
			expectModelBody: true,
			expectStreamOpt: true,
		},
		{
			name:            "claude",
			client:          NewClaudeClient,
			provider:        domain.ProviderClaude,
			baseURLSuffix:   "/v1",
			apiKeyEnv:       "ANTHROPIC_API_KEY",
			apiKeyValue:     "claude-key",
			wantPath:        "/v1/chat/completions",
			wantHeaderName:  "Authorization",
			wantHeaderValue: "Bearer claude-key",
			expectModelBody: true,
			expectStreamOpt: true,
		},
		{
			name:            "chatgpt",
			client:          NewChatGPTClient,
			provider:        domain.ProviderChatGPT,
			baseURLSuffix:   "/v1",
			apiKeyEnv:       "OPENAI_API_KEY",
			apiKeyValue:     "openai-key",
			wantPath:        "/v1/chat/completions",
			wantHeaderName:  "Authorization",
			wantHeaderValue: "Bearer openai-key",
			expectModelBody: true,
			expectStreamOpt: true,
		},
		{
			name:            "azure",
			client:          NewAzureClient,
			provider:        domain.ProviderAzure,
			apiKeyEnv:       "AZURE_OPENAI_API_KEY",
			apiKeyValue:     "azure-key",
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
			client:          NewVLLMClient,
			provider:        domain.ProviderVLLM,
			baseURLSuffix:   "/v1",
			apiKeyEnv:       "VLLM_API_KEY",
			apiKeyValue:     "vllm-key",
			wantPath:        "/v1/chat/completions",
			wantHeaderName:  "Authorization",
			wantHeaderValue: "Bearer vllm-key",
			expectModelBody: true,
			expectStreamInc: true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.apiKeyEnv, test.apiKeyValue)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n")
				_, _ = io.WriteString(w, "data: [DONE]\n\n")
			}))
			defer server.Close()

			client := test.client()
			baseURL := server.URL + test.baseURLSuffix
			if test.provider == domain.ProviderAzure {
				baseURL = server.URL
			}

			result, err := client.Chat(context.Background(), domain.ProviderSettings{
				BaseURL:   baseURL,
				Model:     "deployment-a",
				APIKeyEnv: test.apiKeyEnv,
			}, ChatRequest{
				Model:    "deployment-a",
				Messages: []Message{{Role: "user", Content: "hello"}},
			}, nil)
			if err != nil {
				t.Fatalf("chat: %v", err)
			}
			if result.Content != "hello" || result.Usage.TotalTokens != 3 {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestBuildOpenAICompatibleRequestAppliesVLLMModelProfiles(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		templateEnv  string
		templateBody string
		wantKey      string
		wantVal      any
	}{
		{
			name:         "qwen-3.5 enables thinking",
			model:        "Qwen3.5-Coder",
			templateEnv:  "ORCH_VLLM_CHAT_TEMPLATE_QWEN35",
			templateBody: "{{ qwen35_template }}",
			wantKey:      "enable_thinking",
			wantVal:      true,
		},
		{
			name:         "deepseek enables thinking",
			model:        "deepseek-r1",
			templateEnv:  "ORCH_VLLM_CHAT_TEMPLATE_DEEPSEEK",
			templateBody: "{{ deepseek_template }}",
			wantKey:      "thinking",
			wantVal:      true,
		},
		{
			name:         "gemma4 has family template only",
			model:        "gemma-4-27b-it",
			templateEnv:  "ORCH_VLLM_CHAT_TEMPLATE_GEMMA4",
			templateBody: "{{ gemma4_template }}",
		},
		{
			name:         "glm has family template only",
			model:        "glm-4.5",
			templateEnv:  "ORCH_VLLM_CHAT_TEMPLATE_GLM",
			templateBody: "{{ glm_template }}",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			if test.templateEnv != "" {
				t.Setenv(test.templateEnv, test.templateBody)
			}
			request, err := buildOpenAICompatibleRequest(domain.ProviderVLLM, ChatRequest{
				Model:    test.model,
				Messages: []Message{{Role: "user", Content: "hello"}},
			})
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if strings.TrimSpace(test.templateBody) != "" && request.ChatTemplate != test.templateBody {
				t.Fatalf("unexpected chat_template: %q", request.ChatTemplate)
			}

			if test.wantKey == "" {
				if len(request.ChatTemplateKwargs) != 0 {
					t.Fatalf("expected empty chat_template_kwargs, got %+v", request.ChatTemplateKwargs)
				}
				return
			}
			if len(request.ChatTemplateKwargs) == 0 {
				t.Fatalf("expected chat_template_kwargs for model %s", test.model)
			}
			if got := request.ChatTemplateKwargs[test.wantKey]; got != test.wantVal {
				t.Fatalf("unexpected chat_template_kwargs[%s]: %v", test.wantKey, got)
			}
		})
	}
}

func TestBuildOpenAICompatibleRequestKeepsNonVLLMClearFromTemplateKwargs(t *testing.T) {
	t.Parallel()

	request, err := buildOpenAICompatibleRequest(domain.ProviderChatGPT, ChatRequest{
		Model:    "gpt-5",
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if request.ChatTemplate != "" {
		t.Fatalf("expected non-vLLM request to have no chat_template, got %q", request.ChatTemplate)
	}
	if len(request.ChatTemplateKwargs) != 0 {
		t.Fatalf("expected non-vLLM request to have no chat_template_kwargs, got %+v", request.ChatTemplateKwargs)
	}
}
