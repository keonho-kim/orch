package apiserver

import (
	"net/http"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/orchestrator"
)

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/exec" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var body struct {
		Prompt string `json:"prompt"`
		Mode   string `json:"mode"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mode, err := domain.ParseRunMode(body.Mode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	runID, err := s.service.SubmitPromptMode(body.Prompt, mode)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	status := s.service.Status()
	writeJSON(w, http.StatusAccepted, map[string]string{
		"run_id":       runID,
		"session_id":   status.CurrentSession.SessionID,
		"status_url":   s.discovery.BaseURL + "/v1/exec/" + runID,
		"events_url":   s.discovery.BaseURL + "/v1/exec/" + runID + "/events",
		"approval_url": s.discovery.BaseURL + "/v1/exec/" + runID + "/approval",
	})
}

func (s *Server) handleExecPath(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/v1/exec/")
	runID, remainder, _ := strings.Cut(trimmed, "/")
	runID = strings.TrimSpace(runID)
	if runID == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	switch {
	case remainder == "":
		s.handleExecStatus(w, r, runID)
	case remainder == "events":
		s.handleExecEvents(w, r, runID)
	case remainder == "approval":
		s.handleExecApproval(w, r, runID)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (s *Server) handleExecStatus(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	snapshot, err := s.service.RunSnapshot(runID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleExecEvents(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	flusher, ok := writeSSEHeaders(w)
	if !ok {
		return
	}

	snapshot, err := s.service.RunSnapshot(runID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	initial := orchestrator.ServiceEvent{
		Type:      "snapshot",
		SessionID: snapshot.Record.SessionID,
		RunID:     runID,
		Message:   "initial snapshot",
		Payload: map[string]any{
			"run": snapshot,
		},
	}
	if err := writeSSEEvent(w, flusher, initial); err != nil {
		return
	}

	events, cancel := s.service.SubscribeEvents()
	defer cancel()
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.RunID != runID {
				continue
			}
			if err := writeSSEEvent(w, flusher, event); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleExecApproval(w http.ResponseWriter, r *http.Request, runID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		Approved bool `json:"approved"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.service.ResolveApproval(runID, body.Approved); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"approved": body.Approved})
}
