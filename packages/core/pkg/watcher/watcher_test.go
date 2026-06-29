package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
)

func TestFileWatcherSyncAndTrack(t *testing.T) {
	// 1. Setup in-memory SQLite DB
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize DB: %v", err)
	}
	defer database.Close()

	// Seed workspace & project for foreign keys
	wsID := "w-watcher-test"
	projID := "proj-watcher-test"
	_, _ = database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Test Workspace")
	_, _ = database.Conn.Exec(`INSERT INTO projects (id, workspace_id, name) VALUES (?, ?, ?)`, projID, wsID, "Test Project")

	// 2. Setup temp directory structure
	tempDir, err := os.MkdirTemp("", "openbowl-watcher-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	file1 := filepath.Join(tempDir, "hello.txt")
	if err := os.WriteFile(file1, []byte("hello world"), 0644); err != nil {
		t.Fatalf("Failed to write mock file 1: %v", err)
	}

	// 3. Initialize & Start Watcher
	fw, err := NewFileWatcher(database, projID, tempDir)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	fw.Start()
	defer fw.Stop()

	// Give the watcher loop a small fraction of a second to index on startup
	time.Sleep(100 * time.Millisecond)

	// Verify initial sync indexed the file
	var count int
	err = database.Conn.QueryRow(`SELECT COUNT(*) FROM file_references`).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 file indexed initial, got %d", count)
	}

	// 4. Test fsnotify trigger file creation
	file2 := filepath.Join(tempDir, "sparks.log")
	if err := os.WriteFile(file2, []byte("memory sparks"), 0644); err != nil {
		t.Fatalf("Failed to create mock file 2: %v", err)
	}

	// Wait briefly for fsnotify event loop to process and write to SQLite
	time.Sleep(150 * time.Millisecond)

	err = database.Conn.QueryRow(`SELECT COUNT(*) FROM file_references`).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 files indexed after fsnotify trigger, got %d", count)
	}

	// 5. Test file deletion
	if err := os.Remove(file2); err != nil {
		t.Fatalf("Failed to remove file 2: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	err = database.Conn.QueryRow(`SELECT COUNT(*) FROM file_references`).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 file indexed after deleting file 2, got %d", count)
	}
}
