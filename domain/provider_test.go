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
	if got := settings.MissingProviderFields(ProviderGemini); !slices.Equal(got, []string{"Model"}) {
		t.Fatalf("unexpected Gemini missing fields: %+v", got)
	}
	if got := settings.MissingProviderFields(ProviderVertex); !slices.Equal(got, []string{"Model"}) {
		t.Fatalf("unexpected Vertex missing fields: %+v", got)
	}
	if got := settings.MissingProviderFields(ProviderBedrock); !slices.Equal(got, []string{"Base URL", "Model"}) {
		t.Fatalf("unexpected Bedrock missing fields: %+v", got)
	}
	if got := settings.MissingProviderFields(ProviderAzure); !slices.Equal(got, []string{"Base URL", "Model"}) {
		t.Fatalf("unexpected Azure missing fields: %+v", got)
	}
}

func TestSettingsSecretEnvNamesIncludeCloudProviders(t *testing.T) {
	t.Parallel()

	settings := Settings{
		Providers: ProviderCatalog{
			VLLM:    ProviderSettings{APIKeyEnv: "VLLM_API_KEY"},
			Gemini:  ProviderSettings{APIKeyEnv: "GEMINI_API_KEY"},
			Claude:  ProviderSettings{APIKeyEnv: "ANTHROPIC_API_KEY"},
			ChatGPT: ProviderSettings{APIKeyEnv: "OPENAI_API_KEY"},
		},
	}

	got := settings.SecretEnvNames()
	want := []string{"VLLM_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY", "AWS_BEARER_TOKEN_BEDROCK", "ANTHROPIC_API_KEY", "AZURE_OPENAI_API_KEY", "OPENAI_API_KEY"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected secret env names: %+v", got)
	}
}
