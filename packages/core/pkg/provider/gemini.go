package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type GeminiProvider struct {
	BaseURL string
}

func NewGeminiProvider() *GeminiProvider {
	return &GeminiProvider{
		BaseURL: "https://generativelanguage.googleapis.com/v1beta",
	}
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role"` // "user", "model"
	Parts []geminiPart `json:"parts"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiRequest struct {
	SystemInstruction *geminiSystemInstruction `json:"systemInstruction,omitempty"`
	Contents          []geminiContent          `json:"contents"`
	GenerationConfig  map[string]interface{}   `json:"generationConfig,omitempty"`
}

type geminiCandidate struct {
	Content struct {
		Parts []geminiPart `json:"parts"`
		Role  string       `json:"role"`
	} `json:"content"`
	FinishReason string `json:"finishReason"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate    `json:"candidates"`
	UsageMetadata *geminiUsageMetadata `json:"usageMetadata,omitempty"`
}

func (p *GeminiProvider) transformRequest(req *CompletionRequest) *geminiRequest {
	gReq := &geminiRequest{
		Contents:         make([]geminiContent, 0),
		GenerationConfig: make(map[string]interface{}),
	}

	if req.Temperature > 0 {
		gReq.GenerationConfig["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		gReq.GenerationConfig["maxOutputTokens"] = req.MaxTokens
	}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			gReq.SystemInstruction = &geminiSystemInstruction{
				Parts: []geminiPart{{Text: msg.Content}},
			}
		} else {
			role := "user"
			if msg.Role == "assistant" {
				role = "model"
			}
			gReq.Contents = append(gReq.Contents, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: msg.Content}},
			})
		}
	}

	return gReq
}

func (p *GeminiProvider) getModelName(model string) string {
	if !strings.HasPrefix(model, "models/") {
		return "models/" + model
	}
	return model
}

func (p *GeminiProvider) Completion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	model := p.getModelName(req.Model)
	url := fmt.Sprintf("%s/%s:generateContent?key=%s", p.BaseURL, model, req.APIKey)

	gReq := p.transformRequest(req)
	jsonData, err := json.Marshal(gReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("gemini returned no content candidates")
	}

	content := apiResp.Candidates[0].Content.Parts[0].Text
	tokensPrompt := 0
	tokensCompletion := 0

	if apiResp.UsageMetadata != nil {
		tokensPrompt = apiResp.UsageMetadata.PromptTokenCount
		tokensCompletion = apiResp.UsageMetadata.CandidatesTokenCount
	} else {
		// Fallback approximations
		tokensPrompt = len(jsonData) / 4
		tokensCompletion = len(content) / 4
	}

	return &CompletionResponse{
		Content:          content,
		TokensPrompt:     tokensPrompt,
		TokensCompletion: tokensCompletion,
		Cost:             GetTokenCost(req.Model, tokensPrompt, tokensCompletion),
	}, nil
}

func (p *GeminiProvider) CompletionStream(ctx context.Context, req *CompletionRequest, stream chan<- *StreamChunk) error {
	model := p.getModelName(req.Model)
	// streamGenerateContent returns SSE-like JSON chunks
	url := fmt.Sprintf("%s/%s:streamGenerateContent?key=%s", p.BaseURL, model, req.APIKey)

	gReq := p.transformRequest(req)
	jsonData, err := json.Marshal(gReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("gemini API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	go func() {
		defer resp.Body.Close()
		defer close(stream)

		reader := bufio.NewReader(resp.Body)
		tokensPrompt := 0
		tokensCompletion := 0

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					stream <- &StreamChunk{Done: true, Error: err.Error()}
				} else {
					stream <- &StreamChunk{
						Done:             true,
						TokensPrompt:     tokensPrompt,
						TokensCompletion: tokensCompletion,
						Cost:             GetTokenCost(req.Model, tokensPrompt, tokensCompletion),
					}
				}
				break
			}

			line = strings.TrimSpace(line)
			if line == "" || line == "[" || line == "]" || line == "," {
				continue
			}

			// Clean line (Gemini streams array items that might prefix with comma)
			line = strings.TrimPrefix(line, ",")
			line = strings.TrimSpace(line)

			var chunk geminiResponse
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}

			if chunk.UsageMetadata != nil {
				tokensPrompt = chunk.UsageMetadata.PromptTokenCount
				tokensCompletion = chunk.UsageMetadata.CandidatesTokenCount
			}

			if len(chunk.Candidates) > 0 && len(chunk.Candidates[0].Content.Parts) > 0 {
				deltaText := chunk.Candidates[0].Content.Parts[0].Text
				if deltaText != "" {
					stream <- &StreamChunk{Content: deltaText, Done: false}
				}
			}
		}
	}()

	return nil
}

func (p *GeminiProvider) ValidateCredentials(ctx context.Context, apiKey string, apiURL string) (bool, error) {
	// Query list models to validate key
	url := fmt.Sprintf("%s/models?key=%s", p.BaseURL, apiKey)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		return false, fmt.Errorf("invalid API credentials")
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("endpoint returned error status %d", resp.StatusCode)
	}

	return true, nil
}
