package prompts

import (
	"path/filepath"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/adapters"
)

const planSystemPrompt = `
You are the planning session for this workspace.

Produce a concrete implementation plan only.
Do not execute the plan.
On each iteration, verify the product contract and bootstrap guidance again.
Use only these commands through the exec tool:
- cd <path>
- ot read --path <path>
- ot list [--path <path>]
- ot search [--path <path>] [--name <glob>] [--content <pattern>]

Do not mutate files, run builds, or use any command other than cd, ot read, ot list, and ot search.
Use bootstrap/USER.md only for durable user information.
Use bootstrap/SKILLS.md as the skill index and inspect only relevant bootstrap/skills/<skill-name>/ directories.
User prompts may include @filename or #dir-name references. Treat them as resolved workspace references when they appear in the dynamic context.
When a user explicitly names $<skill-name>, prioritize that skill's instructions.
`

const reactSystemPrompt = `
You are the ReAct deep execution session for this workspace.

Before acting, verify the current request against the product contract, bootstrap guidance, and any cached plan.
Use bootstrap/USER.md only for durable user information.
Use bootstrap/SKILLS.md as the skill index and inspect only relevant bootstrap/skills/<skill-name>/ directories.
User prompts may include @filename or #dir-name references. Treat them as resolved workspace references when they appear in the dynamic context.
When a user explicitly names $<skill-name>, prioritize that skill's instructions.
Use the provided exec tool instead of assuming file contents.
Prefer ot read --path <path> for file content and quick directory inspection.
Prefer ot list for long directory listings and ot search for curated name/content search.
Use ot pointer --value <ot-pointer> when compact or chatHistory text references a current-session transcript pointer that needs the raw JSONL line contents.
Use ot subagent --prompt <task> when a bounded child run should work in a separate child session.
Use rg or find directly only when task execution needs search behavior outside the curated OT commands.
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
	selectedSkills string,
	chatHistory string,
	resolvedReferences string,
	activePlan domain.PlanCache,
	draftPlan string,
) string {
	sections := []string{
		"Current working directory:\n" + DisplayWorkspacePath(record.WorkspacePath, record.CurrentCwd),
		"PRODUCT.md:\n" + strings.TrimSpace(product),
		"AGENTS.md:\n" + strings.TrimSpace(agents),
		"bootstrap/USER.md:\n" + strings.TrimSpace(user),
		"bootstrap/SKILLS.md:\n" + strings.TrimSpace(skills),
		"Available tools for this call:\n" + adapters.ToolSummary(record.Mode),
	}
	if strings.TrimSpace(selectedSkills) != "" {
		sections = append(sections, "Selected skill content for this call:\n"+strings.TrimSpace(selectedSkills))
	}
	if strings.TrimSpace(chatHistory) != "" {
		sections = append(sections, ".orch/chatHistory.md:\n"+strings.TrimSpace(chatHistory))
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
