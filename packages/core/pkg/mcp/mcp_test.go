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

	tool0 := toolsList[0].(map[string]interface{})
	if tool0["name"] != "list_workspace_memories" {
		t.Errorf("Unexpected tool name: %s", tool0["name"])
	}
}
