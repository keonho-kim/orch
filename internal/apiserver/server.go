package apiserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/orchestrator"
)

type Discovery struct {
	BaseURL   string    `json:"base_url"`
	Token     string    `json:"token"`
	PID       int       `json:"pid"`
	SessionID string    `json:"session_id"`
	RepoRoot  string    `json:"repo_root"`
	StartedAt time.Time `json:"started_at"`
}

type Server struct {
	service   *orchestrator.Service
	paths     config.Paths
	http      *http.Server
	listener  net.Listener
	discovery Discovery
	closeOnce sync.Once
}

func Start(ctx context.Context, service *orchestrator.Service, paths config.Paths) (*Server, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen api server: %w", err)
	}

	status := service.Status()
	discovery := Discovery{
		BaseURL:   "http://" + listener.Addr().String(),
		Token:     token,
		PID:       os.Getpid(),
		SessionID: status.CurrentSession.SessionID,
		RepoRoot:  paths.RepoRoot,
		StartedAt: time.Now(),
	}

	server := &Server{
		service:   service,
		paths:     paths,
		listener:  listener,
		discovery: discovery,
	}
	server.http = &http.Server{
		Handler:           server.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := server.writeDiscoveryFiles(); err != nil {
		_ = listener.Close()
		return nil, err
	}

	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	go func() {
		_ = server.http.Serve(listener)
	}()

	return server, nil
}

func (s *Server) Discovery() Discovery {
	return s.discovery
}

func (s *Server) Close() error {
	var closeErr error
	s.closeOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if s.http != nil {
			closeErr = s.http.Shutdown(ctx)
		}
		if err := s.removeDiscoveryFiles(); err != nil && closeErr == nil {
			closeErr = err
		}
	})
	return closeErr
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/v1/status", s.auth(http.HandlerFunc(s.handleStatus)))
	mux.Handle("/v1/events", s.auth(http.HandlerFunc(s.handleEvents)))
	mux.Handle("/v1/exec", s.auth(http.HandlerFunc(s.handleExec)))
	mux.Handle("/v1/exec/", s.auth(http.HandlerFunc(s.handleExecPath)))
	mux.Handle("/v1/history", s.auth(http.HandlerFunc(s.handleHistory)))
	mux.Handle("/v1/history/latest", s.auth(http.HandlerFunc(s.handleHistoryLatest)))
	mux.Handle("/v1/history/restore", s.auth(http.HandlerFunc(s.handleHistoryRestore)))
	mux.Handle("/v1/config", s.auth(http.HandlerFunc(s.handleConfig)))
	return mux
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		want := "Bearer " + s.discovery.Token
		if authHeader != want {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) discoveryFilePath() string {
	return filepath.Join(s.paths.APIDir, s.discovery.SessionID+".json")
}

func (s *Server) currentDiscoveryPath() string {
	return filepath.Join(s.paths.APIDir, "current.json")
}

func (s *Server) writeDiscoveryFiles() error {
	if err := os.MkdirAll(s.paths.APIDir, 0o755); err != nil {
		return fmt.Errorf("create api discovery dir: %w", err)
	}
	if err := writeJSONFileAtomic(s.discoveryFilePath(), s.discovery); err != nil {
		return err
	}
	if err := writeJSONFileAtomic(s.currentDiscoveryPath(), s.discovery); err != nil {
		return err
	}
	return nil
}

func (s *Server) removeDiscoveryFiles() error {
	if err := os.Remove(s.discoveryFilePath()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove api discovery file: %w", err)
	}

	data, err := os.ReadFile(s.currentDiscoveryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read current api discovery file: %w", err)
	}

	var current Discovery
	if err := json.Unmarshal(data, &current); err != nil {
		return fmt.Errorf("decode current api discovery file: %w", err)
	}
	if current.BaseURL == s.discovery.BaseURL && current.SessionID == s.discovery.SessionID {
		if err := os.Remove(s.currentDiscoveryPath()); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove current api discovery file: %w", err)
		}
	}
	return nil
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func writeJSONFileAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')

	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp %s: %w", filepath.Base(path), err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write temp %s: %w", filepath.Base(path), err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename %s: %w", filepath.Base(path), err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": strings.TrimSpace(message)})
}

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
