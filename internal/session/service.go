package session

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/prompts"
)

type MaintenanceRunner interface {
	Run(ctx context.Context, provider domain.Provider, model string, systemPrompt string, userPrompt string) (string, error)
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
	maintenanceMu sync.Mutex
}

func NewService(manager *Manager, runner MaintenanceRunner) *Service {
	return &Service{
		manager: manager,
		runner:  runner,
	}
}

func (s *Service) Create(
	workspacePath string,
	provider domain.Provider,
	model string,
	startedAt time.Time,
	parentSessionID string,
	parentRunID string,
	parentTaskID string,
	workerRole domain.AgentRole,
	taskTitle string,
	taskContract string,
	taskStatus string,
) (domain.SessionMetadata, error) {
	return s.manager.Create(
		workspacePath,
		provider,
		model,
		startedAt,
		parentSessionID,
		parentRunID,
		parentTaskID,
		workerRole,
		taskTitle,
		taskContract,
		taskStatus,
	)
}

func (s *Service) LoadMetadata(sessionID string) (domain.SessionMetadata, error) {
	return s.manager.LoadMetadata(sessionID)
}

func (s *Service) SaveMetadata(meta domain.SessionMetadata) error {
	return s.manager.SaveMetadata(meta)
}

func (s *Service) ListSessions(limit int) ([]domain.SessionMetadata, error) {
	return s.manager.ListSessions(limit)
}

func (s *Service) LatestSessionID() (string, error) {
	return s.manager.LatestSessionID()
}

func (s *Service) AppendContextSnapshot(meta domain.SessionMetadata, runID string, snapshot domain.ContextSnapshot) (domain.SessionMetadata, error) {
	return s.appendRecord(meta, domain.SessionRecord{
		RunID:           runID,
		Type:            domain.SessionRecordContext,
		ContextSnapshot: &snapshot,
	})
}

func (s *Service) LatestContextSnapshot(sessionID string, runID string) (domain.ContextSnapshot, error) {
	records, err := s.manager.LoadRecords(sessionID)
	if err != nil {
		return domain.ContextSnapshot{}, err
	}
	for index := len(records) - 1; index >= 0; index-- {
		record := records[index]
		if record.Type != domain.SessionRecordContext || record.ContextSnapshot == nil {
			continue
		}
		if strings.TrimSpace(runID) != "" && record.RunID != runID {
			continue
		}
		return *record.ContextSnapshot, nil
	}
	return domain.ContextSnapshot{}, fmt.Errorf("no context snapshot found")
}

