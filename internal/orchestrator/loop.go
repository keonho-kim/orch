package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
	"github.com/keonho-kim/orch/internal/knowledge"
	"github.com/keonho-kim/orch/internal/prompts"
	"github.com/keonho-kim/orch/internal/session"
	"github.com/keonho-kim/orch/internal/tooling"
	"github.com/keonho-kim/orch/internal/userprefs"
)

type iterationInputs struct {
	record             domain.RunRecord
	toolsDoc           string
	userMemory         string
	chatHistory        string
	selectedSkills     []selectedSkill
	resolvedReferences string
	memorySnapshot     domain.MemorySnapshot
	activePlan         domain.PlanCache
	draftPlan          string
	meta               domain.SessionMetadata
	currentContext     session.Context
	inheritedContext   session.Context
}

type runExecution struct {
	record           domain.RunRecord
	client           adapters.Client
	snapshot         Snapshot
	providerSettings domain.ProviderSettings
	messages         []adapters.Message
	limit            int
}

func (s *Service) executeRun(ctx context.Context, runID string) {
	execution, ok, err := s.prepareRunExecution(runID)
	if err != nil {
		_ = s.failRun(runID, err)
		return
	}
	if !ok {
		return
	}

<<<<<<< HEAD
	for iteration := 1; iteration <= execution.limit; iteration++ {
		nextExecution, terminated := s.runIteration(ctx, runID, execution, iteration)
=======
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

		inputs, err := s.loadIterationInputs(state.record, state.selectedSkills, state.resolvedRefs, state.memorySnapshot, snapshot.ActivePlan, strings.TrimSpace(state.draft))
		if err != nil {
			_ = s.failRun(runID, err)
			return
		}
		contextMessage := inputs.prompt()
		if err := s.appendSessionContextSnapshot(runID, inputs.snapshot()); err != nil {
			_ = s.failRun(runID, err)
			return
		}
		systemPrompt, err := s.buildSystemPrompt(state.record)
		if err != nil {
			_ = s.failRun(runID, err)
			return
		}

		result, err := client.Chat(ctx, providerSettings, adapters.ChatRequest{
			Model: record.Model,
			Messages: append([]adapters.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "system", Content: contextMessage},
			}, messages...),
			Tools: adapters.ToolCatalog(record.Mode, record.AgentRole),
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
		if err := s.appendSessionAssistant(runID, result); err != nil {
			_ = s.failRun(runID, err)
			return
		}
		messages = append(messages, adapters.ToAssistantMessage(result))
		if len(result.ToolCalls) == 0 {
			_ = s.completeRun(runID)
			return
		}
		terminated, err := s.executeToolCalls(ctx, runID, snapshot.Settings, result.ToolCalls, &messages)
		if err != nil {
			_ = s.failRun(runID, err)
			return
		}
>>>>>>> cef7a8c (update)
		if terminated {
			return
		}
		execution = nextExecution
	}

	_ = s.failRun(runID, fmt.Errorf("ralph iteration limit reached (%d)", execution.limit))
}

func (s *Service) runIteration(ctx context.Context, runID string, execution runExecution, iteration int) (runExecution, bool) {
	if err := s.beginIteration(runID, iteration, execution.limit); err != nil {
		return execution, true
	}
	state, ok := s.state(runID)
	if !ok {
		return execution, true
	}
	result, ok := s.requestIterationResult(ctx, runID, execution, state)
	if !ok {
		return execution, true
	}
	return s.finishIteration(ctx, runID, execution, result)
}

func (s *Service) beginIteration(runID string, iteration int, limit int) error {
	if err := s.setRunIteration(runID, iteration); err != nil {
		return err
	}
	return s.updateRunTask(runID, fmt.Sprintf("Ralph %d/%d", iteration, limit))
}

func (s *Service) requestIterationResult(ctx context.Context, runID string, execution runExecution, state *runState) (adapters.ChatResult, bool) {
	inputs, err := s.loadIterationInputs(state.record, state.selectedSkills, state.resolvedRefs, execution.snapshot.ActivePlan, strings.TrimSpace(state.draft))
	if err != nil {
		_ = s.failRun(runID, err)
		return adapters.ChatResult{}, false
	}
	contextMessage := inputs.prompt()
	if err := s.appendSessionContextSnapshot(runID, inputs.snapshot()); err != nil {
		_ = s.failRun(runID, err)
		return adapters.ChatResult{}, false
	}
	result, err := s.executeIterationChat(ctx, runID, execution, contextMessage)
	if err != nil {
		if ctx.Err() != nil {
			_ = s.cancelRun(runID, ctx.Err())
			return adapters.ChatResult{}, false
		}
		_ = s.failRun(runID, err)
		return adapters.ChatResult{}, false
	}
	return result, true
}

