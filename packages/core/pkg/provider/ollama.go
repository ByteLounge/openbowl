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

type OllamaProvider struct {
	DefaultURL string
}

func NewOllamaProvider() *OllamaProvider {
	return &OllamaProvider{
		DefaultURL: "http://localhost:11434",
	}
}

type ollamaMessage struct {
	Role    string   `json:"role"` // "system", "user", "assistant"
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type ollamaRequest struct {
	Model    string                 `json:"model"`
	Messages []ollamaMessage        `json:"messages"`
	Options  map[string]interface{} `json:"options,omitempty"`
	Stream   bool                   `json:"stream"`
}

type ollamaResponse struct {
	Model           string        `json:"model"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	PromptEvalCount int           `json:"prompt_eval_count"`
	EvalCount       int           `json:"eval_count"`
}

func (p *OllamaProvider) getURL(req *CompletionRequest) string {
	if req.APIURL != "" {
		return strings.TrimSuffix(req.APIURL, "/")
	}
	return p.DefaultURL
}

func (p *OllamaProvider) transformRequest(req *CompletionRequest) *ollamaRequest {
	oReq := &ollamaRequest{
		Model:    req.Model,
		Messages: make([]ollamaMessage, 0),
		Options:  make(map[string]interface{}),
	}

	if req.Temperature > 0 {
		oReq.Options["temperature"] = req.Temperature
	}
	if req.MaxTokens > 0 {
		oReq.Options["num_predict"] = req.MaxTokens
	}

	for _, msg := range req.Messages {
		role := msg.Role
		if role == "model" {
			role = "assistant"
		}
		oReq.Messages = append(oReq.Messages, ollamaMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	return oReq
}

func (p *OllamaProvider) Completion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	url := fmt.Sprintf("%s/api/chat", p.getURL(req))

	oReq := p.transformRequest(req)
	oReq.Stream = false

	jsonData, err := json.Marshal(oReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
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
		return nil, fmt.Errorf("ollama API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	tokensPrompt := apiResp.PromptEvalCount
	tokensCompletion := apiResp.EvalCount

	if tokensPrompt == 0 {
		tokensPrompt = len(jsonData) / 4
		tokensCompletion = len(apiResp.Message.Content) / 4
	}

	return &CompletionResponse{
		Content:          apiResp.Message.Content,
		TokensPrompt:     tokensPrompt,
		TokensCompletion: tokensCompletion,
		Cost:             0.0, // Local execution cost is $0
	}, nil
}

func (p *OllamaProvider) CompletionStream(ctx context.Context, req *CompletionRequest, stream chan<- *StreamChunk) error {
	url := fmt.Sprintf("%s/api/chat", p.getURL(req))

	oReq := p.transformRequest(req)
	oReq.Stream = true

	jsonData, err := json.Marshal(oReq)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
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
		return fmt.Errorf("ollama API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	go func() {
		defer resp.Body.Close()
		defer close(stream)

		reader := bufio.NewReader(resp.Body)
		tokensPrompt := 0
		tokensCompletion := 0

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					stream <- &StreamChunk{Done: true, Error: err.Error()}
				} else {
					stream <- &StreamChunk{
						Done:             true,
						TokensPrompt:     tokensPrompt,
						TokensCompletion: tokensCompletion,
						Cost:             0.0,
					}
				}
				break
			}

			if len(line) == 0 {
				continue
			}

			var chunk ollamaResponse
			if err := json.Unmarshal(line, &chunk); err != nil {
				continue
			}

			if chunk.PromptEvalCount > 0 {
				tokensPrompt = chunk.PromptEvalCount
			}
			if chunk.EvalCount > 0 {
				tokensCompletion = chunk.EvalCount
			}

			if chunk.Message.Content != "" {
				stream <- &StreamChunk{Content: chunk.Message.Content, Done: false}
			}

			if chunk.Done {
				stream <- &StreamChunk{
					Done:             true,
					TokensPrompt:     tokensPrompt,
					TokensCompletion: tokensCompletion,
					Cost:             0.0,
				}
				break
			}
		}
	}()

	return nil
}

func (p *OllamaProvider) ValidateCredentials(ctx context.Context, apiKey string, apiURL string) (bool, error) {
	// Ping main server endpoint (e.g. GET /)
	url := p.getURL(&CompletionRequest{APIURL: apiURL})
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("local server unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("server returned status code %d", resp.StatusCode)
	}

	return true, nil
}
