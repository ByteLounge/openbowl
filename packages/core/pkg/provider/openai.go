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

type OpenAIProvider struct {
	DefaultURL string
}

func NewOpenAIProvider() *OpenAIProvider {
	return &OpenAIProvider{
		DefaultURL: "https://api.openai.com/v1",
	}
}

type openAIChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type openAIDeltaChoice struct {
	Index        int `json:"index"`
	Delta        struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAICompletionResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

type openAIStreamChunk struct {
	ID      string              `json:"id"`
	Object  string              `json:"object"`
	Created int64               `json:"created"`
	Model   string              `json:"model"`
	Choices []openAIDeltaChoice `json:"choices"`
	Usage   *openAIUsage        `json:"usage,omitempty"`
}

func (p *OpenAIProvider) getURL(req *CompletionRequest) string {
	if req.APIURL != "" {
		return strings.TrimSuffix(req.APIURL, "/")
	}
	return p.DefaultURL
}

func (p *OpenAIProvider) Completion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", p.getURL(req))

	bodyMap := map[string]interface{}{
		"model":       req.Model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
		"stream":      false,
	}
	if req.MaxTokens > 0 {
		bodyMap["max_tokens"] = req.MaxTokens
	}

	jsonData, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", req.APIKey))
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider API returned error status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp openAICompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices returned from model completion")
	}

	tokensPrompt := apiResp.Usage.PromptTokens
	tokensCompletion := apiResp.Usage.CompletionTokens
	
	// If API doesn't return usage (e.g. some local endpoints), estimate it
	if tokensPrompt == 0 {
		for _, m := range req.Messages {
			tokensPrompt += len(m.Content) / 4 // rough char check fallback
		}
		tokensCompletion = len(apiResp.Choices[0].Message.Content) / 4
	}

	return &CompletionResponse{
		Content:          apiResp.Choices[0].Message.Content,
		TokensPrompt:     tokensPrompt,
		TokensCompletion: tokensCompletion,
		Cost:             GetTokenCost(req.Model, tokensPrompt, tokensCompletion),
	}, nil
}

func (p *OpenAIProvider) CompletionStream(ctx context.Context, req *CompletionRequest, stream chan<- *StreamChunk) error {
	url := fmt.Sprintf("%s/chat/completions", p.getURL(req))

	bodyMap := map[string]interface{}{
		"model":       req.Model,
		"messages":    req.Messages,
		"temperature": req.Temperature,
		"stream":      true,
		"stream_options": map[string]interface{}{
			"include_usage": true, // Fetch usage statistics from streaming if supported
		},
	}
	if req.MaxTokens > 0 {
		bodyMap["max_tokens"] = req.MaxTokens
	}

	jsonData, err := json.Marshal(bodyMap)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", req.APIKey))
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("provider API returned status %d: %s", resp.StatusCode, string(respBody))
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
				}
				break
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// SSE protocol prefixes events with "data: "
			if !strings.HasPrefix(line, "data:") {
				continue
			}

			dataStr := strings.TrimPrefix(line, "data:")
			dataStr = strings.TrimSpace(dataStr)

			if dataStr == "[DONE]" {
				stream <- &StreamChunk{
					Done:             true,
					TokensPrompt:     tokensPrompt,
					TokensCompletion: tokensCompletion,
					Cost:             GetTokenCost(req.Model, tokensPrompt, tokensCompletion),
				}
				break
			}

			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
				// Some proxies output intermediate empty objects
				continue
			}

			// Parse usage if returned
			if chunk.Usage != nil {
				tokensPrompt = chunk.Usage.PromptTokens
				tokensCompletion = chunk.Usage.CompletionTokens
			}

			if len(chunk.Choices) > 0 {
				deltaText := chunk.Choices[0].Delta.Content
				if deltaText != "" {
					stream <- &StreamChunk{Content: deltaText, Done: false}
					tokensCompletion++ // fallback counting increment
				}
			}
		}
	}()

	return nil
}

func (p *OpenAIProvider) ValidateCredentials(ctx context.Context, apiKey string, apiURL string) (bool, error) {
	// Simple validation: hit /models with standard token check
	url := fmt.Sprintf("%s/models", p.DefaultURL)
	if apiURL != "" {
		url = fmt.Sprintf("%s/models", strings.TrimSuffix(apiURL, "/"))
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	if apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, fmt.Errorf("invalid authentication credentials")
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("endpoint returned error status %d", resp.StatusCode)
	}

	return true, nil
}
