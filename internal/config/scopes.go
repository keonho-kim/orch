package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

type Scope string

const (
	ScopeManaged   Scope = "managed"
	ScopeUser      Scope = "user"
	ScopeProject   Scope = "project"
	ScopeLocal     Scope = "local"
	ScopeEffective Scope = "effective"
	ScopeBuiltin   Scope = "builtin"
)

type SettingKey string

type SourceInfo struct {
	Scope Scope
	File  string
}

type SourceMap map[SettingKey]SourceInfo

type ProviderSettingsPatch struct {
	BaseURL   *string `json:"base_url,omitempty"`
	Model     *string `json:"model,omitempty"`
	APIKeyEnv *string `json:"api_key_env,omitempty"`
}

type ProviderCatalogPatch struct {
	Ollama  *ProviderSettingsPatch `json:"ollama,omitempty"`
	VLLM    *ProviderSettingsPatch `json:"vllm,omitempty"`
	Gemini  *ProviderSettingsPatch `json:"gemini,omitempty"`
	Vertex  *ProviderSettingsPatch `json:"vertex,omitempty"`
	Bedrock *ProviderSettingsPatch `json:"bedrock,omitempty"`
	Claude  *ProviderSettingsPatch `json:"claude,omitempty"`
	Azure   *ProviderSettingsPatch `json:"azure,omitempty"`
	ChatGPT *ProviderSettingsPatch `json:"chatgpt,omitempty"`
}

type ScopeSettings struct {
	DefaultProvider   *string               `json:"default_provider,omitempty"`
	Providers         *ProviderCatalogPatch `json:"providers,omitempty"`
	ApprovalPolicy    *string               `json:"approval_policy,omitempty"`
	SelfDrivingMode   *bool                 `json:"self_driving_mode,omitempty"`
	ReactRalphIter    *int                  `json:"react_ralph_iter,omitempty"`
	PlanRalphIter     *int                  `json:"plan_ralph_iter,omitempty"`
	CompactThresholdK *int                  `json:"compact_threshold_k,omitempty"`
}

type ResolvedSettings struct {
	Effective domain.Settings
	Sources   SourceMap
	Scopes    map[Scope]ScopeSettings
	Files     map[Scope]string
}

const (
	KeyDefaultProvider   SettingKey = "default_provider"
	KeyApprovalPolicy    SettingKey = "approval_policy"
	KeySelfDrivingMode   SettingKey = "self_driving_mode"
	KeyReactRalphIter    SettingKey = "react_ralph_iter"
	KeyPlanRalphIter     SettingKey = "plan_ralph_iter"
	KeyCompactThresholdK SettingKey = "compact_threshold_k"
)

func ParseScope(value string) (Scope, error) {
	switch Scope(strings.ToLower(strings.TrimSpace(value))) {
	case ScopeManaged:
		return ScopeManaged, nil
	case ScopeUser:
		return ScopeUser, nil
	case ScopeProject:
		return ScopeProject, nil
	case ScopeLocal:
		return ScopeLocal, nil
	case ScopeEffective, "":
		return ScopeEffective, nil
	default:
		return "", fmt.Errorf("unsupported config scope %q", value)
	}
}

func EditableScopes() []Scope {
	return []Scope{ScopeUser, ScopeProject, ScopeLocal}
}

func ManagedScopes() []Scope {
	return []Scope{ScopeManaged, ScopeUser, ScopeProject, ScopeLocal}
}

func AllSettingKeys() []SettingKey {
	keys := []SettingKey{KeyDefaultProvider}
	for _, provider := range domain.Providers() {
		keys = append(keys,
			ProviderBaseURLKey(provider),
			ProviderModelKey(provider),
		)
		if providerSupportsAPIKeyEnv(provider) {
			keys = append(keys, ProviderAPIKeyEnvKey(provider))
		}
	}
	keys = append(keys,
		KeyApprovalPolicy,
		KeySelfDrivingMode,
		KeyReactRalphIter,
		KeyPlanRalphIter,
		KeyCompactThresholdK,
	)
	return keys
}

func ProviderBaseURLKey(provider domain.Provider) SettingKey {
	return SettingKey(fmt.Sprintf("providers.%s.base_url", provider.String()))
}

func ProviderModelKey(provider domain.Provider) SettingKey {
	return SettingKey(fmt.Sprintf("providers.%s.model", provider.String()))
}

func ProviderAPIKeyEnvKey(provider domain.Provider) SettingKey {
	return SettingKey(fmt.Sprintf("providers.%s.api_key_env", provider.String()))
}

