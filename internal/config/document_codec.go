package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/keonho-kim/orch/domain"
)

type ProviderDocument struct {
	Endpoint  string `toml:"endpoint"`
	Model     string `toml:"model"`
	APIKey    string `toml:"api_key"`
	Reasoning string `toml:"reasoning"`
}

type ProviderCatalogDocument struct {
	Ollama  ProviderDocument `toml:"ollama"`
	VLLM    ProviderDocument `toml:"vllm"`
	Gemini  ProviderDocument `toml:"gemini"`
	Vertex  ProviderDocument `toml:"vertex"`
	Bedrock ProviderDocument `toml:"bedrock"`
	Claude  ProviderDocument `toml:"claude"`
	Azure   ProviderDocument `toml:"azure"`
	ChatGPT ProviderDocument `toml:"chatgpt"`
}

type Document struct {
	Provider          string                  `toml:"provider"`
	ApprovalPolicy    string                  `toml:"approval_policy"`
	SelfDrivingMode   bool                    `toml:"self_driving_mode"`
	ReactRalphIter    int                     `toml:"react_ralph_iter"`
	PlanRalphIter     int                     `toml:"plan_ralph_iter"`
	CompactThresholdK int                     `toml:"compact_threshold_k"`
	Providers         ProviderCatalogDocument `toml:"providers"`
}

type ConfigState struct {
	Path     string
	Document Document
	Settings domain.Settings
}

func DefaultDocument() Document {
	settings := domain.Settings{}
	settings.Normalize()
	return DocumentFromSettings(settings)
}

func DocumentFromSettings(settings domain.Settings) Document {
	settings.Normalize()

	document := Document{
		Provider:          settings.DefaultProvider.String(),
		ApprovalPolicy:    string(settings.ApprovalPolicy),
		SelfDrivingMode:   settings.SelfDrivingMode,
		ReactRalphIter:    settings.ReactRalphIter,
		PlanRalphIter:     settings.PlanRalphIter,
		CompactThresholdK: settings.CompactThresholdK,
	}
	for _, provider := range domain.Providers() {
		source := settings.ConfigFor(provider)
		target := document.Providers.Provider(provider)
		target.Endpoint = source.Endpoint
		target.Model = source.Model
		target.APIKey = source.APIKey
		target.Reasoning = source.Reasoning
	}
	return document
}

func (d *Document) Normalize() {
	defaults := DefaultDocument()
	if strings.TrimSpace(d.ApprovalPolicy) == "" {
		d.ApprovalPolicy = defaults.ApprovalPolicy
	}
	if d.ReactRalphIter <= 0 {
		d.ReactRalphIter = defaults.ReactRalphIter
	}
	if d.PlanRalphIter <= 0 {
		d.PlanRalphIter = defaults.PlanRalphIter
	}
	if d.CompactThresholdK <= 0 {
		d.CompactThresholdK = defaults.CompactThresholdK
	}

	for _, provider := range domain.Providers() {
		target := d.Providers.Provider(provider)
		source := defaults.Providers.Provider(provider)
		if strings.TrimSpace(target.Endpoint) == "" {
			target.Endpoint = source.Endpoint
		}
		if normalized, err := domain.ParseReasoningValue(target.Reasoning); err == nil {
			target.Reasoning = normalized
		}
	}
}

