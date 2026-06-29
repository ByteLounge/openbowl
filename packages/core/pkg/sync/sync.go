package sync

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
	"github.com/openbowl/openbowl/packages/core/pkg/models"
)

type SyncManager struct {
	DB           *db.DB
	CloudBaseURL string
	WorkspaceID  string
	SyncInterval time.Duration
	quit         chan struct{}
}

func NewSyncManager(database *db.DB, cloudURL string, workspaceID string) *SyncManager {
	return &SyncManager{
		DB:           database,
		CloudBaseURL: cloudURL,
		WorkspaceID:  workspaceID,
		SyncInterval: 30 * time.Second, // default interval
		quit:         make(chan struct{}),
	}
}

// Start spawns the background sync worker loop
func (sm *SyncManager) Start() {
	go sm.syncLoop()
	log.Printf("Sync Manager started for Workspace %s (Interval: %v)", sm.WorkspaceID, sm.SyncInterval)
}

// Stop halts the background sync worker loop
func (sm *SyncManager) Stop() {
	close(sm.quit)
}

func (sm *SyncManager) syncLoop() {
	ticker := time.NewTicker(sm.SyncInterval)
	defer ticker.Stop()

	// Initial sync push/pull on startup
	if err := sm.Reconcile(); err != nil {
		log.Printf("[Sync Engine] Initial reconciliation failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := sm.Reconcile(); err != nil {
				log.Printf("[Sync Engine] Periodic sync failed: %v", err)
			}
		case <-sm.quit:
			log.Println("[Sync Engine] Background sync loop stopped.")
			return
		}
	}
}

