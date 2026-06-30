# Component Hierarchy & UI Structure - OpenBowl

OpenBowl's desktop interface is built using **React + Vite** wrapped inside a **Tauri** desktop window. The design follows Linear and Raycast principles: high information density, dark-mode default, glassmorphism, responsive split panels, and strict keyboard-first usability.

---

## 1. Application Layout Matrix

```text
+-----------------------------------------------------------------------------+
| Sidebar Panel            | Top Bar (Command Search, Status, Provider)       |
| [Workspace Switcher]     +--------------------------------------------------+
|                          | Workspace Canvas (Active Project Area)           |
| > Projects               |                                                  |
|   - OpenBowl Core        |                                                  |
|   - Docs                 |                                                  |
|                          |                                                  |
| > Tasks                  |                                                  |
|   - [ ] Build SDK        |                                                  |
|   - [x] Write PRD        |                                                  |
|                          |                                                  |
| > Active Conversations   |                                                  |
|   - Context Strategy     |                                                  |
|   - DB Optimization      |                                                  |
|                          |                                                  |
| > Memory Panel           |                                                  |
|                          |                                                  |
| > Settings               |                                                  |
+--------------------------+--------------------------------------------------+
```

---

## 2. Component Tree Hierarchy

```text
App
├── Providers (ThemeProvider, QueryClientProvider, ToastProvider)
└── MainLayout
    ├── CommandPalette (Raycast-style search & action executor)
    ├── Sidebar
    │   ├── WorkspaceSelector (Dropdown switcher)
    │   ├── ProjectList (Project links + inline action menu)
    │   ├── TaskSummaryList (Quick view of active tasks)
    │   ├── ConversationHistoryList (Grouped by date, search filter)
    │   └── SidebarFooter (User info, sync status, settings toggle)
    ├── TopBar
    │   ├── Breadcrumbs (Workspace > Project > Conversation)
    │   ├── ModelSelector (Provider + Model dropdown switcher)
    │   ├── ActiveContextIndicator (Indicator showing active memories and files count)
    │   └── SyncIndicator (Offline, Connected, Syncing states)
    └── WorkspaceCanvas (Dynamic Route Area)
        ├── ConversationCanvas (Active chat session)
        │   ├── ContextEngineHeader (Visual map of injected context tokens)
        │   ├── VirtualizedMessageList (Fast scroll message components)
        │   │   └── MessageItem (Role-based rendering, action buttons, syntax highlighter)
        │   │       ├── CodeBlock (Syntax highlighting, copy, expand/collapse)
        │   │       └── MemoryMarker (Highlights text segments converted to memories)
        │   └── ChatInputArea (Rich text area with autocomplete)
        │       ├── AttachmentBar (File pills, token calculation metrics)
        │       ├── SlashCommandsMenu (Inline popover for commands)
        │       └── SendButton (Controls stream trigger, stop generation)
        ├── TaskCanvas (Kanban & List views)
        │   ├── TaskBoardColumn (Todo, In Progress, Completed)
        │   │   └── TaskCard (Interactive tasks with due dates, tags, associated memories)
        │   └── TaskCreationModal (Inline quick creation)
        ├── MemoryCanvas (Memory hub)
        │   ├── MemorySearchFilter (Category tags, search inputs)
        │   └── MemoryGrid
        │       └── MemoryCard (Editable content, toggle state, deletion, origin links)
        └── SettingsCanvas
            ├── ProviderConfigPanel (Inputs for API Keys, Base URLs, test connection button)
            ├── PluginManagerPanel (List of loaded extensions, toggle keys)
            └── BackupExportPanel (Export to .obowl, import backups)
```

---

## 3. UI State Management (Zustand Stores)

We separate local UI state (transient, layout-based) from cached DB resources (managed via TanStack Query).

### 3.1 Workspace Store (`useWorkspaceStore`)

- `activeWorkspaceId`: `string`
- `activeProjectId`: `string | null`
- `activeConversationId`: `string | null`
- `isSidebarExpanded`: `boolean`
- `setActiveWorkspace(id: string)`: `void`
- `setActiveProject(id: string | null)`: `void`
- `setActiveConversation(id: string | null)`: `void`

### 3.2 Chat Generation Store (`useChatStore`)

- `generatingConversations`: `Record<string, boolean>` (Tracks which conversations are currently streaming)
- `abortedRequests`: `Record<string, boolean>`
- `startGeneration(convId: string)`: `void`
- `abortGeneration(convId: string)`: `void`

---

## 4. UI Library & Theming Design Tokens

- **Engine**: Tailwind CSS + `shadcn/ui` primitive components.
- **Palette (Dark Mode Preferred)**:
  - `--background`: `240 10% 3.9%` (Deep carbon black)
  - `--card`: `240 10% 6%` (Subtle dark card background)
  - `--primary`: `263.4 70% 50.4%` (Electric violet)
  - `--accent`: `240 3.7% 15.9%` (Muted border/hover highlights)
  - `--font-sans`: `Inter, system-ui, sans-serif`
  - `--font-mono`: `Geist Mono, JetBrains Mono, monospace`
- **Micro-Animations**:
  - Hover transitions: `transition-all duration-200 ease-in-out`
  - Command palette fade-in: `animate-in fade-in-80 zoom-in-95 duration-100`
  - Streaming cursor: `after:content-['▋'] after:animate-pulse after:ml-0.5`
