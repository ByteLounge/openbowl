package provider

import (
	"context"
)

// Message is the unified structure for LLM conversation logs
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// CompletionRequest is the unified request parameters passed to SDK adapters
type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float32   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
	APIKey      string    `json:"api_key,omitempty"`
	APIURL      string    `json:"api_url,omitempty"`
}

// CompletionResponse is the unified response struct returned on non-streaming calls
type CompletionResponse struct {
	Content          string  `json:"content"`
	TokensPrompt     int     `json:"tokens_prompt"`
	TokensCompletion int     `json:"tokens_completion"`
	Cost             float64 `json:"cost"`
}

// StreamChunk represents a delta text packet sent during streaming completions
type StreamChunk struct {
	Content          string  `json:"content"`
	TokensPrompt     int     `json:"tokens_prompt,omitempty"`
	TokensCompletion int     `json:"tokens_completion,omitempty"`
	Cost             float64 `json:"cost,omitempty"`
	Done             bool    `json:"done"`
	Error            string  `json:"error,omitempty"`
}

// Provider defines the interface that all LLM vendors must implement
type Provider interface {
	Completion(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
	CompletionStream(ctx context.Context, req *CompletionRequest, stream chan<- *StreamChunk) error
	ValidateCredentials(ctx context.Context, apiKey string, apiURL string) (bool, error)
}

// GetTokenCost calculates estimated dollar cost based on model and token counts
func GetTokenCost(model string, tokensPrompt int, tokensCompletion int) float64 {
	// Cost per million tokens
	var inputCost, outputCost float64

	switch model {
	// OpenAI
	case "gpt-4o":
		inputCost = 5.0
		outputCost = 15.0
	case "gpt-4-turbo":
		inputCost = 10.0
		outputCost = 30.0
	case "gpt-3.5-turbo":
		inputCost = 0.50
		outputCost = 1.50

	// Anthropic
	case "claude-3-5-sonnet":
		inputCost = 3.0
		outputCost = 15.0
	case "claude-3-opus":
		inputCost = 15.0
		outputCost = 75.0
	case "claude-3-haiku":
		inputCost = 0.25
		outputCost = 1.25

	// Gemini
	case "gemini-1.5-pro":
		inputCost = 7.0
		outputCost = 21.0
	case "gemini-1.5-flash":
		inputCost = 0.35
		outputCost = 1.05

	// DeepSeek
	case "deepseek-chat", "deepseek-reasoner":
		inputCost = 0.14
		outputCost = 0.28

	// Groq / OpenRouter averages (conservative estimate)
	case "llama3-70b-8192":
		inputCost = 0.59
		outputCost = 0.79

	// Local models (Ollama, LM Studio)
	default:
		return 0.0
	}

	costIn := (float64(tokensPrompt) / 1000000.0) * inputCost
	costOut := (float64(tokensCompletion) / 1000000.0) * outputCost
	return costIn + costOut
}
