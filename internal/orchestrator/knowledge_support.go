package orchestrator

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func (s *Service) sessionSearchForRun(
	record domain.RunRecord,
	query string,
	limit int,
) ([]domain.SessionSearchResult, error) {
	if s.knowledge == nil {
		return nil, nil
	}
	workspacePath := s.workspacePathForSession(record.SessionID)
	return s.knowledge.SessionSearch(s.ctx, workspacePath, query, limit)
}

func (s *Service) memorySearchForRun(
	record domain.RunRecord,
	query string,
	limit int,
) ([]domain.MemorySearchResult, error) {
	if s.knowledge == nil {
		return nil, nil
	}
	workspacePath := s.workspacePathForSession(record.SessionID)
	return s.knowledge.MemorySearch(s.ctx, workspacePath, query, limit)
}

func (s *Service) listSkillsForRun(
	record domain.RunRecord,
	status string,
	limit int,
) ([]domain.SkillRecord, error) {
	if s.knowledge == nil {
		return nil, nil
	}
	workspacePath := s.workspacePathForSession(record.SessionID)
	return s.knowledge.ListSkills(s.ctx, workspacePath, status, limit)
}

func (s *Service) getSkillForRun(
	_ domain.RunRecord,
	skillID string,
) (domain.SkillRecord, error) {
	if s.knowledge == nil {
		return domain.SkillRecord{}, fmt.Errorf("knowledge service is not configured")
	}
	return s.knowledge.GetSkill(s.ctx, skillID)
}

func (s *Service) commitMemoryForRun(
	record domain.RunRecord,
	request domain.OTRequest,
) (domain.MemoryEntry, error) {
	if s.knowledge == nil {
		return domain.MemoryEntry{}, fmt.Errorf("knowledge service is not configured")
	}
	kind, err := parseMemoryKind(request.MemoryKind)
	if err != nil {
		return domain.MemoryEntry{}, err
	}
	sessionMeta, err := s.sessions.LoadMetadata(record.SessionID)
	if err != nil {
		return domain.MemoryEntry{}, err
	}
	return s.knowledge.CommitMemory(context.Background(), domain.MemoryEntry{
		Kind:             kind,
		WorkspacePath:    chooseNonEmpty(sessionMeta.WorkspacePath, s.paths.RepoRoot),
		SourceSessionID:  sessionMeta.SessionID,
		SourceRunID:      record.RunID,
		Title:            strings.TrimSpace(request.MemoryTitle),
		Content:          strings.TrimSpace(request.MemoryContent),
		EvidencePointers: append([]string(nil), request.EvidencePointers...),
		Status:           domain.MemoryStatusActive,
		Fingerprint:      commitFingerprint(kind, request),
	})
}

func (s *Service) proposeSkillForRun(
	record domain.RunRecord,
	request domain.OTRequest,
) (domain.SkillRecord, error) {
	if s.knowledge == nil {
		return domain.SkillRecord{}, fmt.Errorf("knowledge service is not configured")
	}
	sessionMeta, err := s.sessions.LoadMetadata(record.SessionID)
	if err != nil {
		return domain.SkillRecord{}, err
	}
	return s.knowledge.ProposeSkill(context.Background(), domain.SkillRecord{
		SkillID:       strings.TrimSpace(request.SkillID),
		WorkspacePath: chooseNonEmpty(sessionMeta.WorkspacePath, s.paths.RepoRoot),
		Name:          strings.TrimSpace(request.SkillName),
		Summary:       strings.TrimSpace(request.SkillSummary),
		Content:       strings.TrimSpace(request.SkillContent),
		Status:        domain.SkillStatusDraft,
		Fingerprint:   knowledgeFingerprintFromRequest(request),
	})
}

func (s *Service) workspacePathForSession(sessionID string) string {
	meta, err := s.sessions.LoadMetadata(sessionID)
	if err != nil {
		return s.paths.RepoRoot
	}
	return chooseNonEmpty(meta.WorkspacePath, s.paths.RepoRoot)
}

func parseMemoryKind(value string) (domain.MemoryKind, error) {
	switch domain.MemoryKind(strings.TrimSpace(value)) {
	case domain.MemoryKindUserProfile:
		return domain.MemoryKindUserProfile, nil
	case domain.MemoryKindWorkspaceFacts:
		return domain.MemoryKindWorkspaceFacts, nil
	case domain.MemoryKindTaskLessons:
		return domain.MemoryKindTaskLessons, nil
	case domain.MemoryKindProcedure:
		return domain.MemoryKindProcedure, nil
	default:
		return "", fmt.Errorf("unsupported memory kind %q", value)
	}
}

func chooseNonEmpty(primary string, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
}

func commitFingerprint(kind domain.MemoryKind, request domain.OTRequest) string {
	return knowledgeFingerprintFromValues(string(kind), request.MemoryTitle, request.MemoryContent, strings.Join(request.EvidencePointers, ","))
}

func knowledgeFingerprintFromRequest(request domain.OTRequest) string {
	return knowledgeFingerprintFromValues(request.SkillName, request.SkillSummary, request.SkillContent)
}

func knowledgeFingerprintFromValues(parts ...string) string {
	h := sha1.New()
	for _, part := range parts {
		h.Write([]byte(strings.TrimSpace(part)))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
