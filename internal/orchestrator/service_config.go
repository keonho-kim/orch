package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
	"github.com/keonho-kim/orch/internal/config"
)

func (s *Service) NeedsSettingsConfiguration() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.settings.DefaultProvider == "" {
		return true
	}
	return !s.settings.IsProviderReady(s.settings.DefaultProvider)
}

func (s *Service) SaveSettings(settings domain.Settings) error {
	return s.SaveScopeSettings(config.ScopeProject, config.ScopeSettingsFromDomainSettings(settings))
}

func (s *Service) ConfigState() config.ResolvedSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.configState
}

func (s *Service) SaveScopeSettings(scope config.Scope, settings config.ScopeSettings) error {
	if err := s.migrateLegacyDefaultProvider(); err != nil {
		return err
	}
	if err := config.SaveScopeSettings(s.paths, scope, settings); err != nil {
		return err
	}
	return s.reloadResolvedSettings(fmt.Sprintf("Settings saved to %s scope.", scopeDisplayName(scope)))
}

func (s *Service) UnsetScopeSettings(scope config.Scope, keys []config.SettingKey) error {
	if err := s.migrateLegacyDefaultProvider(); err != nil {
		return err
	}
	if err := config.UnsetScopeSettings(s.paths, scope, keys); err != nil {
		return err
	}
	return s.reloadResolvedSettings(fmt.Sprintf("Settings updated in %s scope.", scopeDisplayName(scope)))
}

func (s *Service) reloadResolvedSettings(message string) error {
	resolvedSettings, err := config.LoadResolvedSettings(s.paths)
	if err != nil {
		return err
	}
	if resolvedSettings.Effective.DefaultProvider != "" {
		if err := s.store.SaveDefaultProvider(s.ctx, resolvedSettings.Effective.DefaultProvider); err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.settings = resolvedSettings.Effective
	s.configState = resolvedSettings
	s.mu.Unlock()

	s.publish(UIEvent{Type: "config_updated", Message: message})
	return nil
}

func (s *Service) migrateLegacyDefaultProvider() error {
	s.mu.RLock()
	resolved := s.configState
	s.mu.RUnlock()
	if _, ok := resolved.Sources[config.KeyDefaultProvider]; ok {
		return nil
	}

	stored, err := s.store.LoadSettings(s.ctx)
	if err != nil || stored.DefaultProvider == "" {
		return nil
	}

	userSettings, err := config.LoadScopeSettings(s.paths, config.ScopeUser)
	if err != nil {
		return err
	}
	provider := stored.DefaultProvider.String()
	userSettings.DefaultProvider = &provider
	return config.SaveScopeSettings(s.paths, config.ScopeUser, userSettings)
}

func (s *Service) DiscoverOllama(ctx context.Context, baseURL string) (string, []string, error) {
	models, normalized, err := adapters.ListOllamaModels(ctx, baseURL)
	if err != nil {
		return "", nil, err
	}
	return normalized, models, nil
}

func scopeDisplayName(scope config.Scope) string {
	value := string(scope)
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}
