package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/knowledge"
)

const (
	hookRunCompleted     = "run_completed"
	hookTaskCompleted    = "task_completed"
	hookSessionCompacted = "session_compacted"
	hookSessionFinalized = "session_finalized"
	hookApprovalResolved = "approval_resolved"
)

type HookEvent struct {
	Type           string
	SessionID      string
	RunID          string
	RunRecord      domain.RunRecord
	SessionMeta    domain.SessionMetadata
	Outcome        domain.TaskOutcome
	SessionSummary string
}

type HookHandler func(context.Context, HookEvent) error

type HookBus struct {
	mu       sync.RWMutex
	handlers map[string][]HookHandler
}

func NewHookBus() *HookBus {
	return &HookBus{handlers: make(map[string][]HookHandler)}
}

func (b *HookBus) Register(eventType string, handler HookHandler) {
	if b == nil || handler == nil {
		return
	}
	b.mu.Lock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
	b.mu.Unlock()
}

func (b *HookBus) Emit(ctx context.Context, event HookEvent) error {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	handlers := append([]HookHandler(nil), b.handlers[event.Type]...)
	b.mu.RUnlock()

	var joined error
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			joined = errors.Join(joined, err)
		}
	}
	return joined
}

func (s *Service) registerHooks() {
	if s.hooks == nil || s.knowledge == nil {
		return
	}

	s.hooks.Register(hookTaskCompleted, func(ctx context.Context, event HookEvent) error {
		if event.Outcome.Status != domain.TaskStatusCompleted {
			return nil
		}
		return s.knowledge.LearnFromTask(ctx, knowledge.LearningInput{
			WorkspacePath:  event.SessionMeta.WorkspacePath,
			Prompt:         event.RunRecord.Prompt,
			SessionMeta:    event.SessionMeta,
			Outcome:        event.Outcome,
			SessionSummary: event.SessionSummary,
		})
	})
}

func (s *Service) emitHookEvent(event HookEvent) {
	if s.hooks == nil {
		return
	}
	if err := s.hooks.Emit(s.ctx, event); err != nil && strings.TrimSpace(event.RunID) != "" {
		_ = s.appendRunEvent(event.RunID, "hook_error", fmt.Sprintf("%s: %v", event.Type, err))
	}
}
