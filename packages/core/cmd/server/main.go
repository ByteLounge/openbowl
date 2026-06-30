package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/openbowl/openbowl/packages/core/pkg/db"
	"github.com/openbowl/openbowl/packages/core/pkg/provider"
	"github.com/openbowl/openbowl/packages/core/pkg/watcher"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all cross-origins for local desktop app development
	},
}

type WSEvent struct {
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

type ChatStartPayload struct {
	ConversationID string `json:"conversation_id"`
	ProviderType   string `json:"provider_type"`
	ModelName      string `json:"model_name"`
	Prompt         string `json:"prompt"`
	APIKey         string `json:"api_key"`
	APIURL         string `json:"api_url"`
}

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3010"
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "openbowl.db"
	}

	// Initialize Local SQLite Database
	database, err := db.NewDB(dbPath)
	if err != nil {
		log.Fatalf("Fatal: Database setup failed: %v", err)
	}
	defer database.Close()

	// Start File Watcher on active workspace
	workspaceDir := os.Getenv("WORKSPACE_DIR")
	if workspaceDir == "" {
		workspaceDir = "."
	}
	fw, err := watcher.NewFileWatcher(database, "proj-core-default", workspaceDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize file watcher: %v", err)
	} else {
		fw.Start()
		defer fw.Stop()
	}

	// Initialize Gin Router
	r := gin.Default()

	// CORS Policies
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
	}))

	// API Routing
	api := r.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":      "healthy",
				"version":     "0.1.0",
				"environment": "development",
			})
		})

		// GET Workspaces
		api.GET("/workspaces", func(c *gin.Context) {
			rows, err := database.Conn.Query(`SELECT id, name FROM workspaces`)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			workspaces := []gin.H{}
			for rows.Next() {
				var id, name string
				if err := rows.Scan(&id, &name); err == nil {
					workspaces = append(workspaces, gin.H{"id": id, "name": name})
				}
			}
			c.JSON(http.StatusOK, workspaces)
		})

		// POST Workspaces
		api.POST("/workspaces", func(c *gin.Context) {
			var input struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if input.ID == "" {
				input.ID = "w-" + time.Now().Format("20060102150405")
			}
			_, err := database.Conn.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, input.ID, input.Name)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "created", "id": input.ID})
		})

		// GET Projects
		api.GET("/projects", func(c *gin.Context) {
			workspaceID := c.Query("workspace_id")
			var rows *sql.Rows
			var err error
			if workspaceID != "" {
				rows, err = database.Conn.Query(`SELECT id, workspace_id, name, COALESCE(description, '') FROM projects WHERE workspace_id = ?`, workspaceID)
			} else {
				rows, err = database.Conn.Query(`SELECT id, workspace_id, name, COALESCE(description, '') FROM projects`)
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			projects := []gin.H{}
			for rows.Next() {
				var id, wsID, name, desc string
				if err := rows.Scan(&id, &wsID, &name, &desc); err == nil {
					projects = append(projects, gin.H{"id": id, "workspace_id": wsID, "name": name, "description": desc})
				}
			}
			c.JSON(http.StatusOK, projects)
		})

		// POST Projects
		api.POST("/projects", func(c *gin.Context) {
			var input struct {
				ID          string `json:"id"`
				WorkspaceID string `json:"workspace_id"`
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if input.ID == "" {
				input.ID = "p-" + time.Now().Format("20060102150405")
			}
			_, err := database.Conn.Exec(`INSERT INTO projects (id, workspace_id, name, description) VALUES (?, ?, ?, ?)`,
				input.ID, input.WorkspaceID, input.Name, input.Description)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "created", "id": input.ID})
		})

		// GET Tasks
		api.GET("/projects/:id/tasks", func(c *gin.Context) {
			projID := c.Param("id")
			rows, err := database.Conn.Query(`SELECT id, project_id, title, COALESCE(description, ''), status FROM tasks WHERE project_id = ?`, projID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			tasks := []gin.H{}
			for rows.Next() {
				var id, pID, title, desc, status string
				if err := rows.Scan(&id, &pID, &title, &desc, &status); err == nil {
					tasks = append(tasks, gin.H{"id": id, "project_id": pID, "title": title, "description": desc, "status": status})
				}
			}
			c.JSON(http.StatusOK, tasks)
		})

		// POST Tasks
		api.POST("/projects/:id/tasks", func(c *gin.Context) {
			projID := c.Param("id")
			var input struct {
				ID          string `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Status      string `json:"status"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if input.ID == "" {
				input.ID = "t-" + time.Now().Format("20060102150405")
			}
			if input.Status == "" {
				input.Status = "todo"
			}
			_, err := database.Conn.Exec(`INSERT INTO tasks (id, project_id, title, description, status) VALUES (?, ?, ?, ?, ?)`,
				input.ID, projID, input.Title, input.Description, input.Status)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "created", "id": input.ID})
		})

		// PATCH Tasks
		api.PATCH("/tasks/:id", func(c *gin.Context) {
			taskID := c.Param("id")
			var input struct {
				Status string `json:"status"`
			}
			if err := c.ShouldBindJSON(&input); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			_, err := database.Conn.Exec(`UPDATE tasks SET status = ? WHERE id = ?`, input.Status, taskID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "updated"})
		})

		// GET Memories
		api.GET("/workspaces/:id/memories", func(c *gin.Context) {
			wsID := c.Param("id")
			rows, err := database.Conn.Query(`SELECT id, workspace_id, category, content, is_active FROM memories WHERE workspace_id = ?`, wsID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			defer rows.Close()

			memories := []gin.H{}
			for rows.Next() {
				var id, wID, cat, content string
				var isActive int
				if err := rows.Scan(&id, &wID, &cat, &content, &isActive); err == nil {
					memories = append(memories, gin.H{"id": id, "workspace_id": wID, "category": cat, "content": content, "is_active": isActive == 1})
				}
			}
			c.JSON(http.StatusOK, memories)
		})

		// GET Compiled Context for browser extensions
		api.GET("/projects/:id/context", func(c *gin.Context) {
			projID := c.Param("id")

			var projName, wsID string
			err := database.Conn.QueryRow(`SELECT name, workspace_id FROM projects WHERE id = ?`, projID).Scan(&projName, &wsID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			rows, err := database.Conn.Query(`SELECT title, status FROM tasks WHERE project_id = ?`, projID)
			var tasksStr strings.Builder
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var title, status string
					if err := rows.Scan(&title, &status); err == nil {
						tasksStr.WriteString(fmt.Sprintf("- [%s] %s\n", status, title))
					}
				}
			}

			mRows, err := database.Conn.Query(`SELECT category, content FROM memories WHERE workspace_id = ? AND is_active = 1`, wsID)
			var memStr strings.Builder
			if err == nil {
				defer mRows.Close()
				for mRows.Next() {
					var cat, content string
					if err := mRows.Scan(&cat, &content); err == nil {
						memStr.WriteString(fmt.Sprintf("- [%s]: %s\n", cat, content))
					}
				}
			}

			var promptPkg strings.Builder
			promptPkg.WriteString(fmt.Sprintf("=== OPENBOWL CONTEXT INJECTION (Project: %s) ===\n", projName))
			promptPkg.WriteString("Active decisions & preferences:\n")
			if memStr.Len() > 0 {
				promptPkg.WriteString(memStr.String())
			} else {
				promptPkg.WriteString("- None\n")
			}
			promptPkg.WriteString("\nProject task roadmap:\n")
			if tasksStr.Len() > 0 {
				promptPkg.WriteString(tasksStr.String())
			} else {
				promptPkg.WriteString("- None\n")
			}
			promptPkg.WriteString("================================================\n")

			c.JSON(http.StatusOK, gin.H{
				"project_id":   projID,
				"context_text": promptPkg.String(),
			})
		})
	}

	// WebSocket handler for streaming completions
	r.GET("/ws/chat", func(c *gin.Context) {
		wsHandler(c.Writer, c.Request)
	})

	log.Printf("OpenBowl Core sidecar starting on http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Fatal: Server execution failed: %v", err)
	}
}

