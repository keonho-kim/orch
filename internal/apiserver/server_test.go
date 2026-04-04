package apiserver

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
	"github.com/keonho-kim/orch/internal/orchestrator"
	"github.com/keonho-kim/orch/internal/session"
	sqlitestore "github.com/keonho-kim/orch/internal/store/sqlite"
	"github.com/keonho-kim/orch/internal/tooling"
)

func TestStartWritesAndRemovesDiscoveryFiles(t *testing.T) {
	server, _, cleanup := newTestServer(t)
	defer cleanup()

	if !strings.HasPrefix(server.Discovery().BaseURL, "http://127.0.0.1:") {
		t.Fatalf("expected local-only base url, got %q", server.Discovery().BaseURL)
	}
	if _, err := os.Stat(filepath.Join(server.paths.APIDir, server.Discovery().SessionID+".json")); err != nil {
		t.Fatalf("expected session discovery file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(server.paths.APIDir, "current.json")); err != nil {
		t.Fatalf("expected current discovery file: %v", err)
	}

	if err := server.Close(); err != nil {
		t.Fatalf("close server: %v", err)
	}
	if _, err := os.Stat(filepath.Join(server.paths.APIDir, server.Discovery().SessionID+".json")); !os.IsNotExist(err) {
		t.Fatalf("expected session discovery file removal, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(server.paths.APIDir, "current.json")); !os.IsNotExist(err) {
		t.Fatalf("expected current discovery file removal, stat err=%v", err)
	}
}

func TestStatusRequiresBearerToken(t *testing.T) {
	server, _, cleanup := newTestServer(t)
	defer cleanup()

	response, err := http.Get(server.Discovery().BaseURL + "/v1/status")
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: %d", response.StatusCode)
	}
}

func TestAuthAcceptsMatchingBearerToken(t *testing.T) {
	server, _, cleanup := newTestServer(t)
	defer cleanup()

	request := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	request.Header.Set("Authorization", "Bearer "+server.Discovery().Token)
	recorder := httptest.NewRecorder()

	called := false
	server.auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, request)

	if !called {
		t.Fatal("expected auth middleware to pass matching bearer token")
	}
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: %d", recorder.Code)
	}
}

func TestWriteSSEHeadersRejectsNonFlusherWriter(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	writer := &nonFlusherResponseWriter{header: http.Header{}}
	_ = request

	flusher, ok := writeSSEHeaders(writer)
	if ok || flusher != nil {
		t.Fatal("expected non-flusher writer to be rejected")
	}
	if writer.status != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", writer.status)
	}
	if !strings.Contains(writer.body.String(), "streaming is not supported") {
		t.Fatalf("unexpected response body: %q", writer.body.String())
	}
}

func TestRemoveDiscoveryFilesPreservesOtherCurrentDiscovery(t *testing.T) {
	server, _, cleanup := newTestServer(t)
	defer cleanup()

	other := Discovery{
		BaseURL:   "http://127.0.0.1:9999",
		Token:     "other",
		PID:       1,
		SessionID: "other-session",
		RepoRoot:  server.paths.RepoRoot,
		StartedAt: time.Now(),
	}
	if err := writeJSONFileAtomic(server.currentDiscoveryPath(), other); err != nil {
		t.Fatalf("write other current discovery: %v", err)
	}

	if err := server.removeDiscoveryFiles(); err != nil {
		t.Fatalf("remove discovery files: %v", err)
	}

	if _, err := os.Stat(server.discoveryFilePath()); !os.IsNotExist(err) {
		t.Fatalf("expected session discovery removal, stat err=%v", err)
	}
	data, err := os.ReadFile(server.currentDiscoveryPath())
	if err != nil {
		t.Fatalf("read current discovery: %v", err)
	}
	var got Discovery
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode current discovery: %v", err)
	}
	if got.SessionID != other.SessionID || got.BaseURL != other.BaseURL {
		t.Fatalf("unexpected current discovery: %+v", got)
	}
}

