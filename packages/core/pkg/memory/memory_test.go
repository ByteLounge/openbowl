package memory

import (
	"testing"
	"time"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
	"github.com/openbowl/openbowl/packages/core/pkg/models"
)

func TestMemoryRegexExtraction(t *testing.T) {
	// 1. Setup temporary in-memory database
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory DB: %v", err)
	}
	defer database.Close()

	// Seed workspace and project dependencies
	wsID := "ws-test-999"
	projID := "proj-test-999"
	convID := "conv-test-999"

	_, _ = database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Test WS")
	_, _ = database.Conn.Exec(`INSERT INTO projects (id, workspace_id, name) VALUES (?, ?, ?)`, projID, wsID, "Test Proj")
	_, _ = database.Conn.Exec(`INSERT INTO conversations (id, project_id, title) VALUES (?, ?, ?)`, convID, projID, "Test Conv")

	engine := NewMemoryEngine(database, nil) // Nil config to force regex mode

	// Test message containing decision and preference patterns
	msg := &models.Message{
		ID:             "msg-1",
		ConversationID: convID,
		Role:           "user",
		Content:        "We decided to use SQLite for memory tables. I prefer functional components. TODO: Write memory tests.",
	}

	memories := engine.extractWithRegex(msg)

	// Verify extractions
	if len(memories) != 3 {
		t.Fatalf("Expected 3 extracted memories, got %d", len(memories))
	}

	// Verify categories
	categories := make(map[string]bool)
	for _, m := range memories {
		categories[m.Category] = true
	}

	if !categories["decision"] {
		t.Error("Expected to extract a decision memory")
	}
	if !categories["preference"] {
		t.Error("Expected to extract a preference memory")
	}
	if !categories["todo"] {
		t.Error("Expected to extract a todo memory")
	}
}

func TestMemoryQueueAndSearch(t *testing.T) {
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to create memory DB: %v", err)
	}
	defer database.Close()

	wsID := "ws-test-777"
	projID := "proj-test-777"
	convID := "conv-test-777"

	_, _ = database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Test WS")
	_, _ = database.Conn.Exec(`INSERT INTO projects (id, workspace_id, name) VALUES (?, ?, ?)`, projID, wsID, "Test Proj")
	_, _ = database.Conn.Exec(`INSERT INTO conversations (id, project_id, title) VALUES (?, ?, ?)`, convID, projID, "Test Conv")

	engine := NewMemoryEngine(database, nil)
	engine.Start(1) // Start one background worker
	defer engine.Stop()

	// Push message to the queue
	msg := &models.Message{
		ID:             "msg-async-1",
		ConversationID: convID,
		Role:           "user",
		Content:        "We decided to implement the factory registry pattern.",
	}

	// Seed the message record to satisfy source_message_id foreign keys!
	_, err = database.Conn.Exec(`
		INSERT INTO messages (id, conversation_id, role, content, status) 
		VALUES (?, ?, ?, ?, ?)`, msg.ID, msg.ConversationID, msg.Role, msg.Content, "sent")
	if err != nil {
		t.Fatalf("Failed to seed message record: %v", err)
	}

	engine.QueueMessage(msg)

	// Wait briefly for background processing to persist the record
	time.Sleep(150 * time.Millisecond)

	// Test Search
	results, err := engine.Search(wsID, "factory", "decision")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 search result, got %d", len(results))
	}

	if results[0].Content != "use implement the factory registry pattern" && results[0].Content != "implement the factory registry pattern" {
		t.Errorf("Unexpected memory content: %s", results[0].Content)
	}
}
