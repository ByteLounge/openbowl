package sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
	"github.com/openbowl/openbowl/packages/core/pkg/models"
)

func TestLogMutationAndOutbox(t *testing.T) {
	// 1. Setup DB
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	wsID := "ws-test-sync"
	_, err = database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Test Workspace")
	if err != nil {
		t.Fatalf("Failed to seed workspace: %v", err)
	}
	task := models.Task{
		ID:        "task-sync-1",
		ProjectID: "proj-1",
		Title:     "Synchronize Outbox Logs",
		Status:    "in_progress",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	tx, err := database.Conn.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Log local mutation
	err = LogMutation(tx, wsID, "task", task.ID, "INSERT", task)
	if err != nil {
		t.Fatalf("LogMutation failed: %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Verify operation was recorded
	var op models.SyncOperation
	err = database.Conn.QueryRow(`
		SELECT id, workspace_id, entity_type, entity_id, operation_type, payload, sequence_number
		FROM sync_operations WHERE workspace_id = ?`, wsID).
		Scan(&op.ID, &op.WorkspaceID, &op.EntityType, &op.EntityID, &op.OperationType, &op.Payload, &op.SequenceNumber)

	if err != nil {
		t.Fatalf("Failed to query outbox: %v", err)
	}

	if op.EntityType != "task" || op.OperationType != "INSERT" || op.SequenceNumber != 1 {
		t.Errorf("Unexpected outbox record: %+v", op)
	}

	var parsedTask models.Task
	if err := json.Unmarshal([]byte(op.Payload), &parsedTask); err != nil {
		t.Fatalf("Failed to parse payload: %v", err)
	}

	if parsedTask.Title != task.Title {
		t.Errorf("Payload title mismatch: got %s, want %s", parsedTask.Title, task.Title)
	}
}

func TestSyncReconcilePushPull(t *testing.T) {
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	wsID := "ws-test-reconcile"
	_, _ = database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Test Workspace")
	_, _ = database.Conn.Exec(`INSERT INTO projects (id, workspace_id, name) VALUES (?, ?, ?)`, "proj-1", wsID, "Test Project")

	// Pre-seed an unsynced local mutation
	task := models.Task{
		ID:        "task-local",
		ProjectID: "proj-1",
		Title:     "Local Unsynced Task",
		Status:    "todo",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	tx, _ := database.Conn.Begin()
	_ = LogMutation(tx, wsID, "task", task.ID, "INSERT", task)
	_ = tx.Commit()

	// Remote pull operations payload stub
	remoteTask := models.Task{
		ID:        "task-remote",
		ProjectID: "proj-1",
		Title:     "Remote Synced Task",
		Status:    "completed",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	remotePayloadBytes, _ := json.Marshal(remoteTask)

	remoteOps := []models.SyncOperation{
		{
			WorkspaceID:    wsID,
			EntityType:     "task",
			EntityID:       remoteTask.ID,
			OperationType:  "INSERT",
			Payload:        string(remotePayloadBytes),
			SequenceNumber: 10,
			AppliedAt:      time.Now(),
		},
	}

	pushCalled := false
	pullCalled := false

	// Mock server listening to sync requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/sync/push" {
			pushCalled = true
			w.WriteHeader(http.StatusOK)
		} else if r.URL.Path == "/api/v1/sync/pull" {
			pullCalled = true
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(remoteOps)
		}
	}))
	defer server.Close()

	sm := NewSyncManager(database, server.URL, wsID)

	// Run Reconcile
	err = sm.Reconcile()
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if !pushCalled {
		t.Error("Expected push endpoint to be called")
	}
	if !pullCalled {
		t.Error("Expected pull endpoint to be called")
	}

	// Verify local database state updates
	var localSyncedTime string
	err = database.Conn.QueryRow(`SELECT COALESCE(synced_at, '') FROM sync_operations WHERE entity_id = 'task-local'`).Scan(&localSyncedTime)
	if err != nil {
		t.Fatalf("Failed to query synced local operation: %v", err)
	}
	if localSyncedTime == "" {
		t.Error("Expected local sync operation to be marked as synced")
	}

	// Check that the remote task was successfully integrated locally
	var remoteTaskTitle string
	err = database.Conn.QueryRow(`SELECT title FROM tasks WHERE id = 'task-remote'`).Scan(&remoteTaskTitle)
	if err != nil {
		t.Fatalf("Failed to fetch pulled task: %v", err)
	}

	if remoteTaskTitle != "Remote Synced Task" {
		t.Errorf("Expected task title 'Remote Synced Task', got %s", remoteTaskTitle)
	}
}
