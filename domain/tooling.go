package domain

type OTRequest struct {
	Op               string   `json:"op"`
	Path             string   `json:"path,omitempty"`
	StartLine        int      `json:"start_line,omitempty"`
	EndLine          int      `json:"end_line,omitempty"`
	Query            string   `json:"query,omitempty"`
	Limit            int      `json:"limit,omitempty"`
	NamePattern      string   `json:"name_pattern,omitempty"`
	ContentPattern   string   `json:"content_pattern,omitempty"`
	Content          string   `json:"content,omitempty"`
	Patch            string   `json:"patch,omitempty"`
	Check            string   `json:"check,omitempty"`
	MemoryKind       string   `json:"memory_kind,omitempty"`
	MemoryTitle      string   `json:"memory_title,omitempty"`
	MemoryContent    string   `json:"memory_content,omitempty"`
	SkillID          string   `json:"skill_id,omitempty"`
	SkillName        string   `json:"skill_name,omitempty"`
	SkillSummary     string   `json:"skill_summary,omitempty"`
	SkillContent     string   `json:"skill_content,omitempty"`
	TaskID           string   `json:"task_id,omitempty"`
	TaskTitle        string   `json:"task_title,omitempty"`
	TaskContract     string   `json:"task_contract,omitempty"`
	Message          string   `json:"message,omitempty"`
	Wait             bool     `json:"wait,omitempty"`
	WaitProvided     bool     `json:"-"`
	StatusFilter     string   `json:"status_filter,omitempty"`
	Summary          string   `json:"summary,omitempty"`
	ChangedPaths     []string `json:"changed_paths,omitempty"`
	ChecksRun        []string `json:"checks_run,omitempty"`
	EvidencePointers []string `json:"evidence_pointers,omitempty"`
	Followups        []string `json:"followups,omitempty"`
	ErrorKind        string   `json:"error_kind,omitempty"`
}

type SubagentTask struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Contract      string `json:"contract"`
	StartFilePath string `json:"start_file_path,omitempty"`
}

type ToolResult struct {
	ToolCallID string
	Name       string
	Content    string
}
