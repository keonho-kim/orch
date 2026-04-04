package tooling

import (
	"fmt"

	"github.com/keonho-kim/orch/domain"
)

func classifyOTApproval(record domain.RunRecord, settings domain.Settings, request domain.OTRequest) (bool, string, error) {
	spec, ok := lookupOTOperation(request.Op)
	if !ok {
		return false, "", fmt.Errorf("unsupported ot op %q", request.Op)
	}
	return spec.Approval(record, settings, request)
}

func noApproval(domain.RunRecord, domain.Settings, domain.OTRequest) (bool, string, error) {
	return false, "", nil
}

func approvalForWrite(domain.RunRecord, domain.Settings, domain.OTRequest) (bool, string, error) {
	return true, "ot write requires approval.", nil
}

func approvalForPatch(domain.RunRecord, domain.Settings, domain.OTRequest) (bool, string, error) {
	return true, "ot patch requires approval.", nil
}

func approvalForCheck(record domain.RunRecord, settings domain.Settings, request domain.OTRequest) (bool, string, error) {
	if settings.SelfDrivingMode && normalizeRecordRole(record) == domain.AgentRoleWorker {
		return false, "", nil
	}
	return true, "ot check requires approval.", nil
}
