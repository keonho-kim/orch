package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/keonho-kim/orch/internal/orchestrator"
)

func writeSSEHeaders(w http.ResponseWriter) (http.Flusher, bool) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return nil, false
	}
	return flusher, true
}

func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event orchestrator.ServiceEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal sse event: %w", err)
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
