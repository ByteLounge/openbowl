# Product Requirements Document (PRD) - OpenBowl

## 1. Introduction & Vision

**OpenBowl** is the **universal context layer for AI**. It is designed to solve the problem of vendor lock-in, fragmented contexts, and conversation loss across AI chat clients, IDE extensions, and model providers.

In OpenBowl, the **application owns the memory**, and the **providers only generate responses**.

When a user switches between ChatGPT, Claude, Gemini, Grok, DeepSeek, or local LLMs (Ollama, LM Studio), OpenBowl automatically reconstructs the active context, extracts memories, formats the project constraints, and passes a unified context package to the active provider. The user experiences complete continuity without manual copy-pasting.

### Core Philosophy

- **Context Ownership**: The workspace belongs to the user, not the provider.
- **Provider Agnosticism**: Switching providers is a single click and has zero impact on session history, tasks, or UI.
- **Compact, Rich Context**: Instead of flooding model windows with raw chat history, OpenBowl extracts and injects structured context packages.
- **Offline-First & Developer-Grade**: Designed with local-first databases (SQLite), fast startup times, rich keyboard shortcuts, and developer-centric aesthetics.

---

## 2. Target Audience & Personas

1. **The Polyglot Developer**: Uses Claude 3.5 Sonnet for architectural design, Gemini 1.5 Pro for deep reasoning and code reviews, Groq/DeepSeek for rapid iterations, and local Llama3 via Ollama for offline work or private data.
2. **The Context Collector**: Manages multiple concurrent projects, tasks, and libraries, and is frustrated by having to re-explain the codebase structure to AI models in every new session.
3. **The Local-First Advocate**: Demands that their search, memories, code snippets, and workspace metadata remain local, encrypted, and fast.

---

## 3. Epic & Feature Breakdown

### EP-01: Universal Provider SDK & Layer

- **Requirement**: Abstract all API differences (parameters, streaming protocols, message structures) into a unified interface.
- **Supported Out-of-the-Box**:
  - OpenAI, Anthropic, Gemini, Groq, OpenRouter, DeepSeek, Ollama, LM Studio.
- **Streaming Protocol**: Unified WebSocket/Server-Sent Events (SSE) adapter for streaming text, function calls, and usage statistics.
- **Rate-Limit & Cost Tracking**: Track tokens consumed (prompt/completion/cached) and estimated cost per provider, workspace, and conversation.

### EP-02: Context Engine

- **Requirement**: Construct a token-efficient context package injected before each completion request.
- **Context Assembly**:
  - **Workspace Goals & Current Objective**: High-level targets.
  - **Active Task & Completed Tasks**: Current focus.
  - **Important Decisions**: Extracted architectural selections.
  - **Code Context**: Relevant file references, code snippets, and directory maps.
  - **Timeline**: Sequence of latest actions.
- **Context Optimization**: Algorithms (e.g., token-count-based pruning, vector-search relevance, and summarization) to shrink prompt sizes while maintaining reasoning continuity.

### EP-03: Memory Engine

- **Requirement**: Automatically parse conversation lines to isolate structured knowledge.
- **Extraction Pipelines**: Asynchronous worker pipeline that extracts:
  - _Todos & Deadlines_: Unresolved tasks.
  - _Decisions_: "We chose SQLite over PostgreSQL for local sync."
  - _User Preferences_: "Prefer functional style React components."
  - _Facts & References_: API endpoints, library versions, documentation URLs.
- **Storage & Search**: Searchable memory database. Users can edit, delete, or flag memory items to control what gets injected.

### EP-04: Workspace Management

- **Requirement**: A desktop-native container for organizing developer work.
- **Entities**:
  - **Project**: A collection of conversations, documents, and settings.
  - **File Reference System**: Links to local workspace directories (monitored via file-watcher).
  - **Task Manager**: Kanban/List view of tasks tied directly to active objectives.
  - **Timeline/History**: Unified log of user commands, task completions, and provider switches.

### EP-05: Offline-First Synchronization

- **Requirement**: Local data must be secure, fast, and syncable across devices without conflicts.
- **Engine**: SQLite as primary database in the desktop client.
- **Operation Logging**: Every modification (e.g., creating a task, updating memory) is captured as an ordered operation.
- **Sync Protocol**: Push/pull delta operations to a central OpenBowl sync service, designed to support CRDT (Conflict-Free Replicated Data Types) merge strategies.

### EP-06: Hybrid Search

- **Requirement**: Instant keyboard-driven lookup of past answers, decisions, and files.
- **Full-Text Search (FTS)**: SQLite FTS5 for exact word matches and lexical search.
- **Semantic Search**: Vector embeddings (local via ONNX/transformers or via API) stored in pgvector (cloud) / SQLite extension `sqlite-vss` or simple vector index (local).
- **Metadata Filtering**: Filter by provider, tag, project, and time range.

### EP-07: Plugin & MCP SDK

- **Requirement**: Extensible core architecture.
- **Plugin Capabilities**:
  - Inject new model providers.
  - Register slash commands (`/explain`, `/deploy`).
  - Add custom context enrichers (e.g., git branch info, test coverage).
- **Model Context Protocol (MCP)**:
  - **MCP Client**: OpenBowl connects to external MCP servers (e.g., database inspect, filesystem, GitHub).
  - **MCP Server**: OpenBowl exposes workspace context (tasks, memories, documents) to external IDEs (Cursor, VS Code) or CLI tools.

### EP-08: Import & Export

- **Requirement**: Freedom of data migration.
- **Formats**:
  - Import from ChatGPT, Claude, and Gemini backup JSON files.
  - Export workspaces to markdown, JSON, or custom `.obowl` format (a zipped bundle containing workspace state, SQLite DB file, and file attachments).

---

## 4. Non-Functional & Quality Requirements

- **Performance**: Startup time `< 300ms`. Chat UI must maintain 60 FPS under long scroll virtualized lists. Typing latency `< 16ms`.
- **Security**: API keys stored in OS-level secure enclaves (macOS Keychain, Windows Credential Manager, Linux Secret Service) using Tauri's secure storage bindings.
- **UX/Aesthetics**: High-fidelity dark mode, custom Inter/Outfit font, glassmorphism panel styles, and keyboard-driven commands (Command Palette).
- **Testing Coverage**:
  - Unit testing: `> 80%` on core Packages.
  - E2E: Coverage for critical flows (provider switching, workspace creation, offline sync mock).

---

## 5. Success Metrics

1. **Context Compression Ratio**: Average prompt token savings of `30% - 50%` compared to raw conversation replays, without drop in output accuracy.
2. **Key Switching Latency**: Time to switch provider and send first token is identical to direct API calls (overhead `< 50ms`).
3. **Local Sync Sync-conflict Resolution Rate**: `100%` automated resolution for operation-based logs.
4. **User Engagement**: Core operations (search, provider switch) performed keyboard-only.
