package adapters

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
