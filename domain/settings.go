package domain

import (
	"fmt"
	"strings"
)

type ApprovalPolicy string

const (
	ApprovalConfirmMutations ApprovalPolicy = "confirm_mutations"
)

type Settings struct {
	DefaultProvider   Provider        `json:"default_provider"`
	Providers         ProviderCatalog `json:"providers"`
	ApprovalPolicy    ApprovalPolicy  `json:"approval_policy"`
	SelfDrivingMode   bool            `json:"self_driving_mode"`
	ReactRalphIter    int             `json:"react_ralph_iter"`
	PlanRalphIter     int             `json:"plan_ralph_iter"`
	CompactThresholdK int             `json:"compact_threshold_k"`
}

func (s *Settings) Normalize() {
	if strings.TrimSpace(s.Providers.Ollama.BaseURL) == "" {
		s.Providers.Ollama.BaseURL = "http://localhost:11434/v1"
	}
	if strings.TrimSpace(s.Providers.VLLM.BaseURL) == "" {
		s.Providers.VLLM.BaseURL = "http://localhost:8000/v1"
	}
	if strings.TrimSpace(s.Providers.Gemini.BaseURL) == "" {
		s.Providers.Gemini.BaseURL = "https://generativelanguage.googleapis.com/v1beta/openai"
	}
	if strings.TrimSpace(s.Providers.Vertex.BaseURL) == "" {
		s.Providers.Vertex.BaseURL = "https://aiplatform.googleapis.com/v1"
	}
	if strings.TrimSpace(s.Providers.Claude.BaseURL) == "" {
		s.Providers.Claude.BaseURL = "https://api.anthropic.com/v1"
	}
	if strings.TrimSpace(s.Providers.ChatGPT.BaseURL) == "" {
		s.Providers.ChatGPT.BaseURL = "https://api.openai.com/v1"
	}

	normalizeProviderAuth := func(provider Provider, settings *ProviderSettings) {
		if settings.Auth.Kind == "" && strings.TrimSpace(settings.APIKeyEnv) != "" {
			settings.Auth.Kind = ProviderAuthEnv
			settings.Auth.Env = strings.TrimSpace(settings.APIKeyEnv)
		}
		settings.Auth = settings.Auth.Normalize(provider)
		if settings.Auth.Kind == ProviderAuthEnv && strings.TrimSpace(settings.APIKeyEnv) == "" {
			settings.APIKeyEnv = settings.Auth.Env
		}
	}

	normalizeProviderAuth(ProviderOllama, &s.Providers.Ollama)
	normalizeProviderAuth(ProviderVLLM, &s.Providers.VLLM)
	normalizeProviderAuth(ProviderGemini, &s.Providers.Gemini)
	normalizeProviderAuth(ProviderVertex, &s.Providers.Vertex)
	normalizeProviderAuth(ProviderBedrock, &s.Providers.Bedrock)
	normalizeProviderAuth(ProviderClaude, &s.Providers.Claude)
	normalizeProviderAuth(ProviderAzure, &s.Providers.Azure)
	normalizeProviderAuth(ProviderChatGPT, &s.Providers.ChatGPT)

	if s.ApprovalPolicy == "" {
		s.ApprovalPolicy = ApprovalConfirmMutations
	}
	if s.ReactRalphIter <= 0 {
		s.ReactRalphIter = 3
	}
	if s.PlanRalphIter <= 0 {
		s.PlanRalphIter = 3
	}
	if s.CompactThresholdK <= 0 {
		s.CompactThresholdK = 100
	}
}

func (s Settings) ConfigFor(provider Provider) ProviderSettings {
	switch provider {
	case ProviderOllama:
		return s.Providers.Ollama
	case ProviderVLLM:
		return s.Providers.VLLM
	case ProviderGemini:
		return s.Providers.Gemini
	case ProviderVertex:
		return s.Providers.Vertex
	case ProviderBedrock:
		return s.Providers.Bedrock
	case ProviderClaude:
		return s.Providers.Claude
	case ProviderAzure:
		return s.Providers.Azure
	case ProviderChatGPT:
		return s.Providers.ChatGPT
	default:
		return ProviderSettings{}
	}
}

func (s Settings) HasProviderModel(provider Provider) bool {
	return strings.TrimSpace(s.ConfigFor(provider).Model) != ""
}

func (s Settings) MissingProviderFields(provider Provider) []string {
	normalized := s
	normalized.Normalize()
	config := normalized.ConfigFor(provider)

	missing := make([]string, 0, 3)
	if providerRequiresBaseURL(provider) && strings.TrimSpace(config.BaseURL) == "" {
		missing = append(missing, "Base URL")
	}
	if providerRequiresModel(provider) && strings.TrimSpace(config.Model) == "" {
		missing = append(missing, "Model")
	}
	if providerRequiresAuth(provider) && !config.Auth.IsConfigured(true) && strings.TrimSpace(config.APIKeyEnv) == "" {
		missing = append(missing, "Auth")
	}
	return missing
}

func (s Settings) IsProviderReady(provider Provider) bool {
	return len(s.MissingProviderFields(provider)) == 0
}

func (s Settings) ProviderConfigError(provider Provider) error {
	missing := s.MissingProviderFields(provider)
	if len(missing) == 0 {
		return nil
	}
	if len(missing) == 1 && missing[0] == "Model" {
		return fmt.Errorf("model is not configured for %s", provider.DisplayName())
	}
	return fmt.Errorf("%s is not configured; missing %s", provider.DisplayName(), strings.Join(missing, ", "))
}

func (s Settings) SecretEnvNames() []string {
	normalized := s
	normalized.Normalize()

	seen := make(map[string]struct{})
	names := make([]string, 0, len(providerOrder))
	for _, provider := range providerOrder {
		name := normalized.ConfigFor(provider).Auth.SecretEnvName()
		if name == "" {
			name = strings.TrimSpace(normalized.ConfigFor(provider).APIKeyEnv)
		}
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}
