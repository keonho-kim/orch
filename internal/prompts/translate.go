package prompts

import (
	"fmt"
	"strings"
)

const detectLanguagePrompt = `
You are a language detection service.
Return exactly one token and nothing else.
Allowed outputs:
- kor
- en
- ch
Choose the dominant language of the user's request.
Do not add punctuation, markdown, code fences, JSON, or explanation.
`

const translatePromptTemplate = `
You are a translation service.
Translate the user input into %s.
Rules:
- return only the translated text
- preserve meaning exactly
- preserve code blocks, commands, flags, file paths, filenames, and quoted literals
- do not add explanations, notes, bullet points, or code fences
`

const multilingualRequestTemplate = `
The following three blocks describe the same user request.
Detected language: %s
Respond only in %s.
Use all three blocks as equivalent references.

[Original]
%s

[Korean]
%s

[English]
%s

[Chinese]
%s
`

func DetectLanguagePrompt() string {
	return strings.TrimSpace(detectLanguagePrompt)
}

func TranslatePrompt(targetLanguage string) string {
	return fmt.Sprintf(strings.TrimSpace(translatePromptTemplate), targetLanguage)
}

func MultilingualRequestPrompt(
	original string,
	korean string,
	english string,
	chinese string,
	detected string,
	responseLanguage string,
) string {
	return strings.TrimSpace(fmt.Sprintf(
		multilingualRequestTemplate,
		detected,
		responseLanguage,
		original,
		korean,
		english,
		chinese,
	))
}
