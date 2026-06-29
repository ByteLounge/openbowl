package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGenerateAndVerifyJWT(t *testing.T) {
	wsID := "w-auth-test-123"
	
	// 1. Generate valid token
	token, err := GenerateJWT(wsID)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if len(strings.Split(token, ".")) != 3 {
		t.Fatalf("Token does not have 3 parts: %s", token)
	}

	// 2. Verify token
	claims, err := VerifyJWT(token)
	if err != nil {
		t.Fatalf("Failed to verify valid token: %v", err)
	}

	if claims.WorkspaceID != wsID {
		t.Errorf("Expected WorkspaceID %s, got %s", wsID, claims.WorkspaceID)
	}

	// 3. Verify signature tampering detection
	tamperedToken := token + "tamper"
	_, err = VerifyJWT(tamperedToken)
	if err == nil {
		t.Error("Expected error when verifying tampered token, got nil")
	}

	// 4. Verify expired token detection
	expiredClaims := JWTClaims{
		WorkspaceID: wsID,
		ExpiresAt:   time.Now().Add(-1 * time.Hour).Unix(),
	}
	expiredToken, _ := GenerateJWT(wsID)
	// Inject expired payload manually
	parts := strings.Split(expiredToken, ".")
	claimsBytes, _ := json.Marshal(expiredClaims)
	payload := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signingInput := parts[0] + "." + payload
	h := hmac.New(sha256.New, jwtSecretKey)
	h.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	expiredTokenCombined := signingInput + "." + signature

	_, err = VerifyJWT(expiredTokenCombined)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("Expected token expiration error, got %v", err)
	}
}