func (s *Service) finishIteration(ctx context.Context, runID string, execution runExecution, result adapters.ChatResult) (runExecution, bool) {
	s.setRunDraft(runID, result.Content)
	if err := s.appendSessionAssistant(runID, result); err != nil {
		_ = s.failRun(runID, err)
		return execution, true
	}
	execution.messages = append(execution.messages, adapters.ToAssistantMessage(result))
	if len(result.ToolCalls) == 0 {
		_ = s.completeRun(runID)
		return execution, true
	}
	terminated, err := s.executeToolCalls(ctx, runID, execution.snapshot.Settings, result.ToolCalls, &execution.messages)
	if err != nil {
		_ = s.failRun(runID, err)
		return execution, true
	}
	if terminated {
		return execution, true
	}
	execution.snapshot = s.Snapshot()
	return execution, false
}

func (s *Service) executeToolCalls(
	ctx context.Context,
	runID string,
	settings domain.Settings,
	calls []domain.ToolCall,
	messages *[]adapters.Message,
) (bool, error) {
	if s.agentRole == domain.AgentRoleGateway && allDelegateCalls(s.tooling, calls) {
		return s.executeDelegateBatch(ctx, runID, calls, messages)
	}

	for _, call := range calls {
		terminated, err := s.executeSingleToolCall(ctx, runID, settings, call, messages)
		if err != nil || terminated {
			return terminated, err
		}
	}
	return false, nil
}

func (s *Service) executeSingleToolCall(
	ctx context.Context,
	runID string,
	settings domain.Settings,
	call domain.ToolCall,
	messages *[]adapters.Message,
) (bool, error) {
	state, ok := s.state(runID)
	if !ok {
		return false, nil
	}

	if err := s.updateRunTask(runID, "Tool: "+call.Name); err != nil {
		return false, err
	}

	review, err := s.tooling.Review(state.record.WorkspacePath, state.record, state.env, settings, call)
	if err != nil {
		return false, err
	}
	if review.RequiresApproval {
		approved, err := s.awaitApproval(ctx, runID, call, review.Reason)
		if err != nil {
			return false, err
		}
		if !approved {
			return false, fmt.Errorf("tool execution denied")
		}
	}

	execution, err := s.tooling.Execute(ctx, state.record.WorkspacePath, state.record, state.env, call)
	if err != nil {
		return false, err
	}
	return s.recordToolExecution(runID, call, execution, messages)
}

func (s *Service) executeDelegateBatch(
	ctx context.Context,
	runID string,
	calls []domain.ToolCall,
	messages *[]adapters.Message,
) (bool, error) {
	state, ok := s.state(runID)
	if !ok {
		return false, nil
	}

	type result struct {
		index     int
		call      domain.ToolCall
		execution tooling.Execution
		err       error
	}

	results := make([]result, len(calls))
	sem := make(chan struct{}, 2)
	done := make(chan result, len(calls))
	for index, call := range calls {
		index := index
		call := call
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			execution, err := s.tooling.Execute(ctx, state.record.WorkspacePath, state.record, state.env, call)
			done <- result{index: index, call: call, execution: execution, err: err}
		}()
	}

	for range calls {
		item := <-done
		if item.err != nil {
			return false, item.err
		}
		results[item.index] = item
	}

	for _, item := range results {
		terminated, err := s.recordToolExecution(runID, item.call, item.execution, messages)
		if err != nil || terminated {
			return terminated, err
		}
	}
	return false, nil
}

func allDelegateCalls(executor *tooling.Executor, calls []domain.ToolCall) bool {
	if len(calls) == 0 {
		return false
	}
	for _, call := range calls {
		request, err := executor.DecodeOTRequest(call)
		if err != nil || request.Op != "delegate" {
			return false
		}
	}
	return true
}

func (s *Service) recordToolExecution(
	runID string,
	call domain.ToolCall,
	execution tooling.Execution,
	messages *[]adapters.Message,
) (bool, error) {
	if execution.NextCwd != "" {
		if err := s.setRunCwd(runID, execution.NextCwd); err != nil {
			return false, err
		}
	}

	formatted := formatToolResult(call, execution.Output)
	if err := s.appendRunEvent(runID, "tool", formatted); err != nil {
		return false, err
	}
	if err := s.appendOutput(runID, formatted+"\n"); err != nil {
		return false, err
	}

	toolResult := domain.ToolResult{
		ToolCallID: call.ID,
		Name:       call.Name,
		Content:    execution.Output,
	}
	*messages = append(*messages, adapters.ToToolMessage(toolResult))
	if err := s.appendSessionTool(runID, toolResult); err != nil {
		return false, err
	}
	record, ok := s.RunRecord(runID)
	if ok {
		if err := s.updateRunTaskOutcome(record, execution); err != nil {
			return false, err
		}
	}

	switch execution.TerminalStatus {
	case domain.StatusCompleted:
		return true, s.completeRun(runID)
	case domain.StatusFailed:
		return true, s.failRun(runID, executionFailure(execution))
	default:
		return false, nil
	}
}

