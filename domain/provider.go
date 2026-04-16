package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Provider string

const (
	ProviderOllama  Provider = "ollama"
	ProviderVLLM    Provider = "vllm"
	ProviderGemini  Provider = "gemini"
	ProviderVertex  Provider = "vertex"
	ProviderBedrock Provider = "bedrock"
	ProviderClaude  Provider = "claude"
	ProviderAzure   Provider = "azure"
	ProviderChatGPT Provider = "chatgpt"
)

var providerOrder = []Provider{
	ProviderOllama,
	ProviderVLLM,
	ProviderGemini,
	ProviderVertex,
	ProviderBedrock,
	ProviderClaude,
	ProviderAzure,
	ProviderChatGPT,
}

func (p Provider) String() string {
	return string(p)
}

func (p Provider) DisplayName() string {
	switch p {
	case ProviderOllama:
		return "Ollama"
	case ProviderVLLM:
		return "vLLM"
	case ProviderGemini:
		return "Gemini"
	case ProviderVertex:
		return "Vertex"
	case ProviderBedrock:
		return "Bedrock"
	case ProviderClaude:
		return "Claude"
	case ProviderAzure:
		return "Azure"
	case ProviderChatGPT:
		return "ChatGPT"
	default:
		return strings.ToUpper(string(p))
	}
}

func ParseProvider(value string) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(ProviderOllama):
		return ProviderOllama, nil
	case string(ProviderVLLM):
		return ProviderVLLM, nil
	case string(ProviderGemini):
		return ProviderGemini, nil
	case string(ProviderVertex):
		return ProviderVertex, nil
	case string(ProviderBedrock):
		return ProviderBedrock, nil
	case string(ProviderClaude):
		return ProviderClaude, nil
	case string(ProviderAzure):
		return ProviderAzure, nil
	case string(ProviderChatGPT):
		return ProviderChatGPT, nil
	default:
		return "", fmt.Errorf("unsupported provider %q", value)
	}
}

func Providers() []Provider {
	return append([]Provider(nil), providerOrder...)
}

type ProviderSettings struct {
	BaseURL   string       `json:"base_url"`
	Model     string       `json:"model"`
	Auth      ProviderAuth `json:"auth,omitempty"`
	APIKeyEnv string       `json:"api_key_env,omitempty"`
}

type ProviderAuthKind string

const (
	ProviderAuthNone  ProviderAuthKind = "none"
	ProviderAuthValue ProviderAuthKind = "value"
	ProviderAuthEnv   ProviderAuthKind = "env"
	ProviderAuthFile  ProviderAuthKind = "file"
)

type ProviderAuth struct {
	Kind    ProviderAuthKind `json:"kind,omitempty"`
	Value   string           `json:"value,omitempty"`
	Env     string           `json:"env,omitempty"`
	File    string           `json:"file,omitempty"`
	BaseDir string           `json:"-"`
}

func (s ProviderSettings) NormalizedBaseURL() string {
	return strings.TrimRight(strings.TrimSpace(s.BaseURL), "/")
}

func (a ProviderAuth) Normalize(provider Provider) ProviderAuth {
	next := a
	switch provider {
	case ProviderOllama:
		if next.Kind == "" {
			next.Kind = ProviderAuthNone
		}
	case ProviderVLLM:
		if next.Kind == "" {
			next.Kind = ProviderAuthEnv
		}
		if next.Kind == ProviderAuthEnv && strings.TrimSpace(next.Env) == "" {
			next.Env = "VLLM_API_KEY"
		}
	case ProviderGemini:
		if next.Kind == "" {
			next.Kind = ProviderAuthEnv
		}
		if next.Kind == ProviderAuthEnv && strings.TrimSpace(next.Env) == "" {
			next.Env = "GEMINI_API_KEY"
		}
	case ProviderVertex:
		if next.Kind == "" {
			next.Kind = ProviderAuthEnv
		}
		if next.Kind == ProviderAuthEnv && strings.TrimSpace(next.Env) == "" {
			next.Env = "GOOGLE_API_KEY"
		}
	case ProviderBedrock:
		if next.Kind == "" {
			next.Kind = ProviderAuthEnv
		}
		if next.Kind == ProviderAuthEnv && strings.TrimSpace(next.Env) == "" {
			next.Env = "AWS_BEARER_TOKEN_BEDROCK"
		}
	case ProviderClaude:
		if next.Kind == "" {
			next.Kind = ProviderAuthEnv
		}
		if next.Kind == ProviderAuthEnv && strings.TrimSpace(next.Env) == "" {
			next.Env = "ANTHROPIC_API_KEY"
		}
	case ProviderAzure:
		if next.Kind == "" {
			next.Kind = ProviderAuthEnv
		}
		if next.Kind == ProviderAuthEnv && strings.TrimSpace(next.Env) == "" {
			next.Env = "AZURE_OPENAI_API_KEY"
		}
	case ProviderChatGPT:
		if next.Kind == "" {
			next.Kind = ProviderAuthEnv
		}
		if next.Kind == ProviderAuthEnv && strings.TrimSpace(next.Env) == "" {
			next.Env = "OPENAI_API_KEY"
		}
	}
	return next
}

