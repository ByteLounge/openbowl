package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/glebarez/go-sqlite"
)

type DB struct {
	Conn *sql.DB
}

// NewDB initializes the SQLite database connection and runs schemas
func NewDB(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
	}

	// Connect using pure Go SQLite driver
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode & foreign keys
	if _, err := conn.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to execute pragmas: %w", err)
	}

	db := &DB{Conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed migrations: %w", err)
	}

	log.Printf("Database initialized at: %s (WAL Mode enabled)", dbPath)
	return db, nil
}

func (db *DB) Close() error {
	if db.Conn != nil {
		return db.Conn.Close()
	}
	return nil
}

// migrate creates tables if they do not exist
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS workspaces (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS projects (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS providers (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		name TEXT NOT NULL,
		provider_type TEXT NOT NULL,
		api_url TEXT,
		is_active INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS conversations (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		title TEXT NOT NULL,
		provider_id TEXT,
		model_name TEXT,
		system_instruction TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
		FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE SET NULL
	);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		conversation_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		tokens_prompt INTEGER DEFAULT 0,
		tokens_completion INTEGER DEFAULT 0,
		cost REAL DEFAULT 0.0,
		status TEXT NOT NULL DEFAULT 'sent',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		status TEXT NOT NULL DEFAULT 'todo',
		due_date DATETIME,
		completed_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS memories (
		id TEXT PRIMARY KEY,
		workspace_id TEXT NOT NULL,
		category TEXT NOT NULL,
		content TEXT NOT NULL,
		source_conversation_id TEXT,
		source_message_id TEXT,
		is_active INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
		FOREIGN KEY (source_conversation_id) REFERENCES conversations(id) ON DELETE SET NULL,
		FOREIGN KEY (source_message_id) REFERENCES messages(id) ON DELETE SET NULL
	);

	CREATE TABLE IF NOT EXISTS file_references (
		id TEXT PRIMARY KEY,
		project_id TEXT NOT NULL,
		relative_path TEXT NOT NULL,
		file_hash TEXT NOT NULL,
		content_summary TEXT,
		indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS sync_operations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		operation_type TEXT NOT NULL,
		payload TEXT NOT NULL,
		sequence_number INTEGER NOT NULL,
		applied_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		synced_at DATETIME,
		FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE
	);
	`

	_, err := db.Conn.Exec(schema)
	if err != nil {
		return err
	}

	// Auto-seed default workspace if database is empty
	var count int
	err = db.Conn.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		log.Println("Seeding local database with default workspace details...")
		wsID := "w-dev-default"
		projID := "proj-core-default"

		tx, err := db.Conn.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		_, _ = tx.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "Default Workspace")
		_, _ = tx.Exec(`INSERT INTO projects (id, workspace_id, name, description) VALUES (?, ?, ?, ?)`,
			projID, wsID, "OpenBowl Core", "Universal context layer development")

		// Seed Tasks
		_, _ = tx.Exec(`INSERT INTO tasks (id, project_id, title, description, status) VALUES (?, ?, ?, ?, ?)`,
			"t-1", projID, "Scaffold Tauri desktop workspace", "Configured rust dependencies", "completed")
		_, _ = tx.Exec(`INSERT INTO tasks (id, project_id, title, description, status) VALUES (?, ?, ?, ?, ?)`,
			"t-2", projID, "Implement Go Provider SDK", "Unified adapter patterns", "completed")
		_, _ = tx.Exec(`INSERT INTO tasks (id, project_id, title, description, status) VALUES (?, ?, ?, ?, ?)`,
			"t-3", projID, "Integrate local SQLite tables", "Created database migration pipelines", "in_progress")

		// Seed Decisions
		_, _ = tx.Exec(`INSERT INTO memories (id, workspace_id, category, content, is_active) VALUES (?, ?, ?, ?, ?)`,
			"m-1", wsID, "decision", "Use CGO-free modernc.org SQLite driver for portability.", 1)
		_, _ = tx.Exec(`INSERT INTO memories (id, workspace_id, category, content, is_active) VALUES (?, ?, ?, ?, ?)`,
			"m-2", wsID, "preference", "Prefer functional React style with custom Vanilla CSS.", 1)

		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