func TestStatusAndEventsEndpoints(t *testing.T) {
	server, svc, cleanup := newTestServer(t)
	defer cleanup()

	statusRequest, _ := http.NewRequest(http.MethodGet, server.Discovery().BaseURL+"/v1/status", nil)
	statusRequest.Header.Set("Authorization", "Bearer "+server.Discovery().Token)
	statusResponse, err := http.DefaultClient.Do(statusRequest)
	if err != nil {
		t.Fatalf("status request: %v", err)
	}
	defer func() {
		_ = statusResponse.Body.Close()
	}()
	if statusResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status response code: %d", statusResponse.StatusCode)
	}
	var statusBody struct {
		Status orchestrator.Status `json:"status"`
		API    Discovery           `json:"api"`
	}
	if err := json.NewDecoder(statusResponse.Body).Decode(&statusBody); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if statusBody.API.SessionID == "" || statusBody.Status.CurrentSession.SessionID == "" {
		t.Fatalf("unexpected status response: %+v", statusBody)
	}

	eventsRequest, _ := http.NewRequest(http.MethodGet, server.Discovery().BaseURL+"/v1/events", nil)
	eventsRequest.Header.Set("Authorization", "Bearer "+server.Discovery().Token)
	eventsResponse, err := http.DefaultClient.Do(eventsRequest)
	if err != nil {
		t.Fatalf("events request: %v", err)
	}
	defer func() {
		_ = eventsResponse.Body.Close()
	}()
	reader := bufio.NewReader(eventsResponse.Body)
	eventType, payload := readSSEEvent(t, reader)
	if eventType != "snapshot" || !strings.Contains(payload, `"status"`) {
		t.Fatalf("unexpected initial event: type=%q payload=%q", eventType, payload)
	}

	svc.EmitEvent(orchestrator.ServiceEvent{Type: "snapshot", Message: "hello"})
	eventType, payload = readSSEEvent(t, reader)
	if eventType != "snapshot" || !strings.Contains(payload, `"message":"hello"`) {
		t.Fatalf("unexpected streamed event: type=%q payload=%q", eventType, payload)
	}
}

func TestExecEndpointsAndRunEvents(t *testing.T) {
	server, svc, cleanup := newTestServer(t)
	defer cleanup()

	execResponse := doJSON(t, server, http.MethodPost, "/v1/exec", map[string]string{
		"prompt": "say hello",
	})
	if execResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected exec response code: %d", execResponse.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(execResponse.Body).Decode(&body); err != nil {
		t.Fatalf("decode exec response: %v", err)
	}
	_ = execResponse.Body.Close()
	runID := strings.TrimSpace(body["run_id"])
	if runID == "" {
		t.Fatalf("expected run_id in response: %+v", body)
	}

	statusResponse := doRequest(t, server, http.MethodGet, "/v1/exec/"+runID, "")
	if statusResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected run status code: %d", statusResponse.StatusCode)
	}
	_ = statusResponse.Body.Close()

	eventsRequest, _ := http.NewRequest(http.MethodGet, server.Discovery().BaseURL+"/v1/exec/"+runID+"/events", nil)
	eventsRequest.Header.Set("Authorization", "Bearer "+server.Discovery().Token)
	eventsResponse, err := http.DefaultClient.Do(eventsRequest)
	if err != nil {
		t.Fatalf("run events request: %v", err)
	}
	defer func() {
		_ = eventsResponse.Body.Close()
	}()
	reader := bufio.NewReader(eventsResponse.Body)
	eventType, payload := readSSEEvent(t, reader)
	if eventType != "snapshot" || !strings.Contains(payload, runID) {
		t.Fatalf("unexpected initial run event: type=%q payload=%q", eventType, payload)
	}

	svc.EmitEvent(orchestrator.ServiceEvent{Type: "run_updated", RunID: runID, Message: "tick"})
	eventType, payload = readSSEEvent(t, reader)
	if eventType != "run_updated" || !strings.Contains(payload, `"message":"tick"`) {
		t.Fatalf("unexpected run update event: type=%q payload=%q", eventType, payload)
	}
}