func (s *Service) ListTasks(parentSessionID string, parentRunID string, statusFilter string) ([]domain.TaskView, error) {
	metadata, err := s.manager.ListMetadata(0)
	if err != nil {
		return nil, err
	}

	tasks := make([]domain.TaskView, 0)
	for _, meta := range metadata {
		if strings.TrimSpace(meta.ParentSessionID) != strings.TrimSpace(parentSessionID) {
			continue
		}
		if strings.TrimSpace(parentRunID) != "" && strings.TrimSpace(meta.ParentRunID) != strings.TrimSpace(parentRunID) {
			continue
		}
		task := s.taskViewFromMetadata(meta)
		if strings.TrimSpace(statusFilter) != "" && task.Status != strings.TrimSpace(statusFilter) {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (s *Service) GetTask(parentSessionID string, taskID string) (domain.TaskView, error) {
	metadata, err := s.manager.ListMetadata(0)
	if err != nil {
		return domain.TaskView{}, err
	}

	for _, meta := range metadata {
		if strings.TrimSpace(meta.ParentSessionID) != strings.TrimSpace(parentSessionID) {
			continue
		}
		if strings.TrimSpace(meta.ParentTaskID) != strings.TrimSpace(taskID) {
			continue
		}
		return s.taskViewFromMetadata(meta), nil
	}
	return domain.TaskView{}, fmt.Errorf("task %s not found", strings.TrimSpace(taskID))
}

func (s *Service) Context(meta domain.SessionMetadata) (Context, error) {
	if strings.TrimSpace(meta.SessionID) == "" {
		return Context{}, nil
	}

	records, err := s.manager.LoadRecords(meta.SessionID)
	if err != nil {
		return Context{}, err
	}

	contextRecords := make([]domain.SessionRecord, 0, len(records))
	for _, record := range records {
		if record.Seq <= meta.LastCompactedSeq {
			continue
		}
		switch record.Type {
		case domain.SessionRecordUser, domain.SessionRecordAssistant, domain.SessionRecordTool:
			contextRecords = append(contextRecords, record)
		}
	}

	return Context{
		Summary: strings.TrimSpace(meta.Summary),
		Records: contextRecords,
	}, nil
}

func (s *Service) AppendUser(meta domain.SessionMetadata, runID string, prompt string) (domain.SessionMetadata, error) {
	return s.appendRecord(meta, domain.SessionRecord{
		RunID:   runID,
		Type:    domain.SessionRecordUser,
		Content: prompt,
	})
}

func (s *Service) AppendAssistant(meta domain.SessionMetadata, runID string, content string, usage domain.UsageStats) (domain.SessionMetadata, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return meta, nil
	}

	return s.appendRecord(meta, domain.SessionRecord{
		RunID:   runID,
		Type:    domain.SessionRecordAssistant,
		Content: content,
		Usage:   usage,
	})
}

func (s *Service) AppendTool(meta domain.SessionMetadata, runID string, result domain.ToolResult) (domain.SessionMetadata, error) {
	return s.appendRecord(meta, domain.SessionRecord{
		RunID:      runID,
		Type:       domain.SessionRecordTool,
		Content:    result.Content,
		ToolName:   result.Name,
		ToolCallID: result.ToolCallID,
	})
}

func (s *Service) UpdateTitle(meta domain.SessionMetadata, title string) (domain.SessionMetadata, error) {
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()

	title = strings.TrimSpace(title)
	if title == "" {
		return meta, nil
	}
	return s.appendRecord(meta, domain.SessionRecord{
		Type:  domain.SessionRecordTitle,
		Title: title,
	})
}

func (s *Service) ShouldCompact(settings domain.Settings, meta domain.SessionMetadata) bool {
	threshold := settings.CompactThresholdK * 1000
	return threshold > 0 && meta.TokensSinceCompact >= threshold
}

func (s *Service) Compact(ctx context.Context, meta domain.SessionMetadata) (domain.SessionMetadata, error) {
	s.maintenanceMu.Lock()
	defer s.maintenanceMu.Unlock()

	if strings.TrimSpace(meta.SessionID) == "" {
		return meta, nil
	}

	records, err := s.manager.LoadRecords(meta.SessionID)
	if err != nil {
		return meta, err
	}
	if len(records) == 0 {
		return meta, nil
	}

	throughSeq := records[len(records)-1].Seq
	summary, err := s.generateCompactSummary(ctx, meta, records, throughSeq)
	if err != nil {
		summary = buildCompactSummary(records, throughSeq)
	}
	if strings.TrimSpace(summary) == "" {
		return meta, nil
	}

	updated, err := s.appendRecord(meta, domain.SessionRecord{
		Type:       domain.SessionRecordCompact,
		Content:    summary,
		ThroughSeq: throughSeq,
	})
	if err != nil {
		return meta, err
	}
	updated.Summary = summary
	updated.TokensSinceCompact = 0
	updated.UpdatedAt = time.Now()
	if err := s.manager.SaveMetadata(updated); err != nil {
		return meta, err
	}
	return updated, nil
}

func (s *Service) Finalize(ctx context.Context, meta domain.SessionMetadata) (domain.SessionMetadata, error) {
	if strings.TrimSpace(meta.SessionID) == "" {
		return meta, nil
	}

	meta.FinalizePending = true
	meta.UpdatedAt = time.Now()
	if err := s.manager.SaveMetadata(meta); err != nil {
		return meta, err
	}

	title, err := s.DeriveTitle(ctx, meta)
	if err != nil {
		return meta, err
	}
	if updated, err := s.UpdateTitle(meta, title); err == nil {
		meta = updated
	} else {
		return meta, err
	}

	if updated, err := s.Compact(ctx, meta); err == nil {
		meta = updated
	} else {
		return meta, err
	}

	now := time.Now()
	meta.FinalizePending = false
	meta.FinalizedAt = &now
	meta.UpdatedAt = now
	if err := s.manager.SaveMetadata(meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func (s *Service) DeriveTitle(ctx context.Context, meta domain.SessionMetadata) (string, error) {
	records, err := s.manager.LoadRecords(meta.SessionID)
	if err != nil {
		return "", err
	}

	if generated, err := s.generateSessionTitle(ctx, meta, records); err == nil && strings.TrimSpace(generated) != "" {
		return generated, nil
	}
	for _, record := range records {
		if record.Type != domain.SessionRecordUser {
			continue
		}
		value := strings.TrimSpace(record.Content)
		if value == "" {
			continue
		}
		if len(value) > 72 {
			value = value[:72]
		}
		return value, nil
	}
	return meta.Title, nil
}

func (s *Service) appendRecord(meta domain.SessionMetadata, record domain.SessionRecord) (domain.SessionMetadata, error) {
	meta.LastSequence++
	record.Seq = meta.LastSequence
	record.SessionID = meta.SessionID
	record.CreatedAt = time.Now()
	meta.UpdatedAt = record.CreatedAt
	if record.RunID != "" {
		meta.LastRunID = record.RunID
	}
	switch record.Type {
	case domain.SessionRecordCompact:
		meta.LastCompactedSeq = record.ThroughSeq
	case domain.SessionRecordTitle:
		if strings.TrimSpace(record.Title) != "" {
			meta.Title = strings.TrimSpace(record.Title)
		}
	}
	if record.Usage.TotalTokens > 0 {
		meta.TotalTokens += record.Usage.TotalTokens
		meta.TokensSinceCompact += record.Usage.TotalTokens
	}
	if err := s.manager.AppendRecord(meta.SessionID, record); err != nil {
		return meta, err
	}
	if err := s.manager.SaveMetadata(meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func (s *Service) taskViewFromMetadata(meta domain.SessionMetadata) domain.TaskView {
	return domain.TaskView{
		TaskID:               strings.TrimSpace(meta.ParentTaskID),
		Title:                strings.TrimSpace(meta.TaskTitle),
		Status:               inferTaskStatus(meta),
		ParentSessionID:      strings.TrimSpace(meta.ParentSessionID),
		ParentRunID:          strings.TrimSpace(meta.ParentRunID),
		ChildSessionID:       strings.TrimSpace(meta.SessionID),
		ChildRunID:           strings.TrimSpace(meta.LastRunID),
		WorkerRole:           meta.WorkerRole.String(),
		Provider:             meta.Provider.String(),
		Model:                strings.TrimSpace(meta.Model),
		TaskSummary:          strings.TrimSpace(meta.TaskSummary),
		TaskChangedPaths:     append([]string(nil), meta.TaskChangedPaths...),
		TaskChecksRun:        append([]string(nil), meta.TaskChecksRun...),
		TaskEvidencePointers: append([]string(nil), meta.TaskEvidencePointers...),
		TaskFollowups:        append([]string(nil), meta.TaskFollowups...),
		TaskErrorKind:        strings.TrimSpace(meta.TaskErrorKind),
		FinalOutputExcerpt:   s.taskFinalOutputExcerpt(meta),
		StartedAt:            meta.StartedAt,
		UpdatedAt:            meta.UpdatedAt,
		FinalizedAt:          meta.FinalizedAt,
	}
}

func (s *Service) taskFinalOutputExcerpt(meta domain.SessionMetadata) string {
	if strings.TrimSpace(meta.TaskSummary) != "" {
		return clipCompactLineLimit(meta.TaskSummary, 240)
	}

	records, err := s.manager.LoadRecords(meta.SessionID)
	if err != nil {
		return ""
	}
	for index := len(records) - 1; index >= 0; index-- {
		record := records[index]
		switch record.Type {
		case domain.SessionRecordAssistant, domain.SessionRecordTool, domain.SessionRecordUser:
			if strings.TrimSpace(record.Content) == "" {
				continue
			}
			return clipCompactLineLimit(record.Content, 240)
		}
	}
	return ""
}

func inferTaskStatus(meta domain.SessionMetadata) string {
	status := strings.TrimSpace(meta.TaskStatus)
	if status != "" {
		return status
	}
	if meta.FinalizedAt != nil {
		return domain.TaskStatusCompleted
	}
	if strings.TrimSpace(meta.LastRunID) != "" {
		return domain.TaskStatusRunning
	}
	return domain.TaskStatusQueued
}

func (s *Service) generateSessionTitle(ctx context.Context, meta domain.SessionMetadata, records []domain.SessionRecord) (string, error) {
	transcript := buildTranscript(records, 0)
	if strings.TrimSpace(transcript) == "" || s.runner == nil {
		return "", nil
	}
	return s.runner.Run(ctx, meta.Provider, meta.Model, prompts.SessionTitlePrompt(), transcript)
}

func (s *Service) generateCompactSummary(ctx context.Context, meta domain.SessionMetadata, records []domain.SessionRecord, throughSeq int64) (string, error) {
	transcript := buildTranscriptWithPointers(records, throughSeq)
	if strings.TrimSpace(transcript) == "" || s.runner == nil {
		return "", nil
	}

	topicsRaw, err := s.runner.Run(ctx, meta.Provider, meta.Model, prompts.SessionTopicsPrompt(), transcript)
	if err != nil {
		return "", err
	}

	topics := compactTopics(topicsRaw)
	if len(topics) == 0 {
		topics = fallbackCompactTopics(records, throughSeq)
	}
	if len(topics) == 0 {
		return "", fmt.Errorf("no compact topics produced")
	}

	topicSummaries := make([]string, len(topics))
	errs := make([]error, len(topics))
	var wg sync.WaitGroup
	for index, topic := range topics {
		wg.Add(1)
		go func(index int, topic compactTopic) {
			defer wg.Done()
			summary, err := s.runner.Run(ctx, meta.Provider, meta.Model, prompts.SessionTopicSummaryPrompt(topic.Title, joinLines(topic.Lines)), transcript)
			summary = strings.TrimSpace(summary)
			if err == nil && summary == "" {
				err = fmt.Errorf("empty topic summary")
			}
			if err != nil {
				summary = fallbackCompactTopicSummary(records, topic)
				err = nil
			}
			topicSummaries[index] = renderPointerParagraph(summary, topic.Lines)
			errs[index] = err
		}(index, topic)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return "", err
		}
	}
	return strings.Join(append([]string{"Compact summary"}, topicSummaries...), "\n\n"), nil
}

func buildCompactSummary(records []domain.SessionRecord, throughSeq int64) string {
	lines := []string{"Compact summary"}
	for _, record := range records {
		if record.Seq > throughSeq {
			continue
		}
		switch record.Type {
		case domain.SessionRecordUser:
			lines = append(lines, "User: "+clipCompactLine(record.Content))
		case domain.SessionRecordAssistant:
			lines = append(lines, "Assistant: "+clipCompactLine(record.Content))
		case domain.SessionRecordTool:
			lines = append(lines, fmt.Sprintf("Tool %s: %s", record.ToolName, clipCompactLine(record.Content)))
		}
		lines = append(lines, "Pointer: "+FormatOTPointer([]int64{record.Seq}))
	}
	return strings.Join(lines, "\n")
}

func clipCompactLine(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 240 {
		return value
	}
	return value[:240]
}

func buildTranscript(records []domain.SessionRecord, throughSeq int64) string {
	lines := make([]string, 0, len(records))
	for _, record := range records {
		if throughSeq > 0 && record.Seq > throughSeq {
			continue
		}
		switch record.Type {
		case domain.SessionRecordUser:
			lines = append(lines, "User: "+clipCompactLine(record.Content))
		case domain.SessionRecordAssistant:
			if strings.TrimSpace(record.Content) != "" {
				lines = append(lines, "Assistant: "+clipCompactLine(record.Content))
			}
		case domain.SessionRecordTool:
			lines = append(lines, fmt.Sprintf("Tool %s: %s", record.ToolName, clipCompactLine(record.Content)))
		}
	}
	return strings.Join(lines, "\n")
}

func compactTopics(raw string) []compactTopic {
	lines := strings.Split(raw, "\n")
	topics := make([]compactTopic, 0, len(lines))
	for _, line := range lines {
		value := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "-"), "*"))
		if value == "" {
			continue
		}
		title, lineSpec, found := strings.Cut(value, "|")
		if !found {
			continue
		}
		lineSpec = strings.TrimSpace(lineSpec)
		if !strings.HasPrefix(strings.ToLower(lineSpec), "lines=") {
			continue
		}
		topicLines := parseCompactLines(strings.TrimSpace(strings.TrimPrefix(lineSpec, "lines=")))
		if len(topicLines) == 0 {
			continue
		}
		topics = append(topics, compactTopic{
			Title: strings.TrimSpace(title),
			Lines: topicLines,
		})
	}
	if len(topics) > 5 {
		topics = topics[:5]
	}
	return topics
}

func parseCompactLines(raw string) []int64 {
	values := strings.Split(raw, ",")
	lines := make([]int64, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		line, err := strconv.ParseInt(value, 10, 64)
		if err != nil || line <= 0 {
			continue
		}
		lines = append(lines, line)
	}
	return normalizePointerLines(lines)
}

func joinLines(lines []int64) string {
	values := make([]string, 0, len(lines))
	for _, line := range normalizePointerLines(lines) {
		values = append(values, strconv.FormatInt(line, 10))
	}
	return strings.Join(values, ",")
}

func buildTranscriptWithPointers(records []domain.SessionRecord, throughSeq int64) string {
	lines := make([]string, 0, len(records))
	for _, record := range records {
		if throughSeq > 0 && record.Seq > throughSeq {
			continue
		}
		switch record.Type {
		case domain.SessionRecordUser:
			lines = append(lines, fmt.Sprintf("[line %d] User: %s", record.Seq, clipCompactLine(record.Content)))
		case domain.SessionRecordAssistant:
			if strings.TrimSpace(record.Content) != "" {
				lines = append(lines, fmt.Sprintf("[line %d] Assistant: %s", record.Seq, clipCompactLine(record.Content)))
			}
		case domain.SessionRecordTool:
			lines = append(lines, fmt.Sprintf("[line %d] Tool %s: %s", record.Seq, record.ToolName, clipCompactLine(record.Content)))
		}
	}
	return strings.Join(lines, "\n")
}

func fallbackCompactTopics(records []domain.SessionRecord, throughSeq int64) []compactTopic {
	topics := make([]compactTopic, 0)
	for _, record := range records {
		if throughSeq > 0 && record.Seq > throughSeq {
			continue
		}
		if record.Type != domain.SessionRecordUser {
			continue
		}
		title := clipCompactLineLimit(record.Content, 72)
		topic := compactTopic{
			Title: title,
			Lines: []int64{record.Seq},
		}
		topics = append(topics, topic)
		if len(topics) >= 5 {
			break
		}
	}
	return topics
}

func fallbackCompactTopicSummary(records []domain.SessionRecord, topic compactTopic) string {
	if len(topic.Lines) == 0 {
		return topic.Title
	}
	lineSet := make(map[int64]struct{}, len(topic.Lines))
	for _, line := range topic.Lines {
		lineSet[line] = struct{}{}
	}

	parts := make([]string, 0, len(topic.Lines))
	for _, record := range records {
		if _, ok := lineSet[record.Seq]; !ok {
			continue
		}
		switch record.Type {
		case domain.SessionRecordUser:
			parts = append(parts, "User: "+clipCompactLine(record.Content))
		case domain.SessionRecordAssistant:
			parts = append(parts, "Assistant: "+clipCompactLine(record.Content))
		case domain.SessionRecordTool:
			parts = append(parts, fmt.Sprintf("Tool %s: %s", record.ToolName, clipCompactLine(record.Content)))
		}
	}
	if len(parts) == 0 {
		return topic.Title
	}
	return strings.Join(parts, " ")
}

func renderPointerParagraph(summary string, lines []int64) string {
	summary = strings.TrimSpace(summary)
	pointer := FormatOTPointer(lines)
	if pointer == "" {
		return summary
	}
	return summary + "\nPointer: " + pointer
}
