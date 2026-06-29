# Project Roadmap & Milestones - OpenBowl

This roadmap outlines the incremental path to building OpenBowl into a production-grade, open-source universal context layer. The engineering process is structured into sequential, feedback-driven phases to minimize architectural drift.

---

## 1. Project Phases & Milestones

```text
[Phase 1: Specs]  ──>  [Phase 2: Scaffolding]  ──>  [Phase 3: Provider SDK]
       │                        │                            │
       ▼                        ▼                            ▼
  Complete (Current)       Repo & Tooling              Unified API Layers
       
[Phase 4: Context] ──>  [Phase 5: Memory]      ──>  [Phase 6: Sync/Offline]
       │                        │                            │
       ▼                        ▼                            ▼
  Inference Tokens         Auto-Knowledge              Operation Streams

[Phase 7: MCP/Plugins] ──> [Phase 8: Polishing]
       │                        │
       ▼                        ▼
  Extensible APIs          E2E Tests & Launch
```

---

## 2. Milestone Details

### Phase 1: Product & Architecture Specification (Current)
* [x] **Product Requirements Document (PRD)**
* [x] **Software Architecture Document (SAD)**
* [x] **Data Model Design**
* [x] **API & Service Contracts**
* [x] **Component Hierarchy & UI Wireframe Matrix**
* [x] **Development Roadmap & Milestones**

### Phase 2: Monorepo Scaffolding & Core Tooling
* **Objective**: Establish the development workspace and cross-process toolchains.
* **Deliverables**:
  * Set up `pnpm` workspaces for JS packages (apps/desktop, apps/web, packages/ui, packages/types).
  * Initialize the Tauri project workspace with a Go sidecar executor framework.
  * Initialize the Go sidecar directory with a Gin web server, SQLite migrations runner, and protobuf code generation pipeline.
  * Set up global ESLint, Prettier, Tailwind Config, and TypeScript configurations.
  * Configure Vitest for front-end unit testing and Go test suites.

### Phase 3: Universal Provider SDK
* **Objective**: Standardize interface logic for LLM completions.
* **Deliverables**:
  * Implement the Go `Provider` interface and request adapters for OpenAI, Anthropic, Gemini, DeepSeek, and Ollama.
  * Implement streaming payload handlers and standard token usage reporting.
  * Develop a `MockProvider` to enable front-end development without triggering paid API limits.
  * Write unit tests for request/response serialization across all provider formats.

### Phase 4: Context Engine
* **Objective**: Build the intelligence layer for compiling workspace state into dense prompts.
* **Deliverables**:
  * Implement the workspace state reader (reads active task, project goals, timeline).
  * Build the token-budget calculator (allocates budget to instructions, conversation history, local files, and memories).
  * Implement the directory scanner and file reference module.
  * Create context assembly unit tests.

### Phase 5: Memory Engine
* **Objective**: Extract and search project context automatically.
* **Deliverables**:
  * Build the Go asynchronous queue system for background processing.
  * Implement memory extraction parsers (using prompts or regex rules).
  * Integrate SQLite FTS5 for exact search and implement a local vector-index module for semantic queries.
  * Develop the front-end Memory Management Canvas.

### Phase 6: Sync & Offline-First Data Layer
* **Objective**: Ensure operational integrity across intermittent connection states.
* **Deliverables**:
  * Build SQLite operation logging triggers (tracking inserts/updates/deletes on tasks, projects, memories).
  * Implement the local operations outbox and synchronization retry loops.
  * Create a mock synchronization cloud server to demonstrate delta exchange and reconciliation.

### Phase 7: MCP Integrations, Plugins, & Search
* **Objective**: Extend workspace capabilities to external agents.
* **Deliverables**:
  * Implement the Model Context Protocol (MCP) JSON-RPC stdio handler in Go.
  * Build the plugin loader (compiling Javascript plugins or loading dynamically registered configs).
  * Deliver the global hybrid search dashboard (Command Palette).
  * Implement workspace migration utilities (JSON/markdown imports, `.obowl` ZIP package exporter).

### Phase 8: Final Polishing & E2E Testing
* **Objective**: Stabilize UI response rates, resolve critical bugs, and audit production packaging.
* **Deliverables**:
  * Complete E2E testing flows using Playwright.
  * Run performance benchmarks (Tauri startup speeds, virtualized list scroll frames, database search latencies).
  * Compile native build configurations for Windows, macOS, and Linux.

---

## 3. Risk Matrix & Mitigations

| Risk | Impact | Likelihood | Mitigation Strategy |
| :--- | :--- | :--- | :--- |
| **Go Sidecar Tauri Integration overhead** | Medium | Medium | Implement lightweight gRPC or WebSockets over localhost; monitor socket connection lifecycles closely and add automatic sidecar restart logic in Tauri if the process fails. |
| **Local Vector Search compilation problems** | High | Low | If `sqlite-vss` compilation fails on some Windows/macOS installations, fallback to a clean Go-based in-memory vector index (written in pure Go) to avoid platform-compilation issues. |
| **Token Budget Overflows** | High | Medium | Enforce strict maximum limits on file-reference insertion sizes, truncate code snippets dynamically, and prioritize user system instructions above all else. |
| **Sync Conflict Spikes** | Medium | Low | Limit sync scopes to metadata updates (tasks, projects, decisions) and avoid real-time syncing of massive raw message streams unless explicitly toggled by the user. |
