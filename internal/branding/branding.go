package branding

import "strings"

var Version = "dev"

const orchWordmarkASCII = `
             _______                   _____                    _____                    _____
            /::\    \                 /\    \                  /\    \                  /\    \
           /::::\    \               /::\    \                /::\    \                /::\____\
          /::::::\    \             /::::\    \              /::::\    \              /:::/    /
         /::::::::\    \           /::::::\    \            /::::::\    \            /:::/    /
        /:::/~~\:::\    \         /:::/\:::\    \          /:::/\:::\    \          /:::/    /
       /:::/    \:::\    \       /:::/__\:::\    \        /:::/  \:::\    \        /:::/____/
      /:::/    / \:::\    \     /::::\   \:::\    \      /:::/    \:::\    \      /::::\    \
     /:::/____/   \:::\____\   /::::::\   \:::\    \    /:::/    / \:::\    \    /::::::\    \   _____
    |:::|    |     |:::|    | /:::/\:::\   \:::\____\  /:::/    /   \:::\    \  /:::/\:::\    \ /\    \
    |:::|____|     |:::|    |/:::/  \:::\   \:::|    |/:::/____/     \:::\____\/:::/  \:::\    /::\____\
     \:::\    \   /:::/    / \::/   |::::\  /:::|____|\:::\    \      \::/    /\::/    \:::\  /:::/    /
      \:::\    \ /:::/    /   \/____|:::::\/:::/    /  \:::\    \      \/____/  \/____/ \:::\/:::/    /
       \:::\    /:::/    /          |:::::::::/    /    \:::\    \                       \::::::/    /
        \:::\__/:::/    /           |::|\::::/    /      \:::\    \                       \::::/    /
         \::::::::/    /            |::| \::/____/        \:::\    \                      /:::/    /
          \::::::/    /             |::|  ~|               \:::\    \                    /:::/    /
           \::::/    /              |::|   |                \:::\    \                  /:::/    /
            \::/____/               \::|   |                 \:::\____\                /:::/    /
             ~~                      \:|   |                  \::/    /                \::/    /
                                      \|___|                   \/____/                  \/____/
`

var Wordmark = NormalizeASCII(strings.Split(orchWordmarkASCII, "\n"))

func NormalizeASCII(lines []string) []string {
	trimmed := trimEmptyRows(lines)
	if len(trimmed) == 0 {
		return nil
	}

	indent := commonIndent(trimmed)
	normalized := make([]string, 0, len(trimmed))
	for _, line := range trimmed {
		line = strings.TrimRight(line, " ")
		if strings.TrimSpace(line) == "" {
			normalized = append(normalized, "")
			continue
		}
		if indent > 0 && len(line) >= indent {
			line = line[indent:]
		}
		normalized = append(normalized, line)
	}

	return normalized
}

func trimEmptyRows(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

func commonIndent(lines []string) int {
	indent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		width := leadingSpaces(line)
		if indent == -1 || width < indent {
			indent = width
		}
	}
	if indent < 0 {
		return 0
	}
	return indent
}

func leadingSpaces(line string) int {
	count := 0
	for _, ch := range line {
		if ch != ' ' {
			break
		}
		count++
	}
	return count
}
