package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
	"github.com/openbowl/openbowl/packages/core/pkg/memory"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required,omitempty"`
}

type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CallToolResult struct {
	Content []ToolContent `json:"content"`
}

type MCPServer struct {
	DB     *db.DB
	Writer io.Writer
}

func NewMCPServer(database *db.DB, writer io.Writer) *MCPServer {
	if writer == nil {
		writer = os.Stdout
	}
	return &MCPServer{
		DB:     database,
		Writer: writer,
	}
}

// Start listens to Stdin and processes JSON-RPC commands line-by-line
func (s *MCPServer) Start(reader io.Reader) {
	if reader == nil {
		reader = os.Stdin
	}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		s.handleRequest(&req)
	}
}

func (s *MCPServer) handleRequest(req *JSONRPCRequest) {
	switch req.Method {
	case "tools/list":
		s.handleListTools(req.ID)
	case "tools/call":
		s.handleCallTool(req.ID, req.Params)
	default:
		s.sendError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (s *MCPServer) handleListTools(id interface{}) {
	tools := []Tool{
		{
			Name:        "list_workspace_memories",
			Description: "Retrieve active architectural decisions and user preferences in the workspace.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"workspace_id": map[string]interface{}{
						"type":        "string",
						"description": "The workspace UUID identifier.",
					},
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Optional keyword query filter.",
					},
				},
				Required: []string{"workspace_id"},
			},
		},
	}

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
	s.sendResponse(response)
}

func (s *MCPServer) handleCallTool(id interface{}, params json.RawMessage) {
	var callParams CallToolParams
	if err := json.Unmarshal(params, &callParams); err != nil {
		s.sendError(id, -32602, "Invalid params")
		return
	}

	switch callParams.Name {
	case "list_workspace_memories":
		s.executeListWorkspaceMemories(id, callParams.Arguments)
	default:
		s.sendError(id, -32602, fmt.Sprintf("Unknown tool: %s", callParams.Name))
	}
}

func (s *MCPServer) executeListWorkspaceMemories(id interface{}, args json.RawMessage) {
	var params struct {
		WorkspaceID string `json:"workspace_id"`
		Query       string `json:"query"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		s.sendError(id, -32602, "Invalid arguments format")
		return
	}

	// Fetch memories using MemoryEngine search routing
	me := memory.NewMemoryEngine(s.DB, nil)
	memList, err := me.Search(params.WorkspaceID, params.Query, "")
	if err != nil {
		s.sendError(id, 500, fmt.Sprintf("Search error: %v", err))
		return
	}

	var sb strings.Builder
	if len(memList) == 0 {
		sb.WriteString("No active memories or decisions found in this workspace.")
	} else {
		sb.WriteString("Active Workspace Memories & Decisions:\n")
		for _, m := range memList {
			sb.WriteString(fmt.Sprintf("- [%s]: %s\n", m.Category, m.Content))
		}
	}

	result := CallToolResult{
		Content: []ToolContent{
			{
				Type: "text",
				Text: sb.String(),
			},
		},
	}

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.sendResponse(response)
}

func (s *MCPServer) sendResponse(resp JSONRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("MCP Failed to marshal JSON-RPC response: %v", err)
		return
	}
	s.Writer.Write(append(data, '\n'))
}

func (s *MCPServer) sendError(id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
		},
	}
	s.sendResponse(resp)
}
