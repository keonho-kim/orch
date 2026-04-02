package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
)

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleConfigGet(w, r)
	case http.MethodPatch:
		s.handleConfigPatch(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	scope, err := config.ParseScope(strings.TrimSpace(r.URL.Query().Get("scope")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if scope == "" {
		scope = config.ScopeEffective
	}
	showOrigin := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("show_origin")), "true")
	resolved := s.service.ConfigState()

	entries := make(map[string]string, len(config.AllSettingKeys()))
	origins := make(map[string]config.SourceInfo)
	switch scope {
	case config.ScopeEffective:
		for _, key := range config.AllSettingKeys() {
			value, _ := configValueForKey(resolved.Effective, key)
			entries[string(key)] = value
			if showOrigin {
				origins[string(key)] = resolved.Sources[key]
			}
		}
	default:
		scopeSettings := resolved.Scopes[scope]
		scopeFile := resolved.Files[scope]
		for _, key := range config.AllSettingKeys() {
			value, _ := scopeSettings.ValueForKey(key)
			entries[string(key)] = value
			if showOrigin {
				if _, ok := scopeSettings.ValueForKey(key); ok {
					origins[string(key)] = config.SourceInfo{Scope: scope, File: scopeFile}
				}
			}
		}
	}

	response := map[string]any{
		"scope":   scope,
		"entries": entries,
	}
	if showOrigin {
		response["origins"] = origins
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleConfigPatch(w http.ResponseWriter, r *http.Request) {
	scope, err := config.ParseScope(strings.TrimSpace(r.URL.Query().Get("scope")))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if scope == "" {
		scope = config.ScopeProject
	}
	if scope == config.ScopeManaged || scope == config.ScopeEffective || scope == config.ScopeBuiltin {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s scope is read-only", scope))
		return
	}

	var body struct {
		Set   map[string]string `json:"set"`
		Unset []string          `json:"unset"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	store, err := sqlitestore.Open(s.paths.DBPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer store.Close()

	resolved, err := config.LoadResolvedSettings(s.paths)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := migrateLegacyDefaultProvider(s.paths, store, resolved); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	scopeSettings, err := config.LoadScopeSettings(s.paths, scope)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	unsetKeys := make([]config.SettingKey, 0, len(body.Unset))
	for _, raw := range body.Unset {
		key, err := config.ParseSettingKey(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		unsetKeys = append(unsetKeys, key)
		configValueUnset(&scopeSettings, key)
	}

	for rawKey, value := range body.Set {
		key, err := config.ParseSettingKey(rawKey)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		for _, unsetKey := range unsetKeys {
			if unsetKey == key {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("unset conflicts with set for %s", key))
				return
			}
		}
		if err := scopeSettings.SetKey(key, value); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := s.service.SaveScopeSettings(scope, scopeSettings); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"scope": scope,
		"ok":    true,
	})
}

func jsonDecode(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}

func migrateLegacyDefaultProvider(paths config.Paths, store *sqlitestore.Store, resolved config.ResolvedSettings) error {
	if _, ok := resolved.Sources[config.KeyDefaultProvider]; ok {
		return nil
	}

	stored, err := store.LoadSettings(context.Background())
	if err != nil || stored.DefaultProvider == "" {
		return nil
	}

	userSettings, err := config.LoadScopeSettings(paths, config.ScopeUser)
	if err != nil {
		return err
	}
	value := stored.DefaultProvider.String()
	userSettings.DefaultProvider = &value
	return config.SaveScopeSettings(paths, config.ScopeUser, userSettings)
}

func configValueUnset(settings *config.ScopeSettings, key config.SettingKey) {
	switch key {
	case config.KeyDefaultProvider:
		settings.DefaultProvider = nil
	case config.KeyApprovalPolicy:
		settings.ApprovalPolicy = nil
	case config.KeySelfDrivingMode:
		settings.SelfDrivingMode = nil
	case config.KeyReactRalphIter:
		settings.ReactRalphIter = nil
	case config.KeyPlanRalphIter:
		settings.PlanRalphIter = nil
	case config.KeyCompactThresholdK:
		settings.CompactThresholdK = nil
	default:
		provider, attr, ok := parseProviderConfigKey(key)
		if !ok || settings.Providers == nil {
			return
		}
		patch := providerPatchPtr(settings.Providers, provider)
		if patch == nil {
			return
		}
		switch attr {
		case "base_url":
			patch.BaseURL = nil
		case "model":
			patch.Model = nil
		case "api_key_env":
			patch.APIKeyEnv = nil
		}
		if providerPatchEmpty(patch) {
			setProviderPatchPtr(settings.Providers, provider, nil)
		}
		if providerCatalogEmpty(settings.Providers) {
			settings.Providers = nil
		}
	}
}

func configValueForKey(settings domain.Settings, key config.SettingKey) (string, bool) {
	switch key {
	case config.KeyDefaultProvider:
		return settings.DefaultProvider.String(), true
	case config.KeyApprovalPolicy:
		return string(settings.ApprovalPolicy), true
	case config.KeySelfDrivingMode:
		return fmt.Sprintf("%t", settings.SelfDrivingMode), true
	case config.KeyReactRalphIter:
		return fmt.Sprintf("%d", settings.ReactRalphIter), true
	case config.KeyPlanRalphIter:
		return fmt.Sprintf("%d", settings.PlanRalphIter), true
	case config.KeyCompactThresholdK:
		return fmt.Sprintf("%d", settings.CompactThresholdK), true
	default:
		provider, attr, ok := parseProviderConfigKey(key)
		if !ok {
			return "", false
		}
		providerSettings := settings.ConfigFor(provider)
		switch attr {
		case "base_url":
			return providerSettings.BaseURL, true
		case "model":
			return providerSettings.Model, true
		case "api_key_env":
			return providerSettings.APIKeyEnv, true
		default:
			return "", false
		}
	}
}

func parseProviderConfigKey(key config.SettingKey) (domain.Provider, string, bool) {
	raw := strings.TrimPrefix(string(key), "providers.")
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return "", "", false
	}
	provider, err := domain.ParseProvider(parts[0])
	if err != nil {
		return "", "", false
	}
	return provider, parts[1], true
}

func providerPatchPtr(catalog *config.ProviderCatalogPatch, provider domain.Provider) *config.ProviderSettingsPatch {
	switch provider {
	case domain.ProviderOllama:
		return catalog.Ollama
	case domain.ProviderVLLM:
		return catalog.VLLM
	case domain.ProviderGemini:
		return catalog.Gemini
	case domain.ProviderVertex:
		return catalog.Vertex
	case domain.ProviderBedrock:
		return catalog.Bedrock
	case domain.ProviderClaude:
		return catalog.Claude
	case domain.ProviderAzure:
		return catalog.Azure
	case domain.ProviderChatGPT:
		return catalog.ChatGPT
	default:
		return nil
	}
}

func setProviderPatchPtr(catalog *config.ProviderCatalogPatch, provider domain.Provider, patch *config.ProviderSettingsPatch) {
	switch provider {
	case domain.ProviderOllama:
		catalog.Ollama = patch
	case domain.ProviderVLLM:
		catalog.VLLM = patch
	case domain.ProviderGemini:
		catalog.Gemini = patch
	case domain.ProviderVertex:
		catalog.Vertex = patch
	case domain.ProviderBedrock:
		catalog.Bedrock = patch
	case domain.ProviderClaude:
		catalog.Claude = patch
	case domain.ProviderAzure:
		catalog.Azure = patch
	case domain.ProviderChatGPT:
		catalog.ChatGPT = patch
	}
}

func providerPatchEmpty(patch *config.ProviderSettingsPatch) bool {
	return patch == nil || (patch.BaseURL == nil && patch.Model == nil && patch.APIKeyEnv == nil)
}

func providerCatalogEmpty(catalog *config.ProviderCatalogPatch) bool {
	return catalog == nil ||
		(catalog.Ollama == nil &&
			catalog.VLLM == nil &&
			catalog.Gemini == nil &&
			catalog.Vertex == nil &&
			catalog.Bedrock == nil &&
			catalog.Claude == nil &&
			catalog.Azure == nil &&
			catalog.ChatGPT == nil)
}