// LogMutation records a local write operation inside a database transaction outbox
func LogMutation(tx *sql.Tx, workspaceID string, entityType string, entityID string, opType string, entityPayload interface{}) error {
	payloadBytes, err := json.Marshal(entityPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal sync payload: %w", err)
	}

	// Fetch current max sequence number to maintain strict delta ordering
	var currentSeq int64
	err = tx.QueryRow(`SELECT COALESCE(MAX(sequence_number), 0) FROM sync_operations WHERE workspace_id = ?`, workspaceID).Scan(&currentSeq)
	if err != nil {
		return fmt.Errorf("failed to scan max sequence: %w", err)
	}

	nextSeq := currentSeq + 1

	_, err = tx.Exec(`
		INSERT INTO sync_operations (workspace_id, entity_type, entity_id, operation_type, payload, sequence_number, applied_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		workspaceID, entityType, entityID, opType, string(payloadBytes), nextSeq, time.Now())
	
	if err != nil {
		return fmt.Errorf("failed to write sync outbox: %w", err)
	}

	return nil
}

// Reconcile performs a two-way synchronization push/pull operation
func (sm *SyncManager) Reconcile() error {
	// 1. Push Unsynced Operations (Outbox Pattern)
	if err := sm.pushWithBackoff(); err != nil {
		return fmt.Errorf("push outbox failure: %w", err)
	}

	// 2. Pull Remote Operations (Delta Replay)
	if err := sm.pullWithBackoff(); err != nil {
		return fmt.Errorf("pull delta failure: %w", err)
	}

	return nil
}

func (sm *SyncManager) pushWithBackoff() error {
	// Retrieve unsynced operations
	rows, err := sm.DB.Conn.Query(`
		SELECT id, workspace_id, entity_type, entity_id, operation_type, payload, sequence_number, applied_at
		FROM sync_operations 
		WHERE workspace_id = ? AND synced_at IS NULL 
		ORDER BY sequence_number ASC`, sm.WorkspaceID)
	if err != nil {
		return err
	}
	defer rows.Close()

	ops := make([]models.SyncOperation, 0)
	for rows.Next() {
		var op models.SyncOperation
		err := rows.Scan(&op.ID, &op.WorkspaceID, &op.EntityType, &op.EntityID, &op.OperationType, &op.Payload, &op.SequenceNumber, &op.AppliedAt)
		if err == nil {
			ops = append(ops, op)
		}
	}

	if len(ops) == 0 {
		return nil // Nothing to sync
	}

	// Post operations payload to CloudSync endpoint with exponential backoff
	backoff := 1 * time.Second
	maxBackoff := 8 * time.Second
	url := fmt.Sprintf("%s/api/v1/sync/push", sm.CloudBaseURL)

	jsonData, err := json.Marshal(map[string]interface{}{
		"workspace_id": sm.WorkspaceID,
		"operations":   ops,
	})
	if err != nil {
		return err
	}

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			cancel()
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		cancel()

		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			break // Push successful!
		}

		if resp != nil {
			resp.Body.Close()
		}

		log.Printf("[Sync Engine] Cloud push failed, retrying in %v...", backoff)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			return fmt.Errorf("network push threshold exceeded")
		}
	}

	// Update local database operation logs as successfully synced
	tx, err := sm.DB.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	for _, op := range ops {
		_, err := tx.Exec(`UPDATE sync_operations SET synced_at = ? WHERE id = ?`, now, op.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (sm *SyncManager) pullWithBackoff() error {
	// Retrieve local max synced sequence
	var lastSeq int64
	err := sm.DB.Conn.QueryRow(`
		SELECT COALESCE(MAX(sequence_number), 0) 
		FROM sync_operations 
		WHERE workspace_id = ? AND synced_at IS NOT NULL`, sm.WorkspaceID).Scan(&lastSeq)
	if err != nil {
		return err
	}

	// Pull remote mutations delta starting from lastSeq
	backoff := 1 * time.Second
	maxBackoff := 8 * time.Second
	url := fmt.Sprintf("%s/api/v1/sync/pull?workspace_id=%s&since_seq=%d", sm.CloudBaseURL, sm.WorkspaceID, lastSeq)

	var remoteOps []models.SyncOperation

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			cancel()
			return err
		}

		resp, err := http.DefaultClient.Do(req)
		cancel()

		if err == nil && resp.StatusCode == http.StatusOK {
			err = json.NewDecoder(resp.Body).Decode(&remoteOps)
			resp.Body.Close()
			if err == nil {
				break // Pull successful!
			}
		}

		if resp != nil {
			resp.Body.Close()
		}

		log.Printf("[Sync Engine] Cloud pull failed, retrying in %v...", backoff)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			return fmt.Errorf("network pull threshold exceeded")
		}
	}

	if len(remoteOps) == 0 {
		return nil // Up to date
	}

	// Process pulled operations locally
	return sm.applyRemoteOperations(remoteOps)
}

func (sm *SyncManager) applyRemoteOperations(ops []models.SyncOperation) error {
	tx, err := sm.DB.Conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, op := range ops {
		switch op.EntityType {
		case "task":
			var task models.Task
			if err := json.Unmarshal([]byte(op.Payload), &task); err != nil {
				continue
			}

			// Apply Task mutation (Last-Write-Wins based on Update timestamp)
			if op.OperationType == "INSERT" || op.OperationType == "UPDATE" {
				_, err = tx.Exec(`
					INSERT INTO tasks (id, project_id, title, description, status, due_date, completed_at, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT(id) DO UPDATE SET
						title=excluded.title,
						description=excluded.description,
						status=excluded.status,
						due_date=excluded.due_date,
						completed_at=excluded.completed_at,
						updated_at=excluded.updated_at
					WHERE excluded.updated_at > tasks.updated_at`, // LWW Rule
					task.ID, task.ProjectID, task.Title, task.Description, task.Status, task.DueDate, task.CompletedAt, task.CreatedAt, task.UpdatedAt)
			} else if op.OperationType == "DELETE" {
				_, err = tx.Exec(`DELETE FROM tasks WHERE id = ?`, op.EntityID)
			}

		case "memory":
			var m models.Memory
			if err := json.Unmarshal([]byte(op.Payload), &m); err != nil {
				continue
			}

			if op.OperationType == "INSERT" || op.OperationType == "UPDATE" {
				_, err = tx.Exec(`
					INSERT INTO memories (id, workspace_id, category, content, source_conversation_id, source_message_id, is_active, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT(id) DO UPDATE SET
						category=excluded.category,
						content=excluded.content,
						is_active=excluded.is_active,
						updated_at=excluded.updated_at
					WHERE excluded.updated_at > memories.updated_at`, // LWW Rule
					m.ID, m.WorkspaceID, m.Category, m.Content, m.ConversationID, m.MessageID, m.IsActive, m.CreatedAt, m.UpdatedAt)
			} else if op.OperationType == "DELETE" {
				_, err = tx.Exec(`DELETE FROM memories WHERE id = ?`, op.EntityID)
			}
		}

		if err != nil {
			return fmt.Errorf("failed to apply remote operation: %w", err)
		}

		// Save pulled operation locally so we don't request it again
		_, err = tx.Exec(`
			INSERT OR IGNORE INTO sync_operations (workspace_id, entity_type, entity_id, operation_type, payload, sequence_number, applied_at, synced_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			op.WorkspaceID, op.EntityType, op.EntityID, op.OperationType, op.Payload, op.SequenceNumber, op.AppliedAt, time.Now())
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
