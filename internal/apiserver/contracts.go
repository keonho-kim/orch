package apiserver

import (
	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/orchestrator"
)

type serviceStatus interface {
	Status() orchestrator.Status
	RunSnapshot(string) (orchestrator.RunSnapshot, error)
	SubmitPromptMode(string, domain.RunMode) (string, error)
	ResolveApproval(string, bool) error
	SubscribeEvents() (<-chan orchestrator.ServiceEvent, func())
	EmitEvent(orchestrator.ServiceEvent)
	ListSessions(int) ([]domain.SessionMetadata, error)
	LatestSessionID() (string, error)
	RestoreSession(string) error
	SaveSettings(domain.Settings) error
	ConfigState() config.ConfigState
}
