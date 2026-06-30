export interface Workspace {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
}

export interface Project {
  id: string;
  workspaceId: string;
  name: string;
  description?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Provider {
  id: string;
  workspaceId: string;
  name: string;
  providerType:
    | "openai"
    | "anthropic"
    | "gemini"
    | "groq"
    | "openrouter"
    | "deepseek"
    | "ollama"
    | "lmstudio";
  apiUrl?: string;
  isActive: boolean;
  createdAt: string;
}

export interface Conversation {
  id: string;
  projectId: string;
  title: string;
  providerId?: string;
  modelName?: string;
  systemInstruction?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Message {
  id: string;
  conversationId: string;
  role: "system" | "user" | "assistant" | "tool";
  content: string;
  tokensPrompt: number;
  tokensCompletion: number;
  cost: number;
  status: "pending" | "sent" | "error";
  createdAt: string;
}

export interface Task {
  id: string;
  projectId: string;
  title: string;
  description?: string;
  status: "todo" | "in_progress" | "completed" | "archived";
  dueDate?: string;
  completedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Memory {
  id: string;
  workspaceId: string;
  category: "decision" | "todo" | "preference" | "fact" | "architecture";
  content: string;
  sourceConversationId?: string;
  sourceMessageId?: string;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface FileReference {
  id: string;
  projectId: string;
  relativePath: string;
  fileHash: string;
  contentSummary?: string;
  indexedAt: string;
}

export interface SyncOperation {
  id: number;
  workspaceId: string;
  entityType: "project" | "conversation" | "task" | "memory";
  entityId: string;
  operationType: "INSERT" | "UPDATE" | "DELETE";
  payload: string; // JSON string
  sequenceNumber: number;
  appliedAt: string;
  syncedAt?: string;
}
