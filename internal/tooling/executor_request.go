package tooling

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/keonho-kim/orch/domain"
)

func decodeOTRequest(call domain.ToolCall) (domain.OTRequest, error) {
	if call.Name != "ot" {
		return domain.OTRequest{}, fmt.Errorf("unsupported tool %q", call.Name)
	}

	var request domain.OTRequest
	if err := json.Unmarshal([]byte(call.Arguments), &request); err != nil {
		return domain.OTRequest{}, fmt.Errorf("decode ot request: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(call.Arguments), &raw); err == nil {
		_, request.WaitProvided = raw["wait"]
	}
	request.Op = strings.TrimSpace(strings.ToLower(request.Op))
	if request.Op == "" {
		return domain.OTRequest{}, fmt.Errorf("ot.op is required")
	}
	if request.StartLine < 0 || request.EndLine < 0 {
		return domain.OTRequest{}, fmt.Errorf("line ranges must be >= 0")
	}
	return request, nil
}

func validatePathRequest(op string, request domain.OTRequest) error {
	switch op {
	case "context":
		return nil
	case "task_list":
		return nil
	case "task_get":
		if strings.TrimSpace(request.TaskID) == "" {
			return fmt.Errorf("task_get requires task_id")
		}
		return nil
	}
	if op != "list" && strings.TrimSpace(request.Path) == "" {
		return fmt.Errorf("%s requires path", op)
	}
	if op == "search" && strings.TrimSpace(request.NamePattern) == "" && strings.TrimSpace(request.ContentPattern) == "" {
		return fmt.Errorf("search requires name_pattern or content_pattern")
	}
	if op == "read" && request.EndLine > 0 && request.StartLine > request.EndLine {
		return fmt.Errorf("start_line must be <= end_line")
	}
	return nil
}

func terminalTaskMessage(request domain.OTRequest, fallback string) string {
	if message := strings.TrimSpace(request.Message); message != "" {
		return message
	}
	if summary := strings.TrimSpace(request.Summary); summary != "" {
		return summary
	}
	return fallback
}

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

func normalizeTaskID(request domain.OTRequest) string {
	if strings.TrimSpace(request.TaskID) != "" {
		return strings.TrimSpace(request.TaskID)
	}
	title := strings.TrimSpace(request.TaskTitle)
	if title == "" {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "/", "-")
	return fmt.Sprintf("%s-%d", slug, time.Now().UnixNano())
}

func normalizeRecordRole(record domain.RunRecord) domain.AgentRole {
	role, err := domain.ParseAgentRole(record.AgentRole.String())
	if err != nil {
		return domain.AgentRoleGateway
	}
	return role
}
