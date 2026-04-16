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

func validateOTRequest(record domain.RunRecord, request domain.OTRequest) error {
	role := normalizeRecordRole(record)
	op := strings.TrimSpace(request.Op)
	if op == "" {
		return fmt.Errorf("ot.op is required")
	}

	if record.Mode == domain.RunModePlan {
		switch op {
		case "context", "task_list", "task_get", "read", "list", "search", "session_search", "memory_search", "skill_list", "skill_get":
			return validatePathRequest(op, request)
		default:
			return fmt.Errorf("plan mode only allows context, task_list, task_get, read, list, search, session_search, memory_search, skill_list, and skill_get operations")
		}
	}

	switch role {
	case domain.AgentRoleWorker:
		switch op {
		case "context", "task_list", "task_get", "read", "list", "search", "session_search", "memory_search", "skill_list", "skill_get":
			return validatePathRequest(op, request)
		case "write":
			if strings.TrimSpace(request.Path) == "" {
				return fmt.Errorf("write requires path")
			}
			if request.Content == "" {
				return fmt.Errorf("write requires content")
			}
			return nil
		case "patch":
			if strings.TrimSpace(request.Patch) == "" {
				return fmt.Errorf("patch requires patch content")
			}
			return nil
		case "check":
			switch strings.TrimSpace(request.Check) {
			case "go_test", "go_vet", "golangci_lint":
				return nil
			default:
				return fmt.Errorf("unsupported check %q", request.Check)
			}
		case "complete", "fail":
			return nil
		default:
			return fmt.Errorf("worker role does not allow ot op %q", request.Op)
		}
	default:
		switch op {
		case "delegate":
			if strings.TrimSpace(request.TaskContract) == "" {
				return fmt.Errorf("delegate requires task_contract")
			}
			if strings.TrimSpace(request.TaskTitle) == "" {
				return fmt.Errorf("delegate requires task_title")
			}
			return nil
		case "memory_commit":
			if strings.TrimSpace(request.MemoryKind) == "" {
				return fmt.Errorf("memory_commit requires memory_kind")
			}
			if strings.TrimSpace(request.MemoryTitle) == "" {
				return fmt.Errorf("memory_commit requires memory_title")
			}
			if strings.TrimSpace(request.MemoryContent) == "" {
				return fmt.Errorf("memory_commit requires memory_content")
			}
			return nil
		case "skill_propose":
			if strings.TrimSpace(request.SkillName) == "" {
				return fmt.Errorf("skill_propose requires skill_name")
			}
			if strings.TrimSpace(request.SkillSummary) == "" {
				return fmt.Errorf("skill_propose requires skill_summary")
			}
			if strings.TrimSpace(request.SkillContent) == "" {
				return fmt.Errorf("skill_propose requires skill_content")
			}
			return nil
		case "context", "task_list", "task_get", "read", "list", "search", "session_search", "memory_search", "skill_list", "skill_get":
			return validatePathRequest(op, request)
		default:
			return fmt.Errorf("gateway role does not allow ot op %q", request.Op)
		}
	}
}

func validatePathRequest(op string, request domain.OTRequest) error {
	switch op {
	case "context", "task_list", "skill_list":
		return nil
	case "task_get":
		if strings.TrimSpace(request.TaskID) == "" {
			return fmt.Errorf("task_get requires task_id")
		}
		return nil
	case "session_search", "memory_search":
		if strings.TrimSpace(request.Query) == "" {
			return fmt.Errorf("%s requires query", op)
		}
		return nil
	case "skill_get":
		if strings.TrimSpace(request.SkillID) == "" {
			return fmt.Errorf("skill_get requires skill_id")
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

func classifyOTApproval(record domain.RunRecord, settings domain.Settings, request domain.OTRequest) (bool, string, error) {
	switch request.Op {
	case "context", "task_list", "task_get", "read", "list", "search", "session_search", "memory_search", "skill_list", "skill_get", "delegate", "complete", "fail":
		return false, "", nil
	case "write":
		return true, "ot write requires approval.", nil
	case "patch":
		return true, "ot patch requires approval.", nil
	case "check":
		if settings.SelfDrivingMode && normalizeRecordRole(record) == domain.AgentRoleWorker {
			return false, "", nil
		}
		return true, "ot check requires approval.", nil
	case "memory_commit":
		return true, "ot memory_commit requires approval.", nil
	case "skill_propose":
		return true, "ot skill_propose requires approval.", nil
	default:
		return false, "", fmt.Errorf("unsupported ot op %q", request.Op)
	}
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

func terminalTaskMessage(request domain.OTRequest, fallback string) string {
	if message := strings.TrimSpace(request.Message); message != "" {
		return message
	}
	if summary := strings.TrimSpace(request.Summary); summary != "" {
		return summary
	}
	return fallback
}
