package autotranslate

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"orch/domain"
	"orch/internal/adapters"
	"orch/internal/prompts"
)

const (
	LanguageKorean = "kor"
	LanguageEnglish = "en"
	LanguageChinese = "ch"
)

type PreparedPrompt struct {
	AgentPrompt      string
	DetectedLanguage string
	ResponseLanguage string
}

func Detect(
	ctx context.Context,
	client adapters.Client,
	settings domain.ProviderSettings,
	model string,
	rawPrompt string,
) (string, error) {
	result, err := client.Chat(ctx, settings, adapters.ChatRequest{
		Model: model,
		Messages: []adapters.Message{
			{
				Role: "system",
				Content: prompts.DetectLanguagePrompt(),
			},
			{Role: "user", Content: rawPrompt},
		},
	}, nil)
	if err != nil {
		return "", err
	}

	detected := strings.TrimSpace(strings.ToLower(result.Content))
	detected = strings.Trim(detected, "`\"' \n\t\r")
	switch detected {
	case LanguageKorean, LanguageEnglish, LanguageChinese:
		return detected, nil
	default:
		return "", fmt.Errorf("unsupported detected language %q", detected)
	}
}

func PrepareFromDetection(
	ctx context.Context,
	client adapters.Client,
	settings domain.ProviderSettings,
	model string,
	rawPrompt string,
	detected string,
) (PreparedPrompt, error) {
	translations := map[string]string{
		LanguageKorean:  "",
		LanguageEnglish: "",
		LanguageChinese: "",
	}
	translations[detected] = rawPrompt

	targets := map[string]string{}
	switch detected {
	case LanguageKorean:
		targets[LanguageEnglish] = "English"
		targets[LanguageChinese] = "Simplified Chinese"
	case LanguageEnglish:
		targets[LanguageKorean] = "Korean"
		targets[LanguageChinese] = "Simplified Chinese"
	case LanguageChinese:
		targets[LanguageKorean] = "Korean"
		targets[LanguageEnglish] = "English"
	default:
		return PreparedPrompt{}, fmt.Errorf("unsupported detected language %q", detected)
	}

	parallel, err := translatePair(ctx, client, settings, model, rawPrompt, targets)
	if err != nil {
		return PreparedPrompt{}, err
	}
	for key, value := range parallel {
		translations[key] = value
	}

	return PreparedPrompt{
		AgentPrompt: buildPrompt(
			rawPrompt,
			translations[LanguageKorean],
			translations[LanguageEnglish],
			translations[LanguageChinese],
			detected,
		),
		DetectedLanguage: detected,
		ResponseLanguage: detected,
	}, nil
}

func translatePair(
	ctx context.Context,
	client adapters.Client,
	settings domain.ProviderSettings,
	model string,
	source string,
	targets map[string]string,
) (map[string]string, error) {
	type item struct {
		key   string
		value string
		err   error
	}

	results := make(chan item, len(targets))
	var wg sync.WaitGroup

	for key, language := range targets {
		key := key
		language := language
		wg.Add(1)
		go func() {
			defer wg.Done()
			translated, err := translateText(ctx, client, settings, model, source, language)
			results <- item{key: key, value: translated, err: err}
		}()
	}

	wg.Wait()
	close(results)

	translated := make(map[string]string, len(targets))
	for result := range results {
		if result.err != nil {
			return nil, result.err
		}
		translated[result.key] = result.value
	}
	return translated, nil
}

func translateText(
	ctx context.Context,
	client adapters.Client,
	settings domain.ProviderSettings,
	model string,
	source string,
	targetLanguage string,
) (string, error) {
	result, err := client.Chat(ctx, settings, adapters.ChatRequest{
		Model: model,
		Messages: []adapters.Message{
			{
				Role: "system",
				Content: prompts.TranslatePrompt(targetLanguage),
			},
			{Role: "user", Content: source},
		},
	}, nil)
	if err != nil {
		return "", err
	}

	translated := strings.TrimSpace(result.Content)
	if translated == "" {
		return "", fmt.Errorf("translation to %s returned empty content", targetLanguage)
	}
	return translated, nil
}

func buildPrompt(
	original string,
	korean string,
	english string,
	chinese string,
	detected string,
) string {
	return prompts.MultilingualRequestPrompt(
		original,
		korean,
		english,
		chinese,
		detected,
		displayLanguage(detected),
	)
}

func displayLanguage(code string) string {
	switch code {
	case LanguageKorean:
		return "Korean"
	case LanguageEnglish:
		return "English"
	case LanguageChinese:
		return "Chinese"
	default:
		return code
	}
}
