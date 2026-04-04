package tooling

import (
	"context"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

type otOperationSpec struct {
	PlanAllowed    bool
	WorkerAllowed  bool
	GatewayAllowed bool
	Validate       func(domain.OTRequest) error
	Approval       func(domain.RunRecord, domain.Settings, domain.OTRequest) (bool, string, error)
	Execute        func(*Executor, context.Context, string, domain.RunRecord, []string, domain.OTRequest) (Execution, error)
}

var otOperationSpecs = map[string]otOperationSpec{
	"context": {
		PlanAllowed:    true,
		WorkerAllowed:  true,
		GatewayAllowed: true,
		Validate:       validateContextRequest,
		Approval:       noApproval,
		Execute:        executeContextOperation,
	},
	"task_list": {
		PlanAllowed:    true,
		WorkerAllowed:  true,
		GatewayAllowed: true,
		Validate:       validateTaskListRequest,
		Approval:       noApproval,
		Execute:        executeTaskListOperation,
	},
	"task_get": {
		PlanAllowed:    true,
		WorkerAllowed:  true,
		GatewayAllowed: true,
		Validate:       validateTaskGetRequest,
		Approval:       noApproval,
		Execute:        executeTaskGetOperation,
	},
	"read": {
		PlanAllowed:    true,
		WorkerAllowed:  true,
		GatewayAllowed: true,
		Validate:       validateReadRequest,
		Approval:       noApproval,
		Execute:        (*Executor).executeReadOp,
	},
	"list": {
		PlanAllowed:    true,
		WorkerAllowed:  true,
		GatewayAllowed: true,
		Validate:       validateListRequest,
		Approval:       noApproval,
		Execute:        (*Executor).executeListOp,
	},
	"search": {
		PlanAllowed:    true,
		WorkerAllowed:  true,
		GatewayAllowed: true,
		Validate:       validateSearchRequest,
		Approval:       noApproval,
		Execute:        (*Executor).executeSearchOp,
	},
	"write": {
		WorkerAllowed: true,
		Validate:      validateWriteRequest,
		Approval:      approvalForWrite,
		Execute:       (*Executor).executeWriteOp,
	},
	"patch": {
		WorkerAllowed: true,
		Validate:      validatePatchRequest,
		Approval:      approvalForPatch,
		Execute:       (*Executor).executePatchOp,
	},
	"check": {
		WorkerAllowed: true,
		Validate:      validateCheckRequest,
		Approval:      approvalForCheck,
		Execute:       (*Executor).executeCheckOp,
	},
	"delegate": {
		GatewayAllowed: true,
		Validate:       validateDelegateRequest,
		Approval:       noApproval,
		Execute:        executeDelegateOperation,
	},
	"complete": {
		WorkerAllowed: true,
		Validate:      validateNoopRequest,
		Approval:      noApproval,
		Execute:       executeCompleteOperation,
	},
	"fail": {
		WorkerAllowed: true,
		Validate:      validateNoopRequest,
		Approval:      noApproval,
		Execute:       executeFailOperation,
	},
}

func lookupOTOperation(op string) (otOperationSpec, bool) {
	spec, ok := otOperationSpecs[strings.TrimSpace(op)]
	return spec, ok
}
