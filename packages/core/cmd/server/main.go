package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/openbowl/openbowl/packages/core/pkg/db"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all cross-origins for local desktop app development
	},
}

func main() {
	// Load environmental variables if present
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

	// Initialize Gin Router
	r := gin.Default()

	// CORS Policies
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
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

		// Placeholder endpoints for resources
		api.GET("/workspaces", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"workspaces": []interface{}{}})
		})

		api.GET("/projects", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"projects": []interface{}{}})
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
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WS Read error: %v", err)
			break
		}

		log.Printf("WS Received: %s", string(payload))

		// Echo connection loop
		response := []byte(`{"event":"acknowledged","payload":{"message":"OpenBowl websocket channel active."}}`)
		if err := conn.WriteMessage(messageType, response); err != nil {
			log.Printf("WS Write error: %v", err)
			break
		}
	}
}
