package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"

	"orch/domain"
	"orch/internal/autotranslate"
	"orch/internal/userprefs"
)

func (s *Service) preprocessPrompt(
	ctx context.Context,
	runID string,
	record domain.RunRecord,
	settings domain.Settings,
) (string, error) {
	if !settings.AutoTranslate {
		return record.Prompt, nil
	}

	client, ok := s.clients[record.Provider]
	if !ok {
		return record.Prompt, fmt.Errorf("provider %s is not configured", record.Provider)
	}

	if err := s.updateRunTask(runID, "AutoTranslate: detecting language"); err != nil {
		return "", err
	}
	detectedLanguage, err := autotranslate.Detect(ctx, client, settings.ConfigFor(record.Provider), record.Model, record.Prompt)
	if err != nil {
		return s.autoTranslateFallback(runID, record.Prompt, fmt.Errorf("detect language: %w", err))
	}

	_ = s.appendRunEvent(runID, "autotranslate", fmt.Sprintf(
		"Detected language: %s.",
		detectedLanguage,
	))
	if err := s.persistDetectedLanguage(runID, record.WorkspacePath, detectedLanguage); err != nil {
		_ = s.appendRunEvent(runID, "autotranslate", fmt.Sprintf("Could not store detected language preference: %v", err))
	}

	if err := s.updateRunTask(runID, autoTranslateTask(detectedLanguage)); err != nil {
		return "", err
	}
	prepared, err := autotranslate.PrepareFromDetection(ctx, client, settings.ConfigFor(record.Provider), record.Model, record.Prompt, detectedLanguage)
	if err != nil {
		return s.autoTranslateFallback(runID, record.Prompt, fmt.Errorf("translate prompt: %w", err))
	}

	_ = s.appendRunEvent(runID, "autotranslate", fmt.Sprintf(
		"Prepared multilingual prompt. Respond in %s.",
		prepared.ResponseLanguage,
	))
	return prepared.AgentPrompt, nil
}

func (s *Service) autoTranslateFallback(runID string, rawPrompt string, cause error) (string, error) {
	_ = s.appendRunEvent(runID, "autotranslate", fmt.Sprintf(
		"AutoTranslate failed. Falling back to the original prompt: %v",
		cause,
	))
	if err := s.updateRunTask(runID, "AutoTranslate failed; using original prompt"); err != nil {
		return "", err
	}
	return rawPrompt, nil
}

func (s *Service) persistDetectedLanguage(runID string, workspacePath string, language string) error {
	userPath := filepath.Join(workspacePath, "bootstrap", "USER.md")
	written, err := userprefs.EnsureDetectedLanguage(userPath, language)
	if err != nil {
		return err
	}
	if written {
		return s.appendRunEvent(runID, "autotranslate", fmt.Sprintf(
			"Stored detected language preference in bootstrap/USER.md: %s.",
			language,
		))
	}
	return nil
}

func autoTranslateTask(detectedLanguage string) string {
	switch detectedLanguage {
	case autotranslate.LanguageKorean:
		return "AutoTranslate: translating to English and Chinese"
	case autotranslate.LanguageEnglish:
		return "AutoTranslate: translating to Korean and Chinese"
	case autotranslate.LanguageChinese:
		return "AutoTranslate: translating to Korean and English"
	default:
		return "AutoTranslate: translating to the other two languages"
	}
}