func (a ProviderAuth) IsConfigured(required bool) bool {
	switch a.Kind {
	case ProviderAuthNone:
		return !required
	case ProviderAuthValue:
		return strings.TrimSpace(a.Value) != ""
	case ProviderAuthEnv:
		return strings.TrimSpace(a.Env) != ""
	case ProviderAuthFile:
		return strings.TrimSpace(a.File) != ""
	default:
		return !required
	}
}

func (a ProviderAuth) SecretEnvName() string {
	if a.Kind != ProviderAuthEnv {
		return ""
	}
	return strings.TrimSpace(a.Env)
}

func (a ProviderAuth) Resolve(provider Provider) (string, error) {
	switch a.Kind {
	case ProviderAuthNone:
		return "", nil
	case ProviderAuthValue:
		value := strings.TrimSpace(a.Value)
		if value == "" {
			return "", fmt.Errorf("%s auth value is empty", provider.DisplayName())
		}
		return value, nil
	case ProviderAuthEnv:
		name := strings.TrimSpace(a.Env)
		if name == "" {
			return "", fmt.Errorf("%s auth env is not configured", provider.DisplayName())
		}
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			return "", fmt.Errorf("%s auth env %s is not set", provider.DisplayName(), name)
		}
		return value, nil
	case ProviderAuthFile:
		path := strings.TrimSpace(a.File)
		if path == "" {
			return "", fmt.Errorf("%s auth file is not configured", provider.DisplayName())
		}
		if !filepath.IsAbs(path) && strings.TrimSpace(a.BaseDir) != "" {
			path = filepath.Join(a.BaseDir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read %s auth file: %w", provider.DisplayName(), err)
		}
		value := strings.TrimSpace(string(data))
		if value == "" {
			return "", fmt.Errorf("%s auth file %s is empty", provider.DisplayName(), path)
		}
		return value, nil
	default:
		return "", nil
	}
}

type ProviderCatalog struct {
	Ollama  ProviderSettings `json:"ollama"`
	VLLM    ProviderSettings `json:"vllm"`
	Gemini  ProviderSettings `json:"gemini"`
	Vertex  ProviderSettings `json:"vertex"`
	Bedrock ProviderSettings `json:"bedrock"`
	Claude  ProviderSettings `json:"claude"`
	Azure   ProviderSettings `json:"azure"`
	ChatGPT ProviderSettings `json:"chatgpt"`
}

func providerRequiresBaseURL(provider Provider) bool {
	switch provider {
	case ProviderBedrock, ProviderAzure:
		return true
	default:
		return false
	}
}

func providerRequiresModel(provider Provider) bool {
	switch provider {
	case ProviderOllama,
		ProviderVLLM,
		ProviderGemini,
		ProviderVertex,
		ProviderBedrock,
		ProviderClaude,
		ProviderAzure,
		ProviderChatGPT:
		return true
	default:
		return false
	}
}

func providerRequiresAuth(provider Provider) bool {
	switch provider {
	case ProviderGemini,
		ProviderVertex,
		ProviderBedrock,
		ProviderClaude,
		ProviderAzure,
		ProviderChatGPT:
		return true
	default:
		return false
	}
}
