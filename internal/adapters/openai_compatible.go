package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/keonho-kim/orch/domain"
)

type openAICompatibleClient struct {
	provider domain.Provider
}

type openAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type openAICompatibleRequest struct {
	Model              string               `json:"model,omitempty"`
	Messages           []Message            `json:"messages"`
	Tools              []ToolDefinition     `json:"tools,omitempty"`
	Stream             bool                 `json:"stream"`
	StreamIncludeUsage bool                 `json:"stream_include_usage,omitempty"`
	StreamOptions      *openAIStreamOptions `json:"stream_options,omitempty"`
	ChatTemplate       string               `json:"chat_template,omitempty"`
	ChatTemplateKwargs map[string]any       `json:"chat_template_kwargs,omitempty"`
	ReasoningEffort    string               `json:"reasoning_effort,omitempty"`
}

type streamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type streamToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type streamDelta struct {
	Content          string                `json:"content"`
	ReasoningContent string                `json:"reasoning_content"`
	Reasoning        string                `json:"reasoning"`
	ToolCalls        []streamToolCallDelta `json:"tool_calls"`
}

type streamChoice struct {
	Delta        streamDelta `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type streamEnvelope struct {
	Choices []streamChoice `json:"choices"`
	Usage   streamUsage    `json:"usage"`
}

func newOpenAICompatibleClient(provider domain.Provider) Client {
	return openAICompatibleClient{provider: provider}
}

func (c openAICompatibleClient) Provider() domain.Provider {
	return c.provider
}

func (c openAICompatibleClient) Chat(ctx context.Context, settings domain.ProviderSettings, request ChatRequest, onDelta DeltaHandler) (ChatResult, error) {
	payload, err := buildOpenAICompatibleRequest(c.provider, settings, request)
	if err != nil {
		return ChatResult{}, err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return ChatResult{}, fmt.Errorf("marshal chat request: %w", err)
	}

	chatURL, err := openAICompatibleChatURL(c.provider, settings, request.Model)
	if err != nil {
		return ChatResult{}, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(body))
	if err != nil {
		return ChatResult{}, fmt.Errorf("build chat request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if err := applyOpenAICompatibleAuth(httpRequest, c.provider, settings); err != nil {
		return ChatResult{}, err
	}

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return ChatResult{}, fmt.Errorf("send chat request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(response.Body)
		return ChatResult{}, fmt.Errorf("chat request failed: status=%s body=%s", response.Status, strings.TrimSpace(string(data)))
	}

	return readOpenAICompatibleStream(response, onDelta)
}

func buildOpenAICompatibleRequest(provider domain.Provider, settings domain.ProviderSettings, request ChatRequest) (openAICompatibleRequest, error) {
	profile, err := modelProfile(provider, request.Model, settings.Reasoning)
	if err != nil {
		return openAICompatibleRequest{}, err
	}

	payload := openAICompatibleRequest{
		Messages: request.Messages,
		Tools:    request.Tools,
		Stream:   true,
	}

	switch provider {
	case domain.ProviderAzure:
		payload.StreamOptions = &openAIStreamOptions{IncludeUsage: true}
	case domain.ProviderVLLM:
		payload.Model = request.Model
		payload.StreamIncludeUsage = true
	default:
		payload.Model = request.Model
		payload.StreamOptions = &openAIStreamOptions{IncludeUsage: true}
	}

	if profile.ChatTemplate != "" {
		payload.ChatTemplate = profile.ChatTemplate
	}
	if len(profile.ChatTemplateKwargs) > 0 {
		payload.ChatTemplateKwargs = profile.ChatTemplateKwargs
	}
	if profile.ReasoningEffort != "" {
		payload.ReasoningEffort = profile.ReasoningEffort
	}

	return payload, nil
}

type openAIModelProfile struct {
	ChatTemplate       string
	ChatTemplateKwargs map[string]any
	ReasoningEffort    string
}

func modelProfile(provider domain.Provider, model string, reasoning string) (openAIModelProfile, error) {
	reasoning = strings.ToLower(strings.TrimSpace(reasoning))
	switch provider {
	case domain.ProviderChatGPT, domain.ProviderAzure:
		return openAIReasoningProfile(reasoning)
	case domain.ProviderVLLM:
		return vllmProfile(model, reasoning)
	default:
		if reasoning != "" {
			return openAIModelProfile{}, fmt.Errorf("%s does not support configured reasoning transport", provider.DisplayName())
		}
		return openAIModelProfile{}, nil
	}
}

func openAIReasoningProfile(reasoning string) (openAIModelProfile, error) {
	switch reasoning {
	case "":
		return openAIModelProfile{}, nil
	case "true":
		return openAIModelProfile{}, nil
	case "false":
		return openAIModelProfile{ReasoningEffort: "none"}, nil
	case "low", "medium", "high", "xhigh":
		return openAIModelProfile{ReasoningEffort: reasoning}, nil
	default:
		return openAIModelProfile{}, fmt.Errorf("unsupported reasoning value %q", reasoning)
	}
}

func vllmProfile(model string, reasoning string) (openAIModelProfile, error) {
	template := func(envName string) string {
		return strings.TrimSpace(osValue(envName))
	}
	normalized := strings.ToLower(strings.TrimSpace(model))

	switch {
	case strings.Contains(normalized, "qwen") && (strings.Contains(normalized, "3.5") || strings.Contains(normalized, "35")):
		return vllmBoolProfile(template("ORCH_VLLM_CHAT_TEMPLATE_QWEN35"), "enable_thinking", reasoning)
	case strings.Contains(normalized, "deepseek"):
		return vllmBoolProfile(template("ORCH_VLLM_CHAT_TEMPLATE_DEEPSEEK"), "thinking", reasoning)
	case strings.Contains(normalized, "gemma-4"), strings.Contains(normalized, "gemma4"):
		if reasoning != "" {
			return openAIModelProfile{}, fmt.Errorf("vLLM model %s only supports default reasoning behavior in this release", model)
		}
		return openAIModelProfile{ChatTemplate: template("ORCH_VLLM_CHAT_TEMPLATE_GEMMA4")}, nil
	case strings.HasPrefix(normalized, "glm-"), strings.Contains(normalized, "glm-4"):
		if reasoning != "" {
			return openAIModelProfile{}, fmt.Errorf("vLLM model %s only supports default reasoning behavior in this release", model)
		}
		return openAIModelProfile{ChatTemplate: template("ORCH_VLLM_CHAT_TEMPLATE_GLM")}, nil
	default:
		if reasoning != "" {
			return openAIModelProfile{}, fmt.Errorf("vLLM model %s does not support explicit reasoning control", model)
		}
		return openAIModelProfile{}, nil
	}
}

func vllmBoolProfile(chatTemplate string, key string, reasoning string) (openAIModelProfile, error) {
	profile := openAIModelProfile{ChatTemplate: chatTemplate}
	switch reasoning {
	case "":
		profile.ChatTemplateKwargs = map[string]any{key: true}
	case "true":
		profile.ChatTemplateKwargs = map[string]any{key: true}
	case "false":
		profile.ChatTemplateKwargs = map[string]any{key: false}
	default:
		return openAIModelProfile{}, fmt.Errorf("vLLM only supports true/false reasoning in this release")
	}
	return profile, nil
}

func openAICompatibleChatURL(provider domain.Provider, settings domain.ProviderSettings, model string) (string, error) {
	switch provider {
	case domain.ProviderAzure:
		endpoint := strings.TrimSpace(settings.Endpoint)
		if endpoint == "" {
			return "", fmt.Errorf("azure endpoint is required")
		}
		if strings.TrimSpace(model) == "" {
			return "", fmt.Errorf("azure model is required")
		}

		parsed, err := url.Parse(strings.TrimRight(endpoint, "/"))
		if err != nil {
			return "", fmt.Errorf("parse Azure endpoint: %w", err)
		}
		cleanPath := strings.TrimRight(parsed.Path, "/")
		cleanPath = strings.TrimSuffix(cleanPath, "/openai/v1")
		cleanPath = strings.TrimSuffix(cleanPath, "/openai")
		parsed.Path = strings.TrimRight(cleanPath, "/") + "/openai/deployments/" + strings.TrimSpace(model) + "/chat/completions"
		query := parsed.Query()
		query.Set("api-version", "2024-10-21")
		parsed.RawQuery = query.Encode()
		return parsed.String(), nil
	default:
		endpoint := settings.NormalizedEndpoint()
		if endpoint == "" {
			return "", fmt.Errorf("%s endpoint is required", provider.DisplayName())
		}
		return strings.TrimRight(endpoint, "/") + "/chat/completions", nil
	}
}

func applyOpenAICompatibleAuth(request *http.Request, provider domain.Provider, settings domain.ProviderSettings) error {
	switch provider {
	case domain.ProviderAzure:
		apiKey, err := requiredAPIKey(settings, provider)
		if err != nil {
			return err
		}
		request.Header.Set("api-key", apiKey)
		return nil
	case domain.ProviderVLLM:
		apiKey := strings.TrimSpace(settings.APIKey)
		if apiKey != "" {
			request.Header.Set("Authorization", "Bearer "+apiKey)
		}
		return nil
	default:
		apiKey, err := requiredAPIKey(settings, provider)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "Bearer "+apiKey)
		return nil
	}
}

<<<<<<< HEAD
func requiredAPIKey(settings domain.ProviderSettings, provider domain.Provider) (string, error) {
	apiKey := strings.TrimSpace(settings.APIKey)
	if apiKey == "" {
		return "", fmt.Errorf("%s API key is not configured", provider.DisplayName())
	}
	return apiKey, nil
=======
func optionalAPIKey(settings domain.ProviderSettings) (string, error) {
	settings.Auth = settings.Auth.Normalize(domain.ProviderVLLM)
	if settings.Auth.Kind == domain.ProviderAuthNone {
		return "", nil
	}
	value, err := settings.Auth.Resolve(domain.ProviderVLLM)
	if err != nil {
		if settings.Auth.Kind == domain.ProviderAuthEnv && strings.Contains(err.Error(), "is not set") {
			return "", nil
		}
		return "", err
	}
	return value, nil
}

func requiredAPIKey(settings domain.ProviderSettings, provider domain.Provider) (string, error) {
	settings.Auth = settings.Auth.Normalize(provider)
	return settings.Auth.Resolve(provider)
>>>>>>> cef7a8c (update)
}

func readOpenAICompatibleStream(response *http.Response, onDelta DeltaHandler) (ChatResult, error) {
	var content strings.Builder
	var reasoning strings.Builder
	toolCalls := map[int]domain.ToolCall{}
	finishReason := ""
	usage := domain.UsageStats{}

	err := scanStreamLines(response.Body, streamScanOptions{
		AllowComments:   true,
		RequireDataLine: true,
	}, func(payload string) error {
		return applyOpenAICompatibleChunk(payload, onDelta, &content, &reasoning, toolCalls, &finishReason, &usage)
	})
	if err != nil {
		return ChatResult{}, fmt.Errorf("read stream: %w", err)
	}

	return ChatResult{
		Content:      content.String(),
		Reasoning:    reasoning.String(),
		ToolCalls:    orderedToolCalls(toolCalls),
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

func applyOpenAICompatibleChunk(
	payload string,
	onDelta DeltaHandler,
	content *strings.Builder,
	reasoning *strings.Builder,
	toolCalls map[int]domain.ToolCall,
	finishReason *string,
	usage *domain.UsageStats,
) error {
	envelope, err := decodeOpenAICompatibleEnvelope(payload)
	if err != nil {
		return err
	}
	mergeOpenAIUsage(usage, envelope.Usage)
	for _, choice := range envelope.Choices {
		if err := appendOpenAIReasoningDelta(onDelta, reasoning, choice); err != nil {
			return err
		}
		if err := appendOpenAIContentDelta(onDelta, content, choice); err != nil {
			return err
		}
		mergeOpenAIToolCalls(toolCalls, choice)
		if choice.FinishReason != "" {
			*finishReason = choice.FinishReason
		}
	}
	return nil
}

func decodeOpenAICompatibleEnvelope(payload string) (streamEnvelope, error) {
	var envelope streamEnvelope
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		return streamEnvelope{}, fmt.Errorf("decode stream payload: %w", err)
	}
	return envelope, nil
}

func mergeOpenAIUsage(target *domain.UsageStats, usage streamUsage) {
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		return
	}
	*target = domain.UsageStats{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}

func appendOpenAIReasoningDelta(onDelta DeltaHandler, builder *strings.Builder, choice streamChoice) error {
	reasoningChunk := choice.Delta.ReasoningContent
	if reasoningChunk == "" {
		reasoningChunk = choice.Delta.Reasoning
	}
	if reasoningChunk == "" {
		return nil
	}
	builder.WriteString(reasoningChunk)
	return emitDelta(onDelta, Delta{Reasoning: reasoningChunk})
}

func appendOpenAIContentDelta(onDelta DeltaHandler, builder *strings.Builder, choice streamChoice) error {
	if choice.Delta.Content == "" {
		return nil
	}
	builder.WriteString(choice.Delta.Content)
	return emitDelta(onDelta, Delta{Content: choice.Delta.Content})
}

func mergeOpenAIToolCalls(target map[int]domain.ToolCall, choice streamChoice) {
	for _, toolDelta := range choice.Delta.ToolCalls {
		call := target[toolDelta.Index]
		if call.ID == "" && toolDelta.ID != "" {
			call.ID = toolDelta.ID
		}
		if call.Name == "" && toolDelta.Function.Name != "" {
			call.Name = toolDelta.Function.Name
		}
		call.Arguments += toolDelta.Function.Arguments
		target[toolDelta.Index] = call
	}
}

func orderedToolCalls(toolCalls map[int]domain.ToolCall) []domain.ToolCall {
	ordered := make([]domain.ToolCall, 0, len(toolCalls))
	for index := 0; index < len(toolCalls); index++ {
		if call, ok := toolCalls[index]; ok {
			ordered = append(ordered, call)
		}
	}
	return ordered
}

func osValue(name string) string {
	return strings.TrimSpace(getenv(name))
}

var getenv = func(name string) string {
	return os.Getenv(name)
}
