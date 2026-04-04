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

type vertexClient struct{}

type vertexGenerateContentRequest struct {
	SystemInstruction *vertexContent  `json:"systemInstruction,omitempty"`
	Contents          []vertexContent `json:"contents"`
	Tools             []vertexTool    `json:"tools,omitempty"`
}

type vertexTool struct {
	FunctionDeclarations []vertexFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type vertexFunctionDeclaration struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type vertexContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []vertexPart `json:"parts"`
}

type vertexPart struct {
	Text             string                  `json:"text,omitempty"`
	FunctionCall     *vertexFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *vertexFunctionResponse `json:"functionResponse,omitempty"`
}

type vertexFunctionCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

type vertexFunctionResponse struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type vertexGenerateContentResponse struct {
	Candidates    []vertexCandidate `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

type vertexCandidate struct {
	Content      vertexContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

func (vertexClient) Provider() domain.Provider {
	return domain.ProviderVertex
}

func (vertexClient) Chat(ctx context.Context, settings domain.ProviderSettings, request ChatRequest, onDelta DeltaHandler) (ChatResult, error) {
	if strings.TrimSpace(settings.Reasoning) != "" {
		return ChatResult{}, fmt.Errorf("vertex does not support configured reasoning transport")
	}
	apiKey, err := requiredAPIKey(settings, domain.ProviderVertex)
	if err != nil {
		return ChatResult{}, err
	}

	body, err := buildVertexRequest(request)
	if err != nil {
		return ChatResult{}, err
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return ChatResult{}, fmt.Errorf("marshal Vertex chat request: %w", err)
	}

	chatURL, err := vertexChatURL(settings.NormalizedEndpoint(), request.Model, apiKey)
	if err != nil {
		return ChatResult{}, err
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, bytes.NewReader(encoded))
	if err != nil {
		return ChatResult{}, fmt.Errorf("build Vertex chat request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return ChatResult{}, fmt.Errorf("send Vertex chat request: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(response.Body)
		return ChatResult{}, fmt.Errorf("vertex chat failed: status=%s body=%s", response.Status, strings.TrimSpace(string(data)))
	}

	return readVertexStream(response, onDelta)
}

func buildVertexRequest(request ChatRequest) (vertexGenerateContentRequest, error) {
	systemParts := make([]vertexPart, 0, len(request.Messages))
	contents := make([]vertexContent, 0, len(request.Messages))

	for _, message := range request.Messages {
		parts, err := vertexPartsForMessage(message)
		if err != nil {
			return vertexGenerateContentRequest{}, err
		}
		if len(parts) == 0 {
			continue
		}

		switch message.Role {
		case "system":
			systemParts = append(systemParts, parts...)
		case "assistant":
			contents = append(contents, vertexContent{Role: "model", Parts: parts})
		default:
			contents = append(contents, vertexContent{Role: "user", Parts: parts})
		}
	}

	result := vertexGenerateContentRequest{
		Contents: contents,
	}
	if len(systemParts) > 0 {
		result.SystemInstruction = &vertexContent{Parts: systemParts}
	}
	if len(request.Tools) > 0 {
		declarations := make([]vertexFunctionDeclaration, 0, len(request.Tools))
		for _, tool := range request.Tools {
			declarations = append(declarations, vertexFunctionDeclaration{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			})
		}
		result.Tools = []vertexTool{{FunctionDeclarations: declarations}}
	}
	return result, nil
}

func vertexPartsForMessage(message Message) ([]vertexPart, error) {
	parts := make([]vertexPart, 0, 1+len(message.ToolCalls))
	if message.Role != "tool" && strings.TrimSpace(message.Content) != "" {
		parts = append(parts, vertexPart{Text: message.Content})
	}

	switch message.Role {
	case "assistant":
		for _, call := range message.ToolCalls {
			arguments := strings.TrimSpace(call.Function.Arguments)
			if arguments == "" {
				arguments = "{}"
			}
			if !json.Valid([]byte(arguments)) {
				return nil, fmt.Errorf("invalid Vertex function call arguments for %s", call.Function.Name)
			}
			parts = append(parts, vertexPart{
				FunctionCall: &vertexFunctionCall{
					Name: call.Function.Name,
					Args: json.RawMessage(arguments),
				},
			})
		}
	case "tool":
		parts = append(parts, vertexPart{
			FunctionResponse: &vertexFunctionResponse{
				Name: message.Name,
				Response: map[string]any{
					"content": decodeToolResponseContent(message.Content),
				},
			},
		})
	}

	return parts, nil
}

func decodeToolResponseContent(raw string) any {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	var decoded any
	if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
		return decoded
	}
	return trimmed
}

func vertexChatURL(baseURL string, model string, apiKey string) (string, error) {
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("vertex endpoint is required")
	}
	if strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("vertex model is required")
	}

	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", fmt.Errorf("parse Vertex base URL: %w", err)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/publishers/google/models/" + strings.TrimSpace(model) + ":streamGenerateContent"
	query := parsed.Query()
	query.Set("key", apiKey)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func readVertexStream(response *http.Response, onDelta DeltaHandler) (ChatResult, error) {
	var content strings.Builder
	toolCalls := make([]domain.ToolCall, 0)
	finishReason := ""
	usage := domain.UsageStats{}

	err := scanStreamLines(response.Body, streamScanOptions{StripDataPrefix: true}, func(line string) error {
		return applyVertexChunk(line, onDelta, &content, &toolCalls, &finishReason, &usage)
	})
	if err != nil {
		return ChatResult{}, fmt.Errorf("read Vertex stream: %w", err)
	}

	return ChatResult{
		Content:      content.String(),
		ToolCalls:    toolCalls,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

func applyVertexChunk(
	line string,
	onDelta DeltaHandler,
	content *strings.Builder,
	toolCalls *[]domain.ToolCall,
	finishReason *string,
	usage *domain.UsageStats,
) error {
	chunk, err := decodeVertexChunk(line)
	if err != nil {
		return err
	}
	mergeVertexUsage(usage, chunk)
	for _, candidate := range chunk.Candidates {
		if err := appendVertexCandidate(onDelta, content, toolCalls, candidate); err != nil {
			return err
		}
		if candidate.FinishReason != "" {
			*finishReason = candidate.FinishReason
		}
	}
	return nil
}

func decodeVertexChunk(line string) (vertexGenerateContentResponse, error) {
	var chunk vertexGenerateContentResponse
	if err := json.Unmarshal([]byte(line), &chunk); err != nil {
		return vertexGenerateContentResponse{}, fmt.Errorf("decode Vertex stream chunk: %w", err)
	}
	return chunk, nil
}

func mergeVertexUsage(target *domain.UsageStats, chunk vertexGenerateContentResponse) {
	metadata := chunk.UsageMetadata
	if metadata.TotalTokenCount == 0 && metadata.PromptTokenCount == 0 && metadata.CandidatesTokenCount == 0 {
		return
	}
	*target = domain.UsageStats{
		PromptTokens:     metadata.PromptTokenCount,
		CompletionTokens: metadata.CandidatesTokenCount,
		TotalTokens:      metadata.TotalTokenCount,
	}
}

func appendVertexCandidate(
	onDelta DeltaHandler,
	content *strings.Builder,
	toolCalls *[]domain.ToolCall,
	candidate vertexCandidate,
) error {
	for _, part := range candidate.Content.Parts {
		if err := appendVertexPart(onDelta, content, toolCalls, part); err != nil {
			return err
		}
	}
	return nil
}

func appendVertexPart(
	onDelta DeltaHandler,
	content *strings.Builder,
	toolCalls *[]domain.ToolCall,
	part vertexPart,
) error {
	if part.Text != "" {
		content.WriteString(part.Text)
		if err := emitDelta(onDelta, Delta{Content: part.Text}); err != nil {
			return err
		}
	}
	if part.FunctionCall != nil {
		*toolCalls = append(*toolCalls, buildVertexToolCall(part.FunctionCall, len(*toolCalls)+1))
	}
	return nil
}

func buildVertexToolCall(call *vertexFunctionCall, index int) domain.ToolCall {
	arguments := "{}"
	if len(call.Args) > 0 {
		arguments = string(call.Args)
	}
	return domain.ToolCall{
		ID:        fmt.Sprintf("call_%d", index),
		Name:      call.Name,
		Arguments: arguments,
	}
}
