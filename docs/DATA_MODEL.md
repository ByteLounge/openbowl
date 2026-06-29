# Data Model & Database Schema - OpenBowl

OpenBowl uses an offline-first relational database structure. The primary local data store is **SQLite** (using FTS5 for text search and `sqlite-vss` or a flat vector indexing mechanism for local embeddings). The cloud sync service maps this data into **PostgreSQL** with `pgvector`.

---

## 1. Database Schema

Below is the SQL Schema design representing our local SQLite tables.

```sql
-- Workspaces Table
CREATE TABLE workspaces (
    id TEXT PRIMARY KEY, -- UUID
    name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Projects Table
CREATE TABLE projects (
    id TEXT PRIMARY KEY, -- UUID
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- Providers Settings (credentials stored in OS secure key store, not in DB)
CREATE TABLE providers (
    id TEXT PRIMARY KEY, -- UUID
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,         -- e.g., "Primary OpenAI"
    provider_type TEXT NOT NULL,-- e.g., "openai", "anthropic", "ollama"
    api_url TEXT,               -- Custom endpoint if needed (e.g. LM Studio, Ollama)
    is_active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);

-- Conversations Table
CREATE TABLE conversations (
    id TEXT PRIMARY KEY, -- UUID
    project_id TEXT NOT NULL,
    title TEXT NOT NULL,
    provider_id TEXT,    -- The current default provider
    model_name TEXT,     -- e.g., "claude-3-5-sonnet"
    system_instruction TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON SET NULL
);

-- Messages Table
CREATE TABLE messages (
    id TEXT PRIMARY KEY, -- UUID
    conversation_id TEXT NOT NULL,
    role TEXT NOT NULL,         -- "system", "user", "assistant", "tool"
    content TEXT NOT NULL,
    tokens_prompt INTEGER DEFAULT 0,
    tokens_completion INTEGER DEFAULT 0,
    cost REAL DEFAULT 0.0,
    status TEXT NOT NULL DEFAULT 'sent', -- 'pending', 'sent', 'error'
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

-- Tasks Table
CREATE TABLE tasks (
    id TEXT PRIMARY KEY, -- UUID
    project_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'todo', -- 'todo', 'in_progress', 'completed', 'archived'
    due_date DATETIME,
    completed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- Memories Table
CREATE TABLE memories (
    id TEXT PRIMARY KEY, -- UUID
    workspace_id TEXT NOT NULL,
    category TEXT NOT NULL,    -- 'decision', 'todo', 'preference', 'fact', 'architecture'
    content TEXT NOT NULL,
    source_conversation_id TEXT,
    source_message_id TEXT,
    is_active INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
    FOREIGN KEY (source_conversation_id) REFERENCES conversations(id) ON SET NULL,
    FOREIGN KEY (source_message_id) REFERENCES messages(id) ON SET NULL
);

-- File References Table
CREATE TABLE file_references (
    id TEXT PRIMARY KEY, -- UUID
    project_id TEXT NOT NULL,
    relative_path TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    content_summary TEXT,
    indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- Bookmarks Table
CREATE TABLE bookmarks (
    id TEXT PRIMARY KEY, -- UUID
    project_id TEXT NOT NULL,
    entity_type TEXT NOT NULL, -- 'conversation', 'message', 'task', 'memory'
    entity_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- Sync Operations Outbox (Offline-First Delta tracking)
CREATE TABLE sync_operations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id TEXT NOT NULL,
    entity_type TEXT NOT NULL,  -- 'project', 'conversation', 'task', 'memory'
    entity_id TEXT NOT NULL,
    operation_type TEXT NOT NULL,-- 'INSERT', 'UPDATE', 'DELETE'
    payload TEXT NOT NULL,       -- JSON payload of the mutation
    sequence_number INTEGER NOT NULL,
    applied_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    synced_at DATETIME,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
);
```

---

## 2. Text & Vector Search Schemas

### 2.1 Full-Text Search (FTS5) Table
For rapid search of chat logs and memory content, we shadow our messages and memories using SQLite's FTS5 engine:

```sql
CREATE VIRTUAL TABLE fts_idx USING fts5(
    entity_id, 
    entity_type, 
    content, 
    title,
    tokenize='porter unicode61'
);
```
FTS index updates are triggered automatically via SQL triggers on the `messages` and `memories` tables.

### 2.2 Semantic (Vector) Schema
If `sqlite-vss` is used, the vector store is modeled as:
```sql
CREATE VIRTUAL TABLE vss_memories USING vss0(
    memory_id,
    vector(1536) -- Matches standard embedding dimensions (e.g. OpenAI text-embedding-3-small)
);
```

If vector extensions are unavailable (dependent on platform/compilation), OpenBowl defaults to a local **flat float-array index** stored in `.bin` files per project, loaded in-memory inside the Go backend, and searched using cosine similarity.

---

## 3. Data Merging & Sync Protocol

To achieve conflict-free sync, the cloud backend maintains a central sequence number for each workspace.
1. Each mutation triggers a `sync_operations` entry.
2. The local database assigns a local chronological `sequence_number`.
3. When pushing, the local client sends all unsynced operations ordered by sequence.
4. The server validates, applies the operation to the master database, and issues a globally ordered sequence number.
5. If there's a conflict, the client pulls the server-ordered sequence, rolls back uncommitted local changes, runs LWW (Last-Write-Wins) timestamps on fields, applies remote updates, and reapplies remaining local changes.
