package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestNormalizeOllamaBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "default", input: "", want: "http://localhost:11434/v1"},
		{name: "root", input: "http://localhost:11434", want: "http://localhost:11434/v1"},
		{name: "api", input: "http://localhost:11434/api", want: "http://localhost:11434/v1"},
		{name: "v1", input: "http://localhost:11434/v1", want: "http://localhost:11434/v1"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeOllamaBaseURL(test.input)
			if err != nil {
				t.Fatalf("normalize: %v", err)
			}
			if got != test.want {
				t.Fatalf("unexpected normalized URL: %s", got)
			}
		})
	}
}

func TestListOllamaModels(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"qwen2.5-coder:7b"},{"name":"llama3.2:3b"}]}`))
	}))
	defer server.Close()

	models, normalized, err := ListOllamaModels(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if normalized != server.URL+"/v1" {
		t.Fatalf("unexpected normalized URL: %s", normalized)
	}
	if !slices.Equal(models, []string{"llama3.2:3b", "qwen2.5-coder:7b"}) {
		t.Fatalf("unexpected models: %+v", models)
	}
}