// WebSocket handler
func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("WS Client connected from %s", r.RemoteAddr)

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WS Read error: %v", err)
			break
		}

		var event WSEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Printf("WS Unmarshal error: %v", err)
			continue
		}

		if event.Event == "chat.start" {
			var chatReq ChatStartPayload
			if err := json.Unmarshal(event.Payload, &chatReq); err != nil {
				log.Printf("WS Chat payload unmarshal error: %v", err)
				continue
			}

			// Run completion in a background routine
			go handleChatStreaming(conn, &chatReq)
		}
	}
}

func handleChatStreaming(conn *websocket.Conn, req *ChatStartPayload) {
	p, err := provider.GetProvider(req.ProviderType)
	if err != nil {
		sendWSError(conn, req.ConversationID, fmt.Sprintf("Failed to load provider: %v", err))
		return
	}

	streamChan := make(chan *provider.StreamChunk)
	
	// Create context with standard timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pReq := &provider.CompletionRequest{
		Model:       req.ModelName,
		Temperature: 0.7,
		APIKey:      req.APIKey,
		APIURL:      req.APIURL,
		Messages: []provider.Message{
			{Role: "user", Content: req.Prompt},
		},
	}

	err = p.CompletionStream(ctx, pReq, streamChan)
	if err != nil {
		sendWSError(conn, req.ConversationID, fmt.Sprintf("Streaming execution failed: %v", err))
		return
	}

	for chunk := range streamChan {
		if chunk.Error != "" {
			sendWSError(conn, req.ConversationID, chunk.Error)
			return
		}

		if chunk.Done {
			resp := map[string]interface{}{
				"event": "chat.complete",
				"payload": map[string]interface{}{
					"conversation_id":   req.ConversationID,
					"tokens_prompt":     chunk.TokensPrompt, // standard counts
					"tokens_completion": chunk.TokensCompletion,
					"cost":              chunk.Cost,
				},
			}
			data, _ := json.Marshal(resp)
			_ = conn.WriteMessage(websocket.TextMessage, data)
			break
		}

		resp := map[string]interface{}{
			"event": "chat.chunk",
			"payload": map[string]interface{}{
				"conversation_id": req.ConversationID,
				"text":            chunk.Content,
			},
		}
		data, _ := json.Marshal(resp)
		_ = conn.WriteMessage(websocket.TextMessage, data)
	}
}

func sendWSError(conn *websocket.Conn, convID string, errMsg string) {
	resp := map[string]interface{}{
		"event": "chat.error",
		"payload": map[string]interface{}{
			"conversation_id": convID,
			"error":           errMsg,
		},
	}
	data, _ := json.Marshal(resp)
	_ = conn.WriteMessage(websocket.TextMessage, data)
}