func ParseSettingKey(value string) (SettingKey, error) {
	key := SettingKey(strings.TrimSpace(value))
	for _, candidate := range AllSettingKeys() {
		if candidate == key {
			return key, nil
		}
	}
	return "", fmt.Errorf("unsupported setting key %q", value)
}

func providerSupportsAPIKeyEnv(provider domain.Provider) bool {
	switch provider {
	case domain.ProviderOllama:
		return false
	default:
		return true
	}
}

func LooksLikeRepoRoot(path string) bool {
	bootstrapPath := filepath.Join(path, runtimeAssetDirName, bootstrapDirName, "AGENTS.md")
	if !fileExists(bootstrapPath) {
		return false
	}
	markers := []string{
		filepath.Join(path, "go.mod"),
		filepath.Join(path, ".git"),
		filepath.Join(path, settingsFileName),
	}
	for _, marker := range markers {
		if fileExists(marker) || dirExists(marker) {
			return true
		}
	}
	return true
}

func LoadResolvedSettings(paths Paths) (ResolvedSettings, error) {
	scopeFiles := map[Scope]string{
		ScopeManaged: paths.ManagedSettingsFile,
		ScopeUser:    paths.UserSettingsFile,
		ScopeProject: paths.ProjectSettingsFile,
		ScopeLocal:   paths.LocalSettingsFile,
	}
	scopes := make(map[Scope]ScopeSettings, len(scopeFiles))
	for scope := range scopeFiles {
		settings, err := LoadScopeSettings(paths, scope)
		if err != nil {
			return ResolvedSettings{}, err
		}
		scopes[scope] = settings
	}

	return ResolveSettings(scopes, scopeFiles), nil
}

func ResolveSettings(scopes map[Scope]ScopeSettings, scopeFiles map[Scope]string) ResolvedSettings {
	effective := defaultSettings()
	sources := make(SourceMap)
	seedBuiltinSources(&effective, sources)
	for _, scope := range []Scope{ScopeUser, ScopeProject, ScopeLocal, ScopeManaged} {
		applyScopeSettings(&effective, sources, scope, scopes[scope], scopeFiles[scope])
	}
	effective.Normalize()

	return ResolvedSettings{
		Effective: effective,
		Sources:   sources,
		Scopes:    scopes,
		Files:     scopeFiles,
	}
}

func LoadScopeSettings(paths Paths, scope Scope) (ScopeSettings, error) {
	path, err := scopeFile(paths, scope)
	if err != nil {
		return ScopeSettings{}, err
	}
	if path == "" {
		return ScopeSettings{}, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ScopeSettings{}, nil
	}
	if err != nil {
		return ScopeSettings{}, fmt.Errorf("read %s settings: %w", scope, err)
	}

	var settings ScopeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return ScopeSettings{}, fmt.Errorf("parse %s settings: %w", scope, err)
	}
	if settings.DefaultProvider != nil && strings.TrimSpace(*settings.DefaultProvider) != "" {
		if _, err := domain.ParseProvider(*settings.DefaultProvider); err != nil {
			return ScopeSettings{}, fmt.Errorf("parse %s default provider: %w", scope, err)
		}
	}
	return settings, nil
}

func SaveScopeSettings(paths Paths, scope Scope, settings ScopeSettings) error {
	if scope == ScopeManaged || scope == ScopeEffective || scope == ScopeBuiltin {
		return fmt.Errorf("%s settings are read-only", scope)
	}

	path, err := scopeFile(paths, scope)
	if err != nil {
		return err
	}
	if settings.IsEmpty() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove empty %s settings: %w", scope, err)
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s settings directory: %w", scope, err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s settings: %w", scope, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s settings: %w", scope, err)
	}
	if scope == ScopeLocal {
		if err := ensureLocalSettingsIgnored(paths); err != nil {
			return err
		}
	}
	return nil
}

func UnsetScopeSettings(paths Paths, scope Scope, keys []SettingKey) error {
	settings, err := LoadScopeSettings(paths, scope)
	if err != nil {
		return err
	}
	for _, key := range keys {
		unsetScopeSetting(&settings, key)
	}
	return SaveScopeSettings(paths, scope, settings)
}

