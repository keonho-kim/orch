package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	boldTextStyle      = lipgloss.NewStyle().Bold(true)
	italicTextStyle    = lipgloss.NewStyle().Italic(true)
	underlineTextStyle = lipgloss.NewStyle().Underline(true)
)

type markdownTokenStyle int

const (
	stylePlain markdownTokenStyle = iota
	styleBold
	styleItalic
	styleUnderline
)

func renderInlineMarkdown(line string) string {
	var out strings.Builder
	var buf strings.Builder
	style := stylePlain

	flush := func() {
		text := buf.String()
		buf.Reset()
		if text == "" {
			return
		}
		switch style {
		case styleBold:
			out.WriteString(boldTextStyle.Render(text))
		case styleItalic:
			out.WriteString(italicTextStyle.Render(text))
		case styleUnderline:
			out.WriteString(underlineTextStyle.Render(text))
		default:
			out.WriteString(text)
		}
	}

	for index := 0; index < len(line); {
		switch {
		case strings.HasPrefix(line[index:], "**") && (style == styleBold || strings.Contains(line[index+2:], "**")):
			flush()
			style = toggleMarkdownStyle(style, styleBold)
			index += 2
		case strings.HasPrefix(line[index:], "__") && (style == styleUnderline || strings.Contains(line[index+2:], "__")):
			flush()
			style = toggleMarkdownStyle(style, styleUnderline)
			index += 2
		case strings.HasPrefix(line[index:], "*") && (style == styleItalic || strings.Contains(line[index+1:], "*")):
			flush()
			style = toggleMarkdownStyle(style, styleItalic)
			index++
		case strings.HasPrefix(line[index:], "_") && (style == styleItalic || strings.Contains(line[index+1:], "_")):
			flush()
			style = toggleMarkdownStyle(style, styleItalic)
			index++
		default:
			buf.WriteByte(line[index])
			index++
		}
	}

	flush()
	return out.String()
}

func toggleMarkdownStyle(current markdownTokenStyle, next markdownTokenStyle) markdownTokenStyle {
	if current == next {
		return stylePlain
	}
	if current == stylePlain {
		return next
	}
	return current
}
