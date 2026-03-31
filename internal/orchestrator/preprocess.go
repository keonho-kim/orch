package orchestrator

import (
	"context"

	"github.com/keonho-kim/orch/domain"
)

func (s *Service) preprocessPrompt(
	_ context.Context,
	_ string,
	record domain.RunRecord,
	_ domain.Settings,
) (string, error) {
	return record.Prompt, nil
}
