package context

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
	"github.com/openbowl/openbowl/packages/core/pkg/models"
)

type ContextEngine struct {
	DB *db.DB
}

func NewContextEngine(database *db.DB) *ContextEngine {
	return &ContextEngine{DB: database}
}

type AssembleRequest struct {
	WorkspaceID    string `json:"workspace_id"`
	ProjectID      string `json:"project_id"`
	ConversationID string `json:"conversation_id"`
	TokenBudget    int    `json:"token_budget"` // E.g., 8000 tokens
}

type ContextPackage struct {
	SystemPrompt string           `json:"system_prompt"`
	History      []models.Message `json:"history"`
	TotalTokens  int              `json:"total_tokens"`
}

// Approximate tokens based on character length (4 chars = 1 token)
func charCountToTokens(chars int) int {
	return (chars + 3) / 4
}

// Assemble compiles the context package based on the allocation priorities
func (ce *ContextEngine) Assemble(req *AssembleRequest) (*ContextPackage, error) {
	if req.TokenBudget <= 0 {
		req.TokenBudget = 8000 // Default budget
	}

	// 1. Fetch Project Metadata
	var project models.Project
	err := ce.DB.Conn.QueryRow(`
		SELECT id, workspace_id, name, COALESCE(description, ''), created_at, updated_at 
		FROM projects WHERE id = ?`, req.ProjectID).
		Scan(&project.ID, &project.WorkspaceID, &project.Name, &project.Description, &project.CreatedAt, &project.UpdatedAt)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to fetch project: %w", err)
	}

	// 2. Fetch Active Memories / Decisions
	memories := make([]models.Memory, 0)
	rows, err := ce.DB.Conn.Query(`
		SELECT id, workspace_id, category, content, COALESCE(source_conversation_id, ''), COALESCE(source_message_id, ''), is_active, created_at, updated_at
		FROM memories WHERE workspace_id = ? AND is_active = 1`, req.WorkspaceID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var m models.Memory
			if err := rows.Scan(&m.ID, &m.WorkspaceID, &m.Category, &m.Content, &m.ConversationID, &m.MessageID, &m.IsActive, &m.CreatedAt, &m.UpdatedAt); err == nil {
				memories = append(memories, m)
			}
		}
	}

	// 3. Fetch Tasks
	tasks := make([]models.Task, 0)
	tRows, err := ce.DB.Conn.Query(`
		SELECT id, project_id, title, COALESCE(description, ''), status, due_date, completed_at, created_at, updated_at
		FROM tasks WHERE project_id = ? AND status != 'archived'`, req.ProjectID)
	if err == nil {
		defer tRows.Close()
		for tRows.Next() {
			var t models.Task
			if err := tRows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Status, &t.DueDate, &t.CompletedAt, &t.CreatedAt, &t.UpdatedAt); err == nil {
				tasks = append(tasks, t)
			}
		}
	}

	// 4. Fetch File References & Contents
	fileRefs := make([]models.FileReference, 0)
	fRows, err := ce.DB.Conn.Query(`
		SELECT id, project_id, relative_path, file_hash, COALESCE(content_summary, ''), indexed_at
		FROM file_references WHERE project_id = ?`, req.ProjectID)
	if err == nil {
		defer fRows.Close()
		for fRows.Next() {
			var f models.FileReference
			if err := fRows.Scan(&f.ID, &f.ProjectID, &f.RelativePath, &f.FileHash, &f.Summary, &f.IndexedAt); err == nil {
				fileRefs = append(fileRefs, f)
			}
		}
	}

	// 5. Fetch Recent Messages (reverse chronological order, then reverse again to maintain timeline flow)
	mRows, err := ce.DB.Conn.Query(`
		SELECT id, conversation_id, role, content, tokens_prompt, tokens_completion, cost, status, created_at
		FROM messages WHERE conversation_id = ? ORDER BY created_at DESC LIMIT 15`, req.ConversationID)
	
	rawMessages := make([]models.Message, 0)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var msg models.Message
			if err := mRows.Scan(&msg.ID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.TokensPrompt, &msg.TokensCompletion, &msg.Cost, &msg.Status, &msg.CreatedAt); err == nil {
				rawMessages = append(rawMessages, msg)
			}
		}
	}

	// Re-order messages Chronologically (oldest to newest)
	history := make([]models.Message, 0, len(rawMessages))
	for i := len(rawMessages) - 1; i >= 0; i-- {
		history = append(history, rawMessages[i])
	}

	// Build System Prompt Context
	var sb strings.Builder
	sb.WriteString("You are an expert AI assistant working inside OpenBowl.\n\n")

	// Inject Workspace Goal / Project details
	sb.WriteString("### ACTIVE PROJECT GOALS\n")
	sb.WriteString(fmt.Sprintf("Name: %s\n", project.Name))
	if project.Description != "" {
		sb.WriteString(fmt.Sprintf("Objective: %s\n", project.Description))
	}
	sb.WriteString("\n")

	// Budget allocation tracking
	currentTokens := charCountToTokens(sb.Len())

	// Inject Decisions (Priority 1 after goals)
	if len(memories) > 0 {
		var decSb strings.Builder
		decSb.WriteString("### ACTIVE DECISIONS & PREFERENCES\n")
		for _, m := range memories {
			decSb.WriteString(fmt.Sprintf("- [%s]: %s\n", m.Category, m.Content))
		}
		decSb.WriteString("\n")
		
		decTokens := charCountToTokens(decSb.Len())
		if currentTokens+decTokens < req.TokenBudget/4 { // Allocate up to 25% of budget
			sb.WriteString(decSb.String())
			currentTokens += decTokens
		}
	}

	// Inject Tasks (Priority 2)
	if len(tasks) > 0 {
		var taskSb strings.Builder
		taskSb.WriteString("### WORKSPACE TASKS\n")
		for _, t := range tasks {
			taskSb.WriteString(fmt.Sprintf("- [%s] %s (%s)\n", t.Status, t.Title, t.Description))
		}
		taskSb.WriteString("\n")

		taskTokens := charCountToTokens(taskSb.Len())
		if currentTokens+taskTokens < req.TokenBudget/3 { // Allocate up to ~33%
			sb.WriteString(taskSb.String())
			currentTokens += taskTokens
		}
	}

	// Inject File Contents/Code context (Priority 3)
	if len(fileRefs) > 0 {
		sb.WriteString("### RELEVANT PROJECT FILES\n")
		for _, f := range fileRefs {
			// Resolve and read file locally if available
			content := ""
			if _, err := os.Stat(f.RelativePath); err == nil {
				if bytes, err := os.ReadFile(f.RelativePath); err == nil {
					content = string(bytes)
				}
			}
			if content == "" {
				content = f.Summary
			}

			// Restrict file injection to prevent overflow
			fileHeader := fmt.Sprintf("File: %s\n```\n", filepath.Base(f.RelativePath))
			fileFooter := "\n```\n\n"
			
			fileTokens := charCountToTokens(len(fileHeader) + len(content) + len(fileFooter))

			// If file exceeds local budget, truncate it
			if currentTokens+fileTokens > req.TokenBudget/2 { // Limit files to ~50% of prompt budget
				maxContentLen := ((req.TokenBudget / 2) - currentTokens) * 4
				if maxContentLen > 100 {
					content = content[:maxContentLen] + "\n[... Content Truncated by Context Engine ...]"
				} else {
					continue
				}
			}

			sb.WriteString(fileHeader)
			sb.WriteString(content)
			sb.WriteString(fileFooter)
			currentTokens += charCountToTokens(len(fileHeader) + len(content) + len(fileFooter))
		}
	}

	// Compile Chat history tokens
	historyTokens := 0
	for _, h := range history {
		historyTokens += charCountToTokens(len(h.Content))
	}

	// If history + system prompt exceeds the target budget, drop older history items
	for currentTokens+historyTokens > req.TokenBudget && len(history) > 1 {
		// Drop oldest history (skip system messages)
		if history[0].Role == "system" {
			history = history[1:]
		} else {
			history = history[1:]
		}
		
		// Recalculate history tokens
		historyTokens = 0
		for _, h := range history {
			historyTokens += charCountToTokens(len(h.Content))
		}
	}

	return &ContextPackage{
		SystemPrompt: sb.String(),
		History:      history,
		TotalTokens:  currentTokens + historyTokens,
	}, nil
}
