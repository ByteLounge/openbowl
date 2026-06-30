package migration

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/openbowl/openbowl/packages/core/pkg/db"
	"github.com/openbowl/openbowl/packages/core/pkg/models"
)

type MigrationManager struct {
	DB *db.DB
}

func NewMigrationManager(database *db.DB) *MigrationManager {
	return &MigrationManager{DB: database}
}

// ChatGPT conversation backup shapes
type ChatGPTExtract struct {
	Title   string                 `json:"title"`
	Mapping map[string]chatGPTNode `json:"mapping"`
}

type chatGPTNode struct {
	ID      string          `json:"id"`
	Message *chatGPTMessage `json:"message,omitempty"`
}

type chatGPTMessage struct {
	Author struct {
		Role string `json:"role"` // "user", "assistant", "system"
	} `json:"author"`
	Content struct {
		Parts []interface{} `json:"parts,omitempty"` // Can contain strings or structures
	} `json:"content"`
	CreateTime float64 `json:"create_time,omitempty"`
}

// ImportChatGPTBackup imports standard conversations.json exports from OpenAI
func (mm *MigrationManager) ImportChatGPTBackup(projectID string, reader io.Reader) (int, error) {
	var chatList []ChatGPTExtract
	if err := json.NewDecoder(reader).Decode(&chatList); err != nil {
		return 0, fmt.Errorf("failed to decode ChatGPT conversations JSON: %w", err)
	}

	tx, err := mm.DB.Conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	importCount := 0

	for _, chat := range chatList {
		convID := uuid.New().String()

		title := chat.Title
		if title == "" {
			title = "Imported ChatGPT Chat"
		}

		// Insert Conversation record
		_, err = tx.Exec(`
			INSERT INTO conversations (id, project_id, title, model_name, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?)`,
			convID, projectID, title, "gpt-imported", time.Now(), time.Now())
		if err != nil {
			return 0, fmt.Errorf("failed to insert conversation: %w", err)
		}

		// Traverse message nodes mapping (OpenAI logs chat trees)
		for _, node := range chat.Mapping {
			if node.Message == nil || len(node.Message.Content.Parts) == 0 {
				continue
			}

			// Capture string content from parts
			content := ""
			for _, part := range node.Message.Content.Parts {
				if str, ok := part.(string); ok {
					content += str
				}
			}

			if content == "" {
				continue
			}

			role := node.Message.Author.Role
			// Sanitize role names
			if role == "critic" || role == "tool" {
				role = "system"
			}

			createdTime := time.Now()
			if node.Message.CreateTime > 0 {
				createdTime = time.Unix(int64(node.Message.CreateTime), 0)
			}

			msgID := uuid.New().String()
			_, err = tx.Exec(`
				INSERT INTO messages (id, conversation_id, role, content, status, created_at)
				VALUES (?, ?, ?, ?, ?, ?)`,
				msgID, convID, role, content, "sent", createdTime)
			if err != nil {
				return 0, fmt.Errorf("failed to insert message node: %w", err)
			}
		}
		importCount++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return importCount, nil
}

// ExportWorkspaceState serializes the entire workspace SQLite state into a structured JSON string
func (mm *MigrationManager) ExportWorkspaceState(workspaceID string, writer io.Writer) error {
	// Fetch projects
	pRows, err := mm.DB.Conn.Query(`SELECT id, workspace_id, name, COALESCE(description, ''), created_at, updated_at FROM projects WHERE workspace_id = ?`, workspaceID)
	if err != nil {
		return err
	}
	defer pRows.Close()

	projects := make([]models.Project, 0)
	for pRows.Next() {
		var p models.Project
		if err := pRows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt); err == nil {
			projects = append(projects, p)
		}
	}

	// Fetch memories
	mRows, err := mm.DB.Conn.Query(`SELECT id, workspace_id, category, content, COALESCE(source_conversation_id, ''), COALESCE(source_message_id, ''), is_active, created_at, updated_at FROM memories WHERE workspace_id = ?`, workspaceID)
	if err != nil {
		return err
	}
	defer mRows.Close()

	memories := make([]models.Memory, 0)
	for mRows.Next() {
		var m models.Memory
		if err := mRows.Scan(&m.ID, &m.WorkspaceID, &m.Category, &m.Content, &m.ConversationID, &m.MessageID, &m.IsActive, &m.CreatedAt, &m.UpdatedAt); err == nil {
			memories = append(memories, m)
		}
	}

	exportPayload := map[string]interface{}{
		"workspace_id": workspaceID,
		"exported_at":  time.Now(),
		"projects":     projects,
		"memories":     memories,
	}

	return json.NewEncoder(writer).Encode(exportPayload)
}
