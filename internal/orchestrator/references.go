package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const referenceIndexTTL = time.Second

var referenceMentionPattern = regexp.MustCompile(`(^|[\s(])([@#])([A-Za-z0-9._-]+)`)

type referenceResolver struct {
	mu    sync.RWMutex
	cache map[string]workspaceReferenceIndex
}

type workspaceReferenceIndex struct {
	root    string
	builtAt time.Time
	files   map[string][]referenceEntry
	dirs    map[string][]referenceEntry
	names   map[referenceKind][]string
}

type referenceKind string

const (
	referenceKindFile referenceKind = "file"
	referenceKindDir  referenceKind = "dir"
)

type referenceEntry struct {
	Name      string
	AbsPath   string
	RelPath   string
	PathDepth int
}

type referenceMention struct {
	Token string
	Name  string
	Kind  referenceKind
}

type scoredReference struct {
	referenceEntry
	score int
}

func newReferenceResolver() *referenceResolver {
	return &referenceResolver{
		cache: make(map[string]workspaceReferenceIndex),
	}
}

func (r *referenceResolver) Resolve(root string, cwd string, prompt string) (string, error) {
	mentions := parseReferenceMentions(prompt)
	if len(mentions) == 0 {
		return "", nil
	}

	index, err := r.index(root)
	if err != nil {
		return "", err
	}

	lines := make([]string, 0, len(mentions)+1)
	lines = append(lines, "Resolved workspace references:")
	for _, mention := range mentions {
		resolved := resolveReferenceMention(index, cwd, mention)
		lines = append(lines, resolved)
	}
	return strings.Join(lines, "\n"), nil
}

func (r *referenceResolver) index(root string) (workspaceReferenceIndex, error) {
	root = filepath.Clean(root)

	r.mu.RLock()
	cached, ok := r.cache[root]
	r.mu.RUnlock()
	if ok && time.Since(cached.builtAt) < referenceIndexTTL {
		return cached, nil
	}

	built, err := buildWorkspaceReferenceIndex(root)
	if err != nil {
		return workspaceReferenceIndex{}, err
	}

	r.mu.Lock()
	r.cache[root] = built
	r.mu.Unlock()
	return built, nil
}

func buildWorkspaceReferenceIndex(root string) (workspaceReferenceIndex, error) {
	index := workspaceReferenceIndex{
		root:    root,
		builtAt: time.Now(),
		files:   make(map[string][]referenceEntry),
		dirs:    make(map[string][]referenceEntry),
		names: map[referenceKind][]string{
			referenceKindFile: {},
			referenceKindDir:  {},
		},
	}

	fileNames := make(map[string]struct{})
	dirNames := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		name := entry.Name()
		item := referenceEntry{
			Name:      name,
			AbsPath:   path,
			RelPath:   filepath.ToSlash(rel),
			PathDepth: strings.Count(filepath.ToSlash(rel), "/") + 1,
		}

		if entry.IsDir() {
			index.dirs[name] = append(index.dirs[name], item)
			if _, ok := dirNames[name]; !ok {
				dirNames[name] = struct{}{}
				index.names[referenceKindDir] = append(index.names[referenceKindDir], name)
			}
			return nil
		}

		index.files[name] = append(index.files[name], item)
		if _, ok := fileNames[name]; !ok {
			fileNames[name] = struct{}{}
			index.names[referenceKindFile] = append(index.names[referenceKindFile], name)
		}
		return nil
	})
	if err != nil {
		return workspaceReferenceIndex{}, fmt.Errorf("build workspace reference index: %w", err)
	}

	sort.Strings(index.names[referenceKindFile])
	sort.Strings(index.names[referenceKindDir])
	return index, nil
}

func parseReferenceMentions(prompt string) []referenceMention {
	matches := referenceMentionPattern.FindAllStringSubmatch(prompt, -1)
	if len(matches) == 0 {
		return nil
	}

	mentions := make([]referenceMention, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		prefix := match[2]
		name := strings.TrimSpace(match[3])
		if name == "" {
			continue
		}
		kind := referenceKindFile
		if prefix == "#" {
			kind = referenceKindDir
		}
		key := prefix + name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		mentions = append(mentions, referenceMention{
			Token: key,
			Name:  name,
			Kind:  kind,
		})
	}
	return mentions
}

func resolveReferenceMention(index workspaceReferenceIndex, cwd string, mention referenceMention) string {
	candidates := rankReferenceCandidates(index, cwd, mention)
	if len(candidates) == 0 {
		return fmt.Sprintf("- %s not found in the current workspace.", mention.Token)
	}
	if len(candidates) > 1 && candidates[0].score == candidates[1].score {
		parts := make([]string, 0, min(3, len(candidates)))
		for _, candidate := range candidates[:min(3, len(candidates))] {
			parts = append(parts, formatResolvedReference(candidate.referenceEntry))
		}
		return fmt.Sprintf("- %s is ambiguous: %s", mention.Token, strings.Join(parts, "; "))
	}
	return fmt.Sprintf("- %s -> %s", mention.Token, formatResolvedReference(candidates[0].referenceEntry))
}

func rankReferenceCandidates(index workspaceReferenceIndex, cwd string, mention referenceMention) []scoredReference {
	nameMap := index.files
	names := index.names[referenceKindFile]
	if mention.Kind == referenceKindDir {
		nameMap = index.dirs
		names = index.names[referenceKindDir]
	}

	scored := make([]scoredReference, 0)
	query := mention.Name
	lowerQuery := strings.ToLower(query)
	addEntries := func(score int, name string) {
		for _, entry := range nameMap[name] {
			scored = append(scored, scoredReference{
				referenceEntry: entry,
				score:          score - entry.PathDepth - cwdDistance(cwd, entry.AbsPath),
			})
		}
	}

	if _, ok := nameMap[query]; ok {
		addEntries(4000, query)
	}
	for _, name := range names {
		if name == query {
			continue
		}
		lowerName := strings.ToLower(name)
		switch {
		case strings.HasPrefix(lowerName, lowerQuery):
			addEntries(3000, name)
		case strings.Contains(lowerName, lowerQuery):
			addEntries(2000, name)
		}
	}

	sort.SliceStable(scored, func(i int, j int) bool {
		if scored[i].score == scored[j].score {
			if scored[i].PathDepth == scored[j].PathDepth {
				return scored[i].RelPath < scored[j].RelPath
			}
			return scored[i].PathDepth < scored[j].PathDepth
		}
		return scored[i].score > scored[j].score
	})
	return scored
}

func cwdDistance(cwd string, path string) int {
	if strings.TrimSpace(cwd) == "" {
		return 0
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil {
		return 0
	}
	return strings.Count(filepath.ToSlash(rel), "/")
}

func formatResolvedReference(entry referenceEntry) string {
	return fmt.Sprintf("[%s](%s) at %s", entry.Name, entry.AbsPath, entry.AbsPath)
}
