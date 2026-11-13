package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"
	"todoAPI/internal/database"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

const port = "8080"

type apiConfig struct {
	db *database.Queries
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		log.Fatal("Db URL cannot be empty")
	}
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Printf("Error occurred: %v", err)
		log.Fatal("Fatal error, shutting down server")
	}

	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = db.PingContext(ctx)
	cancel()
	if err != nil {
		log.Printf("Error occurred: %v", err)
		log.Fatal("Fatal error, shutting down server")
	}

	dbQueries := database.New(db)
	apiCfg := apiConfig{
		db: dbQueries,
	}

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("POST /api/tasks", apiCfg.handlerCreateTask)
	serveMux.HandleFunc("GET /api/tasks", apiCfg.handlerGetAlltasks)
	serveMux.HandleFunc("GET /api/tasks/{taskID}", apiCfg.handlerGetTaskByID)
	serveMux.HandleFunc("PUT /api/tasks/{taskID}", apiCfg.handlerUpdateTaskByID)
	serveMux.HandleFunc("DELETE /api/tasks/{taskID}", apiCfg.handlerDeleteTaskByID)

	server := http.Server{
		Addr:              ":" + port,
		Handler:           serveMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
	}

	log.Printf("Listening on %v port", port)
	log.Fatal(server.ListenAndServe())
}
