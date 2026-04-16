package domain

import "time"

type MemoryKind string

const (
	MemoryKindUserProfile    MemoryKind = "user_profile"
	MemoryKindWorkspaceFacts MemoryKind = "workspace_facts"
	MemoryKindTaskLessons    MemoryKind = "task_lessons"
	MemoryKindProcedure      MemoryKind = "procedure_memory"
)

type MemoryStatus string

const (
	MemoryStatusActive MemoryStatus = "active"
	MemoryStatusStale  MemoryStatus = "stale"
)

type SkillStatus string

const (
	SkillStatusDraft     SkillStatus = "draft"
	SkillStatusPublished SkillStatus = "published"
)

type PromotionJobStatus string

const (
	PromotionJobPending  PromotionJobStatus = "pending_validation"
	PromotionJobReady    PromotionJobStatus = "ready_for_review"
	PromotionJobPromoted PromotionJobStatus = "promoted"
	PromotionJobRejected PromotionJobStatus = "rejected"
)

type MemoryEntry struct {
	ID               int64        `json:"id"`
	Kind             MemoryKind   `json:"kind"`
	WorkspacePath    string       `json:"workspace_path,omitempty"`
	SourceSessionID  string       `json:"source_session_id,omitempty"`
	SourceRunID      string       `json:"source_run_id,omitempty"`
	Title            string       `json:"title"`
	Content          string       `json:"content"`
	EvidencePointers []string     `json:"evidence_pointers,omitempty"`
	Status           MemoryStatus `json:"status"`
	Fingerprint      string       `json:"fingerprint,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

type MemorySearchResult struct {
	Entry           MemoryEntry `json:"entry"`
	SelectionReason string      `json:"selection_reason,omitempty"`
	Score           int         `json:"score,omitempty"`
}

type MemorySnapshot struct {
	Entries []MemoryEntry `json:"entries,omitempty"`
	Skills  []SkillRecord `json:"skills,omitempty"`
}

type SkillRecord struct {
	SkillID        string      `json:"skill_id"`
	WorkspacePath  string      `json:"workspace_path,omitempty"`
	Name           string      `json:"name"`
	Summary        string      `json:"summary"`
	Content        string      `json:"content"`
	Status         SkillStatus `json:"status"`
	SourceMemoryID int64       `json:"source_memory_id,omitempty"`
	Fingerprint    string      `json:"fingerprint,omitempty"`
	Version        int         `json:"version"`
	ReplayCount    int         `json:"replay_count"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type SkillVersion struct {
	ID             int64     `json:"id"`
	SkillID        string    `json:"skill_id"`
	Version        int       `json:"version"`
	Summary        string    `json:"summary"`
	Content        string    `json:"content"`
	SourceMemoryID int64     `json:"source_memory_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type PromotionJob struct {
	ID                    int64              `json:"id"`
	JobType               string             `json:"job_type"`
	Status                PromotionJobStatus `json:"status"`
	MemoryID              int64              `json:"memory_id,omitempty"`
	SkillID               string             `json:"skill_id,omitempty"`
	Fingerprint           string             `json:"fingerprint,omitempty"`
	ReplayCount           int                `json:"replay_count"`
	RequiredEvidenceCount int                `json:"required_evidence_count"`
	Notes                 string             `json:"notes,omitempty"`
	CreatedAt             time.Time          `json:"created_at"`
	UpdatedAt             time.Time          `json:"updated_at"`
}

type TaskOutcome struct {
	ID               int64     `json:"id"`
	SessionID        string    `json:"session_id"`
	RunID            string    `json:"run_id"`
	TaskID           string    `json:"task_id,omitempty"`
	Title            string    `json:"title,omitempty"`
	Status           string    `json:"status"`
	Summary          string    `json:"summary,omitempty"`
	ChangedPaths     []string  `json:"changed_paths,omitempty"`
	ChecksRun        []string  `json:"checks_run,omitempty"`
	EvidencePointers []string  `json:"evidence_pointers,omitempty"`
	Followups        []string  `json:"followups,omitempty"`
	ErrorKind        string    `json:"error_kind,omitempty"`
	Fingerprint      string    `json:"fingerprint,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type SessionSearchResult struct {
	SessionID       string    `json:"session_id"`
	RunID           string    `json:"run_id,omitempty"`
	RecordSeq       int64     `json:"record_seq,omitempty"`
	RecordType      string    `json:"record_type,omitempty"`
	Title           string    `json:"title,omitempty"`
	Snippet         string    `json:"snippet,omitempty"`
	EvidencePointer string    `json:"evidence_pointer,omitempty"`
	SelectionReason string    `json:"selection_reason,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
