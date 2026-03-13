package userprefs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	startMarker = "<!-- orch:auto-preferences:start -->"
	endMarker   = "<!-- orch:auto-preferences:end -->"
)

func EnsureDetectedLanguage(path string, language string) (bool, error) {
	language = strings.TrimSpace(strings.ToLower(language))
	if language == "" {
		return false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read user preference file: %w", err)
	}
	content := string(data)

	block, found := managedBlock(content)
	if found && detectedLanguage(block) != "" {
		return false, nil
	}

	updated := setManagedBlock(content, language)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create user preference directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return false, fmt.Errorf("write user preference file: %w", err)
	}
	return true, nil
}

func managedBlock(content string) (string, bool) {
	start := strings.Index(content, startMarker)
	end := strings.Index(content, endMarker)
	if start == -1 || end == -1 || end < start {
		return "", false
	}
	start += len(startMarker)
	return strings.TrimSpace(content[start:end]), true
}

func detectedLanguage(block string) string {
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "detected_language") {
			continue
		}
		_, raw, found := strings.Cut(line, "=")
		if !found {
			return ""
		}
		value := strings.TrimSpace(raw)
		value = strings.Trim(value, `"`)
		value = strings.Trim(value, `'`)
		return strings.TrimSpace(strings.ToLower(value))
	}
	return ""
}

func setManagedBlock(content string, language string) string {
	block := fmt.Sprintf("%s\ndetected_language = %q\n%s", startMarker, language, endMarker)

	start := strings.Index(content, startMarker)
	end := strings.Index(content, endMarker)
	if start != -1 && end != -1 && end >= start {
		end += len(endMarker)
		return content[:start] + block + content[end:]
	}

	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return block + "\n"
	}
	return trimmed + "\n\n" + block + "\n"
}
