package workspace

import "strings"

func sanitizeEnv(baseEnv []string, allowedSecretEnv []string) []string {
	allowedPrefixes := []string{
		"GIT_",
		"GO",
		"HTTP_",
		"HTTPS_",
		"NO_PROXY",
		"OLLAMA_",
		"VLLM_",
	}
	allowedKeys := map[string]struct{}{
		"HOME":                {},
		"LANG":                {},
		"LC_ALL":              {},
		"LC_CTYPE":            {},
		"ORCH_SUBAGENT_DEPTH": {},
		"PATH":                {},
		"PWD":                 {},
		"SHELL":               {},
		"TERM":                {},
		"TMPDIR":              {},
		"USER":                {},
		"USERNAME":            {},
	}
	for _, key := range allowedSecretEnv {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		allowedKeys[key] = struct{}{}
	}

	filtered := make([]string, 0, len(baseEnv))
	for _, entry := range baseEnv {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			continue
		}

		if _, ok := allowedKeys[key]; ok {
			filtered = append(filtered, key+"="+value)
			continue
		}

		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(key, prefix) {
				filtered = append(filtered, key+"="+value)
				break
			}
		}
	}

	return filtered
}
