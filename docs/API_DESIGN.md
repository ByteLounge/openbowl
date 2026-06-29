# API Design & Protocol Specification - OpenBowl

OpenBowl components communicate using three primary channels:
1. **REST APIs (HTTP/JSON)**: Used for CRUD operations on workspaces, projects, tasks, and settings. Managed by Go (Gin).
2. **WebSocket Interface**: Used for streaming completions, real-time sync events, and terminal-like execution updates.
3. **gRPC Services**: Used for high-speed inter-process communication (CLI to Core Engine, background workers to Provider SDK).
4. **Model Context Protocol (MCP)**: JSON-RPC over stdio or SSE/HTTP for external agent integrations.

---

## 1. REST API Contract (Local Go Sidecar & Sync Service)

Base URL: `http://localhost:3010/api/v1`

### Workspaces
* `GET /workspaces` - Retrieve all workspaces.
* `POST /workspaces` - Create a new workspace.
* `GET /workspaces/:id/sync/status` - Get sync status and sequence number.

### Projects & Files
* `GET /projects?workspace_id=:id` - Retrieve projects in a workspace.
* `POST /projects` - Create a project.
* `POST /projects/:id/files` - Register/update file reference pointers (paths, hashes).

### Tasks
* `GET /projects/:id/tasks` - Retrieve tasks.
* `POST /projects/:id/tasks` - Create a task.
* `PATCH /tasks/:id` - Update status, title, description, or due date.

### Memories
* `GET /workspaces/:id/memories` - Retrieve extracted memories. Filterable by category, tags, or search query.
* `POST /workspaces/:id/memories` - Manually record a memory.
* `PATCH /memories/:id` - Edit memory content or toggle active status.

---

## 2. WebSocket Protocol (`/ws/chat`)

All chat sessions and LLM generations use a WebSocket channel for bi-directional event streaming.

### Client-to-Server Messages

#### Start Generation (`chat.start`)
```json
{
  "event": "chat.start",
  "payload": {
    "conversation_id": "conv-123",
    "provider_id": "prov-456",
    "model_name": "claude-3-5-sonnet",
    "prompt": "Optimize this binary search tree insertion function...",
    "options": {
      "temperature": 0.2,
      "max_tokens": 4096
    }
  }
}
```

#### Cancel Generation (`chat.cancel`)
```json
{
  "event": "chat.cancel",
  "payload": {
    "conversation_id": "conv-123"
  }
}
```

---

### Server-to-Client Messages

#### Stream Chunk (`chat.chunk`)
```json
{
  "event": "chat.chunk",
  "payload": {
    "conversation_id": "conv-123",
    "text": " ",
    "usage": null
  }
}
```

#### Message Complete (`chat.complete`)
```json
{
  "event": "chat.complete",
  "payload": {
    "conversation_id": "conv-123",
    "message_id": "msg-890",
    "tokens_prompt": 1420,
    "tokens_completion": 310,
    "cost": 0.00512
  }
}
```

#### Memory Extracted Notification (`memory.extracted`)
Sent to UI when the background parser identifies a new decision or fact.
```json
{
  "event": "memory.extracted",
  "payload": {
    "workspace_id": "ws-abc",
    "memory": {
      "id": "mem-555",
      "category": "decision",
      "content": "Chose SHA-256 hashes for tracking local project files."
    }
  }
}
```

---

## 3. gRPC Services (Core SDK Internals)

```protobuf
syntax = "proto3";

package openbowl.core.v1;

option go_package = "packages/core/pb;pb";

service ProviderService {
  rpc Completion(CompletionRequest) returns (CompletionResponse);
  rpc CompletionStream(CompletionRequest) returns (stream CompletionChunk);
  rpc ValidateCredentials(ValidateCredentialsRequest) returns (ValidateCredentialsResponse);
}

service ContextService {
  rpc AssembleContext(AssembleContextRequest) returns (AssembleContextResponse);
  rpc OptimizeContext(OptimizeContextRequest) returns (OptimizeContextResponse);
}

message Message {
  string id = 1;
  string role = 2;
  string content = 3;
}

message CompletionRequest {
  string provider_type = 1;
  string model_name = 2;
  repeated Message messages = 3;
  float temperature = 4;
  int32 max_tokens = 5;
}

message CompletionResponse {
  string message_id = 1;
  string content = 2;
  int32 tokens_prompt = 3;
  int32 tokens_completion = 4;
}

message CompletionChunk {
  string content_delta = 1;
  int32 tokens_prompt = 2;
  int32 tokens_completion = 3;
}

message ValidateCredentialsRequest {
  string provider_type = 1;
  string api_key = 2;
  string api_url = 3;
}

message ValidateCredentialsResponse {
  bool success = 1;
  string error_message = 2;
}

message AssembleContextRequest {
  string project_id = 1;
  string conversation_id = 2;
  int32 token_budget = 3;
}

message AssembleContextResponse {
  string system_prompt = 1;
  repeated Message context_messages = 2;
  int32 total_estimated_tokens = 3;
}

message OptimizeContextRequest {
  string raw_context = 1;
  int32 target_budget = 2;
}

message OptimizeContextResponse {
  string optimized_context = 1;
  int32 tokens_saved = 2;
}
```

---

## 4. Model Context Protocol (MCP) Interface

OpenBowl implements the standard **Model Context Protocol** JSON-RPC 2.0 schema over stdio.

### 4.1 Exposed Tools (OpenBowl as MCP Server)
* `list_workspace_memories`: Retrieve active decisions/facts.
* `get_active_tasks`: Get project tasks and state.
* `get_context_summary`: Produce a serialized context string for a given project.

### 4.2 Embedded Clients (OpenBowl as MCP Client)
OpenBowl connects to configured external MCP servers. The frontend displays active MCP connections in a status bar, letting the user toggle permission levels for external tools.
