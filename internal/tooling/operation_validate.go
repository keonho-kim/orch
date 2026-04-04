package tooling

import (
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

func validateOTRequest(record domain.RunRecord, request domain.OTRequest) error {
	op := strings.TrimSpace(request.Op)
	if op == "" {
		return fmt.Errorf("ot.op is required")
	}
	spec, ok := lookupOTOperation(op)
	if !ok {
		return fmt.Errorf("unsupported ot op %q", request.Op)
	}
	if record.Mode == domain.RunModePlan && !spec.PlanAllowed {
		return fmt.Errorf("plan mode only allows context, task_list, task_get, read, list, and search operations")
	}
	role := normalizeRecordRole(record)
	if role == domain.AgentRoleWorker && !spec.WorkerAllowed {
		return fmt.Errorf("worker role does not allow ot op %q", request.Op)
	}
	if role != domain.AgentRoleWorker && !spec.GatewayAllowed {
		return fmt.Errorf("gateway role does not allow ot op %q", request.Op)
	}
	return spec.Validate(request)
}

func validateNoopRequest(domain.OTRequest) error {
	return nil
}

func validateContextRequest(request domain.OTRequest) error {
	return validatePathRequest("context", request)
}

func validateTaskListRequest(request domain.OTRequest) error {
	return validatePathRequest("task_list", request)
}

func validateTaskGetRequest(request domain.OTRequest) error {
	return validatePathRequest("task_get", request)
}

func validateReadRequest(request domain.OTRequest) error {
	return validatePathRequest("read", request)
}

func validateListRequest(request domain.OTRequest) error {
	return validatePathRequest("list", request)
}

func validateSearchRequest(request domain.OTRequest) error {
	return validatePathRequest("search", request)
}

func validateWriteRequest(request domain.OTRequest) error {
	if strings.TrimSpace(request.Path) == "" {
		return fmt.Errorf("write requires path")
	}
	if request.Content == "" {
		return fmt.Errorf("write requires content")
	}
	return nil
}

func validatePatchRequest(request domain.OTRequest) error {
	if strings.TrimSpace(request.Patch) == "" {
		return fmt.Errorf("patch requires patch content")
	}
	return nil
}

func validateCheckRequest(request domain.OTRequest) error {
	switch strings.TrimSpace(request.Check) {
	case "go_test", "go_vet", "golangci_lint":
		return nil
	default:
		return fmt.Errorf("unsupported check %q", request.Check)
	}
}

func validateDelegateRequest(request domain.OTRequest) error {
	if strings.TrimSpace(request.TaskContract) == "" {
		return fmt.Errorf("delegate requires task_contract")
	}
	if strings.TrimSpace(request.TaskTitle) == "" {
		return fmt.Errorf("delegate requires task_title")
	}
	return nil
}
