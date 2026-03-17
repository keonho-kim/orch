package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var skillMentionPattern = regexp.MustCompile(`\$([A-Za-z0-9][A-Za-z0-9_-]*)`)

func resolveSelectedSkills(workspaceRoot string, prompt string) ([]selectedSkill, error) {
	names := selectedSkillNames(prompt)
	if len(names) == 0 {
		return nil, nil
	}

	selected := make([]selectedSkill, 0, len(names))
	for _, name := range names {
		path := filepath.Join(workspaceRoot, "bootstrap", "skills", name, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("unknown skill $%s", name)
			}
			return nil, fmt.Errorf("read skill $%s: %w", name, err)
		}

		selected = append(selected, selectedSkill{
			Name:    name,
			Content: strings.TrimSpace(string(data)),
			Path:    filepath.ToSlash(filepath.Join("bootstrap", "skills", name, "SKILL.md")),
		})
	}
	return selected, nil
}

func selectedSkillNames(prompt string) []string {
	matches := skillMentionPattern.FindAllStringSubmatch(prompt, -1)
	if len(matches) == 0 {
		return nil
	}

	names := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}
