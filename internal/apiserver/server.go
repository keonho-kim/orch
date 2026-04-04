package apiserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/keonho-kim/orch/internal/config"
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
	service   serviceStatus
	paths     config.Paths
	http      *http.Server
	listener  net.Listener
	discovery Discovery
	closeOnce sync.Once
}

func Start(ctx context.Context, service serviceStatus, paths config.Paths) (*Server, error) {
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
