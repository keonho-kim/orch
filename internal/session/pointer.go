package session

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const otPointerScheme = "ot-pointer://current"

type OTPointer struct {
	Lines []int64
}

func FormatOTPointer(lines []int64) string {
	normalized := normalizePointerLines(lines)
	if len(normalized) == 0 {
		return ""
	}

	values := make([]string, 0, len(normalized))
	for _, line := range normalized {
		values = append(values, strconv.FormatInt(line, 10))
	}

	return otPointerScheme + "?lines=" + url.QueryEscape(strings.Join(values, ","))
}

func ParseOTPointer(value string) (OTPointer, error) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, otPointerScheme) {
		return OTPointer{}, fmt.Errorf("invalid ot pointer")
	}

	queryPart := strings.TrimPrefix(trimmed, otPointerScheme)
	if strings.HasPrefix(queryPart, "/") {
		return OTPointer{}, fmt.Errorf("cross-session ot pointers are not supported")
	}
	queryPart = strings.TrimPrefix(queryPart, "?")
	_, _, found := strings.Cut(trimmed, "?")
	if !found {
		return OTPointer{}, fmt.Errorf("ot pointer query is required")
	}

	values, err := url.ParseQuery(queryPart)
	if err != nil {
		return OTPointer{}, fmt.Errorf("decode ot pointer query: %w", err)
	}
	rawLines := strings.TrimSpace(values.Get("lines"))
	if rawLines == "" {
		return OTPointer{}, fmt.Errorf("ot pointer lines are required")
	}

	lines := make([]int64, 0)
	for _, token := range strings.Split(rawLines, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		line, err := strconv.ParseInt(token, 10, 64)
		if err != nil || line <= 0 {
			return OTPointer{}, fmt.Errorf("invalid ot pointer line %q", token)
		}
		lines = append(lines, line)
	}

	lines = normalizePointerLines(lines)
	if len(lines) == 0 {
		return OTPointer{}, fmt.Errorf("ot pointer lines are required")
	}
	return OTPointer{Lines: lines}, nil
}

func normalizePointerLines(lines []int64) []int64 {
	if len(lines) == 0 {
		return nil
	}

	seen := make(map[int64]struct{}, len(lines))
	normalized := make([]int64, 0, len(lines))
	for _, line := range lines {
		if line <= 0 {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		normalized = append(normalized, line)
	}
	sort.Slice(normalized, func(i int, j int) bool { return normalized[i] < normalized[j] })
	return normalized
}