func TestHistoryAndConfigEndpoints(t *testing.T) {
	server, svc, cleanup := newTestServer(t)
	defer cleanup()

	first := seedHistorySessions(t, server)
	assertHistoryEndpoints(t, server)
	assertRestoreEndpoint(t, server, svc, first.SessionID)
	assertConfigEndpoints(t, server)
}

func seedHistorySessions(t *testing.T, server *Server) domain.SessionMetadata {
	t.Helper()
	manager := session.NewManager(server.paths.SessionsDir)
	first, err := manager.Create(server.paths.RepoRoot, domain.ProviderOllama, "model", time.Now().Add(-time.Hour), "", "", "", domain.AgentRoleGateway, "", "", "")
	if err != nil {
		t.Fatalf("create first session: %v", err)
	}
	second, err := manager.Create(server.paths.RepoRoot, domain.ProviderOllama, "model", time.Now(), "", "", "", domain.AgentRoleGateway, "", "", "")
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}
	for _, meta := range []domain.SessionMetadata{first, second} {
		if err := manager.AppendRecord(meta.SessionID, domain.SessionRecord{
			Seq:       1,
			SessionID: meta.SessionID,
			Type:      domain.SessionRecordUser,
			Content:   meta.SessionID,
			CreatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("append record: %v", err)
		}
	}
	return first
}

func assertHistoryEndpoints(t *testing.T, server *Server) {
	t.Helper()
	historyResponse := doRequest(t, server, http.MethodGet, "/v1/history?limit=10", "")
	if historyResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected history code: %d", historyResponse.StatusCode)
	}
	var historyBody map[string][]domain.SessionMetadata
	if err := json.NewDecoder(historyResponse.Body).Decode(&historyBody); err != nil {
		t.Fatalf("decode history response: %v", err)
	}
	_ = historyResponse.Body.Close()
	if len(historyBody["sessions"]) < 2 {
		t.Fatalf("expected history sessions, got %+v", historyBody)
	}

	latestResponse := doRequest(t, server, http.MethodGet, "/v1/history/latest", "")
	if latestResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected latest code: %d", latestResponse.StatusCode)
	}
	var latestBody map[string]string
	if err := json.NewDecoder(latestResponse.Body).Decode(&latestBody); err != nil {
		t.Fatalf("decode latest response: %v", err)
	}
	_ = latestResponse.Body.Close()
	if latestBody["session_id"] == "" {
		t.Fatalf("expected latest session id: %+v", latestBody)
	}
}

func assertRestoreEndpoint(t *testing.T, server *Server, svc *orchestrator.Service, sessionID string) {
	t.Helper()
	restoreResponse := doJSON(t, server, http.MethodPost, "/v1/history/restore", map[string]string{"session_id": sessionID})
	if restoreResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected restore code: %d", restoreResponse.StatusCode)
	}
	_ = restoreResponse.Body.Close()
	if svc.Status().CurrentSession.SessionID != sessionID {
		t.Fatalf("expected restored session, got %+v", svc.Status())
	}
}

func assertConfigEndpoints(t *testing.T, server *Server) {
	t.Helper()
	configPatchResponse := doJSON(t, server, http.MethodPatch, "/v1/config", map[string]any{
		"provider": "chatgpt",
		"providers": map[string]any{
			"chatgpt": map[string]any{
				"model":   "gpt-4.1",
				"api_key": "1234567890abcdefghijklmnopqrstuvwxyz",
			},
		},
	})
	if configPatchResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected config patch code: %d", configPatchResponse.StatusCode)
	}
	_ = configPatchResponse.Body.Close()

	configGetResponse := doRequest(t, server, http.MethodGet, "/v1/config", "")
	if configGetResponse.StatusCode != http.StatusOK {
		t.Fatalf("unexpected config get code: %d", configGetResponse.StatusCode)
	}
	var configBody struct {
		Path   string          `json:"path"`
		Config config.Document `json:"config"`
	}
	if err := json.NewDecoder(configGetResponse.Body).Decode(&configBody); err != nil {
		t.Fatalf("decode config get response: %v", err)
	}
	_ = configGetResponse.Body.Close()
	if configBody.Config.Provider != "chatgpt" || configBody.Config.Providers.ChatGPT.Model != "gpt-4.1" {
		t.Fatalf("unexpected config body: %+v", configBody)
	}
	if configBody.Config.Providers.ChatGPT.APIKey != "1234567890***vwxyz" {
		t.Fatalf("expected redacted API key, got %+v", configBody.Config.Providers.ChatGPT)
	}
}

