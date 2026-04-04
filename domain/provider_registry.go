package domain

import (
	"fmt"
	"strings"
)

type ProviderSpec struct {
	Provider         Provider
	DisplayName      string
	DefaultEndpoint  string
	RequiresEndpoint bool
	RequiresModel    bool
	RequiresAPIKey   bool
}

var providerSpecs = map[Provider]ProviderSpec{
	ProviderOllama: {
		Provider:         ProviderOllama,
		DisplayName:      "Ollama",
		DefaultEndpoint:  "http://localhost:11434/v1",
		RequiresEndpoint: true,
		RequiresModel:    true,
	},
	ProviderVLLM: {
		Provider:         ProviderVLLM,
		DisplayName:      "vLLM",
		DefaultEndpoint:  "http://localhost:8000/v1",
		RequiresEndpoint: true,
		RequiresModel:    true,
	},
	ProviderGemini: {
		Provider:         ProviderGemini,
		DisplayName:      "Gemini",
		DefaultEndpoint:  "https://generativelanguage.googleapis.com/v1beta/openai",
		RequiresEndpoint: true,
		RequiresModel:    true,
		RequiresAPIKey:   true,
	},
	ProviderVertex: {
		Provider:         ProviderVertex,
		DisplayName:      "Vertex",
		DefaultEndpoint:  "https://aiplatform.googleapis.com/v1",
		RequiresEndpoint: true,
		RequiresModel:    true,
		RequiresAPIKey:   true,
	},
	ProviderBedrock: {
		Provider:         ProviderBedrock,
		DisplayName:      "Bedrock",
		RequiresEndpoint: true,
		RequiresModel:    true,
		RequiresAPIKey:   true,
	},
	ProviderClaude: {
		Provider:         ProviderClaude,
		DisplayName:      "Claude",
		DefaultEndpoint:  "https://api.anthropic.com/v1",
		RequiresEndpoint: true,
		RequiresModel:    true,
		RequiresAPIKey:   true,
	},
	ProviderAzure: {
		Provider:         ProviderAzure,
		DisplayName:      "Azure",
		RequiresEndpoint: true,
		RequiresModel:    true,
		RequiresAPIKey:   true,
	},
	ProviderChatGPT: {
		Provider:         ProviderChatGPT,
		DisplayName:      "ChatGPT",
		DefaultEndpoint:  "https://api.openai.com/v1",
		RequiresEndpoint: true,
		RequiresModel:    true,
		RequiresAPIKey:   true,
	},
}

func ProviderCatalogFor(provider Provider) (ProviderSpec, bool) {
	spec, ok := providerSpecs[provider]
	return spec, ok
}

func MustProviderCatalog(provider Provider) ProviderSpec {
	spec, ok := ProviderCatalogFor(provider)
	if ok {
		return spec
	}
	return ProviderSpec{
		Provider:    provider,
		DisplayName: strings.ToUpper(string(provider)),
	}
}

func ParseReasoningValue(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "", "true", "false", "low", "medium", "high", "xhigh":
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported value %q", value)
	}
}

func (c *ProviderCatalog) Provider(provider Provider) *ProviderSettings {
	switch provider {
	case ProviderOllama:
		return &c.Ollama
	case ProviderVLLM:
		return &c.VLLM
	case ProviderGemini:
		return &c.Gemini
	case ProviderVertex:
		return &c.Vertex
	case ProviderBedrock:
		return &c.Bedrock
	case ProviderClaude:
		return &c.Claude
	case ProviderAzure:
		return &c.Azure
	case ProviderChatGPT:
		return &c.ChatGPT
	default:
		return &c.Ollama
	}
}
