package tooling

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/session"
)

func (r *OTRunner) runPointer(workspaceRoot string, record domain.RunRecord, inspection otInspection) (string, error) {
	pointer, err := session.ParseOTPointer(inspection.Prompt)
	if err != nil {
		return "", err
	}
	sessionID := strings.TrimSpace(pointer.SessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(record.SessionID)
	}
	if sessionID == "" {
		return "", fmt.Errorf("ot pointer requires an active session")
	}

	repoRoot, err := resolveSubagentRepoRoot(workspaceRoot)
	if err != nil {
		return "", err
	}
	path := filepath.Join(repoRoot, ".orch", "sessions", sessionID+".jsonl")
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open pointer session file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	lineSet := make(map[int64]struct{}, len(pointer.Lines))
	for _, line := range pointer.Lines {
		lineSet[line] = struct{}{}
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 4096), 4*1024*1024)
	var output []string
	var lineNo int64
	for scanner.Scan() {
		lineNo++
		if _, ok := lineSet[lineNo]; !ok {
			continue
		}
		output = append(output, fmt.Sprintf("%d:%s", lineNo, scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan pointer session file: %w", err)
	}
	if len(output) == 0 {
		return "", fmt.Errorf("ot pointer did not resolve any lines")
	}
	return strings.Join(output, "\n"), nil
}