func ScopeSettingsFromDomainSettings(settings domain.Settings) ScopeSettings {
	settings.Normalize()
	result := ScopeSettings{
		DefaultProvider:   stringPtr(settings.DefaultProvider.String()),
		Providers:         &ProviderCatalogPatch{},
		ApprovalPolicy:    stringPtr(string(settings.ApprovalPolicy)),
		SelfDrivingMode:   boolPtr(settings.SelfDrivingMode),
		ReactRalphIter:    intPtr(settings.ReactRalphIter),
		PlanRalphIter:     intPtr(settings.PlanRalphIter),
		CompactThresholdK: intPtr(settings.CompactThresholdK),
	}
	for _, provider := range domain.Providers() {
		config := settings.ConfigFor(provider)
		patch := &ProviderSettingsPatch{
			BaseURL: stringPtr(config.BaseURL),
			Model:   stringPtr(config.Model),
		}
		if providerSupportsAPIKeyEnv(provider) {
			patch.APIKeyEnv = stringPtr(config.APIKeyEnv)
		}
		setProviderPatch(result.Providers, provider, patch)
	}
	return result
}

func (s ScopeSettings) IsEmpty() bool {
	return s.DefaultProvider == nil &&
		(s.Providers == nil || s.Providers.isEmpty()) &&
		s.ApprovalPolicy == nil &&
		s.SelfDrivingMode == nil &&
		s.ReactRalphIter == nil &&
		s.PlanRalphIter == nil &&
		s.CompactThresholdK == nil
}

func (s ScopeSettings) ValueForKey(key SettingKey) (string, bool) {
	switch key {
	case KeyDefaultProvider:
		return stringValue(s.DefaultProvider)
	case KeyApprovalPolicy:
		return stringValue(s.ApprovalPolicy)
	case KeySelfDrivingMode:
		return boolValue(s.SelfDrivingMode)
	case KeyReactRalphIter:
		return intValue(s.ReactRalphIter)
	case KeyPlanRalphIter:
		return intValue(s.PlanRalphIter)
	case KeyCompactThresholdK:
		return intValue(s.CompactThresholdK)
	default:
		provider, attr, ok := parseProviderSettingKey(key)
		if !ok || s.Providers == nil {
			return "", false
		}
		patch := providerPatch(s.Providers, provider)
		if patch == nil {
			return "", false
		}
		switch attr {
		case "base_url":
			return stringValue(patch.BaseURL)
		case "model":
			return stringValue(patch.Model)
		case "api_key_env":
			return stringValue(patch.APIKeyEnv)
		default:
			return "", false
		}
	}
}

func (s *ScopeSettings) SetKey(key SettingKey, value string) error {
	switch key {
	case KeyDefaultProvider:
		s.DefaultProvider = stringPtr(strings.TrimSpace(value))
		return nil
	case KeyApprovalPolicy:
		s.ApprovalPolicy = stringPtr(strings.TrimSpace(value))
		return nil
	case KeySelfDrivingMode:
		boolValue, err := parseBool(value)
		if err != nil {
			return err
		}
		s.SelfDrivingMode = boolPtr(boolValue)
		return nil
	case KeyReactRalphIter:
		intValue, err := parseInt(value)
		if err != nil {
			return err
		}
		s.ReactRalphIter = intPtr(intValue)
		return nil
	case KeyPlanRalphIter:
		intValue, err := parseInt(value)
		if err != nil {
			return err
		}
		s.PlanRalphIter = intPtr(intValue)
		return nil
	case KeyCompactThresholdK:
		intValue, err := parseInt(value)
		if err != nil {
			return err
		}
		s.CompactThresholdK = intPtr(intValue)
		return nil
	default:
		provider, attr, ok := parseProviderSettingKey(key)
		if !ok {
			return fmt.Errorf("unsupported setting key %q", key)
		}
		if s.Providers == nil {
			s.Providers = &ProviderCatalogPatch{}
		}
		patch := ensureProviderPatch(s.Providers, provider)
		switch attr {
		case "base_url":
			patch.BaseURL = stringPtr(strings.TrimSpace(value))
		case "model":
			patch.Model = stringPtr(strings.TrimSpace(value))
		case "api_key_env":
			patch.APIKeyEnv = stringPtr(strings.TrimSpace(value))
		default:
			return fmt.Errorf("unsupported setting key %q", key)
		}
		return nil
	}
}

