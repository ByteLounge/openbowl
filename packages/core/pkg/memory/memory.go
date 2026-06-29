package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/openbowl/openbowl/packages/core/pkg/db"
	"github.com/openbowl/openbowl/packages/core/pkg/models"
	"github.com/openbowl/openbowl/packages/core/pkg/provider"
)

type MemoryEngine struct {
	DB          *db.DB
	WorkerQueue chan *models.Message
	wg          sync.WaitGroup
	quit        chan struct{}
	LLMConfig   *LLMConfig
}

type LLMConfig struct {
	ProviderType string
	ModelName    string
	APIKey       string
	APIURL       string
}

type ExtractedMemory struct {
	Category string `json:"category"` // 'decision', 'todo', 'preference', 'fact', 'architecture'
	Content  string `json:"content"`
}

func NewMemoryEngine(database *db.DB, llmConfig *LLMConfig) *MemoryEngine {
	me := &MemoryEngine{
		DB:          database,
		WorkerQueue: make(chan *models.Message, 100),
		quit:        make(chan struct{}),
		LLMConfig:   llmConfig,
	}
	return me
}

// Start spawns the background worker pool to process messages
func (me *MemoryEngine) Start(workerCount int) {
	for i := 0; i < workerCount; i++ {
		me.wg.Add(1)
		go me.worker(i)
	}
	log.Printf("Memory Engine background workers started (Count: %d)", workerCount)
}

// Stop terminates the workers gracefully
func (me *MemoryEngine) Stop() {
	close(me.quit)
	me.wg.Wait()
	log.Println("Memory Engine background workers stopped.")
}

func (me *MemoryEngine) QueueMessage(msg *models.Message) {
	me.WorkerQueue <- msg
}

func (me *MemoryEngine) worker(id int) {
	defer me.wg.Done()
	for {
		select {
		case msg := <-me.WorkerQueue:
			log.Printf("[Memory Worker %d] Analyzing message %s...", id, msg.ID)
			memories, err := me.Extract(msg)
			if err != nil {
				log.Printf("[Memory Worker %d] Extraction error for message %s: %v", id, msg.ID, err)
				continue
			}
			
			// Save extracted memories to database
			for _, m := range memories {
				err := me.SaveMemory(&m)
				if err != nil {
					log.Printf("[Memory Worker %d] Save error for memory %s: %v", id, m.ID, err)
				} else {
					log.Printf("[Memory Worker %d] Successfully recorded [%s] memory: %s", id, m.Category, m.Content)
				}
			}

		case <-me.quit:
			return
		}
	}
}

// Extract implements hybrid extraction (LLM extraction with a regex fallback)
func (me *MemoryEngine) Extract(msg *models.Message) ([]models.Memory, error) {
	// Skip assistant/system messages for memory parsing (we analyze user instructions/confirmations)
	if msg.Role != "user" {
		return nil, nil
	}

	// 1. Attempt LLM-Based Extraction if config is provided
	if me.LLMConfig != nil && me.LLMConfig.APIKey != "" {
		memories, err := me.extractWithLLM(msg)
		if err == nil && len(memories) > 0 {
			return memories, nil
		}
		// Fallback if LLM parsing failed or returned empty results
	}

	// 2. Offline Regex-Based Rule Fallback
	return me.extractWithRegex(msg), nil
}

