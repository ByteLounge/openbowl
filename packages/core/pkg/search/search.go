package search

import (
	"encoding/json"
	"math"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
	"github.com/openbowl/openbowl/packages/core/pkg/models"
)

// CosineSimilarity computes the similarity score between two float32 vectors
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}
	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0.0 || normB == 0.0 {
		return 0.0
	}
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

type SearchResult struct {
	Memory models.Memory `json:"memory"`
	Score  float32       `json:"score"`
}

type SearchManager struct {
	DB *db.DB
}

func NewSearchManager(database *db.DB) *SearchManager {
	return &SearchManager{DB: database}
}

// SetupTable creates the helper table storing JSON-serialized float vectors mapping to memories
func (sm *SearchManager) SetupTable() error {
	schema := `
	CREATE TABLE IF NOT EXISTS memory_embeddings (
		memory_id TEXT PRIMARY KEY,
		embedding_json TEXT NOT NULL,
		FOREIGN KEY (memory_id) REFERENCES memories(id) ON DELETE CASCADE
	);
	`
	_, err := sm.DB.Conn.Exec(schema)
	return err
}

// SaveEmbedding serializes and stores the vector embedding for a memory node
func (sm *SearchManager) SaveEmbedding(memoryID string, vector []float32) error {
	data, err := json.Marshal(vector)
	if err != nil {
		return err
	}

	query := `
	INSERT INTO memory_embeddings (memory_id, embedding_json)
	VALUES (?, ?)
	ON CONFLICT(memory_id) DO UPDATE SET embedding_json = excluded.embedding_json
	`
	_, err = sm.DB.Conn.Exec(query, memoryID, string(data))
	return err
}

// SearchMemories computes cosine similarities and returns the top-K matches sorted by distance
func (sm *SearchManager) SearchMemories(workspaceID string, queryVector []float32, limit int) ([]SearchResult, error) {
	query := `
		SELECT m.id, m.workspace_id, m.category, m.content, COALESCE(m.source_conversation_id, ''), 
		       COALESCE(m.source_message_id, ''), m.is_active, m.created_at, m.updated_at, me.embedding_json
		FROM memories m
		JOIN memory_embeddings me ON m.id = me.memory_id
		WHERE m.workspace_id = ? AND m.is_active = 1
	`
	rows, err := sm.DB.Conn.Query(query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []SearchResult{}

	for rows.Next() {
		var m models.Memory
		var embedJSON string
		err := rows.Scan(
			&m.ID, &m.WorkspaceID, &m.Category, &m.Content, &m.ConversationID,
			&m.MessageID, &m.IsActive, &m.CreatedAt, &m.UpdatedAt, &embedJSON,
		)
		if err != nil {
			continue
		}

		var vector []float32
		if err := json.Unmarshal([]byte(embedJSON), &vector); err != nil {
			continue
		}

		score := CosineSimilarity(queryVector, vector)
		results = append(results, SearchResult{
			Memory: m,
			Score:  score,
		})
	}

	// Sort results descending by score
	// Custom bubble or selection sort to keep code simple & CGO-free
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Apply limit constraint
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}
