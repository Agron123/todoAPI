package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
	"todoAPI/internal/database"

	"github.com/google/uuid"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

const port = "8080"

type apiConfig struct {
	db *database.Queries
}

type Task struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Created_at  time.Time `json:"created_at"`
	Updated_at  time.Time `json:"updated_at"`
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type returnError struct {
		Error string `json:"error"`
	}

	err := returnError{
		Error: msg,
	}

	respondWithJSON(w, code, err)
}

func (cfg *apiConfig) handlerCreateTask(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Title       string  `json:"title"`
		Status      string  `json:"status"`
		Description *string `json:"description"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	description := sql.NullString{}
	if params.Description != nil {
		description.String = *params.Description
		description.Valid = true
	} else {
		description.Valid = false
	}

	taskParams := database.CreateTaskParams{
		Title:       params.Title,
		Status:      params.Status,
		Description: description,
	}

	dbTask, err := cfg.db.CreateTask(r.Context(), taskParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	task := Task{
		ID:          dbTask.ID,
		Title:       dbTask.Title,
		Description: dbTask.Description.String,
		Status:      dbTask.Status,
		Created_at:  dbTask.CreatedAt,
		Updated_at:  dbTask.UpdatedAt,
	}

	respondWithJSON(w, http.StatusCreated, task)

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

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("POST /api/tasks", apiCfg.handlerCreateTask)
	server := http.Server{
		Addr:    ":" + port,
		Handler: serveMux,
	}

	log.Printf("Listening on %v port", port)
	log.Fatal(server.ListenAndServe())
}
