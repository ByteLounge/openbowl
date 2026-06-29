package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/glebarez/go-sqlite"
)

type SyncOperation struct {
	ID             int64     `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	EntityType     string    `json:"entity_type"`
	EntityID       string    `json:"entity_id"`
	OperationType  string    `json:"operation_type"`
	Payload        string    `json:"payload"`
	SequenceNumber int64     `json:"sequence_number"`
	AppliedAt      time.Time `json:"applied_at"`
}

var dbConn *sql.DB
var jwtSecretKey = []byte("openbowl-super-secret-key-change-in-prod")

// JWT Claims structure
type JWTClaims struct {
	WorkspaceID string `json:"workspace_id"`
	ExpiresAt   int64  `json:"expires_at"`
}

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3020"
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "openbowl_sync.db"
	}

	secret := os.Getenv("JWT_SECRET")
	if secret != "" {
		jwtSecretKey = []byte(secret)
	}

	// Initialize Database
	var err error
	dbConn, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Fatal: Failed to connect to Sync DB: %v", err)
	}
	defer dbConn.Close()

	if _, err := dbConn.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		log.Fatalf("Fatal: Failed to set WAL mode: %v", err)
	}

	// Create Sync schema
	schema := `
	CREATE TABLE IF NOT EXISTS sync_operations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		operation_type TEXT NOT NULL,
		payload TEXT NOT NULL,
		sequence_number INTEGER NOT NULL,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_sync_workspace_seq ON sync_operations(workspace_id, sequence_number);
	`
	if _, err := dbConn.Exec(schema); err != nil {
		log.Fatalf("Fatal: Failed to initialize Sync Schema: %v", err)
	}

	log.Printf("Sync Cloud DB initialized at: %s", dbPath)

	r := gin.Default()
	r.Use(cors.Default())

	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "online", "service": "openbowl-sync-engine"})
	})

	// Login endpoint to generate tokens for workspaces
	r.POST("/api/v1/auth/login", handleLogin)

	// Auth secured group
	secured := r.Group("/api/v1")
	secured.Use(AuthMiddleware())
	{
		secured.POST("/sync/push", handlePush)
		secured.GET("/sync/pull", handlePull)
	}

	log.Printf("OpenBowl Sync Backend starting on http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Fatal: Server execution failed: %v", err)
	}
}

// Generate JWT token string using native HMAC-SHA256
func GenerateJWT(workspaceID string) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	
	claims := JWTClaims{
		WorkspaceID: workspaceID,
		ExpiresAt:   time.Now().Add(24 * time.Hour).Unix(),
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	
	payload := base64.RawURLEncoding.EncodeToString(claimsBytes)
	signingInput := header + "." + payload

	h := hmac.New(sha256.New, jwtSecretKey)
	h.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return signingInput + "." + signature, nil
}

// Verify JWT token string
func VerifyJWT(tokenString string) (*JWTClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}

	h := hmac.New(sha256.New, jwtSecretKey)
	h.Write([]byte(signingInput))
	expectedSignature := h.Sum(nil)

	if !hmac.Equal(signature, expectedSignature) {
		return nil, fmt.Errorf("invalid signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, err
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("token expired")
	}

	return &claims, nil
}

// AuthMiddleware secures sync endpoints using JWT
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer <token>"})
			c.Abort()
			return
		}

		claims, err := VerifyJWT(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Unauthorized: %v", err)})
			c.Abort()
			return
		}

		c.Set("workspace_id", claims.WorkspaceID)
		c.Next()
	}
}

func handleLogin(c *gin.Context) {
	var input struct {
		WorkspaceID string `json:"workspace_id"`
		Passkey     string `json:"passkey"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// For local development, allow any passkey matching 'openbowl-dev'
	if input.Passkey != "openbowl-dev" && input.Passkey != "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid workspace passkey credentials"})
		return
	}

	token, err := GenerateJWT(input.WorkspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token generation failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"expires_in": 86400,
	})
}

func handlePush(c *gin.Context) {
	authWorkspaceID := c.MustGet("workspace_id").(string)

	var req struct {
		WorkspaceID string          `json:"workspace_id"`
		Operations  []SyncOperation `json:"operations"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user is authorized for this workspace
	if req.WorkspaceID != authWorkspaceID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Cannot push to another workspace"})
		return
	}

	if len(req.Operations) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing operations payload"})
		return
	}

	tx, err := dbConn.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()

	// Fetch current max sequence number on the cloud server
	var currentSeq int64
	err = tx.QueryRow(`SELECT COALESCE(MAX(sequence_number), 0) FROM sync_operations WHERE workspace_id = ?`, req.WorkspaceID).Scan(&currentSeq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	nextSeq := currentSeq + 1

	for _, op := range req.Operations {
		_, err = tx.Exec(`
			INSERT INTO sync_operations (workspace_id, entity_type, entity_id, operation_type, payload, sequence_number, applied_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			req.WorkspaceID, op.EntityType, op.EntityID, op.OperationType, op.Payload, nextSeq, time.Now())
		
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		nextSeq++
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "synced", "processed_count": len(req.Operations)})
}

func handlePull(c *gin.Context) {
	authWorkspaceID := c.MustGet("workspace_id").(string)
	workspaceID := c.Query("workspace_id")
	sinceSeqStr := c.Query("since_seq")

	if workspaceID == "" || sinceSeqStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing workspace_id or since_seq parameter"})
		return
	}

	if workspaceID != authWorkspaceID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden: Cannot pull from another workspace"})
		return
	}

	sinceSeq, err := strconv.ParseInt(sinceSeqStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid since_seq format"})
		return
	}

	rows, err := dbConn.Query(`
		SELECT id, workspace_id, entity_type, entity_id, operation_type, payload, sequence_number, applied_at
		FROM sync_operations
		WHERE workspace_id = ? AND sequence_number > ?
		ORDER BY sequence_number ASC`, workspaceID, sinceSeq)
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	ops := make([]SyncOperation, 0)
	for rows.Next() {
		var op SyncOperation
		err := rows.Scan(&op.ID, &op.WorkspaceID, &op.EntityType, &op.EntityID, &op.OperationType, &op.Payload, &op.SequenceNumber, &op.AppliedAt)
		if err == nil {
			ops = append(ops, op)
		}
	}

	c.JSON(http.StatusOK, ops)
}
