package orchestrator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func mergedRuntimeEnv(base []string, overlay map[string]string) []string {
	values := make(map[string]string, len(base)+len(overlay))
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		values[key] = value
	}
	for key, value := range overlay {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		if strings.TrimSpace(value) == "" {
			delete(values, trimmedKey)
			continue
		}
		values[trimmedKey] = value
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func (s *Service) toolBaseEnv(role domain.AgentRole) []string {
	s.mu.RLock()
	effectiveEnv := s.configState.EffectiveEnv
	s.mu.RUnlock()

	env := mergedRuntimeEnv(os.Environ(), effectiveEnv.Global)
	switch role {
	case domain.AgentRoleWorker:
		env = mergedRuntimeEnv(env, effectiveEnv.Worker)
	default:
		env = mergedRuntimeEnv(env, effectiveEnv.Gateway)
	}
	env = mergedRuntimeEnv(env, map[string]string{"ORCH_OT_TOOLS_DIR": filepath.Dir(s.paths.OTToolsAssets)})
	return env
}

func (s *Service) otExtraEnv() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	next := make(map[string]string, len(s.configState.EffectiveEnv.OT))
	for key, value := range s.configState.EffectiveEnv.OT {
		next[key] = value
	}
	return next
}
