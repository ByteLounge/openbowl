package models

import "time"

type Workspace struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type Project struct {
	ID          string    `json:"id" db:"id"`
	WorkspaceID string    `json:"workspace_id" db:"workspace_id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type Provider struct {
	ID           string    `json:"id" db:"id"`
	WorkspaceID  string    `json:"workspace_id" db:"workspace_id"`
	Name         string    `json:"name" db:"name"`
	ProviderType string    `json:"provider_type" db:"provider_type"` // e.g., "openai", "anthropic", "ollama"
	APIURL       string    `json:"api_url,omitempty" db:"api_url"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

type Conversation struct {
	ID                string    `json:"id" db:"id"`
	ProjectID         string    `json:"project_id" db:"project_id"`
	Title             string    `json:"title" db:"title"`
	ProviderID        string    `json:"provider_id,omitempty" db:"provider_id"`
	ModelName         string    `json:"model_name" db:"model_name"`
	SystemInstruction string    `json:"system_instruction,omitempty" db:"system_instruction"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type Message struct {
	ID               string    `json:"id" db:"id"`
	ConversationID   string    `json:"conversation_id" db:"conversation_id"`
	Role             string    `json:"role" db:"role"` // "system", "user", "assistant", "tool"
	Content          string    `json:"content" db:"content"`
	TokensPrompt     int       `json:"tokens_prompt" db:"tokens_prompt"`
	TokensCompletion int       `json:"tokens_completion" db:"tokens_completion"`
	Cost             float64   `json:"cost" db:"cost"`
	Status           string    `json:"status" db:"status"` // "pending", "sent", "error"
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
}

type Task struct {
	ID          string     `json:"id" db:"id"`
	ProjectID   string     `json:"project_id" db:"project_id"`
	Title       string     `json:"title" db:"title"`
	Description string     `json:"description" db:"description"`
	Status      string     `json:"status" db:"status"` // "todo", "in_progress", "completed", "archived"
	DueDate     *time.Time `json:"due_date,omitempty" db:"due_date"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

type Memory struct {
	ID             string    `json:"id" db:"id"`
	WorkspaceID    string    `json:"workspace_id" db:"workspace_id"`
	Category       string    `json:"category" db:"category"` // "decision", "todo", "preference", "fact", "architecture"
	Content        string    `json:"content" db:"content"`
	ConversationID string    `json:"source_conversation_id,omitempty" db:"source_conversation_id"`
	MessageID      string    `json:"source_message_id,omitempty" db:"source_message_id"`
	IsActive       bool      `json:"is_active" db:"is_active"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

type FileReference struct {
	ID           string    `json:"id" db:"id"`
	ProjectID    string    `json:"project_id" db:"project_id"`
	RelativePath string    `json:"relative_path" db:"relative_path"`
	FileHash     string    `json:"file_hash" db:"file_hash"`
	Summary      string    `json:"content_summary" db:"content_summary"`
	IndexedAt    time.Time `json:"indexed_at" db:"indexed_at"`
}

type SyncOperation struct {
	ID             int64      `json:"id" db:"id"`
	WorkspaceID    string     `json:"workspace_id" db:"workspace_id"`
	EntityType     string     `json:"entity_type" db:"entity_type"`
	EntityID       string     `json:"entity_id" db:"entity_id"`
	OperationType  string     `json:"operation_type" db:"operation_type"` // "INSERT", "UPDATE", "DELETE"
	Payload        string     `json:"payload" db:"payload"`
	SequenceNumber int64      `json:"sequence_number" db:"sequence_number"`
	AppliedAt      time.Time  `json:"applied_at" db:"applied_at"`
	SyncedAt       *time.Time `json:"synced_at,omitempty" db:"synced_at"`
}
