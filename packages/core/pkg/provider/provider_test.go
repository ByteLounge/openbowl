package provider

import (
	"math"
	"testing"
)

func TestGetTokenCost(t *testing.T) {
	tests := []struct {
		model            string
		promptTokens     int
		completionTokens int
		expectedCost     float64
	}{
		{"gpt-4o", 1000000, 1000000, 20.0},
		{"claude-3-5-sonnet", 1000000, 1000000, 18.0},
		{"gemini-1.5-flash", 1000000, 1000000, 1.40},
		{"deepseek-chat", 1000000, 1000000, 0.42},
		{"ollama-local", 5000, 2000, 0.0},
	}

	delta := 0.000001
	for _, tt := range tests {
		cost := GetTokenCost(tt.model, tt.promptTokens, tt.completionTokens)
		if math.Abs(cost-tt.expectedCost) > delta {
			t.Errorf("GetTokenCost(%s, %d, %d) = %f; want %f",
				tt.model, tt.promptTokens, tt.completionTokens, cost, tt.expectedCost)
		}
	}
}

func TestGetProviderFactory(t *testing.T) {
	validTypes := []string{"openai", "groq", "deepseek", "gemini", "anthropic", "ollama"}
	for _, vt := range validTypes {
		p, err := GetProvider(vt)
		if err != nil {
			t.Errorf("GetProvider(%s) returned error: %v", vt, err)
		}
		if p == nil {
			t.Errorf("GetProvider(%s) returned nil provider", vt)
		}
	}

	_, err := GetProvider("unknown-provider")
	if err == nil {
		t.Error("GetProvider(unknown-provider) should have failed")
	}
}
