package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"todoAPI/internal/database"

	"github.com/google/uuid"
)

type Task struct {
	ID          uuid.UUID `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	params := parameters{}
	var extra any
	var maxBytesError *http.MaxBytesError
	err := decoder.Decode(&params)
	if err != nil {
		if errors.As(err, &maxBytesError) {
			log.Printf("Error occurred: %v", err)
			respondWithError(w, http.StatusRequestEntityTooLarge, "Request size is too big")
			return
		}
		log.Printf("Error occurred: %v", err)
		respondWithError(w, http.StatusBadRequest, "Incorrect JSON data")
		return
	}

	err = decoder.Decode(&extra)
	if err != io.EOF {
		if errors.As(err, &maxBytesError) {
			log.Printf("Error occurred: %v", err)
			respondWithError(w, http.StatusRequestEntityTooLarge, "Request size is too big")
			return
		}
		log.Printf("Error occurred: %v", err)
		respondWithError(w, http.StatusBadRequest, "Request body must contain a single JSON object")
		return
	}

	if strings.TrimSpace(params.Title) == "" || strings.Contains(params.Title, "\x00") {
		respondWithError(w, http.StatusBadRequest, "Invalid title")
		return
	}

	if params.Status != "todo" && params.Status != "in_progress" && params.Status != "done" {
		respondWithError(w, http.StatusBadRequest, "Status can only be todo, in_progress, done")
		return
	}

	description := sql.NullString{}
	if params.Description != nil {
		description.String = *params.Description
		if strings.Contains(*params.Description, "\x00") {
			respondWithError(w, http.StatusBadRequest, "Invalid description")
			return
		}
		description.Valid = true
	} else {
		description.Valid = false
	}

	taskParams := database.CreateTaskParams{
		Title:       params.Title,
		Status:      params.Status,
		Description: description,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	dbTask, err := cfg.db.CreateTask(ctx, taskParams)
	if err != nil {
		log.Printf("Error occurred: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	task := Task{
		ID:          dbTask.ID,
		Title:       dbTask.Title,
		Description: dbTask.Description.String,
		Status:      dbTask.Status,
		CreatedAt:   dbTask.CreatedAt,
		UpdatedAt:   dbTask.UpdatedAt,
	}

	respondWithJSON(w, http.StatusCreated, task)

}

func (cfg *apiConfig) handlerGetAlltasks(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	var dbTasks []database.Task
	var err error
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if status == "" {
		dbTasks, err = cfg.db.GetAllTasks(ctx)

	} else if status != "todo" && status != "in_progress" && status != "done" {
		respondWithError(w, http.StatusBadRequest, "Status must be one of: done, in_progress, todo")
		return

	} else {
		dbTasks, err = cfg.db.GetAllTasksByStatus(ctx, status)

	}

	if err != nil {
		log.Printf("Error occurred: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	tasks := []Task{}
	for _, value := range dbTasks {
		tasks = append(tasks, Task{
			ID:          value.ID,
			Title:       value.Title,
			Description: value.Description.String,
			Status:      value.Status,
			CreatedAt:   value.CreatedAt,
			UpdatedAt:   value.UpdatedAt,
		})
	}

	respondWithJSON(w, http.StatusOK, tasks)

}

func (cfg *apiConfig) handlerGetTaskByID(w http.ResponseWriter, r *http.Request) {
	taskID, err := uuid.Parse(r.PathValue("taskID"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Incorrect task ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	dbTask, err := cfg.db.GetTaskByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Task with this ID does not exist")
			return
		}
		log.Printf("Error occurred: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return

	}

	task := Task{
		ID:          dbTask.ID,
		Title:       dbTask.Title,
		Description: dbTask.Description.String,
		Status:      dbTask.Status,
		CreatedAt:   dbTask.CreatedAt,
		UpdatedAt:   dbTask.UpdatedAt,
	}

	respondWithJSON(w, http.StatusOK, task)
}

func (cfg *apiConfig) handlerUpdateTaskByID(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Title       string  `json:"title"`
		Status      string  `json:"status"`
		Description *string `json:"description"`
	}

	taskID, err := uuid.Parse(r.PathValue("taskID"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Incorrect task ID")
		return
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	params := parameters{}
	var extra any
	var maxBytesError *http.MaxBytesError
	err = decoder.Decode(&params)
	if err != nil {
		if errors.As(err, &maxBytesError) {
			log.Printf("Error occurred: %v", err)
			respondWithError(w, http.StatusRequestEntityTooLarge, "Request size is too big")
			return
		}
		log.Printf("Error occurred: %v", err)
		respondWithError(w, http.StatusBadRequest, "Incorrect JSON data")
		return
	}

	err = decoder.Decode(&extra)
	if err != io.EOF {
		if errors.As(err, &maxBytesError) {
			log.Printf("Error occurred: %v", err)
			respondWithError(w, http.StatusRequestEntityTooLarge, "Request size is too big")
			return
		}
		log.Printf("Error occurred: %v", err)
		respondWithError(w, http.StatusBadRequest, "Request body must contain a single JSON object")
		return
	}

	if strings.TrimSpace(params.Title) == "" || strings.Contains(params.Title, "\x00") {
		respondWithError(w, http.StatusBadRequest, "Invalid title")
		return
	}

	if params.Status != "todo" && params.Status != "in_progress" && params.Status != "done" {
		respondWithError(w, http.StatusBadRequest, "Status can only be todo, in_progress, done")
		return
	}

	description := sql.NullString{}
	if params.Description != nil {
		description.String = *params.Description
		if strings.Contains(*params.Description, "\x00") {
			respondWithError(w, http.StatusBadRequest, "Invalid description")
			return
		}
		description.Valid = true
	} else {
		description.Valid = false
	}

	updateParams := database.UpdateTaskByIDParams{
		Title:       params.Title,
		Status:      params.Status,
		Description: description,
		ID:          taskID,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	updatedTask, err := cfg.db.UpdateTaskByID(ctx, updateParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Task with this ID does not exist")
			return
		}
		log.Printf("Error occurred: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	task := Task{
		ID:          updatedTask.ID,
		Title:       updatedTask.Title,
		Status:      updatedTask.Status,
		Description: updatedTask.Description.String,
		CreatedAt:   updatedTask.CreatedAt,
		UpdatedAt:   updatedTask.UpdatedAt,
	}

	respondWithJSON(w, http.StatusOK, task)
}

func (cfg *apiConfig) handlerDeleteTaskByID(w http.ResponseWriter, r *http.Request) {
	taskID, err := uuid.Parse(r.PathValue("taskID"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Incorrect task ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	_, err = cfg.db.DeleteTaskByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "Task with this ID does not exist")
			return
		}
		log.Printf("Error occurred: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
