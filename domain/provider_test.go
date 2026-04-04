package domain

import (
	"slices"
	"testing"
)

func TestParseProviderSupportsCloudProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  Provider
	}{
		{input: "gemini", want: ProviderGemini},
		{input: "vertex", want: ProviderVertex},
		{input: "bedrock", want: ProviderBedrock},
		{input: "claude", want: ProviderClaude},
		{input: "azure", want: ProviderAzure},
		{input: "chatgpt", want: ProviderChatGPT},
	}

	for _, test := range tests {
		test := test
		t.Run(test.input, func(t *testing.T) {
			t.Parallel()

			got, err := ParseProvider(test.input)
			if err != nil {
				t.Fatalf("parse provider: %v", err)
			}
			if got != test.want {
				t.Fatalf("unexpected provider: %s", got)
			}
		})
	}
}

func TestSettingsMissingProviderFieldsAreProviderSpecific(t *testing.T) {
	t.Parallel()

	settings := Settings{}
	settings.Normalize()

	if got := settings.MissingProviderFields(ProviderVLLM); !slices.Equal(got, []string{"Model"}) {
		t.Fatalf("unexpected vLLM missing fields: %+v", got)
	}
	if got := settings.MissingProviderFields(ProviderGemini); !slices.Equal(got, []string{"Model", "API Key"}) {
		t.Fatalf("unexpected Gemini missing fields: %+v", got)
	}
	if got := settings.MissingProviderFields(ProviderAzure); !slices.Equal(got, []string{"Endpoint", "Model", "API Key"}) {
		t.Fatalf("unexpected Azure missing fields: %+v", got)
	}
}

func TestSettingsNormalizeAppliesDefaultEndpoints(t *testing.T) {
	t.Parallel()

	settings := Settings{}
	settings.Normalize()

	if settings.Providers.Ollama.Endpoint != "http://localhost:11434/v1" {
		t.Fatalf("unexpected Ollama endpoint: %q", settings.Providers.Ollama.Endpoint)
	}
	if settings.Providers.ChatGPT.Endpoint != "https://api.openai.com/v1" {
		t.Fatalf("unexpected ChatGPT endpoint: %q", settings.Providers.ChatGPT.Endpoint)
	}
}