func applyScopeSettings(settings *domain.Settings, sources SourceMap, scope Scope, patch ScopeSettings, file string) {
	if patch.DefaultProvider != nil && strings.TrimSpace(*patch.DefaultProvider) != "" {
		parsed, err := domain.ParseProvider(*patch.DefaultProvider)
		if err == nil {
			settings.DefaultProvider = parsed
			sources[KeyDefaultProvider] = SourceInfo{Scope: scope, File: file}
		}
	}
	if patch.Providers != nil {
		for _, provider := range domain.Providers() {
			providerPatch := providerPatch(patch.Providers, provider)
			if providerPatch == nil {
				continue
			}
			target := providerSettingsPtr(settings, provider)
			if target == nil {
				continue
			}
			if providerPatch.BaseURL != nil && strings.TrimSpace(*providerPatch.BaseURL) != "" {
				target.BaseURL = strings.TrimSpace(*providerPatch.BaseURL)
				sources[ProviderBaseURLKey(provider)] = SourceInfo{Scope: scope, File: file}
			}
			if providerPatch.Model != nil && strings.TrimSpace(*providerPatch.Model) != "" {
				target.Model = strings.TrimSpace(*providerPatch.Model)
				sources[ProviderModelKey(provider)] = SourceInfo{Scope: scope, File: file}
			}
			if providerPatch.APIKeyEnv != nil && strings.TrimSpace(*providerPatch.APIKeyEnv) != "" {
				target.APIKeyEnv = strings.TrimSpace(*providerPatch.APIKeyEnv)
				sources[ProviderAPIKeyEnvKey(provider)] = SourceInfo{Scope: scope, File: file}
			}
		}
	}
	if patch.ApprovalPolicy != nil && strings.TrimSpace(*patch.ApprovalPolicy) != "" {
		settings.ApprovalPolicy = domain.ApprovalPolicy(strings.TrimSpace(*patch.ApprovalPolicy))
		sources[KeyApprovalPolicy] = SourceInfo{Scope: scope, File: file}
	}
	if patch.SelfDrivingMode != nil {
		settings.SelfDrivingMode = *patch.SelfDrivingMode
		sources[KeySelfDrivingMode] = SourceInfo{Scope: scope, File: file}
	}
	if patch.ReactRalphIter != nil && *patch.ReactRalphIter > 0 {
		settings.ReactRalphIter = *patch.ReactRalphIter
		sources[KeyReactRalphIter] = SourceInfo{Scope: scope, File: file}
	}
	if patch.PlanRalphIter != nil && *patch.PlanRalphIter > 0 {
		settings.PlanRalphIter = *patch.PlanRalphIter
		sources[KeyPlanRalphIter] = SourceInfo{Scope: scope, File: file}
	}
	if patch.CompactThresholdK != nil && *patch.CompactThresholdK > 0 {
		settings.CompactThresholdK = *patch.CompactThresholdK
		sources[KeyCompactThresholdK] = SourceInfo{Scope: scope, File: file}
	}
}

func seedBuiltinSources(settings *domain.Settings, sources SourceMap) {
	for _, key := range AllSettingKeys() {
		if _, ok := effectiveValueForKey(*settings, key); !ok {
			continue
		}
		sources[key] = SourceInfo{Scope: ScopeBuiltin}
	}
}

func effectiveValueForKey(settings domain.Settings, key SettingKey) (string, bool) {
	switch key {
	case KeyDefaultProvider:
		return settings.DefaultProvider.String(), settings.DefaultProvider != ""
	case KeyApprovalPolicy:
		return string(settings.ApprovalPolicy), true
	case KeySelfDrivingMode:
		return fmt.Sprintf("%t", settings.SelfDrivingMode), true
	case KeyReactRalphIter:
		return fmt.Sprintf("%d", settings.ReactRalphIter), true
	case KeyPlanRalphIter:
		return fmt.Sprintf("%d", settings.PlanRalphIter), true
	case KeyCompactThresholdK:
		return fmt.Sprintf("%d", settings.CompactThresholdK), true
	default:
		provider, attr, ok := parseProviderSettingKey(key)
		if !ok {
			return "", false
		}
		config := settings.ConfigFor(provider)
		switch attr {
		case "base_url":
			return config.BaseURL, true
		case "model":
			return config.Model, true
		case "api_key_env":
			return config.APIKeyEnv, true
		default:
			return "", false
		}
	}
}

