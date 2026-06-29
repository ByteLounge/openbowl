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

type AnthropicProvider struct {
	BaseURL string
}

func NewAnthropicProvider() *AnthropicProvider {
	return &AnthropicProvider{
		BaseURL: "https://api.anthropic.com/v1",
	}
}

type anthropicMessage struct {
	Role    string `json:"role"` // "user", "assistant"
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float32            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model string         `json:"model"`
	Usage anthropicUsage `json:"usage"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta *struct {
		Type string `json:"type,omitempty"`
		Text string `json:"text,omitempty"`
	} `json:"delta,omitempty"`
	Message *struct {
		ID    string         `json:"id"`
		Model string         `json:"model"`
		Usage anthropicUsage `json:"usage"`
	} `json:"message,omitempty"`
	Usage *anthropicUsage `json:"usage,omitempty"`
}

func (p *AnthropicProvider) transformRequest(req *CompletionRequest) *anthropicRequest {
	aReq := &anthropicRequest{
		Model:     req.Model,
		Messages:  make([]anthropicMessage, 0),
		MaxTokens: 4096, // Anthropic requires max_tokens
	}

	if req.MaxTokens > 0 {
		aReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature > 0 {
		aReq.Temperature = req.Temperature
	}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			aReq.System = msg.Content
		} else {
			role := msg.Role
			if role == "model" {
				role = "assistant"
			}
			aReq.Messages = append(aReq.Messages, anthropicMessage{
				Role:    role,
				Content: msg.Content,
			})
		}
	}

	return aReq
}

func (p *AnthropicProvider) Completion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	url := fmt.Sprintf("%s/messages", p.BaseURL)

	aReq := p.transformRequest(req)
	aReq.Stream = false

	jsonData, err := json.Marshal(aReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", req.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("anthropic returned empty content blocks")
	}

	contentText := apiResp.Content[0].Text
	tokensPrompt := apiResp.Usage.InputTokens
	tokensCompletion := apiResp.Usage.OutputTokens

	return &CompletionResponse{
		Content:          contentText,
		TokensPrompt:     tokensPrompt,
		TokensCompletion: tokensCompletion,
		Cost:             GetTokenCost(req.Model, tokensPrompt, tokensCompletion),
	}, nil
}

func (p *AnthropicProvider) CompletionStream(ctx context.Context, req *CompletionRequest, stream chan<- *StreamChunk) error {
	url := fmt.Sprintf("%s/messages", p.BaseURL)

	aReq := p.transformRequest(req)
	aReq.Stream = true

	jsonData, err := json.Marshal(aReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", req.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("anthropic API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	go func() {
		defer resp.Body.Close()
		defer close(stream)

		reader := bufio.NewReader(resp.Body)
		tokensPrompt := 0
		tokensCompletion := 0

		var eventName string

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
			if line == "" {
				continue
			}

			if strings.HasPrefix(line, "event:") {
				eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
				continue
			}

			if !strings.HasPrefix(line, "data:") {
				continue
			}

			dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(dataStr), &event); err != nil {
				continue
			}

			// Capture prompt token usage on message start
			if event.Message != nil && event.Message.Usage.InputTokens > 0 {
				tokensPrompt = event.Message.Usage.InputTokens
			}

			// Capture completion tokens usage on message delta
			if event.Usage != nil {
				tokensCompletion = event.Usage.OutputTokens
			}

			switch eventName {
			case "content_block_delta":
				if event.Delta != nil && event.Delta.Text != "" {
					stream <- &StreamChunk{Content: event.Delta.Text, Done: false}
				}
			case "message_delta":
				if event.Delta != nil && event.Usage != nil {
					tokensCompletion = event.Usage.OutputTokens
				}
			case "message_stop":
				stream <- &StreamChunk{
					Done:             true,
					TokensPrompt:     tokensPrompt,
					TokensCompletion: tokensCompletion,
					Cost:             GetTokenCost(req.Model, tokensPrompt, tokensCompletion),
				}
				return
			}
		}
	}()

	return nil
}

func (p *AnthropicProvider) ValidateCredentials(ctx context.Context, apiKey string, apiURL string) (bool, error) {
	// Send an empty completion request to trigger key check
	url := fmt.Sprintf("%s/messages", p.BaseURL)
	aReq := &anthropicRequest{
		Model:     "claude-3-haiku-20240307",
		Messages:  []anthropicMessage{{Role: "user", Content: "Ping"}},
		MaxTokens: 1,
	}

	jsonData, _ := json.Marshal(aReq)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, fmt.Errorf("invalid API credentials")
	}

	return true, nil
}
