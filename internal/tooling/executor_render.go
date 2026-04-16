package tooling

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func renderTaskOutcomeOutput(request domain.OTRequest, message string) string {
	lines := []string{message}
	if len(request.ChangedPaths) > 0 {
		lines = append(lines, "changed_paths: "+strings.Join(request.ChangedPaths, ", "))
	}
	if len(request.ChecksRun) > 0 {
		lines = append(lines, "checks_run: "+strings.Join(request.ChecksRun, ", "))
	}
	if len(request.EvidencePointers) > 0 {
		lines = append(lines, "evidence_pointers: "+strings.Join(request.EvidencePointers, ", "))
	}
	if len(request.Followups) > 0 {
		lines = append(lines, "followups: "+strings.Join(request.Followups, " | "))
	}
	if kind := strings.TrimSpace(request.ErrorKind); kind != "" {
		lines = append(lines, "error_kind: "+kind)
	}
	return strings.Join(lines, "\n")
}

func renderContextSnapshot(snapshot domain.ContextSnapshot) string {
	lines := []string{
		"session_id: " + snapshot.SessionID,
		"run_id: " + snapshot.RunID,
		"provider: " + snapshot.Provider,
		"model: " + snapshot.Model,
		"workspace_path: " + snapshot.WorkspacePath,
		"current_cwd: " + snapshot.CurrentCwd,
		fmt.Sprintf("compact_summary_present: %t", snapshot.CompactSummaryPresent),
		fmt.Sprintf("post_compact_record_count: %d", snapshot.PostCompactRecordCount),
		fmt.Sprintf("inherited_summary_present: %t", snapshot.InheritedSummaryPresent),
		fmt.Sprintf("inherited_record_count: %d", snapshot.InheritedRecordCount),
		"selected_skills: " + strings.Join(snapshot.SelectedSkills, ", "),
		fmt.Sprintf("resolved_reference_count: %d", snapshot.ResolvedReferenceCount),
		fmt.Sprintf("user_memory_present: %t", snapshot.UserMemoryPresent),
		fmt.Sprintf("chat_history_excerpt_bytes: %d", snapshot.ChatHistoryExcerptBytes),
		fmt.Sprintf("plan_cache_present: %t", snapshot.PlanCachePresent),
		fmt.Sprintf("frozen_memory_entry_count: %d", snapshot.FrozenMemoryEntryCount),
		fmt.Sprintf("frozen_skill_count: %d", snapshot.FrozenSkillCount),
	}
	return strings.Join(lines, "\n")
}

func renderJSON(value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal ot result: %w", err)
	}
	return string(data), nil
}