func newTestServer(t *testing.T) (*Server, *orchestrator.Service, func()) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	repoRoot := t.TempDir()
	paths, err := config.ResolvePaths(repoRoot)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	writeBootstrapAssets(t, paths)
	if err := config.EnsureRuntimePaths(paths); err != nil {
		t.Fatalf("ensure runtime paths: %v", err)
	}

	if err := config.SaveSettings(paths, domain.Settings{
		DefaultProvider: domain.ProviderOllama,
		Providers: domain.ProviderCatalog{
			Ollama: domain.ProviderSettings{
				Endpoint: "http://localhost:11434/v1",
				Model:    "test-model",
			},
		},
	}); err != nil {
		t.Fatalf("save project settings: %v", err)
	}

	store, err := sqlitestore.Open(paths.DBPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	service, err := orchestrator.NewService(ctx, store, tooling.NewExecutor(), paths, orchestrator.BootOptions{})
	if err != nil {
		cancel()
		_ = store.Close()
		t.Fatalf("new service: %v", err)
	}

	server, err := Start(ctx, service, paths)
	if err != nil {
		cancel()
		_ = store.Close()
		t.Fatalf("start api server: %v", err)
	}

	cleanup := func() {
		_ = server.Close()
		cancel()
		_ = store.Close()
	}
	return server, service, cleanup
}

type nonFlusherResponseWriter struct {
	header http.Header
	body   strings.Builder
	status int
}

func (w *nonFlusherResponseWriter) Header() http.Header {
	return w.header
}

func (w *nonFlusherResponseWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *nonFlusherResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

func writeBootstrapAssets(t *testing.T, paths config.Paths) {
	t.Helper()

	files := map[string]string{
		filepath.Join(paths.BootstrapAssets, "AGENTS.md"):                             "# test\n",
		filepath.Join(paths.BootstrapAssets, "SKILLS.md"):                             "# skills\n",
		filepath.Join(paths.BootstrapAssets, "TOOLS.md"):                              "# tools\n",
		filepath.Join(paths.BootstrapAssets, "USER.md"):                               "# user\n",
		filepath.Join(paths.BootstrapAssets, "system-prompt", "gateway", "AGENTS.md"): "# gateway\n",
		filepath.Join(paths.BootstrapAssets, "system-prompt", "worker", "AGENTS.md"):  "# worker\n",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create bootstrap dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write bootstrap file %s: %v", path, err)
		}
	}
}

func doRequest(t *testing.T, server *Server, method string, path string, body string) *http.Response {
	t.Helper()

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request, err := http.NewRequest(method, server.Discovery().BaseURL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+server.Discovery().Token)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return response
}

func doJSON(t *testing.T, server *Server, method string, path string, body any) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	return doRequest(t, server, method, path, string(data))
}

func readSSEEvent(t *testing.T, reader *bufio.Reader) (string, string) {
	t.Helper()

	done := make(chan struct {
		event string
		data  string
		err   error
	}, 1)
	go func() {
		var eventType string
		var data strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				done <- struct {
					event string
					data  string
					err   error
				}{err: err}
				return
			}
			line = strings.TrimRight(line, "\n")
			if line == "" {
				done <- struct {
					event string
					data  string
					err   error
				}{event: eventType, data: data.String()}
				return
			}
			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimSpace(strings.TrimPrefix(line, "event: "))
			}
			if strings.HasPrefix(line, "data: ") {
				data.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data: ")))
			}
		}
	}()

	select {
	case result := <-done:
		if result.err != nil {
			t.Fatalf("read sse event: %v", result.err)
		}
		return result.event, result.data
	case <-time.After(3 * time.Second):
		t.Fatal("timed out reading sse event")
		return "", ""
	}
}
