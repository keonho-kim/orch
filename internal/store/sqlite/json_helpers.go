package sqlite

import (
	"encoding/json"
	"fmt"
	"strings"
)

func encodeStringSlice(values []string) string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	data, _ := json.Marshal(normalized)
	return string(data)
}

func decodeStringSlice(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, fmt.Errorf("decode string slice: %w", err)
	}
	return values, nil
}

func encodeJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func ftsPhraseQuery(query string) string {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return ""
	}
	return `"` + strings.ReplaceAll(trimmed, `"`, `""`) + `"`
}
