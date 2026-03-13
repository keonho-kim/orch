package prompts

import (
	"path/filepath"
	"strings"

	"orch/domain"
)

const planSystemPrompt = `
You are the planning session for this workspace.

Produce a concrete implementation plan only.
Do not execute the plan.
On each iteration, verify the product contract and bootstrap guidance again.
Use only these commands through the exec tool:
- cd <path>
- ot read --path <path>

Do not mutate files, run builds, or use any command other than cd and ot read.
Use bootstrap/USER.md only for durable user information.
Use bootstrap/SKILLS.md as the skill index and inspect only relevant bootstrap/skills/<skill-name>/ directories.
`

const reactSystemPrompt = `
You are the ReAct deep execution session for this workspace.

Before acting, verify the current request against the product contract, bootstrap guidance, and any cached plan.
Use bootstrap/USER.md only for durable user information.
Use bootstrap/SKILLS.md as the skill index and inspect only relevant bootstrap/skills/<skill-name>/ directories.
Use the provided exec tool instead of assuming file contents.
Prefer ot read --path <path> for file and directory inspection.
Use rg or find for search and discovery tasks.
Use custom tools/*.sh scripts only when the curated OT commands do not cover the task.
Prefer small, auditable changes and explain your intent through tool usage.
Do not use shell interpolation. Use argv-style command arguments only.
`

func SystemPrompt(mode domain.RunMode) string {
	if mode == domain.RunModePlan {
		return strings.TrimSpace(planSystemPrompt)
	}
	return strings.TrimSpace(reactSystemPrompt)
}

func IterationContext(
	record domain.RunRecord,
	product string,
	agents string,
	user string,
	skills string,
	activePlan domain.PlanCache,
	draftPlan string,
) string {
	sections := []string{
		"Current working directory:\n" + DisplayWorkspacePath(record.WorkspacePath, record.CurrentCwd),
		"PRODUCT.md:\n" + strings.TrimSpace(product),
		"AGENTS.md:\n" + strings.TrimSpace(agents),
		"bootstrap/USER.md:\n" + strings.TrimSpace(user),
		"bootstrap/SKILLS.md:\n" + strings.TrimSpace(skills),
	}

	switch record.Mode {
	case domain.RunModePlan:
		if strings.TrimSpace(draftPlan) != "" {
			sections = append(sections, "Current draft plan:\n"+strings.TrimSpace(draftPlan))
		}
	case domain.RunModeReact:
		if strings.TrimSpace(activePlan.Content) != "" {
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
