package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
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

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3020" // Sync server runs on port 3020 by default
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "openbowl_sync.db"
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

	// Push local client mutations
	r.POST("/api/v1/sync/push", handlePush)

	// Pull remote mutations delta
	r.GET("/api/v1/sync/pull", handlePull)

	log.Printf("OpenBowl Sync Backend starting on http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Fatal: Server execution failed: %v", err)
	}
}

func handlePush(c *gin.Context) {
	var req struct {
		WorkspaceID string          `json:"workspace_id"`
		Operations  []SyncOperation `json:"operations"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.WorkspaceID == "" || len(req.Operations) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing workspace_id or operations payload"})
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
		// Insert operation delta generating a globally ordered sequence key
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
	workspaceID := c.Query("workspace_id")
	sinceSeqStr := c.Query("since_seq")

	if workspaceID == "" || sinceSeqStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing workspace_id or since_seq parameter"})
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
