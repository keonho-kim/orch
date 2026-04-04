package prompts

const (
	chatHistoryUserPrompt      = "Summarize this user request for a rolling conversation digest in 1-3 sentences."
	chatHistoryAssistantPrompt = "Summarize this assistant response for a rolling conversation digest in 1-3 sentences."
)

func ChatHistoryUserPrompt() string {
	return chatHistoryUserPrompt
}

func ChatHistoryAssistantPrompt() string {
	return chatHistoryAssistantPrompt
}
