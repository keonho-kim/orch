package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func MigrateLegacyJSON(paths Paths) error {
	globalExists := fileExists(paths.GlobalSettingsFile)
	projectExists := fileExists(paths.ProjectSettingsFile)
	if globalExists && (projectExists || strings.TrimSpace(paths.RepoRoot) == "") {
		return nil
	}

	globalLegacy, projectLegacy, effectiveLegacy, err := loadLegacySettings(paths)
	if err != nil {
		return err
	}

	if !globalExists && !globalLegacy.IsEmpty() {
		if err := SaveScopeSettings(paths, ScopeGlobal, globalLegacy); err != nil {
			return err
		}
		if err := archiveLegacyFile(paths.LegacyUserSettingsFile); err != nil {
			return err
		}
		if err := archiveLegacyFile(paths.LegacyManagedSettings); err != nil {
			return err
		}
	}

	if strings.TrimSpace(paths.RepoRoot) != "" && !projectExists {
		projectPatch := diffScopeSettings(effectiveLegacy, globalLegacy)
		if !projectLegacy.IsEmpty() {
			projectPatch = mergeScopeSettings(projectPatch, projectLegacy)
		}
		if !projectPatch.IsEmpty() {
			if err := SaveScopeSettings(paths, ScopeProject, projectPatch); err != nil {
				return err
			}
		}
		if err := archiveLegacyFile(paths.LegacyProjectSettings); err != nil {
			return err
		}
		if err := archiveLegacyFile(paths.LegacyLocalSettings); err != nil {
			return err
		}
	}

	return nil
}

func loadLegacySettings(paths Paths) (ScopeSettings, ScopeSettings, ScopeSettings, error) {
	global := ScopeSettings{}
	project := ScopeSettings{}
	for _, item := range []struct {
		path   string
		target *ScopeSettings
	}{
		{path: paths.LegacyManagedSettings, target: &global},
		{path: paths.LegacyUserSettingsFile, target: &global},
		{path: paths.LegacyProjectSettings, target: &project},
		{path: paths.LegacyLocalSettings, target: &project},
	} {
		legacy, err := loadLegacyScopeSettings(item.path)
		if err != nil {
			return ScopeSettings{}, ScopeSettings{}, ScopeSettings{}, err
		}
		if item.target == &global {
			global = mergeScopeSettings(global, legacy)
		} else {
			project = mergeScopeSettings(project, legacy)
		}
	}
	return global, project, mergeScopeSettings(global, project), nil
}

func loadLegacyScopeSettings(path string) (ScopeSettings, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return ScopeSettings{}, nil
	}
	if err != nil {
		return ScopeSettings{}, fmt.Errorf("read legacy settings %s: %w", path, err)
	}
	var legacy legacyScopeSettings
	if err := json.Unmarshal(data, &legacy); err != nil {
		return ScopeSettings{}, fmt.Errorf("parse legacy settings %s: %w", path, err)
	}
	return convertLegacyScopeSettings(legacy), nil
}

func convertLegacyScopeSettings(legacy legacyScopeSettings) ScopeSettings {
	result := ScopeSettings{
		Version:           intPtr(1),
		DefaultProvider:   legacy.DefaultProvider,
		ApprovalPolicy:    legacy.ApprovalPolicy,
		SelfDrivingMode:   legacy.SelfDrivingMode,
		ReactRalphIter:    legacy.ReactRalphIter,
		PlanRalphIter:     legacy.PlanRalphIter,
		CompactThresholdK: legacy.CompactThresholdK,
	}
	if legacy.Providers != nil {
		result.Providers = &ProviderCatalogPatch{}
		for _, provider := range domain.Providers() {
			legacyPatch := legacyProviderPatch(legacy.Providers, provider)
			if legacyPatch == nil {
				continue
			}
			patch := &ProviderSettingsPatch{
				BaseURL:   legacyPatch.BaseURL,
				Model:     legacyPatch.Model,
				APIKeyEnv: legacyPatch.APIKeyEnv,
			}
			if legacyPatch.APIKeyEnv != nil {
				patch.Auth = &ProviderAuthPatch{
					Kind: stringPtr(string(domain.ProviderAuthEnv)),
					Env:  legacyPatch.APIKeyEnv,
				}
			}
			setProviderPatch(result.Providers, provider, patch)
		}
		if result.Providers.isEmpty() {
			result.Providers = nil
		}
	}
	return result
}

func archiveLegacyFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("stat legacy file %s: %w", path, err)
	}
	backupPath := path + ".bak"
	if err := os.Rename(path, backupPath); err != nil {
		return fmt.Errorf("archive legacy file %s: %w", path, err)
	}
	return nil
}
