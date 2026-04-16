package prompts

import (
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

const (
	planModeSuffix = `
Plan mode is read-only.
Use only read, list, and search operations.
Do not delegate, write, patch, or run checks.
Return a concrete plan only.
`

	reactGatewaySuffix = `
You are running as the gateway agent.
Use delegation for executable work whenever practical.
Do not mutate files directly.
Do not run validation checks directly.
`

	reactWorkerSuffix = `
You are running as the worker agent.
You must stay inside the assigned task contract.
Do not delegate.
Do not broaden scope beyond the assigned task.
`
)

func SystemPrompt(mode domain.RunMode, role domain.AgentRole, common string, roleSpecific string) string {
	sections := make([]string, 0, 4)
	if strings.TrimSpace(common) != "" {
		sections = append(sections, strings.TrimSpace(common))
	}
	if strings.TrimSpace(roleSpecific) != "" {
		sections = append(sections, strings.TrimSpace(roleSpecific))
	}

	if mode == domain.RunModePlan {
		sections = append(sections, strings.TrimSpace(planModeSuffix))
	} else if role == domain.AgentRoleWorker {
		sections = append(sections, strings.TrimSpace(reactWorkerSuffix))
	} else {
		sections = append(sections, strings.TrimSpace(reactGatewaySuffix))
	}

	return strings.Join(sections, "\n\n")
}

func IterationContext(
	record domain.RunRecord,
	role domain.AgentRole,
	tools string,
	user string,
	chatHistory string,
	frozenKnowledge string,
	selectedSkills string,
	resolvedReferences string,
	activePlan domain.PlanCache,
	draftPlan string,
	taskTitle string,
	taskContract string,
	taskStatus string,
) string {
	sections := []string{
		"Agent role:\n" + role.DisplayName(),
		"Current working directory:\n" + DisplayWorkspacePath(record.WorkspacePath, record.CurrentCwd),
	}
	if strings.TrimSpace(tools) != "" {
		sections = append(sections, "bootstrap/TOOLS.md:\n"+strings.TrimSpace(tools))
	}
	if strings.TrimSpace(user) != "" {
		sections = append(sections, "bootstrap/USER.md:\n"+strings.TrimSpace(user))
	}
	if strings.TrimSpace(chatHistory) != "" {
		sections = append(sections, ".orch/chatHistory.md:\n"+strings.TrimSpace(chatHistory))
	}
	if strings.TrimSpace(frozenKnowledge) != "" {
		sections = append(sections, "Frozen memory snapshot:\n"+strings.TrimSpace(frozenKnowledge))
	}
	if strings.TrimSpace(taskTitle) != "" {
		sections = append(sections, "Assigned task title:\n"+strings.TrimSpace(taskTitle))
	}
	if strings.TrimSpace(taskStatus) != "" {
		sections = append(sections, "Assigned task status:\n"+strings.TrimSpace(taskStatus))
	}
	if strings.TrimSpace(taskContract) != "" {
		sections = append(sections, "Assigned task contract:\n"+strings.TrimSpace(taskContract))
	}
	if strings.TrimSpace(selectedSkills) != "" {
		sections = append(sections, "Selected skill content for this call:\n"+strings.TrimSpace(selectedSkills))
	}
	if strings.TrimSpace(resolvedReferences) != "" {
		sections = append(sections, "Resolved workspace references for this request:\n"+strings.TrimSpace(resolvedReferences))
	}

	switch record.Mode {
	case domain.RunModePlan:
		if strings.TrimSpace(draftPlan) != "" {
			sections = append(sections, "Current draft plan:\n"+strings.TrimSpace(draftPlan))
		}
	case domain.RunModeReact:
		if role == domain.AgentRoleGateway && strings.TrimSpace(activePlan.Content) != "" {
			sections = append(sections, "Active cached plan:\n"+strings.TrimSpace(activePlan.Content))
		}
	}

	return strings.Join(sections, "\n\n")
}

func DisplayWorkspacePath(workspaceRoot string, currentCwd string) string {
	if strings.TrimSpace(currentCwd) == "" {
		return "."
	}
	rel, err := filepath.Rel(workspaceRoot, currentCwd)
	if err != nil || rel == "." {
		return "."
	}
	return filepath.ToSlash(rel)
}
