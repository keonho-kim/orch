package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func fitLine(line string, width int) string {
	if width <= 0 {
		return ""
	}

	line = ansi.Truncate(line, width, "")
	padding := width - lipgloss.Width(line)
	if padding > 0 {
		line += strings.Repeat(" ", padding)
	}

	return line
}

func wrapLine(line string, width int) []string {
	if width <= 0 {
		return nil
	}
	if line == "" {
		return []string{""}
	}

	wrapped := ansi.Wrap(line, width, "")
	return strings.Split(wrapped, "\n")
}

func renderViewport(lines []string, width int, height int) string {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	normalized := make([]string, 0, height)
	for _, line := range lines {
		if len(normalized) == height {
			break
		}
		normalized = append(normalized, fitLine(line, width))
	}

	for len(normalized) < height {
		normalized = append(normalized, strings.Repeat(" ", width))
	}

	return strings.Join(normalized, "\n")
}
