package mcp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
)

func TestMCPListTools(t *testing.T) {
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	var output bytes.Buffer
	server := NewMCPServer(database, &output)

	// Simulate tools/list JSON-RPC request
	input := `{"jsonrpc":"2.0","method":"tools/list","id":42}` + "\n"
	server.Start(strings.NewReader(input))

	var resp JSONRPCResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v. Output raw: %s", err, output.String())
	}

	if resp.ID.(float64) != 42 {
		t.Errorf("Expected response ID 42, got %v", resp.ID)
	}

	resultMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Result is not a map")
	}

	toolsList, ok := resultMap["tools"].([]interface{})
	if !ok || len(toolsList) == 0 {
		t.Fatal("Tools list missing or empty")
	}

	if len(toolsList) != 3 {
		t.Errorf("Expected 3 tools registered, got %d", len(toolsList))
	}
}

func TestMCPCallListTasks(t *testing.T) {
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Seed workspace and project
	wsID := "w-mcp-tasks"
	projID := "proj-mcp-tasks"
	_, _ = database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Workspace")
	_, _ = database.Conn.Exec(`INSERT INTO projects (id, workspace_id, name) VALUES (?, ?, ?)`, projID, wsID, "Project")

	// Seed task
	_, _ = database.Conn.Exec(`
		INSERT INTO tasks (id, project_id, title, status) 
		VALUES (?, ?, ?, ?)`, "t-mcp-1", projID, "Register MCP Server Schema", "in_progress")

	var output bytes.Buffer
	server := NewMCPServer(database, &output)

	// Call list_workspace_tasks JSON-RPC method
	input := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"list_workspace_tasks","arguments":{"project_id":"proj-mcp-tasks"}},"id":101}` + "\n"
	server.Start(strings.NewReader(input))

	var resp JSONRPCResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	resultMap, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Response Result is not a map")
	}

	contentList, ok := resultMap["content"].([]interface{})
	if !ok || len(contentList) == 0 {
		t.Fatal("Response Content is missing or empty")
	}

	content0 := contentList[0].(map[string]interface{})
	textVal := content0["text"].(string)

	if !strings.Contains(textVal, "Register MCP Server Schema") || !strings.Contains(textVal, "in_progress") {
		t.Errorf("Expected task details in returned text, got: %s", textVal)
	}
}

func TestMCPCallListFiles(t *testing.T) {
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	wsID := "w-mcp-files"
	projID := "proj-mcp-files"
	_, _ = database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Workspace")
	_, _ = database.Conn.Exec(`INSERT INTO projects (id, workspace_id, name) VALUES (?, ?, ?)`, projID, wsID, "Project")

	// Seed file reference
	_, _ = database.Conn.Exec(`
		INSERT INTO file_references (id, project_id, relative_path, file_hash) 
		VALUES (?, ?, ?, ?)`, "f-mcp-1", projID, "src/index.ts", "hash-abc-123")

	var output bytes.Buffer
	server := NewMCPServer(database, &output)

	// Call list_workspace_files
	input := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"list_workspace_files","arguments":{"project_id":"proj-mcp-files"}},"id":102}` + "\n"
	server.Start(strings.NewReader(input))

	var resp JSONRPCResponse
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	resultMap := resp.Result.(map[string]interface{})
	contentList := resultMap["content"].([]interface{})
	content0 := contentList[0].(map[string]interface{})
	textVal := content0["text"].(string)

	if !strings.Contains(textVal, "src/index.ts") || !strings.Contains(textVal, "hash-abc") {
		t.Errorf("Expected file path and hashed details, got: %s", textVal)
	}
}
