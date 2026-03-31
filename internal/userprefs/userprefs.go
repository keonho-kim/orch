package userprefs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	startMarker = "<!-- orch:managed-memory:start -->"
	endMarker   = "<!-- orch:managed-memory:end -->"
)

func UpsertManagedValue(path string, key string, value string) (bool, error) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read user memory file: %w", err)
	}
	content := string(data)

	block, _ := managedBlock(content)
	if managedValue(block, key) == value {
		return false, nil
	}

	updated := setManagedValue(content, key, value)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create user memory directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return false, fmt.Errorf("write user memory file: %w", err)
	}
	return true, nil
}

func ReadMemoryExcerpt(path string, maxManagedBytes int, maxUserBytes int) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read user memory file: %w", err)
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", nil
	}

	managed, found := managedBlock(content)
	user := removeManagedBlock(content)
	user = clipSuffix(strings.TrimSpace(user), maxUserBytes)
	if found {
		managed = clipSuffix(strings.TrimSpace(managed), maxManagedBytes)
	}

	sections := make([]string, 0, 2)
	if user != "" {
		sections = append(sections, "User memory:\n"+user)
	}
	if managed != "" {
		sections = append(sections, "Managed memory:\n"+managed)
	}
	return strings.Join(sections, "\n\n"), nil
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

func managedValue(block string, key string) string {
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, key) {
			continue
		}
		_, raw, found := strings.Cut(line, "=")
		if !found {
			return ""
		}
		value := strings.TrimSpace(raw)
		value = strings.Trim(value, `"`)
		value = strings.Trim(value, `'`)
		return strings.TrimSpace(value)
	}
	return ""
}

func setManagedValue(content string, key string, value string) string {
	block, found := managedBlock(content)
	lines := make([]string, 0)
	if found && strings.TrimSpace(block) != "" {
		for _, line := range strings.Split(block, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
				continue
			}
			lines = append(lines, trimmed)
		}
	}
	lines = append(lines, fmt.Sprintf("%s = %q", key, value))
	managed := startMarker + "\n" + strings.Join(lines, "\n") + "\n" + endMarker

	start := strings.Index(content, startMarker)
	end := strings.Index(content, endMarker)
	if start != -1 && end != -1 && end >= start {
		end += len(endMarker)
		return content[:start] + managed + content[end:]
	}

	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return managed + "\n"
	}
	return trimmed + "\n\n" + managed + "\n"
}

func removeManagedBlock(content string) string {
	start := strings.Index(content, startMarker)
	end := strings.Index(content, endMarker)
	if start == -1 || end == -1 || end < start {
		return content
	}
	end += len(endMarker)
	return strings.TrimSpace(content[:start] + content[end:])
}

func clipSuffix(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[len(value)-limit:])
}
