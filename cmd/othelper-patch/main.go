package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var hunkHeaderPattern = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func main() {
	if err := run(os.Args[1:], os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "patch: %v\n", err)
		os.Exit(1)
	}
}

type filePatch struct {
	OldPath string
	NewPath string
	Hunks   []hunk
}

type hunk struct {
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []hunkLine
}

type hunkLine struct {
	Kind byte
	Text string
}

func run(args []string, stdin io.Reader) error {
	if err := validateArgs(args); err != nil {
		return err
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("read patch: %w", err)
	}
	patches, err := parsePatch(string(data))
	if err != nil {
		return err
	}
	for _, patch := range patches {
		if err := applyFilePatch(patch); err != nil {
			return err
		}
	}
	return nil
}

func validateArgs(args []string) error {
	for _, arg := range args {
		switch strings.TrimSpace(arg) {
		case "-p0", "-u":
		default:
			return fmt.Errorf("unsupported arg %q", arg)
		}
	}
	return nil
}

func parsePatch(content string) ([]filePatch, error) {
	lines := splitLines(content)
	patches := make([]filePatch, 0)

	for index := 0; index < len(lines); {
		line := lines[index]
		if !strings.HasPrefix(line, "--- ") {
			index++
			continue
		}
		if index+1 >= len(lines) || !strings.HasPrefix(lines[index+1], "+++ ") {
			return nil, fmt.Errorf("expected +++ line after %q", strings.TrimSpace(line))
		}

		patch := filePatch{
			OldPath: parsePatchPath(lines[index], "--- "),
			NewPath: parsePatchPath(lines[index+1], "+++ "),
		}
		index += 2

		for index < len(lines) {
			if strings.HasPrefix(lines[index], "--- ") {
				break
			}
			if !strings.HasPrefix(lines[index], "@@ ") {
				index++
				continue
			}

			hunk, next, err := parseHunk(lines, index)
			if err != nil {
				return nil, err
			}
			patch.Hunks = append(patch.Hunks, hunk)
			index = next
		}

		if len(patch.Hunks) == 0 && patch.NewPath != "/dev/null" && patch.OldPath != "/dev/null" {
			return nil, fmt.Errorf("patch for %s has no hunks", patch.NewPath)
		}
		patches = append(patches, patch)
	}

	if len(patches) == 0 {
		return nil, fmt.Errorf("no patch content found")
	}
	return patches, nil
}

func parseHunk(lines []string, start int) (hunk, int, error) {
	header := strings.TrimSpace(lines[start])
	matches := hunkHeaderPattern.FindStringSubmatch(header)
	if len(matches) != 5 {
		return hunk{}, 0, fmt.Errorf("invalid hunk header %q", header)
	}

	parsed := hunk{
		OldStart: parsePositive(matches[1]),
		OldCount: parseCount(matches[2]),
		NewStart: parsePositive(matches[3]),
		NewCount: parseCount(matches[4]),
	}

	index := start + 1
	for index < len(lines) {
		line := lines[index]
		if strings.HasPrefix(line, "@@ ") || strings.HasPrefix(line, "--- ") {
			break
		}
		if line == "" {
			index++
			continue
		}
		prefix := line[0]
		switch prefix {
		case ' ', '+', '-':
			parsed.Lines = append(parsed.Lines, hunkLine{
				Kind: prefix,
				Text: line[1:],
			})
		case '\\':
			// Ignore "No newline at end of file" marker.
		default:
			return hunk{}, 0, fmt.Errorf("unsupported hunk line %q", strings.TrimSpace(line))
		}
		index++
	}

	return parsed, index, nil
}

func applyFilePatch(patch filePatch) error {
	switch {
	case patch.NewPath == "/dev/null":
		return deleteFile(patch)
	case patch.OldPath == "/dev/null":
		return createFile(patch)
	default:
		return updateFile(patch)
	}
}

func deleteFile(patch filePatch) error {
	path := patch.OldPath
	data, mode, err := readExistingFile(path)
	if err != nil {
		return err
	}
	lines := splitLines(string(data))
	if _, err := applyHunks(lines, patch.Hunks); err != nil {
		return fmt.Errorf("apply delete patch %s: %w", path, err)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	_ = mode
	return nil
}

func createFile(patch filePatch) error {
	lines, err := applyHunks(nil, patch.Hunks)
	if err != nil {
		return fmt.Errorf("apply create patch %s: %w", patch.NewPath, err)
	}
	return writePatchedFile(patch.NewPath, strings.Join(lines, ""), 0o644)
}

func updateFile(patch filePatch) error {
	data, mode, err := readExistingFile(patch.OldPath)
	if err != nil {
		return err
	}
	lines := splitLines(string(data))
	output, err := applyHunks(lines, patch.Hunks)
	if err != nil {
		return fmt.Errorf("apply patch %s: %w", patch.OldPath, err)
	}

	target := patch.NewPath
	if err := writePatchedFile(target, strings.Join(output, ""), mode); err != nil {
		return err
	}
	if target != patch.OldPath {
		if err := os.Remove(patch.OldPath); err != nil {
			return fmt.Errorf("remove renamed source %s: %w", patch.OldPath, err)
		}
	}
	return nil
}

func applyHunks(original []string, hunks []hunk) ([]string, error) {
	cursor := 0
	output := make([]string, 0, len(original))

	for _, h := range hunks {
		start := h.OldStart - 1
		if h.OldStart == 0 {
			start = 0
		}
		if start < cursor {
			return nil, fmt.Errorf("overlapping hunks at line %d", h.OldStart)
		}
		if start > len(original) {
			return nil, fmt.Errorf("hunk start %d exceeds file length %d", h.OldStart, len(original))
		}

		output = append(output, original[cursor:start]...)
		cursor = start

		for _, line := range h.Lines {
			switch line.Kind {
			case ' ':
				if cursor >= len(original) || original[cursor] != line.Text {
					return nil, fmt.Errorf("context mismatch at line %d", cursor+1)
				}
				output = append(output, original[cursor])
				cursor++
			case '-':
				if cursor >= len(original) || original[cursor] != line.Text {
					return nil, fmt.Errorf("delete mismatch at line %d", cursor+1)
				}
				cursor++
			case '+':
				output = append(output, line.Text)
			}
		}
	}

	output = append(output, original[cursor:]...)
	return output, nil
}

func writePatchedFile(path string, content string, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o644
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent for %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		return fmt.Errorf("write patched file %s: %w", path, err)
	}
	return nil
}

func readExistingFile(path string) ([]byte, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("stat %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}
	return data, info.Mode().Perm(), nil
}

func parsePatchPath(line string, prefix string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func splitLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func parsePositive(value string) int {
	count, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return count
}

func parseCount(value string) int {
	if strings.TrimSpace(value) == "" {
		return 1
	}
	return parsePositive(value)
}
