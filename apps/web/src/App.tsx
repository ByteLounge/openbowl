import { useState, useEffect, useRef } from "react";
import {
  Layers,
  FolderGit2,
  Cpu,
  Database,
  Send,
  Zap,
  Globe,
  Plus,
  Sun,
  Moon,
} from "lucide-react";
import {
  approximateTokens,
  formatCost,
} from "../../../packages/shared/src/index";
import {
  Provider,
  Message,
  Task,
  Memory,
} from "../../../packages/types/src/index";

const API_BASE = "http://localhost:3010/api/v1";
const WS_BASE = "ws://localhost:3010/ws/chat";

const initialProviders: Provider[] = [
  {
    id: "p-1",
    workspaceId: "w-dev-default",
    name: "Anthropic Claude",
    providerType: "anthropic",
    isActive: true,
    createdAt: "",
  },
  {
    id: "p-2",
    workspaceId: "w-dev-default",
    name: "Google Gemini",
    providerType: "gemini",
    isActive: true,
    createdAt: "",
  },
  {
    id: "p-3",
    workspaceId: "w-dev-default",
    name: "OpenAI GPT-4",
    providerType: "openai",
    isActive: true,
    createdAt: "",
  },
  {
    id: "p-4",
    workspaceId: "w-dev-default",
    name: "DeepSeek V3",
    providerType: "deepseek",
    isActive: true,
    createdAt: "",
  },
  {
    id: "p-5",
    workspaceId: "w-dev-default",
    name: "Ollama (Local)",
    providerType: "ollama",
    apiUrl: "http://localhost:11434",
    isActive: true,
    createdAt: "",
  },
];

