package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/branding"
)

func (m Model) renderChatTimeline(width int) string {
	sections := []string{m.renderChatHeader()}

	runs := orderedRuns(m.snapshot.Runs)
	if len(runs) == 0 {
		sections = append(sections, subtleStyle.Render("No runs yet. Start with a request below."))
		return strings.Join(sections, "\n\n")
	}

	for index, run := range runs {
		section := m.renderRunSection(run, width, index > 0)
		if strings.TrimSpace(section) == "" {
			continue
		}
		sections = append(sections, section)
	}
	if len(sections) == 1 {
		return strings.Join(sections, "\n")
	}
	return strings.Join(sections, "\n\n")
}

func (m Model) renderChatHeader() string {
	lines := make([]string, 0, len(branding.Wordmark)+1)
	for _, line := range branding.Wordmark {
		lines = append(lines, titleStyle.Render(line))
	}
	lines = append(lines, subtleStyle.Render("Version")+" "+branding.Version)
	return strings.Join(lines, "\n")
}

func (m Model) renderRunSection(run domain.RunRecord, width int, withSeparator bool) string {
	thinkingBlock := ""
	if run.RunID == m.snapshot.CurrentRunID {
		thinkingBlock = m.renderThinkingBlock(width)
	}
	output := m.runOutput(run)

	parts := make([]string, 0, 5)
	if withSeparator {
		parts = append(parts, renderRunSeparator(width))
	}
	parts = append(parts, subtleStyle.Render(m.runMeta(run)))
	parts = append(parts, renderMessageBlock("USER", run.Prompt, width, userLabelStyle, userBoxStyle, renderWrappedLines))
	if thinkingBlock != "" {
		parts = append(parts, thinkingBlock)
	}
	parts = append(parts, renderMessageBlock("ORCH", output, width, assistantLabelStyle, assistantBoxStyle, renderStyledLines))
	return strings.Join(parts, "\n")
}

func (m Model) renderThinkingBlock(width int) string {
	thinking := strings.TrimSpace(m.snapshot.CurrentThinking)
	if thinking == "" {
		return ""
	}
	if !m.showThinking {
		return "\n" + thinkingInlineStyle.Render("THINKING ...") + "\n"
	}

	frameWidth := thinkBoxStyle.GetHorizontalFrameSize()
	contentWidth := max(1, width-frameWidth-2)
	lines := []string{thinkLabelStyle.Render("THINK"), ""}
	for _, line := range renderWrappedLines(thinking, contentWidth, "THINKING ...") {
		lines = append(lines, thinkContentStyle.Render(line))
	}
	return thinkBoxStyle.Width(max(1, width-frameWidth)).Render(strings.Join(lines, "\n"))
}

func (m Model) runMeta(run domain.RunRecord) string {
	mode := ""
	if run.Mode == domain.RunModePlan {
		mode = " | PLAN"
	}
	return fmt.Sprintf("%s%s | %s", run.RunID, mode, run.Status)
}

func (m Model) runOutput(run domain.RunRecord) string {
	if run.RunID == m.snapshot.CurrentRunID {
		return m.snapshot.CurrentOutput
	}
	return run.FinalOutput
}

func renderMessageBlock(
	label string,
	body string,
	width int,
	labelStyle lipgloss.Style,
	boxStyle lipgloss.Style,
	render func(string, int, string) []string,
) string {
	frameWidth := boxStyle.GetHorizontalFrameSize()
	contentWidth := max(1, width-frameWidth-2)
	lines := render(body, contentWidth, "No output yet.")
	content := strings.Join(lines, "\n")
	box := boxStyle.Width(max(1, width-frameWidth)).Render(content)
	return labelStyle.Render(label) + "\n" + box
}

func renderRunSeparator(width int) string {
	separatorWidth := max(12, width/4)
	separator := strings.Repeat("-", separatorWidth)
	padding := max(0, (width-separatorWidth)/2)
	return strings.Repeat(" ", padding) + subtleStyle.Render(separator)
}

func orderedRuns(runs []domain.RunRecord) []domain.RunRecord {
	ordered := make([]domain.RunRecord, 0, len(runs))
	for index := len(runs) - 1; index >= 0; index-- {
		ordered = append(ordered, runs[index])
	}
	return ordered
}
