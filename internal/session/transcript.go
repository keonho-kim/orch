package session

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

type CompactTopic struct {
	Title string
	Lines []int64
}

func BuildCompactSummary(records []domain.SessionRecord, throughSeq int64) string {
	lines := []string{"Compact summary"}
	for _, record := range records {
		if record.Seq > throughSeq {
			continue
		}
		switch record.Type {
		case domain.SessionRecordUser:
			lines = append(lines, "User: "+clipCompactLine(record.Content))
		case domain.SessionRecordAssistant:
			lines = append(lines, "Assistant: "+clipCompactLine(record.Content))
		case domain.SessionRecordTool:
			lines = append(lines, fmt.Sprintf("Tool %s: %s", record.ToolName, clipCompactLine(record.Content)))
		}
		lines = append(lines, "Pointer: "+FormatOTPointer([]int64{record.Seq}))
	}
	return strings.Join(lines, "\n")
}

func BuildTranscript(records []domain.SessionRecord, throughSeq int64) string {
	lines := make([]string, 0, len(records))
	for _, record := range records {
		if throughSeq > 0 && record.Seq > throughSeq {
			continue
		}
		switch record.Type {
		case domain.SessionRecordUser:
			lines = append(lines, "User: "+clipCompactLine(record.Content))
		case domain.SessionRecordAssistant:
			if strings.TrimSpace(record.Content) != "" {
				lines = append(lines, "Assistant: "+clipCompactLine(record.Content))
			}
		case domain.SessionRecordTool:
			lines = append(lines, fmt.Sprintf("Tool %s: %s", record.ToolName, clipCompactLine(record.Content)))
		}
	}
	return strings.Join(lines, "\n")
}

func BuildTranscriptWithPointers(records []domain.SessionRecord, throughSeq int64) string {
	lines := make([]string, 0, len(records))
	for _, record := range records {
		if throughSeq > 0 && record.Seq > throughSeq {
			continue
		}
		switch record.Type {
		case domain.SessionRecordUser:
			lines = append(lines, fmt.Sprintf("[line %d] User: %s", record.Seq, clipCompactLine(record.Content)))
		case domain.SessionRecordAssistant:
			if strings.TrimSpace(record.Content) != "" {
				lines = append(lines, fmt.Sprintf("[line %d] Assistant: %s", record.Seq, clipCompactLine(record.Content)))
			}
		case domain.SessionRecordTool:
			lines = append(lines, fmt.Sprintf("[line %d] Tool %s: %s", record.Seq, record.ToolName, clipCompactLine(record.Content)))
		}
	}
	return strings.Join(lines, "\n")
}

func ParseCompactTopics(raw string) []CompactTopic {
	lines := strings.Split(raw, "\n")
	topics := make([]CompactTopic, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "-"), "*"))
		if value == "" {
			continue
		}
		title, lineSpec, found := strings.Cut(value, "|")
		if !found {
			continue
		}
		lineSpec = strings.TrimSpace(lineSpec)
		if !strings.HasPrefix(strings.ToLower(lineSpec), "lines=") {
			continue
		}
		topicLines := parseCompactLines(strings.TrimSpace(strings.TrimPrefix(lineSpec, "lines=")))
		if len(topicLines) == 0 {
			continue
		}
		topics = append(topics, CompactTopic{
			Title: strings.TrimSpace(title),
			Lines: topicLines,
		})
	}
	if len(topics) > 5 {
		topics = topics[:5]
	}
	return topics
}

func JoinCompactLines(lines []int64) string {
	values := make([]string, 0, len(lines))
	for _, line := range normalizePointerLines(lines) {
		values = append(values, strconv.FormatInt(line, 10))
	}
	return strings.Join(values, ",")
}

func FallbackCompactTopics(records []domain.SessionRecord, throughSeq int64) []CompactTopic {
	topics := make([]CompactTopic, 0)
	for _, record := range records {
		if throughSeq > 0 && record.Seq > throughSeq {
			continue
		}
		if record.Type != domain.SessionRecordUser {
			continue
		}
		topics = append(topics, CompactTopic{
			Title: clipCompactLineLimit(record.Content, 72),
			Lines: []int64{record.Seq},
		})
		if len(topics) >= 5 {
			break
		}
	}
	return topics
}

func FallbackCompactTopicSummary(records []domain.SessionRecord, topic CompactTopic) string {
	if len(topic.Lines) == 0 {
		return topic.Title
	}
	lineSet := make(map[int64]struct{}, len(topic.Lines))
	for _, line := range topic.Lines {
		lineSet[line] = struct{}{}
	}

	parts := make([]string, 0, len(topic.Lines))
	for _, record := range records {
		if _, ok := lineSet[record.Seq]; !ok {
			continue
		}
		switch record.Type {
		case domain.SessionRecordUser:
			parts = append(parts, "User: "+clipCompactLine(record.Content))
		case domain.SessionRecordAssistant:
			parts = append(parts, "Assistant: "+clipCompactLine(record.Content))
		case domain.SessionRecordTool:
			parts = append(parts, fmt.Sprintf("Tool %s: %s", record.ToolName, clipCompactLine(record.Content)))
		}
	}
	if len(parts) == 0 {
		return topic.Title
	}
	return strings.Join(parts, " ")
}

func RenderPointerParagraph(summary string, lines []int64) string {
	summary = strings.TrimSpace(summary)
	pointer := FormatOTPointer(lines)
	if pointer == "" {
		return summary
	}
	return summary + "\nPointer: " + pointer
}

func clipCompactLine(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 240 {
		return value
	}
	return value[:240]
}

func clipCompactLineLimit(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

func parseCompactLines(raw string) []int64 {
	values := strings.Split(raw, ",")
	lines := make([]int64, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		line, err := strconv.ParseInt(value, 10, 64)
		if err != nil || line <= 0 {
			continue
		}
		lines = append(lines, line)
	}
	return normalizePointerLines(lines)
}
