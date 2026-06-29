package context

import (
	"os"
	"strings"
	"testing"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
)

func TestContextAssemble(t *testing.T) {
	// 1. Setup temporary in-memory database
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory DB: %v", err)
	}
	defer database.Close()

	// 2. Seed test workspace and projects
	wsID := "ws-test-123"
	projID := "proj-test-123"
	convID := "conv-test-123"

	_, err = database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Test Workspace")
	if err != nil {
		t.Fatalf("Failed to seed workspace: %v", err)
	}

	_, err = database.Conn.Exec(`INSERT INTO projects (id, workspace_id, name, description) VALUES (?, ?, ?, ?)`,
		projID, wsID, "Test Project", "Build the ultimate test suite")
	if err != nil {
		t.Fatalf("Failed to seed project: %v", err)
	}

	// 3. Seed active memories
	_, err = database.Conn.Exec(`
		INSERT INTO memories (id, workspace_id, category, content, is_active) 
		VALUES (?, ?, ?, ?, ?)`, "mem-1", wsID, "decision", "SQLite DB format preferred.", 1)
	if err != nil {
		t.Fatalf("Failed to seed memory: %v", err)
	}

	// 4. Seed tasks
	_, err = database.Conn.Exec(`
		INSERT INTO tasks (id, project_id, title, description, status) 
		VALUES (?, ?, ?, ?, ?)`, "task-1", projID, "Write Engine Tests", "Test token counts and pruning.", "todo")
	if err != nil {
		t.Fatalf("Failed to seed task: %v", err)
	}

	// 5. Seed file reference
	tempFile, err := os.CreateTemp("", "openbowl-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	fileContent := "const answer = 42;\n// Ultimate Answer"
	if _, err := tempFile.WriteString(fileContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	_, err = database.Conn.Exec(`
		INSERT INTO file_references (id, project_id, relative_path, file_hash, content_summary) 
		VALUES (?, ?, ?, ?, ?)`, "file-1", projID, tempFile.Name(), "hash-123", "Short Javascript code.")
	if err != nil {
		t.Fatalf("Failed to seed file reference: %v", err)
	}

	// 5.5 Seed conversation
	_, err = database.Conn.Exec(`
		INSERT INTO conversations (id, project_id, title) 
		VALUES (?, ?, ?)`, convID, projID, "Test Conversation")
	if err != nil {
		t.Fatalf("Failed to seed conversation: %v", err)
	}

	// 6. Seed messages
	messages := []struct {
		id      string
		role    string
		content string
		created string
	}{
		{"msg-1", "user", "Hello there", "2026-06-29T10:00:00Z"},
		{"msg-2", "assistant", "Hi! How can I help you?", "2026-06-29T10:01:00Z"},
		{"msg-3", "user", "What is the token budget algorithm?", "2026-06-29T10:02:00Z"},
	}

	for _, m := range messages {
		_, err = database.Conn.Exec(`
			INSERT INTO messages (id, conversation_id, role, content, status, created_at) 
			VALUES (?, ?, ?, ?, ?, ?)`, m.id, convID, m.role, m.content, "sent", m.created)
		if err != nil {
			t.Fatalf("Failed to seed message %s: %v", m.id, err)
		}
	}

	// 7. Initialize Context Engine & Assemble
	engine := NewContextEngine(database)
	req := &AssembleRequest{
		WorkspaceID:    wsID,
		ProjectID:      projID,
		ConversationID: convID,
		TokenBudget:    1000, // Small token budget to trigger some allocations
	}

	pkg, err := engine.Assemble(req)
	if err != nil {
		t.Fatalf("Assemble returned error: %v", err)
	}

	// 8. Assertions
	if pkg == nil {
		t.Fatal("Assemble returned nil package")
	}

	// Verify decisions were injected
	if !strings.Contains(pkg.SystemPrompt, "SQLite DB format preferred.") {
		t.Errorf("SystemPrompt missing memory context: %s", pkg.SystemPrompt)
	}

	// Verify tasks were injected
	if !strings.Contains(pkg.SystemPrompt, "Write Engine Tests") {
		t.Errorf("SystemPrompt missing task context: %s", pkg.SystemPrompt)
	}

	// Verify file content was read and injected
	if !strings.Contains(pkg.SystemPrompt, "const answer = 42;") {
		t.Errorf("SystemPrompt missing file content: %s", pkg.SystemPrompt)
	}

	// Verify conversation history is populated and chronologically ordered
	if len(pkg.History) != 3 {
		t.Errorf("Expected 3 messages in history, got %d", len(pkg.History))
	} else {
		if pkg.History[0].Content != "Hello there" {
			t.Errorf("History not ordered chronologically: first item is %s", pkg.History[0].Content)
		}
	}

	// 9. Test Pruning under tight budget
	req.TokenBudget = 80 // Extremely tight budget (320 characters total)
	prunedPkg, err := engine.Assemble(req)
	if err != nil {
		t.Fatalf("Assemble under tight budget failed: %v", err)
	}

	if prunedPkg.TotalTokens > req.TokenBudget {
		t.Errorf("Pruning failed: total tokens (%d) exceeds budget (%d)", prunedPkg.TotalTokens, req.TokenBudget)
	}
}

// Stub function to mock DB interface
func createStubDB(t *testing.T) *db.DB {
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize stub db: %v", err)
	}
	return database
}