func parseProviderSettingKey(key SettingKey) (domain.Provider, string, bool) {
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

func scopeFile(paths Paths, scope Scope) (string, error) {
	switch scope {
	case ScopeManaged:
		return paths.ManagedSettingsFile, nil
	case ScopeUser:
		return paths.UserSettingsFile, nil
	case ScopeProject:
		return paths.ProjectSettingsFile, nil
	case ScopeLocal:
		return paths.LocalSettingsFile, nil
	case ScopeEffective, ScopeBuiltin:
		return "", fmt.Errorf("%s settings do not have a backing file", scope)
	default:
		return "", fmt.Errorf("unsupported config scope %q", scope)
	}
}

func managedSettingsPath() string {
	if override := strings.TrimSpace(os.Getenv("ORCH_MANAGED_SETTINGS")); override != "" {
		return override
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join("/Library/Application Support", appDirName, "managed-settings.json")
	case "windows":
		programData := strings.TrimSpace(os.Getenv("ProgramData"))
		if programData == "" {
			programData = filepath.Join(os.Getenv("SystemDrive")+"\\", "ProgramData")
		}
		return filepath.Join(programData, appDirName, "managed-settings.json")
	default:
		return filepath.Join("/etc", appDirName, "managed-settings.json")
	}
}

func ensureLocalSettingsIgnored(paths Paths) error {
	gitDir := filepath.Join(paths.RepoRoot, ".git")
	if !dirExists(gitDir) {
		return nil
	}
	excludePath := filepath.Join(gitDir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return fmt.Errorf("create git exclude directory: %w", err)
	}

	data, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read git exclude: %w", err)
	}
	entry := ".orch/settings.local.json"
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if slices.Contains(lines, entry) {
		return nil
	}
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		data = append(data, '\n')
	}
	data = append(data, []byte(entry+"\n")...)
	if err := os.WriteFile(excludePath, data, 0o644); err != nil {
		return fmt.Errorf("write git exclude: %w", err)
	}
	return nil
}

func unsetScopeSetting(settings *ScopeSettings, key SettingKey) {
	switch key {
	case KeyDefaultProvider:
		settings.DefaultProvider = nil
	case KeyApprovalPolicy:
		settings.ApprovalPolicy = nil
	case KeySelfDrivingMode:
		settings.SelfDrivingMode = nil
	case KeyReactRalphIter:
		settings.ReactRalphIter = nil
	case KeyPlanRalphIter:
		settings.PlanRalphIter = nil
	case KeyCompactThresholdK:
		settings.CompactThresholdK = nil
	default:
		provider, attr, ok := parseProviderSettingKey(key)
		if !ok || settings.Providers == nil {
			return
		}
		patch := providerPatch(settings.Providers, provider)
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
		if patch.isEmpty() {
			setProviderPatch(settings.Providers, provider, nil)
		}
		if settings.Providers.isEmpty() {
			settings.Providers = nil
		}
	}
}

func stringValue(value *string) (string, bool) {
	if value == nil {
		return "", false
	}
	return *value, true
}

func boolValue(value *bool) (string, bool) {
	if value == nil {
		return "", false
	}
	return fmt.Sprintf("%t", *value), true
}

func intValue(value *int) (string, bool) {
	if value == nil {
		return "", false
	}
	return fmt.Sprintf("%d", *value), true
}

func parseBool(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean %q", raw)
	}
}

func parseInt(raw string) (int, error) {
	var value int
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &value); err != nil {
		return 0, fmt.Errorf("invalid integer %q", raw)
	}
	return value, nil
}

func providerPatch(catalog *ProviderCatalogPatch, provider domain.Provider) *ProviderSettingsPatch {
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

func ensureProviderPatch(catalog *ProviderCatalogPatch, provider domain.Provider) *ProviderSettingsPatch {
	if patch := providerPatch(catalog, provider); patch != nil {
		return patch
	}
	patch := &ProviderSettingsPatch{}
	setProviderPatch(catalog, provider, patch)
	return patch
}

func setProviderPatch(catalog *ProviderCatalogPatch, provider domain.Provider, patch *ProviderSettingsPatch) {
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

func providerSettingsPtr(settings *domain.Settings, provider domain.Provider) *domain.ProviderSettings {
	switch provider {
	case domain.ProviderOllama:
		return &settings.Providers.Ollama
	case domain.ProviderVLLM:
		return &settings.Providers.VLLM
	case domain.ProviderGemini:
		return &settings.Providers.Gemini
	case domain.ProviderVertex:
		return &settings.Providers.Vertex
	case domain.ProviderBedrock:
		return &settings.Providers.Bedrock
	case domain.ProviderClaude:
		return &settings.Providers.Claude
	case domain.ProviderAzure:
		return &settings.Providers.Azure
	case domain.ProviderChatGPT:
		return &settings.Providers.ChatGPT
	default:
		return nil
	}
}

func (p *ProviderSettingsPatch) isEmpty() bool {
	return p == nil || (p.BaseURL == nil && p.Model == nil && p.APIKeyEnv == nil)
}

func (p *ProviderCatalogPatch) isEmpty() bool {
	return p == nil ||
		(p.Ollama == nil &&
			p.VLLM == nil &&
			p.Gemini == nil &&
			p.Vertex == nil &&
			p.Bedrock == nil &&
			p.Claude == nil &&
			p.Azure == nil &&
			p.ChatGPT == nil)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	return &value
}
