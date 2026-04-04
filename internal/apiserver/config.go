package apiserver

import (
	"net/http"
	"strings"

	"github.com/keonho-kim/orch/domain"
	"github.com/keonho-kim/orch/internal/config"
)

type providerPatch struct {
	Endpoint  *string `json:"endpoint,omitempty"`
	Model     *string `json:"model,omitempty"`
	APIKey    *string `json:"api_key,omitempty"`
	Reasoning *string `json:"reasoning,omitempty"`
}

type configPatchRequest struct {
	Provider          *string       `json:"provider,omitempty"`
	ApprovalPolicy    *string       `json:"approval_policy,omitempty"`
	SelfDrivingMode   *bool         `json:"self_driving_mode,omitempty"`
	ReactRalphIter    *int          `json:"react_ralph_iter,omitempty"`
	PlanRalphIter     *int          `json:"plan_ralph_iter,omitempty"`
	CompactThresholdK *int          `json:"compact_threshold_k,omitempty"`
	Providers         *providerList `json:"providers,omitempty"`
}

type providerList struct {
	Ollama  *providerPatch `json:"ollama,omitempty"`
	VLLM    *providerPatch `json:"vllm,omitempty"`
	Gemini  *providerPatch `json:"gemini,omitempty"`
	Vertex  *providerPatch `json:"vertex,omitempty"`
	Bedrock *providerPatch `json:"bedrock,omitempty"`
	Claude  *providerPatch `json:"claude,omitempty"`
	Azure   *providerPatch `json:"azure,omitempty"`
	ChatGPT *providerPatch `json:"chatgpt,omitempty"`
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleConfigGet(w, r)
	case http.MethodPatch:
		s.handleConfigPatch(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	if raw := strings.TrimSpace(r.URL.RawQuery); raw != "" {
		writeError(w, http.StatusBadRequest, "config endpoint no longer supports query parameters")
		return
	}

	state := s.service.ConfigState()
	document := state.Document
	for _, provider := range domain.Providers() {
		target := document.Providers.Provider(provider)
		target.APIKey = config.MaskSecret(target.APIKey)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path":   state.Path,
		"config": document,
	})
}

func (s *Server) handleConfigPatch(w http.ResponseWriter, r *http.Request) {
	var body configPatchRequest
	if err := jsonDecode(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	document, err := config.LoadDocument(s.paths)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := config.ApplyDocumentPatch(&document, documentPatchFromRequest(body)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	settings, err := document.ToSettings()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.service.SaveSettings(settings); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"path": s.paths.ConfigFile,
		"ok":   true,
	})
}

func documentPatchFromRequest(body configPatchRequest) config.DocumentPatch {
	patch := config.DocumentPatch{}
	if body.Provider != nil {
		patch.Provider = body.Provider
	}
	if body.ApprovalPolicy != nil {
		patch.ApprovalPolicy = body.ApprovalPolicy
	}
	if body.SelfDrivingMode != nil {
		patch.SelfDrivingMode = body.SelfDrivingMode
	}
	if body.ReactRalphIter != nil {
		patch.ReactRalphIter = body.ReactRalphIter
	}
	if body.PlanRalphIter != nil {
		patch.PlanRalphIter = body.PlanRalphIter
	}
	if body.CompactThresholdK != nil {
		patch.CompactThresholdK = body.CompactThresholdK
	}
	if body.Providers != nil {
		if body.Providers.Ollama != nil {
			patch.SetProviderPatch(domain.ProviderOllama, providerPatchFromRequest(*body.Providers.Ollama))
		}
		if body.Providers.VLLM != nil {
			patch.SetProviderPatch(domain.ProviderVLLM, providerPatchFromRequest(*body.Providers.VLLM))
		}
		if body.Providers.Gemini != nil {
			patch.SetProviderPatch(domain.ProviderGemini, providerPatchFromRequest(*body.Providers.Gemini))
		}
		if body.Providers.Vertex != nil {
			patch.SetProviderPatch(domain.ProviderVertex, providerPatchFromRequest(*body.Providers.Vertex))
		}
		if body.Providers.Bedrock != nil {
			patch.SetProviderPatch(domain.ProviderBedrock, providerPatchFromRequest(*body.Providers.Bedrock))
		}
		if body.Providers.Claude != nil {
			patch.SetProviderPatch(domain.ProviderClaude, providerPatchFromRequest(*body.Providers.Claude))
		}
		if body.Providers.Azure != nil {
			patch.SetProviderPatch(domain.ProviderAzure, providerPatchFromRequest(*body.Providers.Azure))
		}
		if body.Providers.ChatGPT != nil {
			patch.SetProviderPatch(domain.ProviderChatGPT, providerPatchFromRequest(*body.Providers.ChatGPT))
		}
	}
	return patch
}

func providerPatchFromRequest(patch providerPatch) config.ProviderPatch {
	return config.ProviderPatch{
		Endpoint:  patch.Endpoint,
		Model:     patch.Model,
		APIKey:    patch.APIKey,
		Reasoning: patch.Reasoning,
	}
}
