package domain

import "time"

type MessageHistoryEntry struct {
	ID        int64
	Prompt    string
	CreatedAt time.Time
}

type SessionRecordType string

const (
	SessionRecordUser      SessionRecordType = "user"
	SessionRecordAssistant SessionRecordType = "assistant"
	SessionRecordTool      SessionRecordType = "tool"
	SessionRecordCompact   SessionRecordType = "compact"
	SessionRecordTitle     SessionRecordType = "title"
	SessionRecordContext   SessionRecordType = "context_snapshot"
)

const (
	TaskStatusQueued    = "queued"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusCancelled = "cancelled"
)

type SessionRecord struct {
	Seq             int64             `json:"seq"`
	SessionID       string            `json:"session_id"`
	RunID           string            `json:"run_id,omitempty"`
	Type            SessionRecordType `json:"type"`
	Content         string            `json:"content,omitempty"`
	Title           string            `json:"title,omitempty"`
	ToolName        string            `json:"tool_name,omitempty"`
	ToolCallID      string            `json:"tool_call_id,omitempty"`
	ThroughSeq      int64             `json:"through_seq,omitempty"`
	ContextSnapshot *ContextSnapshot  `json:"context_snapshot,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	Usage           UsageStats        `json:"usage,omitempty"`
}

type SessionMetadata struct {
	SessionID            string     `json:"session_id"`
	WorkspacePath        string     `json:"workspace_path"`
	ParentSessionID      string     `json:"parent_session_id,omitempty"`
	ParentRunID          string     `json:"parent_run_id,omitempty"`
	ParentTaskID         string     `json:"parent_task_id,omitempty"`
	WorkerRole           AgentRole  `json:"worker_role,omitempty"`
	TaskTitle            string     `json:"task_title,omitempty"`
	TaskContract         string     `json:"task_contract,omitempty"`
	TaskStatus           string     `json:"task_status,omitempty"`
	TaskSummary          string     `json:"task_summary,omitempty"`
	TaskChangedPaths     []string   `json:"task_changed_paths,omitempty"`
	TaskChecksRun        []string   `json:"task_checks_run,omitempty"`
	TaskEvidencePointers []string   `json:"task_evidence_pointers,omitempty"`
	TaskFollowups        []string   `json:"task_followups,omitempty"`
	TaskErrorKind        string     `json:"task_error_kind,omitempty"`
	Provider             Provider   `json:"provider"`
	Model                string     `json:"model"`
	Title                string     `json:"title"`
	Summary              string     `json:"summary"`
	StartedAt            time.Time  `json:"started_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	LastSequence         int64      `json:"last_sequence"`
	LastCompactedSeq     int64      `json:"last_compacted_seq"`
	TokensSinceCompact   int        `json:"tokens_since_compact"`
	TotalTokens          int        `json:"total_tokens"`
	FinalizePending      bool       `json:"finalize_pending"`
	FinalizedAt          *time.Time `json:"finalized_at,omitempty"`
	LastRunID            string     `json:"last_run_id,omitempty"`
}

type SubagentResult struct {
	ChildSessionID       string   `json:"child_session_id"`
	ChildRunID           string   `json:"child_run_id"`
	TaskID               string   `json:"task_id,omitempty"`
	TaskTitle            string   `json:"task_title,omitempty"`
	TaskStatus           string   `json:"task_status,omitempty"`
	WorkerRole           string   `json:"worker_role,omitempty"`
	Status               string   `json:"status"`
	TaskSummary          string   `json:"task_summary,omitempty"`
	TaskChangedPaths     []string `json:"task_changed_paths,omitempty"`
	TaskChecksRun        []string `json:"task_checks_run,omitempty"`
	TaskEvidencePointers []string `json:"task_evidence_pointers,omitempty"`
	TaskFollowups        []string `json:"task_followups,omitempty"`
	TaskErrorKind        string   `json:"task_error_kind,omitempty"`
	FinalOutput          string   `json:"final_output"`
	Truncated            bool     `json:"truncated"`
	Error                string   `json:"error,omitempty"`
}

type TaskView struct {
	TaskID               string     `json:"task_id,omitempty"`
	Title                string     `json:"title,omitempty"`
	Status               string     `json:"status,omitempty"`
	ParentSessionID      string     `json:"parent_session_id,omitempty"`
	ParentRunID          string     `json:"parent_run_id,omitempty"`
	ChildSessionID       string     `json:"child_session_id,omitempty"`
	ChildRunID           string     `json:"child_run_id,omitempty"`
	WorkerRole           string     `json:"worker_role,omitempty"`
	Provider             string     `json:"provider,omitempty"`
	Model                string     `json:"model,omitempty"`
	TaskSummary          string     `json:"task_summary,omitempty"`
	TaskChangedPaths     []string   `json:"task_changed_paths,omitempty"`
	TaskChecksRun        []string   `json:"task_checks_run,omitempty"`
	TaskEvidencePointers []string   `json:"task_evidence_pointers,omitempty"`
	TaskFollowups        []string   `json:"task_followups,omitempty"`
	TaskErrorKind        string     `json:"task_error_kind,omitempty"`
	FinalOutputExcerpt   string     `json:"final_output_excerpt,omitempty"`
	StartedAt            time.Time  `json:"started_at,omitempty"`
	UpdatedAt            time.Time  `json:"updated_at,omitempty"`
	FinalizedAt          *time.Time `json:"finalized_at,omitempty"`
}

type ContextSnapshot struct {
	SessionID               string   `json:"session_id"`
	RunID                   string   `json:"run_id"`
	Provider                string   `json:"provider"`
	Model                   string   `json:"model"`
	WorkspacePath           string   `json:"workspace_path"`
	CurrentCwd              string   `json:"current_cwd"`
	CompactSummaryPresent   bool     `json:"compact_summary_present"`
	PostCompactRecordCount  int      `json:"post_compact_record_count"`
	InheritedSummaryPresent bool     `json:"inherited_summary_present"`
	InheritedRecordCount    int      `json:"inherited_record_count"`
	SelectedSkills          []string `json:"selected_skills,omitempty"`
	ResolvedReferenceCount  int      `json:"resolved_reference_count"`
	UserMemoryPresent       bool     `json:"user_memory_present"`
	ChatHistoryExcerptBytes int      `json:"chat_history_excerpt_bytes"`
	PlanCachePresent        bool     `json:"plan_cache_present"`
	FrozenMemoryEntryCount  int      `json:"frozen_memory_entry_count"`
	FrozenSkillCount        int      `json:"frozen_skill_count"`
}
