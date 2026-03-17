package prompts

const (
	sessionTitlePrompt  = "Return a short session title in plain text. Keep it under 8 words."
	sessionTopicsPrompt = "Extract 3 to 5 major conversation topics in order of first appearance. Return one topic per line using this exact format: <topic> | lines=<comma-separated transcript line numbers>."
)

func SessionTitlePrompt() string {
	return sessionTitlePrompt
}

func SessionTopicsPrompt() string {
	return sessionTopicsPrompt
}

func SessionTopicSummaryPrompt(topic string, lines string) string {
	return "Summarize the transcript only for this topic in chronological order: " + topic + ". Focus on decisions, open questions, pending work, and risks. Use these transcript lines as the primary support: " + lines
}
