package tooling

import (
	"context"
	"fmt"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

type Executor struct {
	ot              *OTRunner
	contextSnapshot func(domain.RunRecord) (domain.ContextSnapshot, error)
	listTasks       func(domain.RunRecord, string) ([]domain.TaskView, error)
	getTask         func(domain.RunRecord, string) (domain.TaskView, error)
	sessionSearch   func(domain.RunRecord, string, int) ([]domain.SessionSearchResult, error)
	memorySearch    func(domain.RunRecord, string, int) ([]domain.MemorySearchResult, error)
	listSkills      func(domain.RunRecord, string, int) ([]domain.SkillRecord, error)
	getSkill        func(domain.RunRecord, string) (domain.SkillRecord, error)
	commitMemory    func(domain.RunRecord, domain.OTRequest) (domain.MemoryEntry, error)
	proposeSkill    func(domain.RunRecord, domain.OTRequest) (domain.SkillRecord, error)
}

type Execution struct {
	Output               string
	RequiresApproval     bool
	Reason               string
	NextCwd              string
	TerminalStatus       domain.RunStatus
	TerminalMessage      string
	TaskSummary          string
	TaskChangedPaths     []string
	TaskChecksRun        []string
	TaskEvidencePointers []string
	TaskFollowups        []string
	TaskErrorKind        string
}

func NewExecutor() *Executor {
	return &Executor{ot: NewOTRunner()}
}

func (e *Executor) SetOTEnvResolver(resolver func() map[string]string) {
	if e == nil || e.ot == nil {
		return
	}
	e.ot.SetExtraEnvResolver(resolver)
}

func (e *Executor) SetStateResolvers(
	contextSnapshot func(domain.RunRecord) (domain.ContextSnapshot, error),
	listTasks func(domain.RunRecord, string) ([]domain.TaskView, error),
	getTask func(domain.RunRecord, string) (domain.TaskView, error),
	sessionSearch func(domain.RunRecord, string, int) ([]domain.SessionSearchResult, error),
	memorySearch func(domain.RunRecord, string, int) ([]domain.MemorySearchResult, error),
	listSkills func(domain.RunRecord, string, int) ([]domain.SkillRecord, error),
	getSkill func(domain.RunRecord, string) (domain.SkillRecord, error),
	commitMemory func(domain.RunRecord, domain.OTRequest) (domain.MemoryEntry, error),
	proposeSkill func(domain.RunRecord, domain.OTRequest) (domain.SkillRecord, error),
) {
	e.contextSnapshot = contextSnapshot
	e.listTasks = listTasks
	e.getTask = getTask
	e.sessionSearch = sessionSearch
	e.memorySearch = memorySearch
	e.listSkills = listSkills
	e.getSkill = getSkill
	e.commitMemory = commitMemory
	e.proposeSkill = proposeSkill
}

func (e *Executor) DecodeOTRequest(call domain.ToolCall) (domain.OTRequest, error) {
	return decodeOTRequest(call)
}

func (e *Executor) Review(workspaceRoot string, record domain.RunRecord, env []string, settings domain.Settings, call domain.ToolCall) (Execution, error) {
	request, err := decodeOTRequest(call)
	if err != nil {
		return Execution{}, err
	}
	if err := validateOTRequest(record, request); err != nil {
		return Execution{}, err
	}

	requiresApproval, reason, err := classifyOTApproval(record, settings, request)
	if err != nil {
		return Execution{}, err
	}
	return Execution{
		RequiresApproval: requiresApproval,
		Reason:           reason,
	}, nil
}

func (e *Executor) Execute(ctx context.Context, workspaceRoot string, record domain.RunRecord, env []string, call domain.ToolCall) (Execution, error) {
	request, err := decodeOTRequest(call)
	if err != nil {
		return Execution{}, err
	}
	if err := validateOTRequest(record, request); err != nil {
		return Execution{}, err
	}

	switch strings.TrimSpace(request.Op) {
	case "context":
		if e.contextSnapshot == nil {
			return Execution{}, fmt.Errorf("context snapshot resolver is not configured")
		}
		snapshot, err := e.contextSnapshot(record)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: renderContextSnapshot(snapshot)}, nil
	case "task_list":
		if e.listTasks == nil {
			return Execution{}, fmt.Errorf("task list resolver is not configured")
		}
		tasks, err := e.listTasks(record, request.StatusFilter)
		if err != nil {
			return Execution{}, err
		}
		output, err := renderJSON(tasks)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "task_get":
		if e.getTask == nil {
			return Execution{}, fmt.Errorf("task resolver is not configured")
		}
		task, err := e.getTask(record, request.TaskID)
		if err != nil {
			return Execution{}, err
		}
		output, err := renderJSON(task)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "session_search":
		if e.sessionSearch == nil {
			return Execution{}, fmt.Errorf("session search resolver is not configured")
		}
		results, err := e.sessionSearch(record, request.Query, request.Limit)
		if err != nil {
			return Execution{}, err
		}
		output, err := renderJSON(results)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "memory_search":
		if e.memorySearch == nil {
			return Execution{}, fmt.Errorf("memory search resolver is not configured")
		}
		results, err := e.memorySearch(record, request.Query, request.Limit)
		if err != nil {
			return Execution{}, err
		}
		output, err := renderJSON(results)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "skill_list":
		if e.listSkills == nil {
			return Execution{}, fmt.Errorf("skill list resolver is not configured")
		}
		skills, err := e.listSkills(record, request.StatusFilter, request.Limit)
		if err != nil {
			return Execution{}, err
		}
		output, err := renderJSON(skills)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "skill_get":
		if e.getSkill == nil {
			return Execution{}, fmt.Errorf("skill resolver is not configured")
		}
		skill, err := e.getSkill(record, request.SkillID)
		if err != nil {
			return Execution{}, err
		}
		output, err := renderJSON(skill)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "read":
		return e.executeRead(ctx, workspaceRoot, record, env, request)
	case "list":
		return e.executeList(ctx, workspaceRoot, record, env, request)
	case "search":
		return e.executeSearch(ctx, workspaceRoot, record, env, request)
	case "write":
		return e.executeWrite(ctx, workspaceRoot, record, env, request)
	case "patch":
		return e.executePatch(ctx, workspaceRoot, record, env, request)
	case "check":
		return e.executeCheck(ctx, workspaceRoot, record, env, request)
	case "delegate":
		wait := !request.WaitProvided || request.Wait
		output, err := e.ot.RunDelegateTask(ctx, workspaceRoot, record, env, domain.SubagentTask{
			ID:       normalizeTaskID(request),
			Title:    strings.TrimSpace(request.TaskTitle),
			Contract: strings.TrimSpace(request.TaskContract),
		}, wait)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "memory_commit":
		if e.commitMemory == nil {
			return Execution{}, fmt.Errorf("memory commit resolver is not configured")
		}
		entry, err := e.commitMemory(record, request)
		if err != nil {
			return Execution{}, err
		}
		output, err := renderJSON(entry)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "skill_propose":
		if e.proposeSkill == nil {
			return Execution{}, fmt.Errorf("skill propose resolver is not configured")
		}
		skill, err := e.proposeSkill(record, request)
		if err != nil {
			return Execution{}, err
		}
		output, err := renderJSON(skill)
		if err != nil {
			return Execution{}, err
		}
		return Execution{Output: output}, nil
	case "complete":
		message := terminalTaskMessage(request, "Worker task completed.")
		return Execution{
			Output:               renderTaskOutcomeOutput(request, message),
			TerminalStatus:       domain.StatusCompleted,
			TerminalMessage:      message,
			TaskSummary:          strings.TrimSpace(request.Summary),
			TaskChangedPaths:     append([]string(nil), request.ChangedPaths...),
			TaskChecksRun:        append([]string(nil), request.ChecksRun...),
			TaskEvidencePointers: append([]string(nil), request.EvidencePointers...),
			TaskFollowups:        append([]string(nil), request.Followups...),
			TaskErrorKind:        strings.TrimSpace(request.ErrorKind),
		}, nil
	case "fail":
		message := terminalTaskMessage(request, "Worker task failed.")
		return Execution{
			Output:               renderTaskOutcomeOutput(request, message),
			TerminalStatus:       domain.StatusFailed,
			TerminalMessage:      message,
			TaskSummary:          strings.TrimSpace(request.Summary),
			TaskChangedPaths:     append([]string(nil), request.ChangedPaths...),
			TaskChecksRun:        append([]string(nil), request.ChecksRun...),
			TaskEvidencePointers: append([]string(nil), request.EvidencePointers...),
			TaskFollowups:        append([]string(nil), request.Followups...),
			TaskErrorKind:        strings.TrimSpace(request.ErrorKind),
		}, nil
	default:
		return Execution{}, fmt.Errorf("unsupported ot op %q", request.Op)
	}
}
