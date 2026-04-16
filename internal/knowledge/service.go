package knowledge

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
)

var nonAlphaNumeric = regexp.MustCompile(`[^a-z0-9]+`)

type Service struct {
	store *sqlitestore.Store
}

type LearningInput struct {
	WorkspacePath  string
	Prompt         string
	SessionMeta    domain.SessionMetadata
	Outcome        domain.TaskOutcome
	SessionSummary string
}

func NewService(store *sqlitestore.Store) *Service {
	return &Service{store: store}
}

func (s *Service) BuildFrozenSnapshot(
	ctx context.Context,
	workspacePath string,
	query string,
) (domain.MemorySnapshot, error) {
	if s == nil || s.store == nil {
		return domain.MemorySnapshot{}, nil
	}

	memories, err := s.store.SearchMemoryEntries(ctx, workspacePath, query, 4)
	if err != nil {
		return domain.MemorySnapshot{}, err
	}
	skills, err := s.store.SearchSkills(ctx, workspacePath, query, string(domain.SkillStatusPublished), 2)
	if err != nil {
		return domain.MemorySnapshot{}, err
	}

	snapshot := domain.MemorySnapshot{
		Entries: make([]domain.MemoryEntry, 0, len(memories)),
		Skills:  skills,
	}
	for _, item := range memories {
		if item.Entry.Kind == domain.MemoryKindProcedure {
			continue
		}
		snapshot.Entries = append(snapshot.Entries, item.Entry)
	}
	return snapshot, nil
}

