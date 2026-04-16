package session

import (
	"context"
	"sync"

	"github.com/keonho-kim/orch/domain"
)

type MaintenanceRunner interface {
	Run(ctx context.Context, provider domain.Provider, model string, systemPrompt string, userPrompt string) (string, error)
}

type Mirror interface {
	UpsertSession(ctx context.Context, meta domain.SessionMetadata) error
	AppendSessionMessage(ctx context.Context, record domain.SessionRecord) error
}

type Context struct {
	Summary string
	Records []domain.SessionRecord
}

type compactTopic struct {
	Title string
	Lines []int64
}

type Service struct {
	manager       *Manager
	runner        MaintenanceRunner
	mirror        Mirror
	maintenanceMu sync.Mutex
}

func NewService(manager *Manager, runner MaintenanceRunner) *Service {
	return &Service{
		manager: manager,
		runner:  runner,
	}
}

func (s *Service) SetMirror(mirror Mirror) {
	s.mirror = mirror
}
