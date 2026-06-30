package search

import (
	"testing"

	"github.com/openbowl/openbowl/packages/core/pkg/db"
)

func TestCosineSimilarity(t *testing.T) {
	vecA := []float32{1.0, 2.0, 3.0}
	vecB := []float32{1.0, 2.0, 3.0}

	score := CosineSimilarity(vecA, vecB)
	if score < 0.99 || score > 1.01 {
		t.Errorf("Expected similarity 1.0 for identical vectors, got %f", score)
	}

	vecC := []float32{-1.0, -2.0, -3.0}
	scoreOpposite := CosineSimilarity(vecA, vecC)
	if scoreOpposite > -0.99 || scoreOpposite < -1.01 {
		t.Errorf("Expected similarity -1.0 for opposite vectors, got %f", scoreOpposite)
	}

	vecD := []float32{0.0, 0.0, 0.0}
	scoreZero := CosineSimilarity(vecA, vecD)
	if scoreZero != 0.0 {
		t.Errorf("Expected similarity 0.0 for zero vector, got %f", scoreZero)
	}
}

func TestSearchMemoriesSemantic(t *testing.T) {
	database, err := db.NewDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Seed workspace and memories
	wsID := "w-search-test"
	if _, err := database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Workspace"); err != nil {
		t.Fatalf("Workspace insert failed: %v", err)
	}

	if _, err := database.Conn.Exec(`
		INSERT INTO memories (id, workspace_id, category, content, is_active) 
		VALUES (?, ?, ?, ?, ?)`, "m-search-1", wsID, "decision", "SQLite DB format preferred", 1); err != nil {
		t.Fatalf("Memory 1 insert failed: %v", err)
	}

	if _, err := database.Conn.Exec(`
		INSERT INTO memories (id, workspace_id, category, content, is_active) 
		VALUES (?, ?, ?, ?, ?)`, "m-search-2", wsID, "preference", "React CSS components style", 1); err != nil {
		t.Fatalf("Memory 2 insert failed: %v", err)
	}

	sm := NewSearchManager(database)
	if err := sm.SetupTable(); err != nil {
		t.Fatalf("SetupTable failed: %v", err)
	}

	// Save vectors (3 dimensions for simplicity)
	// SQLite vector: [1.0, 0.0, 0.0]
	// React vector:  [0.0, 1.0, 0.0]
	if err := sm.SaveEmbedding("m-search-1", []float32{1.0, 0.0, 0.0}); err != nil {
		t.Fatalf("SaveEmbedding failed: %v", err)
	}
	if err := sm.SaveEmbedding("m-search-2", []float32{0.0, 1.0, 0.0}); err != nil {
		t.Fatalf("SaveEmbedding failed: %v", err)
	}

	// Search query matching SQLite vector closely
	queryVec := []float32{0.95, 0.05, 0.0}
	results, err := sm.SearchMemories(wsID, queryVec, 10)
	if err != nil {
		t.Fatalf("SearchMemories failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 search results, got %d", len(results))
	}

	// Top score should be m-search-1 (SQLite decision)
	if results[0].Memory.ID != "m-search-1" {
		t.Errorf("Expected top match to be m-search-1, got %s", results[0].Memory.ID)
	}

	if results[0].Score < 0.90 {
		t.Errorf("Expected high similarity score for m-1, got %f", results[0].Score)
	}

	// Verify limit boundary
	limitedResults, err := sm.SearchMemories(wsID, queryVec, 1)
	if err != nil {
		t.Fatalf("SearchMemories failed: %v", err)
	}
	if len(limitedResults) != 1 {
		t.Errorf("Expected exactly 1 result due to limit check, got %d", len(limitedResults))
	}
}
