package provider

import (
	"fmt"
)

// GetProvider instantiates the correct Provider adapter based on the provider type.
func GetProvider(providerType string) (Provider, error) {
	switch providerType {
	case "openai", "groq", "deepseek", "openrouter", "lmstudio":
		return NewOpenAIProvider(), nil
	case "gemini":
		return NewGeminiProvider(), nil
	case "anthropic":
		return NewAnthropicProvider(), nil
	case "ollama":
		return NewOllamaProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}
}
