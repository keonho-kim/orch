package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"orch/domain"
	"orch/internal/adapters"
	"orch/internal/prompts"
)

func (s *Service) executeRun(ctx context.Context, runID string) {
	record, ok := s.RunRecord(runID)
	if !ok {
		return
	}

	client, ok := s.clients[record.Provider]
	if !ok {
		_ = s.failRun(runID, fmt.Errorf("provider %s is not configured", record.Provider))
		return
	}

	snapshot := s.Snapshot()
	providerSettings := snapshot.Settings.ConfigFor(record.Provider)
	preparedPrompt, err := s.preprocessPrompt(ctx, runID, record, snapshot.Settings)
	if err != nil {
		_ = s.failRun(runID, err)
		return
	}
	messages := []adapters.Message{
		{Role: "user", Content: preparedPrompt},
	}

	limit := ralphLimit(snapshot.Settings, record.Mode)
	for iteration := 1; iteration <= limit; iteration++ {
		if err := s.setRunIteration(runID, iteration); err != nil {
			return
		}
		if err := s.updateRunTask(runID, fmt.Sprintf("Ralph %d/%d", iteration, limit)); err != nil {
			return
		}

		state, ok := s.state(runID)
		if !ok {
			return
		}

		contextMessage, err := buildIterationContext(state.record, snapshot.ActivePlan, strings.TrimSpace(state.draft))
		if err != nil {
			_ = s.failRun(runID, err)
			return
		}

		result, err := client.Chat(ctx, providerSettings, adapters.ChatRequest{
			Model: record.Model,
			Messages: append([]adapters.Message{
				{Role: "system", Content: prompts.SystemPrompt(record.Mode)},
				{Role: "system", Content: contextMessage},
			}, messages...),
			Tools: adapters.ToolCatalog(record.Mode),
		}, func(delta adapters.Delta) error {
			if delta.Reasoning != "" {
				if err := s.appendThinking(runID, delta.Reasoning); err != nil {
					return err
				}
			}
			if delta.Content != "" {
				if err := s.appendOutput(runID, delta.Content); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			if ctx.Err() != nil {
				_ = s.cancelRun(runID, ctx.Err())
				return
			}
			_ = s.failRun(runID, err)
			return
		}

		s.setRunDraft(runID, result.Content)
		messages = append(messages, adapters.ToAssistantMessage(result))
		if len(result.ToolCalls) == 0 {
			_ = s.completeRun(runID)
			return
		}

		for _, call := range result.ToolCalls {
			state, ok := s.state(runID)
			if !ok {
				return
			}

			if err := s.updateRunTask(runID, "Tool: "+call.Name); err != nil {
				return
			}

			review, err := s.tooling.Review(state.record.WorkspacePath, state.record, snapshot.Settings, call)
			if err != nil {
				_ = s.failRun(runID, err)
				return
			}
			if review.RequiresApproval {
				approved, err := s.awaitApproval(ctx, runID, call, review.Reason)
				if err != nil {
					_ = s.cancelRun(runID, err)
					return
				}
				if !approved {
					_ = s.cancelRun(runID, fmt.Errorf("tool execution denied"))
					return
				}
			}

			execution, err := s.tooling.Execute(ctx, state.record.WorkspacePath, state.record, state.env, call)
			if err != nil {
				_ = s.failRun(runID, err)
				return
			}
			if execution.NextCwd != "" {
				if err := s.setRunCwd(runID, execution.NextCwd); err != nil {
					_ = s.failRun(runID, err)
					return
				}
			}

			formatted := formatToolResult(call, execution.Output)
			if err := s.appendRunEvent(runID, "tool", formatted); err != nil {
				_ = s.failRun(runID, err)
				return
			}
			if err := s.appendOutput(runID, formatted+"\n"); err != nil {
				return
			}

			messages = append(messages, adapters.ToToolMessage(domain.ToolResult{
				ToolCallID: call.ID,
				Name:       call.Name,
				Content:    execution.Output,
			}))
			snapshot = s.Snapshot()
		}
	}

	_ = s.failRun(runID, fmt.Errorf("ralph iteration limit reached (%d)", limit))
}

func (s *Service) awaitApproval(ctx context.Context, runID string, call domain.ToolCall, reason string) (bool, error) {
	request := domain.ApprovalRequest{
		RunID:  runID,
		Call:   call,
		Reason: reason,
	}
	response := make(chan bool, 1)

	s.mu.Lock()
	state, ok := s.runs[runID]
	if !ok {
		s.mu.Unlock()
		return false, fmt.Errorf("run %s not found", runID)
	}
	state.pending = &approvalState{request: request, response: response}
	state.record.Status = domain.StatusAwaitingApproval
	state.record.CurrentTask = "Awaiting approval for " + call.Name
	record := state.record
	s.mu.Unlock()

	if err := s.persistRun(record); err != nil {
		return false, err
	}
	if err := s.appendRunEvent(runID, "approval", formatApprovalRequest(request)); err != nil {
		return false, err
	}
	s.publish(UIEvent{RunID: runID, Message: "Approval required."})

	select {
	case approved := <-response:
		return approved, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (s *Service) appendOutput(runID string, chunk string) error {
	s.mu.Lock()
	state, ok := s.runs[runID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("run %s not found", runID)
	}
	state.output += chunk
	state.record.FinalOutput = state.output
	state.record.UpdatedAt = time.Now()
	record := state.record
	s.mu.Unlock()

	if err := s.persistRun(record); err != nil {
		return err
	}
	s.publish(UIEvent{RunID: runID})
	return nil
}

func (s *Service) appendThinking(runID string, chunk string) error {
	s.mu.Lock()
	state, ok := s.runs[runID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("run %s not found", runID)
	}
	state.thinking += chunk
	state.record.UpdatedAt = time.Now()
	s.mu.Unlock()

	s.publish(UIEvent{RunID: runID})
	return nil
}

func (s *Service) appendRunEvent(runID string, kind string, message string) error {
	if s.store == nil {
		return nil
	}
	return s.store.AppendRunEvent(s.ctx, domain.RunEvent{
		RunID:     runID,
		Kind:      kind,
		Message:   message,
		CreatedAt: time.Now(),
	})
}

func (s *Service) updateRunTask(runID string, task string) error {
	s.mu.Lock()
	state, ok := s.runs[runID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("run %s not found", runID)
	}
	state.record.CurrentTask = domain.ClipTask(task, 72)
	state.record.Status = domain.StatusRunning
	state.record.UpdatedAt = time.Now()
	record := state.record
	s.mu.Unlock()

	if err := s.persistRun(record); err != nil {
		return err
	}
	s.publish(UIEvent{RunID: runID, Message: record.CurrentTask})
	return nil
}

func (s *Service) completeRun(runID string) error {
	s.mu.Lock()
	state, ok := s.runs[runID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("run %s not found", runID)
	}
	state.record.Status = domain.StatusCompleted
	state.record.CurrentTask = "Completed"
	state.record.UpdatedAt = time.Now()
	state.cancel = nil
	record := state.record
	draft := strings.TrimSpace(state.draft)
	output := strings.TrimSpace(state.output)
	s.mu.Unlock()

	if err := s.persistRun(record); err != nil {
		return err
	}
	if record.Mode == domain.RunModePlan && (draft != "" || output != "") {
		content := draft
		if content == "" {
			content = output
		}
		if err := s.saveActivePlan(domain.PlanCache{
			SourceRunID: record.RunID,
			Content:     content,
		}); err != nil {
			return err
		}
	}
	s.publish(UIEvent{RunID: runID, Message: "Run completed."})
	return nil
}

func (s *Service) failRun(runID string, err error) error {
	s.mu.Lock()
	state, ok := s.runs[runID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("run %s not found", runID)
	}
	state.record.Status = domain.StatusFailed
	state.record.CurrentTask = domain.ClipTask(err.Error(), 72)
	state.output = appendFinalNote(state.output, "ERROR: "+err.Error())
	state.record.FinalOutput = state.output
	state.record.UpdatedAt = time.Now()
	state.cancel = nil
	record := state.record
	s.mu.Unlock()

	if err := s.persistRun(record); err != nil {
		return err
	}
	_ = s.appendRunEvent(runID, "error", err.Error())
	s.publish(UIEvent{RunID: runID, Message: err.Error()})
	return nil
}

func (s *Service) cancelRun(runID string, err error) error {
	s.mu.Lock()
	state, ok := s.runs[runID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("run %s not found", runID)
	}
	state.record.Status = domain.StatusCancelled
	message := "Cancelled"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	state.record.CurrentTask = domain.ClipTask(message, 72)
	state.output = appendFinalNote(state.output, message)
	state.record.FinalOutput = state.output
	state.record.UpdatedAt = time.Now()
	state.cancel = nil
	state.pending = nil
	record := state.record
	s.mu.Unlock()

	if err := s.persistRun(record); err != nil {
		return err
	}
	_ = s.appendRunEvent(runID, "cancel", message)
	s.publish(UIEvent{RunID: runID, Message: message})
	return nil
}

func (s *Service) state(runID string) (*runState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.runs[runID]
	return state, ok
}

func buildIterationContext(record domain.RunRecord, activePlan domain.PlanCache, draftPlan string) (string, error) {
	product, err := readWorkspaceFile(record.WorkspacePath, "PRODUCT.md")
	if err != nil {
		return "", err
	}
	agents, err := readWorkspaceFile(record.WorkspacePath, "AGENTS.md")
	if err != nil {
		return "", err
	}
	user, err := readWorkspaceFile(record.WorkspacePath, filepath.Join("bootstrap", "USER.md"))
	if err != nil {
		return "", err
	}
	skills, err := readWorkspaceFile(record.WorkspacePath, filepath.Join("bootstrap", "SKILLS.md"))
	if err != nil {
		return "", err
	}

	return prompts.IterationContext(record, product, agents, user, skills, activePlan, draftPlan), nil
}

func readWorkspaceFile(workspaceRoot string, relativePath string) (string, error) {
	path := filepath.Join(workspaceRoot, relativePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", relativePath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func ralphLimit(settings domain.Settings, mode domain.RunMode) int {
	if mode == domain.RunModePlan {
		return settings.PlanRalphIter
	}
	return settings.ReactRalphIter
}

func formatToolResult(call domain.ToolCall, output string) string {
	if strings.TrimSpace(output) == "" {
		return fmt.Sprintf("[tool %s]\n(no output)", call.Name)
	}
	return fmt.Sprintf("[tool %s]\n%s", call.Name, output)
}

func formatApprovalRequest(request domain.ApprovalRequest) string {
	return fmt.Sprintf("%s: %s (%s)", request.Call.Name, request.Call.Arguments, request.Reason)
}