func (me *MemoryEngine) extractWithLLM(msg *models.Message) ([]models.Memory, error) {
	p, err := provider.GetProvider(me.LLMConfig.ProviderType)
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(`You are an information extraction worker.
Analyze the user message below and extract any structural knowledge, decisions, todos, preferences, or facts.
Return a valid JSON array of objects, where each object contains:
- "category": choose from 'decision', 'todo', 'preference', 'fact', 'architecture'
- "content": a clear, summarized statement of the item.

User Message: "%s"

JSON Output:`, msg.Content)

	req := &provider.CompletionRequest{
		Model:       me.LLMConfig.ModelName,
		Temperature: 0.0,
		MaxTokens:   500,
		APIKey:      me.LLMConfig.APIKey,
		APIURL:      me.LLMConfig.APIURL,
		Messages: []provider.Message{
			{Role: "user", Content: prompt},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := p.Completion(ctx, req)
	if err != nil {
		return nil, err
	}

	// Clean JSON markdown block markers if present in model output
	cleanJSON := resp.Content
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	var extList []ExtractedMemory
	if err := json.Unmarshal([]byte(cleanJSON), &extList); err != nil {
		return nil, fmt.Errorf("failed to parse JSON memory output: %w", err)
	}

	workspaceID, err := me.getWorkspaceIDForMessage(msg.ConversationID)
	if err != nil {
		return nil, err
	}

	memories := make([]models.Memory, 0, len(extList))
	for _, item := range extList {
		memories = append(memories, models.Memory{
			ID:             uuid.New().String(),
			WorkspaceID:    workspaceID,
			Category:       item.Category,
			Content:        item.Content,
			ConversationID: msg.ConversationID,
			MessageID:      msg.ID,
			IsActive:       true,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		})
	}

	return memories, nil
}

func (me *MemoryEngine) extractWithRegex(msg *models.Message) []models.Memory {
	memories := make([]models.Memory, 0)
	workspaceID, err := me.getWorkspaceIDForMessage(msg.ConversationID)
	if err != nil {
		return nil
	}

	// Regex rules
	rules := []struct {
		pattern  *regexp.Regexp
		category string
		cleaner  func(string) string
	}{
		// Decisions: "We decided to X", "Chose X over Y"
		{
			pattern:  regexp.MustCompile(`(?i)(?:we decided to|let's use|choose|chose)\s+([^.\n]+)`),
			category: "decision",
		},
		// Preferences: "I prefer X", "Use X style"
		{
			pattern:  regexp.MustCompile(`(?i)(?:i prefer|prefer|always use)\s+([^.\n]+)`),
			category: "preference",
		},
		// Todos: "TODO: X"
		{
			pattern:  regexp.MustCompile(`(?i)(?:todo:|task:)\s+([^.\n]+)`),
			category: "todo",
		},
	}

	for _, rule := range rules {
		matches := rule.pattern.FindAllStringSubmatch(msg.Content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				content := strings.TrimSpace(match[1])
				if len(content) > 5 { // Skip trivial matches
					memories = append(memories, models.Memory{
						ID:             uuid.New().String(),
						WorkspaceID:    workspaceID,
						Category:       rule.category,
						Content:        content,
						ConversationID: msg.ConversationID,
						MessageID:      msg.ID,
						IsActive:       true,
						CreatedAt:      time.Now(),
						UpdatedAt:      time.Now(),
					})
				}
			}
		}
	}

	return memories
}

func (me *MemoryEngine) getWorkspaceIDForMessage(convID string) (string, error) {
	var workspaceID string
	err := me.DB.Conn.QueryRow(`
		SELECT p.workspace_id 
		FROM projects p 
		JOIN conversations c ON c.project_id = p.id 
		WHERE c.id = ?`, convID).Scan(&workspaceID)
	if err != nil {
		return "", err
	}
	return workspaceID, nil
}

func (me *MemoryEngine) SaveMemory(m *models.Memory) error {
	_, err := me.DB.Conn.Exec(`
		INSERT INTO memories (id, workspace_id, category, content, source_conversation_id, source_message_id, is_active, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.WorkspaceID, m.Category, m.Content, m.ConversationID, m.MessageID, m.IsActive, m.CreatedAt, m.UpdatedAt)
	return err
}

// Search queries memories matching a search text and category filter
func (me *MemoryEngine) Search(workspaceID string, query string, category string) ([]models.Memory, error) {
	var sb strings.Builder
	params := make([]interface{}, 0)

	sb.WriteString(`
		SELECT id, workspace_id, category, content, COALESCE(source_conversation_id, ''), COALESCE(source_message_id, ''), is_active, created_at, updated_at
		FROM memories 
		WHERE workspace_id = ? AND is_active = 1`)
	
	params = append(params, workspaceID)

	if query != "" {
		sb.WriteString(" AND content LIKE ?")
		params = append(params, "%"+query+"%")
	}

	if category != "" {
		sb.WriteString(" AND category = ?")
		params = append(params, category)
	}

	sb.WriteString(" ORDER BY created_at DESC")

	rows, err := me.DB.Conn.Query(sb.String(), params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	memories := make([]models.Memory, 0)
	for rows.Next() {
		var m models.Memory
		err := rows.Scan(&m.ID, &m.WorkspaceID, &m.Category, &m.Content, &m.ConversationID, &m.MessageID, &m.IsActive, &m.CreatedAt, &m.UpdatedAt)
		if err == nil {
			memories = append(memories, m)
		}
	}

	return memories, nil
}