func (s *Service) prepareRunExecution(runID string) (runExecution, bool, error) {
	record, ok := s.RunRecord(runID)
	if !ok {
		return runExecution{}, false, nil
	}
	client, ok := s.clients[record.Provider]
	if !ok {
		return runExecution{}, true, fmt.Errorf("provider %s is not configured", record.Provider)
	}
	snapshot := s.Snapshot()
	sessionMessages, err := s.sessionContextMessages()
	if err != nil {
		return runExecution{}, true, err
	}
	return runExecution{
		record:           record,
		client:           client,
		snapshot:         snapshot,
		providerSettings: snapshot.Settings.ConfigFor(record.Provider),
		messages:         append(sessionMessages, adapters.Message{Role: "user", Content: record.Prompt}),
		limit:            ralphLimit(snapshot.Settings, record.Mode),
	}, true, nil
}

func (s *Service) executeIterationChat(
	ctx context.Context,
	runID string,
	execution runExecution,
	contextMessage string,
) (adapters.ChatResult, error) {
	systemPrompt, err := s.buildSystemPrompt(execution.record)
	if err != nil {
		return adapters.ChatResult{}, err
	}
	request := adapters.ChatRequest{
		Model: execution.record.Model,
		Messages: append([]adapters.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "system", Content: contextMessage},
		}, execution.messages...),
		Tools: adapters.ToolCatalog(execution.record.Mode, execution.record.AgentRole),
	}
	return execution.client.Chat(ctx, execution.providerSettings, request, s.streamRunDelta(runID))
}

func (s *Service) streamRunDelta(runID string) adapters.DeltaHandler {
	return func(delta adapters.Delta) error {
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
	}
}

func (s *Service) updateRunTaskOutcome(record domain.RunRecord, execution tooling.Execution) error {
	if record.AgentRole != domain.AgentRoleWorker || (execution.TerminalStatus == "" && !hasStructuredTaskOutcome(execution)) {
		return nil
	}
	return s.updateCurrentSessionTaskMetadata(
		taskStatusFromRunStatus(execution.TerminalStatus),
		execution.TaskSummary,
		execution.TaskChangedPaths,
		execution.TaskChecksRun,
		execution.TaskEvidencePointers,
		execution.TaskFollowups,
		execution.TaskErrorKind,
	)
}

func taskStatusFromRunStatus(status domain.RunStatus) string {
	switch status {
	case domain.StatusCompleted:
		return domain.TaskStatusCompleted
	case domain.StatusFailed:
		return domain.TaskStatusFailed
	default:
		return ""
	}
}

