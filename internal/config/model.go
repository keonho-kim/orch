package config

import "github.com/keonho-kim/orch/domain"

type Scope string

const (
	ScopeGlobal    Scope = "global"
	ScopeProject   Scope = "project"
	ScopeEffective Scope = "effective"
	ScopeBuiltin   Scope = "builtin"

	// Legacy aliases kept for internal compatibility.
	ScopeManaged Scope = "managed"
	ScopeUser    Scope = "user"
	ScopeLocal   Scope = "local"
)

type SettingKey string

type SourceInfo struct {
	Scope Scope
	File  string
}

type SourceMap map[SettingKey]SourceInfo

type ProviderAuthPatch struct {
	Kind  *string `json:"kind,omitempty"`
	Value *string `json:"value,omitempty"`
	Env   *string `json:"env,omitempty"`
	File  *string `json:"file,omitempty"`
}

type ProviderSettingsPatch struct {
	BaseURL   *string            `json:"base_url,omitempty"`
	Model     *string            `json:"model,omitempty"`
	Auth      *ProviderAuthPatch `json:"auth,omitempty"`
	APIKeyEnv *string            `json:"api_key_env,omitempty"`
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

type InstallPatch struct {
	BinDir *string `json:"bin_dir,omitempty"`
}

type EnvLayersPatch struct {
	Global        map[string]string `json:"global,omitempty"`
	Gateway       map[string]string `json:"gateway,omitempty"`
	Worker        map[string]string `json:"worker,omitempty"`
	OT            map[string]string `json:"ot,omitempty"`
	GlobalDefined bool              `json:"-"`
	GatewayDefined bool             `json:"-"`
	WorkerDefined bool              `json:"-"`
	OTDefined      bool             `json:"-"`
}

type ScopeSettings struct {
	Version           *int                  `json:"version,omitempty"`
	DefaultProvider   *string               `json:"default_provider,omitempty"`
	ApprovalPolicy    *string               `json:"approval_policy,omitempty"`
	SelfDrivingMode   *bool                 `json:"self_driving_mode,omitempty"`
	ReactRalphIter    *int                  `json:"react_ralph_iter,omitempty"`
	PlanRalphIter     *int                  `json:"plan_ralph_iter,omitempty"`
	CompactThresholdK *int                  `json:"compact_threshold_k,omitempty"`
	Providers         *ProviderCatalogPatch `json:"providers,omitempty"`
	Install           *InstallPatch         `json:"install,omitempty"`
	Env               *EnvLayersPatch       `json:"env,omitempty"`
}

type EffectiveInstall struct {
	BinDir string
}

type EffectiveEnv struct {
	Global  map[string]string
	Gateway map[string]string
	Worker  map[string]string
	OT      map[string]string
}

type ResolvedSettings struct {
	Effective        domain.Settings
	EffectiveInstall EffectiveInstall
	EffectiveEnv     EffectiveEnv
	Sources          SourceMap
	Scopes           map[Scope]ScopeSettings
	Files            map[Scope]string
}

type rawConfigFile struct {
	Version  *int                          `toml:"version,omitempty"`
	Orch     *rawOrchConfig                `toml:"orch,omitempty"`
	Install  *InstallPatch                 `toml:"install,omitempty"`
	Provider map[string]*rawProviderConfig `toml:"provider,omitempty"`
	Env      *rawEnvConfig                 `toml:"env,omitempty"`
}

type rawOrchConfig struct {
	DefaultProvider   *string `toml:"default_provider,omitempty"`
	ApprovalPolicy    *string `toml:"approval_policy,omitempty"`
	SelfDrivingMode   *bool   `toml:"self_driving_mode,omitempty"`
	ReactRalphIter    *int    `toml:"react_ralph_iter,omitempty"`
	PlanRalphIter     *int    `toml:"plan_ralph_iter,omitempty"`
	CompactThresholdK *int    `toml:"compact_threshold_k,omitempty"`
}

type rawProviderConfig struct {
	BaseURL *string            `toml:"base_url,omitempty"`
	Model   *string            `toml:"model,omitempty"`
	Auth    *ProviderAuthPatch `toml:"auth,omitempty"`
}

type rawEnvConfig struct {
	Global  map[string]string `toml:"global,omitempty"`
	Gateway map[string]string `toml:"gateway,omitempty"`
	Worker  map[string]string `toml:"worker,omitempty"`
	OT      map[string]string `toml:"ot,omitempty"`
}

type legacyProviderSettingsPatch struct {
	BaseURL   *string `json:"base_url,omitempty"`
	Model     *string `json:"model,omitempty"`
	APIKeyEnv *string `json:"api_key_env,omitempty"`
}

type legacyProviderCatalogPatch struct {
	Ollama  *legacyProviderSettingsPatch `json:"ollama,omitempty"`
	VLLM    *legacyProviderSettingsPatch `json:"vllm,omitempty"`
	Gemini  *legacyProviderSettingsPatch `json:"gemini,omitempty"`
	Vertex  *legacyProviderSettingsPatch `json:"vertex,omitempty"`
	Bedrock *legacyProviderSettingsPatch `json:"bedrock,omitempty"`
	Claude  *legacyProviderSettingsPatch `json:"claude,omitempty"`
	Azure   *legacyProviderSettingsPatch `json:"azure,omitempty"`
	ChatGPT *legacyProviderSettingsPatch `json:"chatgpt,omitempty"`
}

type legacyScopeSettings struct {
	DefaultProvider   *string                     `json:"default_provider,omitempty"`
	Providers         *legacyProviderCatalogPatch `json:"providers,omitempty"`
	ApprovalPolicy    *string                     `json:"approval_policy,omitempty"`
	SelfDrivingMode   *bool                       `json:"self_driving_mode,omitempty"`
	ReactRalphIter    *int                        `json:"react_ralph_iter,omitempty"`
	PlanRalphIter     *int                        `json:"plan_ralph_iter,omitempty"`
	CompactThresholdK *int                        `json:"compact_threshold_k,omitempty"`
}

func (s ScopeSettings) IsEmpty() bool {
	return s.Version == nil &&
		s.DefaultProvider == nil &&
		s.ApprovalPolicy == nil &&
		s.SelfDrivingMode == nil &&
		s.ReactRalphIter == nil &&
		s.PlanRalphIter == nil &&
		s.CompactThresholdK == nil &&
		(s.Providers == nil || s.Providers.isEmpty()) &&
		(s.Install == nil || s.Install.isEmpty()) &&
		(s.Env == nil || s.Env.isEmpty())
}

func (p *ProviderAuthPatch) isEmpty() bool {
	return p == nil || (p.Kind == nil && p.Value == nil && p.Env == nil && p.File == nil)
}

func (p *ProviderSettingsPatch) isEmpty() bool {
	return p == nil || (p.BaseURL == nil && p.Model == nil && p.APIKeyEnv == nil && (p.Auth == nil || p.Auth.isEmpty()))
}

func (p *ProviderCatalogPatch) isEmpty() bool {
	return p == nil || (p.Ollama == nil && p.VLLM == nil && p.Gemini == nil && p.Vertex == nil && p.Bedrock == nil && p.Claude == nil && p.Azure == nil && p.ChatGPT == nil)
}

func (p *InstallPatch) isEmpty() bool {
	return p == nil || p.BinDir == nil
}

func (e *EnvLayersPatch) isEmpty() bool {
	return e == nil || (!e.GlobalDefined && !e.GatewayDefined && !e.WorkerDefined && !e.OTDefined && len(e.Global) == 0 && len(e.Gateway) == 0 && len(e.Worker) == 0 && len(e.OT) == 0)
}
