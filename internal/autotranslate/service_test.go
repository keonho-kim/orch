package autotranslate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
)

func TestDetectReturnsKor(t *testing.T) {
	t.Parallel()

	server := newTranslationTestServer(t)
	defer server.Close()

	detected, err := Detect(context.Background(), adapters.NewVLLMClient(), domain.ProviderSettings{
		BaseURL: server.URL + "/v1",
		Model:   "test-model",
	}, "test-model", "로그인 구현해줘")
	if err != nil {
		t.Fatalf("detect language: %v", err)
	}
	if detected != LanguageKorean {
		t.Fatalf("unexpected detection: %q", detected)
	}
}

func TestPrepareFromDetectionBuildsEnglishBundle(t *testing.T) {
	t.Parallel()

	server := newTranslationTestServer(t)
	defer server.Close()

	prepared, err := PrepareFromDetection(context.Background(), adapters.NewVLLMClient(), domain.ProviderSettings{
		BaseURL: server.URL + "/v1",
		Model:   "test-model",
	}, "test-model", "ship it", LanguageEnglish)
	if err != nil {
		t.Fatalf("prepare from detection: %v", err)
	}

	if !strings.Contains(prepared.AgentPrompt, "[Korean]") || !strings.Contains(prepared.AgentPrompt, "[Chinese]") {
		t.Fatalf("expected multilingual prompt, got %q", prepared.AgentPrompt)
	}
	if !strings.Contains(prepared.AgentPrompt, "KO: ship it") {
		t.Fatalf("expected Korean translation block, got %q", prepared.AgentPrompt)
	}
	if !strings.Contains(prepared.AgentPrompt, "ZH: ship it") {
		t.Fatalf("expected Chinese translation block, got %q", prepared.AgentPrompt)
	}
	if prepared.ResponseLanguage != LanguageEnglish {
		t.Fatalf("unexpected response language: %s", prepared.ResponseLanguage)
	}
}

func TestPrepareFromDetectionBuildsChineseBundle(t *testing.T) {
	t.Parallel()

	server := newTranslationTestServer(t)
	defer server.Close()

	prepared, err := PrepareFromDetection(context.Background(), adapters.NewVLLMClient(), domain.ProviderSettings{
		BaseURL: server.URL + "/v1",
		Model:   "test-model",
	}, "test-model", "中文需求", LanguageChinese)
	if err != nil {
		t.Fatalf("prepare from detection: %v", err)
	}

	if !strings.Contains(prepared.AgentPrompt, "KO: 中文需求") {
		t.Fatalf("expected Korean translation block, got %q", prepared.AgentPrompt)
	}
	if !strings.Contains(prepared.AgentPrompt, "EN: 中文需求") {
		t.Fatalf("expected English translation block, got %q", prepared.AgentPrompt)
	}
	if prepared.ResponseLanguage != LanguageChinese {
		t.Fatalf("unexpected response language: %s", prepared.ResponseLanguage)
	}
}

func newTranslationTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	var mu sync.Mutex
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request adapters.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		systemPrompt := request.Messages[0].Content
		userContent := request.Messages[len(request.Messages)-1].Content

		mu.Lock()
		defer mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		switch {
		case strings.Contains(systemPrompt, "language detection service"):
			content := "kor"
			if strings.Contains(userContent, "ship it") {
				content = "en"
			}
			if strings.Contains(userContent, "中文") {
				content = "ch"
			}
			writeStreamChunk(w, content)
		case strings.Contains(systemPrompt, "Translate the user input into Korean."):
			writeStreamChunk(w, "KO: "+userContent)
		case strings.Contains(systemPrompt, "Translate the user input into English."):
			writeStreamChunk(w, "EN: "+userContent)
		case strings.Contains(systemPrompt, "Translate the user input into Simplified Chinese."):
			writeStreamChunk(w, "ZH: "+userContent)
		default:
			t.Fatalf("unexpected system prompt: %q", systemPrompt)
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
}

func writeStreamChunk(w http.ResponseWriter, content string) {
	_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` + quoteJSONString(content) + `}}]}` + "\n\n"))
}

func quoteJSONString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
