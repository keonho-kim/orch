package apiserver

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/keonho-kim/orch/internal/orchestrator"
)

type statusResponse struct {
	Status orchestrator.Status `json:"status"`
	API    Discovery           `json:"api"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{
		Status: s.service.Status(),
		API:    s.discovery,
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	flusher, ok := writeSSEHeaders(w)
	if !ok {
		return
	}

	initial := orchestrator.ServiceEvent{
		Type:      "snapshot",
		SessionID: s.discovery.SessionID,
		Message:   "initial snapshot",
		Payload: map[string]any{
			"status": statusResponse{
				Status: s.service.Status(),
				API:    s.discovery,
			},
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
			if err := writeSSEEvent(w, flusher, event); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			writeError(w, http.StatusBadRequest, "limit must be >= 0")
			return
		}
		limit = value
	}
	sessions, err := s.service.ListSessions(limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

func (s *Server) handleHistoryLatest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sessionID, err := s.service.LatestSessionID()
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"session_id": sessionID})
}

func (s *Server) handleHistoryRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		SessionID string `json:"session_id"`
	}
	if err := jsonDecode(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(body.SessionID) == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if err := s.service.RestoreSession(body.SessionID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, statusResponse{
		Status: s.service.Status(),
		API:    s.discovery,
	})
}