func (d Document) ToSettings() (domain.Settings, error) {
	d.Normalize()

	settings := domain.Settings{
		ApprovalPolicy:    domain.ApprovalPolicy(strings.TrimSpace(d.ApprovalPolicy)),
		SelfDrivingMode:   d.SelfDrivingMode,
		ReactRalphIter:    d.ReactRalphIter,
		PlanRalphIter:     d.PlanRalphIter,
		CompactThresholdK: d.CompactThresholdK,
	}
	if strings.TrimSpace(d.Provider) != "" {
		provider, err := domain.ParseProvider(d.Provider)
		if err != nil {
			return domain.Settings{}, fmt.Errorf("parse provider: %w", err)
		}
		settings.DefaultProvider = provider
	}

	for _, provider := range domain.Providers() {
		source := d.Providers.Provider(provider)
		reasoning, err := domain.ParseReasoningValue(source.Reasoning)
		if err != nil {
			return domain.Settings{}, fmt.Errorf("%s reasoning: %w", provider.DisplayName(), err)
		}
		target := settings.Providers.Provider(provider)
		target.Endpoint = strings.TrimSpace(source.Endpoint)
		target.Model = strings.TrimSpace(source.Model)
		target.APIKey = strings.TrimSpace(source.APIKey)
		target.Reasoning = reasoning
	}

	settings.Normalize()
	return settings, nil
}

func LoadConfigState(paths Paths) (ConfigState, error) {
	document, err := LoadDocument(paths)
	if err != nil {
		return ConfigState{}, err
	}
	settings, err := document.ToSettings()
	if err != nil {
		return ConfigState{}, err
	}
	return ConfigState{
		Path:     paths.ConfigFile,
		Document: document,
		Settings: settings,
	}, nil
}

func LoadDocument(paths Paths) (Document, error) {
	data, err := os.ReadFile(paths.ConfigFile)
	if os.IsNotExist(err) {
		if legacy := ExistingLegacyConfigFiles(paths); len(legacy) > 0 {
			return Document{}, fmt.Errorf("legacy JSON settings are no longer supported; migrate them manually to %s before running orch: %s", paths.ConfigFile, strings.Join(legacy, ", "))
		}
		return DefaultDocument(), nil
	}
	if err != nil {
		return Document{}, fmt.Errorf("read config file: %w", err)
	}

	var document Document
	meta, err := toml.Decode(string(data), &document)
	if err != nil {
		return Document{}, fmt.Errorf("parse config file: %w", err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		parts := make([]string, 0, len(undecoded))
		for _, item := range undecoded {
			parts = append(parts, item.String())
		}
		return Document{}, fmt.Errorf("config file contains unsupported keys: %s", strings.Join(parts, ", "))
	}
	document.Normalize()
	return document, nil
}

func SaveDocument(paths Paths, document Document) error {
	document.Normalize()

	settings, err := document.ToSettings()
	if err != nil {
		return err
	}
	document = DocumentFromSettings(settings)

	data, err := MarshalDocument(document, false)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(paths.ConfigFile, data, 0o644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	if err := ensureConfigIgnored(paths); err != nil {
		return err
	}
	return nil
}

func MarshalDocument(document Document, redactSecrets bool) ([]byte, error) {
	if redactSecrets {
		for _, provider := range domain.Providers() {
			item := document.Providers.Provider(provider)
			item.APIKey = MaskSecret(item.APIKey)
		}
	}

	var buffer bytes.Buffer
	if err := toml.NewEncoder(&buffer).Encode(document); err != nil {
		return nil, fmt.Errorf("encode config file: %w", err)
	}
	return buffer.Bytes(), nil
}

func MaskSecret(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	if len(runes) <= 15 {
		return "***"
	}
	return string(runes[:10]) + "***" + string(runes[len(runes)-5:])
}

func (catalog *ProviderCatalogDocument) Provider(provider domain.Provider) *ProviderDocument {
	switch provider {
	case domain.ProviderOllama:
		return &catalog.Ollama
	case domain.ProviderVLLM:
		return &catalog.VLLM
	case domain.ProviderGemini:
		return &catalog.Gemini
	case domain.ProviderVertex:
		return &catalog.Vertex
	case domain.ProviderBedrock:
		return &catalog.Bedrock
	case domain.ProviderClaude:
		return &catalog.Claude
	case domain.ProviderAzure:
		return &catalog.Azure
	case domain.ProviderChatGPT:
		return &catalog.ChatGPT
	default:
		return &catalog.Ollama
	}
}