func RenderPrompt(snapshot domain.MemorySnapshot) string {
	sections := make([]string, 0, 2)
	if len(snapshot.Entries) > 0 {
		lines := []string{"Frozen persistent memory:"}
		for _, entry := range snapshot.Entries {
			line := fmt.Sprintf("- [%s] %s: %s", entry.Kind, strings.TrimSpace(entry.Title), strings.TrimSpace(entry.Content))
			lines = append(lines, strings.TrimSpace(line))
			if len(entry.EvidencePointers) > 0 {
				lines = append(lines, "  Evidence: "+strings.Join(entry.EvidencePointers, ", "))
			}
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}
	if len(snapshot.Skills) > 0 {
		lines := []string{"Frozen promoted skills:"}
		for _, skill := range snapshot.Skills {
			lines = append(lines, fmt.Sprintf("- %s (%s): %s", skill.Name, skill.Status, strings.TrimSpace(skill.Summary)))
			if trimmed := strings.TrimSpace(skill.Content); trimmed != "" {
				lines = append(lines, "  "+trimmed)
			}
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}
	return strings.Join(sections, "\n\n")
}

func (s *Service) SessionSearch(
	ctx context.Context,
	workspacePath string,
	query string,
	limit int,
) ([]domain.SessionSearchResult, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.SearchSessionMessages(ctx, workspacePath, query, limit)
}

func (s *Service) MemorySearch(
	ctx context.Context,
	workspacePath string,
	query string,
	limit int,
) ([]domain.MemorySearchResult, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.SearchMemoryEntries(ctx, workspacePath, query, limit)
}

func (s *Service) ListSkills(
	ctx context.Context,
	workspacePath string,
	status string,
	limit int,
) ([]domain.SkillRecord, error) {
	if s == nil || s.store == nil {
		return nil, nil
	}
	return s.store.ListSkills(ctx, workspacePath, status, limit)
}

func (s *Service) GetSkill(ctx context.Context, skillID string) (domain.SkillRecord, error) {
	if s == nil || s.store == nil {
		return domain.SkillRecord{}, fmt.Errorf("knowledge store is not configured")
	}
	return s.store.GetSkill(ctx, skillID)
}

func (s *Service) CommitMemory(ctx context.Context, entry domain.MemoryEntry) (domain.MemoryEntry, error) {
	if s == nil || s.store == nil {
		return domain.MemoryEntry{}, fmt.Errorf("knowledge store is not configured")
	}
	saved, err := s.store.SaveMemoryEntry(ctx, entry)
	if err != nil {
		return domain.MemoryEntry{}, err
	}
	if saved.ID > 0 {
		_ = s.store.LinkMemory(ctx, saved.ID, saved.SourceSessionID, saved.SourceRunID, "commit")
	}
	return saved, nil
}

func (s *Service) ProposeSkill(ctx context.Context, skill domain.SkillRecord) (domain.SkillRecord, error) {
	if s == nil || s.store == nil {
		return domain.SkillRecord{}, fmt.Errorf("knowledge store is not configured")
	}
	if skill.SkillID == "" {
		skill.SkillID = skillID(skill.Name, skill.Fingerprint)
	}
	if skill.Status == "" {
		skill.Status = domain.SkillStatusDraft
	}
	return s.store.SaveSkill(ctx, skill)
}

func (s *Service) LearnFromTask(ctx context.Context, input LearningInput) error {
	if s == nil || s.store == nil {
		return nil
	}

	entries := s.deriveMemoryEntries(input)
	for _, entry := range entries {
		saved, err := s.store.SaveMemoryEntry(ctx, entry)
		if err != nil {
			return err
		}
		if saved.ID > 0 {
			_ = s.store.LinkMemory(ctx, saved.ID, input.SessionMeta.SessionID, input.Outcome.RunID, "learned")
		}
	}

	if err := s.promoteProcedureMemory(ctx, input); err != nil {
		return err
	}
	return nil
}

func (s *Service) deriveMemoryEntries(input LearningInput) []domain.MemoryEntry {
	entries := make([]domain.MemoryEntry, 0, 4)
	now := time.Now()
	workspacePath := chooseWorkspacePath(input.WorkspacePath, input.SessionMeta.WorkspacePath)

	for _, preference := range deriveUserProfileFacts(input.Prompt) {
		entries = append(entries, domain.MemoryEntry{
			Kind:            domain.MemoryKindUserProfile,
			WorkspacePath:   workspacePath,
			SourceSessionID: input.SessionMeta.SessionID,
			SourceRunID:     input.Outcome.RunID,
			Title:           preference.Title,
			Content:         preference.Content,
			Status:          domain.MemoryStatusActive,
			Fingerprint:     preference.Fingerprint,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}

	for _, fact := range deriveWorkspaceFacts(input.Outcome, input.SessionSummary) {
		entries = append(entries, domain.MemoryEntry{
			Kind:            domain.MemoryKindWorkspaceFacts,
			WorkspacePath:   workspacePath,
			SourceSessionID: input.SessionMeta.SessionID,
			SourceRunID:     input.Outcome.RunID,
			Title:           fact.Title,
			Content:         fact.Content,
			Status:          domain.MemoryStatusActive,
			Fingerprint:     fact.Fingerprint,
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}

	if strings.TrimSpace(input.Outcome.Summary) != "" && len(input.Outcome.EvidencePointers) > 0 {
		title := strings.TrimSpace(input.Outcome.Title)
		if title == "" {
			title = clipKnowledgeText(input.Outcome.Summary, 64)
		}
		entries = append(entries, domain.MemoryEntry{
			Kind:             domain.MemoryKindTaskLessons,
			WorkspacePath:    workspacePath,
			SourceSessionID:  input.SessionMeta.SessionID,
			SourceRunID:      input.Outcome.RunID,
			Title:            title,
			Content:          buildTaskLessonContent(input.Outcome),
			EvidencePointers: append([]string(nil), input.Outcome.EvidencePointers...),
			Status:           domain.MemoryStatusActive,
			Fingerprint:      knowledgeFingerprint("task_lesson", input.Outcome.Fingerprint, input.Outcome.Summary),
			CreatedAt:        now,
			UpdatedAt:        now,
		})
	}

	return entries
}

func (s *Service) promoteProcedureMemory(ctx context.Context, input LearningInput) error {
	if strings.TrimSpace(input.Outcome.Fingerprint) == "" || input.Outcome.Status != domain.TaskStatusCompleted {
		return nil
	}
	outcomes, err := s.store.ListTaskOutcomesByFingerprint(ctx, input.Outcome.Fingerprint, 20)
	if err != nil {
		return err
	}
	successCount := 0
	for _, item := range outcomes {
		if item.Status == domain.TaskStatusCompleted {
			successCount++
		}
	}
	if successCount < 2 {
		return nil
	}

	workspacePath := chooseWorkspacePath(input.WorkspacePath, input.SessionMeta.WorkspacePath)
	skill, err := s.store.GetSkillByFingerprint(ctx, workspacePath, input.Outcome.Fingerprint)
	switch {
	case err == nil:
		skill.ReplayCount = successCount
		if successCount >= 3 {
			skill.Status = domain.SkillStatusPublished
		}
		if skill.Content == "" {
			skill.Content = buildProcedureContent(input.Outcome)
		}
		if skill.Summary == "" {
			skill.Summary = clipKnowledgeText(input.Outcome.Summary, 96)
		}
		if _, err := s.store.SaveSkill(ctx, skill); err != nil {
			return err
		}
		_, err = s.store.SavePromotionJob(ctx, domain.PromotionJob{
			JobType:               "procedure_memory",
			Status:                promotionStatus(successCount),
			SkillID:               skill.SkillID,
			Fingerprint:           input.Outcome.Fingerprint,
			ReplayCount:           successCount,
			RequiredEvidenceCount: 3,
			Notes:                 "Promoted from repeated successful task outcomes.",
		})
		return err
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return err
	}

	memory, err := s.store.SaveMemoryEntry(ctx, domain.MemoryEntry{
		Kind:             domain.MemoryKindProcedure,
		WorkspacePath:    workspacePath,
		SourceSessionID:  input.SessionMeta.SessionID,
		SourceRunID:      input.Outcome.RunID,
		Title:            skillTitle(input.Outcome.Title, input.Outcome.Summary),
		Content:          buildProcedureContent(input.Outcome),
		EvidencePointers: append([]string(nil), input.Outcome.EvidencePointers...),
		Status:           domain.MemoryStatusActive,
		Fingerprint:      input.Outcome.Fingerprint,
	})
	if err != nil {
		return err
	}

	skillRecord := domain.SkillRecord{
		SkillID:        skillID(skillTitle(input.Outcome.Title, input.Outcome.Summary), input.Outcome.Fingerprint),
		WorkspacePath:  workspacePath,
		Name:           skillTitle(input.Outcome.Title, input.Outcome.Summary),
		Summary:        clipKnowledgeText(input.Outcome.Summary, 96),
		Content:        buildProcedureContent(input.Outcome),
		Status:         domain.SkillStatusDraft,
		SourceMemoryID: memory.ID,
		Fingerprint:    input.Outcome.Fingerprint,
		ReplayCount:    successCount,
	}
	if successCount >= 3 {
		skillRecord.Status = domain.SkillStatusPublished
	}
	skillRecord, err = s.store.SaveSkill(ctx, skillRecord)
	if err != nil {
		return err
	}
	_, err = s.store.SavePromotionJob(ctx, domain.PromotionJob{
		JobType:               "procedure_memory",
		Status:                promotionStatus(successCount),
		MemoryID:              memory.ID,
		SkillID:               skillRecord.SkillID,
		Fingerprint:           input.Outcome.Fingerprint,
		ReplayCount:           successCount,
		RequiredEvidenceCount: 3,
		Notes:                 "Created from repeated successful task outcomes.",
	})
	return err
}

func promotionStatus(successCount int) domain.PromotionJobStatus {
	if successCount >= 3 {
		return domain.PromotionJobPromoted
	}
	return domain.PromotionJobPending
}

type derivedEntry struct {
	Title       string
	Content     string
	Fingerprint string
}

func deriveUserProfileFacts(prompt string) []derivedEntry {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return nil
	}
	lower := strings.ToLower(trimmed)
	entries := make([]derivedEntry, 0, 2)
	if strings.Contains(lower, "respond in korean") || strings.Contains(trimmed, "한국어") {
		entries = append(entries, derivedEntry{
			Title:       "Preferred response language",
			Content:     "Respond to the user in Korean unless they explicitly ask for another language.",
			Fingerprint: knowledgeFingerprint("user_profile", "response_language", "korean"),
		})
	}
	if strings.Contains(lower, "comments in english") || strings.Contains(trimmed, "영어 주석") {
		entries = append(entries, derivedEntry{
			Title:       "Code comment language",
			Content:     "Write code comments in English.",
			Fingerprint: knowledgeFingerprint("user_profile", "comment_language", "english"),
		})
	}
	return entries
}

func deriveWorkspaceFacts(outcome domain.TaskOutcome, sessionSummary string) []derivedEntry {
	entries := make([]derivedEntry, 0, 2)
	if usesGoToolchain(outcome, sessionSummary) {
		entries = append(entries, derivedEntry{
			Title:       "Go validation workflow",
			Content:     "Use gofmt, go test, go vet, and golangci-lint for repository validation when relevant.",
			Fingerprint: knowledgeFingerprint("workspace_fact", "go_validation", "gofmt go test go vet golangci-lint"),
		})
	}
	return entries
}

func usesGoToolchain(outcome domain.TaskOutcome, sessionSummary string) bool {
	for _, path := range outcome.ChangedPaths {
		if strings.HasSuffix(strings.TrimSpace(path), ".go") {
			return true
		}
	}
	for _, check := range outcome.ChecksRun {
		switch strings.TrimSpace(check) {
		case "go_test", "go_vet", "golangci_lint":
			return true
		}
	}
	return strings.Contains(strings.ToLower(sessionSummary), "go")
}

func buildTaskLessonContent(outcome domain.TaskOutcome) string {
	lines := []string{"Summary: " + strings.TrimSpace(outcome.Summary)}
	if len(outcome.ChangedPaths) > 0 {
		lines = append(lines, "Changed paths: "+strings.Join(outcome.ChangedPaths, ", "))
	}
	if len(outcome.ChecksRun) > 0 {
		lines = append(lines, "Checks: "+strings.Join(outcome.ChecksRun, ", "))
	}
	return strings.Join(lines, "\n")
}

func buildProcedureContent(outcome domain.TaskOutcome) string {
	lines := []string{
		"When to use:",
		"- " + strings.TrimSpace(clipKnowledgeText(outcome.Title, 80)),
		"Recommended workflow:",
	}
	if strings.TrimSpace(outcome.Summary) != "" {
		lines = append(lines, "- Start from the successful pattern captured in the task summary: "+strings.TrimSpace(outcome.Summary))
	}
	if len(outcome.ChangedPaths) > 0 {
		lines = append(lines, "- Inspect and update these paths first: "+strings.Join(outcome.ChangedPaths, ", "))
	}
	if len(outcome.ChecksRun) > 0 {
		lines = append(lines, "- Validate with: "+strings.Join(outcome.ChecksRun, ", "))
	}
	if len(outcome.EvidencePointers) > 0 {
		lines = append(lines, "- Evidence pointers: "+strings.Join(outcome.EvidencePointers, ", "))
	}
	return strings.Join(lines, "\n")
}

func skillTitle(taskTitle string, summary string) string {
	title := strings.TrimSpace(taskTitle)
	if title == "" {
		title = clipKnowledgeText(summary, 48)
	}
	if title == "" {
		title = "Procedure Memory"
	}
	return title
}

func skillID(name string, fingerprint string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	base = nonAlphaNumeric.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "procedure-memory"
	}
	short := fingerprint
	if len(short) > 12 {
		short = short[:12]
	}
	if short == "" {
		short = knowledgeFingerprint(name, time.Now().Format(time.RFC3339Nano))
		if len(short) > 12 {
			short = short[:12]
		}
	}
	return base + "-" + short
}

func knowledgeFingerprint(parts ...string) string {
	h := sha1.New()
	for _, part := range parts {
		h.Write([]byte(strings.TrimSpace(part)))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func clipKnowledgeText(value string, max int) string {
	trimmed := strings.TrimSpace(value)
	if max <= 0 || len([]rune(trimmed)) <= max {
		return trimmed
	}
	runes := []rune(trimmed)
	return string(runes[:max])
}

func chooseWorkspacePath(primary string, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
}

func OutcomeFingerprint(title string, changedPaths []string, checksRun []string) string {
	normalizedPaths := make([]string, 0, len(changedPaths))
	for _, path := range changedPaths {
		trimmed := filepath.ToSlash(strings.TrimSpace(path))
		if trimmed == "" {
			continue
		}
		normalizedPaths = append(normalizedPaths, trimmed)
	}
	normalizedChecks := make([]string, 0, len(checksRun))
	for _, check := range checksRun {
		trimmed := strings.TrimSpace(check)
		if trimmed == "" {
			continue
		}
		normalizedChecks = append(normalizedChecks, trimmed)
	}
	return knowledgeFingerprint(strings.TrimSpace(title), strings.Join(normalizedPaths, ","), strings.Join(normalizedChecks, ","))
}