export default function App() {
  const [providers] = useState<Provider[]>(initialProviders);
  const [activeProviderIndex, setActiveProviderIndex] = useState(4); // Default to Ollama Local
  const [inputText, setInputText] = useState("");

  const [theme, setTheme] = useState<"dark" | "light">(
    (localStorage.getItem("theme") as "dark" | "light") || "dark",
  );

  useEffect(() => {
    document.body.classList.remove("dark-theme", "light-theme");
    document.body.classList.add(`${theme}-theme`);
    localStorage.setItem("theme", theme);
  }, [theme]);

  const toggleTheme = () => {
    setTheme((prev) => (prev === "dark" ? "light" : "dark"));
  };

  // Real Local Database states
  const [, setWorkspaces] = useState<any[]>([]);
  const [projects, setProjects] = useState<any[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [memories, setMemories] = useState<Memory[]>([]);

  const [activeWorkspaceId, setActiveWorkspaceId] =
    useState<string>("w-dev-default");
  const [activeProjectId, setActiveProjectId] =
    useState<string>("proj-core-default");

  const [messages, setMessages] = useState<Message[]>([
    {
      id: "msg-1",
      conversationId: "c-1",
      role: "system",
      content: "System: OpenBowl workspace memory initialized.",
      tokensPrompt: 0,
      tokensCompletion: 0,
      cost: 0,
      status: "sent",
      createdAt: "",
    },
  ]);

  const [websocketStatus, setWebsocketStatus] = useState<
    "disconnected" | "connecting" | "connected"
  >("disconnected");
  const [costAccumulated, setCostAccumulated] = useState(0.0);

  const activeProvider = providers[activeProviderIndex];
  const wsRef = useRef<WebSocket | null>(null);

  // Fetch workspaces and projects on mount
  useEffect(() => {
    fetch(`${API_BASE}/workspaces`)
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data) => {
        setWorkspaces(data);
        if (data.length > 0) setActiveWorkspaceId(data[0].id);
      })
      .catch(() => {
        // Fallback stub if sidecar offline
        setWorkspaces([{ id: "w-dev-default", name: "Default Workspace" }]);
      });

    fetch(`${API_BASE}/projects`)
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data) => {
        setProjects(data);
        if (data.length > 0) setActiveProjectId(data[0].id);
      })
      .catch(() => {
        setProjects([
          {
            id: "proj-core-default",
            workspace_id: "w-dev-default",
            name: "OpenBowl Core",
            description: "Universal context layer",
          },
        ]);
      });
  }, []);

  // Fetch tasks and memories when project/workspace switches
  useEffect(() => {
    if (!activeProjectId) return;
    fetch(`${API_BASE}/projects/${activeProjectId}/tasks`)
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data) => setTasks(data))
      .catch(() => {
        // Fallback
        setTasks([
          {
            id: "t-1",
            projectId: activeProjectId,
            title: "Scaffold Tauri desktop workspace",
            status: "completed",
            createdAt: "",
            updatedAt: "",
          },
          {
            id: "t-2",
            projectId: activeProjectId,
            title: "Implement Go Provider SDK",
            status: "completed",
            createdAt: "",
            updatedAt: "",
          },
          {
            id: "t-3",
            projectId: activeProjectId,
            title: "Integrate local SQLite tables",
            status: "in_progress",
            createdAt: "",
            updatedAt: "",
          },
        ]);
      });
  }, [activeProjectId]);

  useEffect(() => {
    if (!activeWorkspaceId) return;
    fetch(`${API_BASE}/workspaces/${activeWorkspaceId}/memories`)
      .then((res) => (res.ok ? res.json() : Promise.reject()))
      .then((data) => setMemories(data))
      .catch(() => {
        setMemories([
          {
            id: "m-1",
            workspaceId: activeWorkspaceId,
            category: "decision",
            content: "Use CGO-free modernc.org SQLite driver for portability.",
            isActive: true,
            createdAt: "",
            updatedAt: "",
          },
          {
            id: "m-2",
            workspaceId: activeWorkspaceId,
            category: "preference",
            content: "Prefer functional React style with custom Vanilla CSS.",
            isActive: true,
            createdAt: "",
            updatedAt: "",
          },
        ]);
      });
  }, [activeWorkspaceId]);

  // Connect to Go sidecar WebSocket
  useEffect(() => {
    setWebsocketStatus("connecting");
    const ws = new WebSocket(WS_BASE);
    wsRef.current = ws;

    ws.onopen = () => {
      setWebsocketStatus("connected");
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.event === "chat.chunk") {
          const delta = msg.payload.text;
          setMessages((prev) =>
            prev.map((m) => {
              if (m.status === "pending") {
                return { ...m, content: m.content + delta };
              }
              return m;
            }),
          );
        } else if (msg.event === "chat.complete") {
          setMessages((prev) =>
            prev.map((m) => {
              if (m.status === "pending") {
                return {
                  ...m,
                  status: "sent",
                  tokensPrompt: msg.payload.tokens_prompt || 0,
                  tokensCompletion: msg.payload.tokens_completion || 0,
                  cost: msg.payload.cost || 0.0,
                };
              }
              return m;
            }),
          );
          if (msg.payload.cost) {
            setCostAccumulated((prev) => prev + msg.payload.cost);
          }
        } else if (msg.event === "chat.error") {
          setMessages((prev) =>
            prev.map((m) => {
              if (m.status === "pending") {
                return {
                  ...m,
                  content: `Error: ${msg.payload.error}`,
                  status: "error",
                };
              }
              return m;
            }),
          );
        }
      } catch (err) {
        console.error("Failed to parse WebSocket message", err);
      }
    };

    ws.onerror = () => {
      setWebsocketStatus("disconnected");
    };

    ws.onclose = () => {
      setWebsocketStatus("disconnected");
    };

    return () => {
      ws.close();
    };
  }, []);

  const handleSwitchProvider = () => {
    setActiveProviderIndex((prev) => (prev + 1) % providers.length);
  };

  const handleToggleTask = (taskID: string, currentStatus: Task["status"]) => {
    const nextStatus: Task["status"] =
      currentStatus === "completed" ? "in_progress" : "completed";

    // Optimistic UI update
    setTasks((prev) =>
      prev.map((t) =>
        t.id === taskID ? ({ ...t, status: nextStatus } as Task) : t,
      ),
    );

    fetch(`${API_BASE}/tasks/${taskID}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ status: nextStatus }),
    }).catch(() => {
      // Revert if API failed
      setTasks((prev) =>
        prev.map((t) =>
          t.id === taskID ? ({ ...t, status: currentStatus } as Task) : t,
        ),
      );
    });
  };

  const handleSendMessage = () => {
    if (!inputText.trim()) return;

    const userMsg: Message = {
      id: `msg-u-${Date.now()}`,
      conversationId: "c-1",
      role: "user",
      content: inputText,
      tokensPrompt: approximateTokens(inputText),
      tokensCompletion: 0,
      cost: 0,
      status: "sent",
      createdAt: new Date().toISOString(),
    };

    setMessages((prev) => [...prev, userMsg]);
    setInputText("");

    const assistantMsgId = `msg-a-${Date.now()}`;
    const pendingMsg: Message = {
      id: assistantMsgId,
      conversationId: "c-1",
      role: "assistant",
      content: "",
      tokensPrompt: 0,
      tokensCompletion: 0,
      cost: 0,
      status: "pending",
      createdAt: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, pendingMsg]);

    // If WebSocket is connected, stream the response from Go sidecar
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({
          event: "chat.start",
          payload: {
            conversation_id: "c-1",
            provider_type: activeProvider.providerType,
            model_name:
              activeProvider.providerType === "gemini"
                ? "gemini-1.5-flash"
                : activeProvider.providerType === "openai"
                  ? "gpt-4o"
                  : "llama3",
            prompt: inputText,
            api_key:
              activeProvider.providerType === "gemini"
                ? "YOUR_GEMINI_KEY"
                : "dev-key",
            api_url: activeProvider.apiUrl || "",
          },
        }),
      );
    } else {
      // Fallback offline mock response timeout
      setTimeout(() => {
        const responseText = `[Offline Mode] Echo from client wrapper using ${activeProvider.name}. Start Go core binary to enable live completion streams.`;
        setMessages((prev) =>
          prev.map((m) => {
            if (m.id === assistantMsgId) {
              return {
                ...m,
                content: responseText,
                status: "sent",
                tokensCompletion: approximateTokens(responseText),
              };
            }
            return m;
          }),
        );
        setCostAccumulated((prev) => prev + 0.00012);
      }, 700);
    }
  };

  return (
    <div className="app-container">
      {/* SIDEBAR */}
      <aside className="sidebar">
        <div
          style={{
            padding: "24px",
            display: "flex",
            alignItems: "center",
            gap: "12px",
            borderBottom: "1px solid var(--border-subtle)",
          }}
        >
          <div
            style={{
              backgroundColor: "var(--accent-purple)",
              padding: "6px",
              borderRadius: "8px",
            }}
          >
            <Layers size={18} color="white" />
          </div>
          <div>
            <h2 style={{ fontSize: "15px", fontWeight: 600 }}>
              OpenBowl Workspace
            </h2>
            <span style={{ fontSize: "11px", color: "var(--text-muted)" }}>
              Universal Memory Layer
            </span>
          </div>
        </div>

        {/* Project Lists */}
        <div style={{ flex: 1, padding: "16px", overflowY: "auto" }}>
          <div style={{ marginBottom: "24px" }}>
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                marginBottom: "8px",
              }}
            >
              <span
                style={{
                  fontSize: "11px",
                  fontWeight: 600,
                  textTransform: "uppercase",
                  letterSpacing: "0.05em",
                  color: "var(--text-muted)",
                }}
              >
                Projects
              </span>
              <Plus
                size={14}
                style={{ cursor: "pointer", color: "var(--text-muted)" }}
              />
            </div>
            <div
              style={{ display: "flex", flexDirection: "column", gap: "4px" }}
            >
              {projects.map((p) => (
                <div
                  key={p.id}
                  onClick={() => setActiveProjectId(p.id)}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "8px",
                    padding: "8px",
                    borderRadius: "6px",
                    backgroundColor:
                      activeProjectId === p.id
                        ? "var(--bg-tertiary)"
                        : "transparent",
                    fontSize: "13px",
                    cursor: "pointer",
                  }}
                >
                  <FolderGit2 size={15} color="var(--accent-purple)" />
                  <span>{p.name}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Tasks Panel */}
          <div style={{ marginBottom: "24px" }}>
            <span
              style={{
                fontSize: "11px",
                fontWeight: 600,
                textTransform: "uppercase",
                color: "var(--text-muted)",
                display: "block",
                marginBottom: "8px",
              }}
            >
              Tasks
            </span>
            <div
              style={{ display: "flex", flexDirection: "column", gap: "6px" }}
            >
              {tasks.map((t) => (
                <div
                  key={t.id}
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "8px",
                    fontSize: "12px",
                  }}
                >
                  <input
                    type="checkbox"
                    checked={t.status === "completed"}
                    onChange={() => handleToggleTask(t.id, t.status)}
                    style={{
                      accentColor: "var(--accent-purple)",
                      cursor: "pointer",
                    }}
                  />
                  <span
                    style={{
                      color:
                        t.status === "completed"
                          ? "var(--text-muted)"
                          : "var(--text-secondary)",
                      textDecoration:
                        t.status === "completed" ? "line-through" : "none",
                    }}
                  >
                    {t.title}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Extracted Memories */}
          <div>
            <span
              style={{
                fontSize: "11px",
                fontWeight: 600,
                textTransform: "uppercase",
                color: "var(--text-muted)",
                display: "block",
                marginBottom: "8px",
              }}
            >
              Extracted Decisions
            </span>
            <div
              style={{ display: "flex", flexDirection: "column", gap: "8px" }}
            >
              {memories.map((m) => (
                <div
                  key={m.id}
                  style={{
                    padding: "8px",
                    borderRadius: "6px",
                    borderLeft: "2px solid var(--accent-purple)",
                    backgroundColor: "rgba(255,255,255,0.02)",
                    fontSize: "11px",
                    color: "var(--text-secondary)",
                  }}
                >
                  {m.content}
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div
          style={{
            padding: "16px",
            borderTop: "1px solid var(--border-subtle)",
            display: "flex",
            flexDirection: "column",
            gap: "8px",
          }}
        >
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              fontSize: "11px",
              color: "var(--text-muted)",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
              <Globe
                size={12}
                color={
                  websocketStatus === "connected"
                    ? "var(--status-success)"
                    : "var(--status-error)"
                }
              />
              <span>Go Sidecar: {websocketStatus}</span>
            </div>
            <span>v0.1.0</span>
          </div>
        </div>
      </aside>

      {/* CANVAS */}
      <main className="workspace-canvas">
        {/* Header Bar */}
        <header
          style={{
            height: "64px",
            borderBottom: "1px solid var(--border-subtle)",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            padding: "0 24px",
            backgroundColor: "var(--bg-secondary)",
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
            <span style={{ fontSize: "14px", fontWeight: 500 }}>
              Active Model:
            </span>
            <button
              onClick={handleSwitchProvider}
              style={{
                display: "flex",
                alignItems: "center",
                gap: "8px",
                padding: "6px 12px",
                border: "1px solid var(--border-subtle)",
                borderRadius: "6px",
                backgroundColor: "var(--bg-tertiary)",
                color: "var(--text-primary)",
                cursor: "pointer",
                fontSize: "13px",
                transition: "all 0.2s",
              }}
            >
              <Cpu size={14} color="var(--accent-purple)" />
              <span>{activeProvider.name}</span>
            </button>
            <span style={{ fontSize: "11px", color: "var(--text-muted)" }}>
              (Click button to switch provider)
            </span>
          </div>

          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "16px",
              fontSize: "13px",
              color: "var(--text-secondary)",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
              <Zap size={14} color="var(--status-warning)" />
              <span>Cost: {formatCost(costAccumulated)}</span>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
              <Database size={14} color="var(--status-success)" />
              <span>Decisions: {memories.length}</span>
            </div>
            <button
              onClick={toggleTheme}
              className="theme-toggle-btn"
              title={`Switch to ${theme === "dark" ? "light" : "dark"} mode`}
              style={{ marginLeft: "4px" }}
            >
              {theme === "dark" ? (
                <Sun
                  size={15}
                  className="theme-toggle-icon sun"
                  color="#f59e0b"
                />
              ) : (
                <Moon
                  size={15}
                  className="theme-toggle-icon moon"
                  color="var(--accent-purple)"
                />
              )}
            </button>
          </div>
        </header>

        {/* Conversation List */}
        <div
          style={{
            flex: 1,
            padding: "24px",
            overflowY: "auto",
            display: "flex",
            flexDirection: "column",
            gap: "16px",
          }}
        >
          {messages.map((msg) => (
            <div
              key={msg.id}
              style={{
                alignSelf: msg.role === "user" ? "flex-end" : "flex-start",
                maxWidth: "70%",
                padding: "12px 16px",
                borderRadius: "12px",
                backgroundColor:
                  msg.role === "user"
                    ? "var(--accent-purple)"
                    : "var(--bg-secondary)",
                border:
                  msg.role === "user"
                    ? "none"
                    : "1px solid var(--border-subtle)",
                color: "var(--text-primary)",
                fontSize: "14px",
                lineHeight: 1.5,
                boxShadow:
                  msg.role === "user"
                    ? "0 4px 12px rgba(139, 92, 246, 0.2)"
                    : "none",
              }}
            >
              <div>{msg.content}</div>
              {msg.tokensPrompt > 0 && (
                <div
                  style={{
                    display: "flex",
                    justifyContent: "flex-end",
                    marginTop: "6px",
                    fontSize: "9px",
                    color:
                      msg.role === "user"
                        ? "rgba(255,255,255,0.7)"
                        : "var(--text-muted)",
                  }}
                >
                  Tokens: {msg.tokensPrompt}
                </div>
              )}
            </div>
          ))}
        </div>

        {/* Input Dock */}
        <div
          style={{
            padding: "24px",
            borderTop: "1px solid var(--border-subtle)",
            backgroundColor: "var(--bg-secondary)",
          }}
        >
          <div style={{ display: "flex", gap: "12px", position: "relative" }}>
            <input
              type="text"
              placeholder="Ask anything... Use /goal to run autonomous agent tasks"
              value={inputText}
              onChange={(e) => setInputText(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSendMessage()}
              style={{
                flex: 1,
                backgroundColor: "var(--bg-tertiary)",
                border: "1px solid var(--border-subtle)",
                borderRadius: "8px",
                padding: "14px 16px",
                color: "var(--text-primary)",
                fontSize: "14px",
                outline: "none",
                transition: "border-color 0.2s",
              }}
            />
            <button
              onClick={handleSendMessage}
              style={{
                padding: "0 16px",
                borderRadius: "8px",
                border: "none",
                backgroundColor: "var(--accent-purple)",
                color: "white",
                cursor: "pointer",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <Send size={16} />
            </button>
          </div>
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              marginTop: "8px",
              fontSize: "11px",
              color: "var(--text-muted)",
            }}
          >
            <span>Active context: 1 file reference, 2 active memories</span>
            <span>Press Enter to send</span>
          </div>
        </div>
      </main>
    </div>
  );
}