func executionFailure(execution tooling.Execution) error {
	message := strings.TrimSpace(execution.TerminalMessage)
	if message == "" {
		message = strings.TrimSpace(execution.Output)
	}
	if message == "" {
		message = "worker task failed"
	}
	return fmt.Errorf("%s", message)
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
	s.publish(ServiceEvent{Type: "approval_required", RunID: runID, Message: "Approval required."})

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
	s.publish(ServiceEvent{Type: "run_output", RunID: runID})
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

	s.publish(ServiceEvent{Type: "run_thinking", RunID: runID})
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
	s.publish(ServiceEvent{Type: "run_updated", RunID: runID, Message: record.CurrentTask})
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
	if record.AgentRole == domain.AgentRoleGateway && record.Mode == domain.RunModePlan && (draft != "" || output != "") {
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
	content := draft
	if content == "" {
		content = output
	}
	if record.AgentRole == domain.AgentRoleWorker {
		_ = s.updateCurrentSessionTaskMetadata(domain.TaskStatusCompleted, content, nil, nil, nil, nil, "")
	}
	meta, outcome, outcomeErr := s.persistTaskOutcome(record)
	if outcomeErr == nil {
		s.emitHookEvent(HookEvent{
			Type:           hookTaskCompleted,
			SessionID:      record.SessionID,
			RunID:          record.RunID,
			RunRecord:      record,
			SessionMeta:    meta,
			Outcome:        outcome,
			SessionSummary: meta.Summary,
		})
	}
	s.emitHookEvent(HookEvent{
		Type:           hookRunCompleted,
		SessionID:      record.SessionID,
		RunID:          record.RunID,
		RunRecord:      record,
		SessionMeta:    meta,
		SessionSummary: meta.Summary,
	})
	go s.runChatHistoryAssistantSummary(record.SessionID, record.RunID, content)
	go s.runSessionMaintenance(record.SessionID)
	s.publish(ServiceEvent{Type: "run_updated", RunID: runID, Message: "Run completed."})
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
	if record.AgentRole == domain.AgentRoleWorker {
		_ = s.updateCurrentSessionTaskMetadata(domain.TaskStatusFailed, err.Error(), nil, nil, nil, nil, "run_failed")
	}
	if meta, outcome, outcomeErr := s.persistTaskOutcome(record); outcomeErr == nil {
		s.emitHookEvent(HookEvent{
			Type:           hookTaskCompleted,
			SessionID:      record.SessionID,
			RunID:          record.RunID,
			RunRecord:      record,
			SessionMeta:    meta,
			Outcome:        outcome,
			SessionSummary: meta.Summary,
		})
	}
	_ = s.appendRunEvent(runID, "error", err.Error())
	s.publish(ServiceEvent{Type: "run_updated", RunID: runID, Message: err.Error()})
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
	if record.AgentRole == domain.AgentRoleWorker {
		_ = s.updateCurrentSessionTaskMetadata(domain.TaskStatusCancelled, message, nil, nil, nil, nil, "")
		if meta, outcome, outcomeErr := s.persistTaskOutcome(record); outcomeErr == nil {
			s.emitHookEvent(HookEvent{
				Type:           hookTaskCompleted,
				SessionID:      record.SessionID,
				RunID:          record.RunID,
				RunRecord:      record,
				SessionMeta:    meta,
				Outcome:        outcome,
				SessionSummary: meta.Summary,
			})
		}
	}
	_ = s.appendRunEvent(runID, "cancel", message)
	s.publish(ServiceEvent{Type: "run_updated", RunID: runID, Message: message})
	return nil
}

func (s *Service) state(runID string) (*runState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.runs[runID]
	return state, ok
}

func (s *Service) loadIterationInputs(record domain.RunRecord, selectedSkills []selectedSkill, resolvedReferences string, memorySnapshot domain.MemorySnapshot, activePlan domain.PlanCache, draftPlan string) (iterationInputs, error) {
	toolsDoc, err := readWorkspaceFile(record.WorkspacePath, filepath.Join("bootstrap", "TOOLS.md"))
	if err != nil {
		return iterationInputs{}, err
	}
	user, err := loadUserMemory(record.WorkspacePath, record.AgentRole)
	if err != nil {
		return iterationInputs{}, err
	}
	chatHistory, err := s.loadChatHistoryMemory(record.AgentRole)
	if err != nil {
		return iterationInputs{}, err
	}

	meta := domain.SessionMetadata{}
	currentContext := session.Context{}
	if strings.TrimSpace(record.SessionID) != "" {
		meta, err = s.sessionManager.LoadMetadata(record.SessionID)
		if err != nil {
			return iterationInputs{}, err
		}
		currentContext, err = s.sessions.Context(meta)
		if err != nil {
			return iterationInputs{}, err
		}
	}

	s.mu.RLock()
	inherited := s.inheritedCtx
	s.mu.RUnlock()

	return iterationInputs{
		record:             record,
		toolsDoc:           toolsDoc,
		userMemory:         user,
		chatHistory:        chatHistory,
		selectedSkills:     selectedSkills,
		resolvedReferences: resolvedReferences,
		memorySnapshot:     memorySnapshot,
		activePlan:         activePlan,
		draftPlan:          draftPlan,
		meta:               meta,
		currentContext:     currentContext,
		inheritedContext:   inherited,
	}, nil
}

func (s *Service) buildIterationContext(record domain.RunRecord, selectedSkills []selectedSkill, resolvedReferences string, memorySnapshot domain.MemorySnapshot, activePlan domain.PlanCache, draftPlan string) (string, error) {
	inputs, err := s.loadIterationInputs(record, selectedSkills, resolvedReferences, memorySnapshot, activePlan, draftPlan)
	if err != nil {
		return "", err
	}
	return inputs.prompt(), nil
}

func (s *Service) buildSystemPrompt(record domain.RunRecord) (string, error) {
	common, err := readWorkspaceFile(record.WorkspacePath, "AGENTS.md")
	if err != nil {
		return "", err
	}
	rolePrompt, err := readWorkspaceFile(record.WorkspacePath, rolePromptPath(record.AgentRole))
	if err != nil {
		return "", err
	}
	return prompts.SystemPrompt(record.Mode, record.AgentRole, common, rolePrompt), nil
}

func readWorkspaceFile(workspaceRoot string, relativePath string) (string, error) {
	path := filepath.Join(workspaceRoot, relativePath)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", relativePath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func rolePromptPath(role domain.AgentRole) string {
	if role == domain.AgentRoleWorker {
		return filepath.Join("bootstrap", "system-prompt", "worker", "AGENTS.md")
	}
	return filepath.Join("bootstrap", "system-prompt", "gateway", "AGENTS.md")
}

func loadUserMemory(workspaceRoot string, role domain.AgentRole) (string, error) {
	maxManaged := 640
	maxUser := 480
	if role == domain.AgentRoleGateway {
		maxManaged = 1200
		maxUser = 800
	}
	return userprefs.ReadMemoryExcerpt(filepath.Join(workspaceRoot, "bootstrap", "USER.md"), maxManaged, maxUser)
}

func (s *Service) loadChatHistoryMemory(role domain.AgentRole) (string, error) {
	limitEntries := 2
	maxBytes := 800
	if role == domain.AgentRoleGateway {
		limitEntries = 6
		maxBytes = 2400
	}
	return s.sessionManager.ReadChatHistoryRecent(limitEntries, maxBytes)
}

func formatSelectedSkills(selectedSkills []selectedSkill) string {
	if len(selectedSkills) == 0 {
		return ""
	}

	sections := make([]string, 0, len(selectedSkills))
	for _, skill := range selectedSkills {
		content := strings.TrimSpace(skill.Content)
		if content == "" {
			continue
		}

		sections = append(sections, fmt.Sprintf("$%s (%s):\n%s", skill.Name, skill.Path, content))
	}
	return strings.Join(sections, "\n\n")
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

func (i iterationInputs) prompt() string {
	return prompts.IterationContext(
		i.record,
		i.record.AgentRole,
		i.toolsDoc,
		i.userMemory,
		i.chatHistory,
		knowledge.RenderPrompt(i.memorySnapshot),
		formatSelectedSkills(i.selectedSkills),
		i.resolvedReferences,
		i.activePlan,
		i.draftPlan,
		i.meta.TaskTitle,
		i.meta.TaskContract,
		i.meta.TaskStatus,
	)
}

func (i iterationInputs) snapshot() domain.ContextSnapshot {
	return domain.ContextSnapshot{
		SessionID:               i.record.SessionID,
		RunID:                   i.record.RunID,
		Provider:                i.record.Provider.String(),
		Model:                   i.record.Model,
		WorkspacePath:           i.record.WorkspacePath,
		CurrentCwd:              i.record.CurrentCwd,
		CompactSummaryPresent:   strings.TrimSpace(i.currentContext.Summary) != "",
		PostCompactRecordCount:  len(i.currentContext.Records),
		InheritedSummaryPresent: strings.TrimSpace(i.inheritedContext.Summary) != "",
		InheritedRecordCount:    len(i.inheritedContext.Records),
		SelectedSkills:          selectedSkillNamesFromValues(i.selectedSkills),
		ResolvedReferenceCount:  countResolvedReferenceLines(i.resolvedReferences),
		UserMemoryPresent:       strings.TrimSpace(i.userMemory) != "",
		ChatHistoryExcerptBytes: len(i.chatHistory),
		PlanCachePresent:        strings.TrimSpace(i.activePlan.Content) != "",
		FrozenMemoryEntryCount:  len(i.memorySnapshot.Entries),
		FrozenSkillCount:        len(i.memorySnapshot.Skills),
	}
}

func selectedSkillNamesFromValues(values []selectedSkill) []string {
	names := make([]string, 0, len(values))
	for _, item := range values {
		if strings.TrimSpace(item.Name) == "" {
			continue
		}
		names = append(names, item.Name)
	}
	return names
}

func countResolvedReferenceLines(value string) int {
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(value), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- ") {
			count++
		}
	}
	return count
}

func hasStructuredTaskOutcome(execution tooling.Execution) bool {
	return strings.TrimSpace(execution.TaskSummary) != "" ||
		len(execution.TaskChangedPaths) > 0 ||
		len(execution.TaskChecksRun) > 0 ||
		len(execution.TaskEvidencePointers) > 0 ||
		len(execution.TaskFollowups) > 0 ||
		strings.TrimSpace(execution.TaskErrorKind) != ""
}
