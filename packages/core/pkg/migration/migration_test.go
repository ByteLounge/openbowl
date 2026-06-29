package migration

import (
	"bytes"
	"strings"
	"testing"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
)

func TestImportChatGPTBackup(t *testing.T) {
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Seed workspace and project
	wsID := "ws-mig-1"
	projID := "proj-mig-1"
	_, _ = database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Workspace")
	_, _ = database.Conn.Exec(`INSERT INTO projects (id, workspace_id, name) VALUES (?, ?, ?)`, projID, wsID, "Project")

	// Simulated ChatGPT conversations.json output
	chatGPTPayload := `[
		{
			"title": "Migrate Test Chat",
			"mapping": {
				"node-1": {
					"id": "node-1",
					"message": {
						"author": { "role": "user" },
						"content": { "parts": ["Implement SQLite Outbox."] },
						"create_time": 1719600000
					}
				},
				"node-2": {
					"id": "node-2",
					"message": {
						"author": { "role": "assistant" },
						"content": { "parts": ["SQLite transaction blocks created."] },
						"create_time": 1719600060
					}
				}
			}
		}
	]`

	mm := NewMigrationManager(database)
	count, err := mm.ImportChatGPTBackup(projID, strings.NewReader(chatGPTPayload))
	if err != nil {
		t.Fatalf("ChatGPT Import failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 imported chat, got %d", count)
	}

	// Verify imported messages are in SQLite
	var importedCount int
	err = database.Conn.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&importedCount)
	if err != nil {
		t.Fatalf("Failed to query messages count: %v", err)
	}

	if importedCount != 2 {
		t.Errorf("Expected 2 imported message nodes, got %d", importedCount)
	}
}

func TestExportWorkspaceState(t *testing.T) {
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	wsID := "ws-mig-2"
	_, _ = database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Workspace 2")
	_, _ = database.Conn.Exec(`INSERT INTO projects (id, workspace_id, name) VALUES (?, ?, ?)`, "proj-2", wsID, "Project 2")

	mm := NewMigrationManager(database)
	var output bytes.Buffer

	err = mm.ExportWorkspaceState(wsID, &output)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	rawJSON := output.String()
	if !strings.Contains(rawJSON, "ws-mig-2") || !strings.Contains(rawJSON, "Project 2") {
		t.Errorf("Expected exported payload to contain workspace details: %s", rawJSON)
	}
}
