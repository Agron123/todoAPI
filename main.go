package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"todoAPI/internal/database"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

const port = "8080"

type apiConfig struct {
	db *database.Queries
}

func (cfg *apiConfig) handlerCreateTask(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)
	apiCfg := apiConfig{
		db: dbQueries,
	}

	serveMux := http.NewServeMux
	server := http.Server{
		Addr:    ":" + port,
		Handler: serveMux(),
	}

	log.Printf("Listening on %v port", port)
	log.Fatal(server.ListenAndServe())
}
