package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"todoAPI/internal/database"

	_ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) (*sql.DB, *apiConfig) {
	t.Helper()

	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL is not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("Failed to open test DB: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to connect to test DB: %v", err)
	}

	cfg := &apiConfig{
		db: database.New(db),
	}

	return db, cfg
}

func TestCreateTaskSuccess(t *testing.T) {
	db, cfg := setupTestDB(t)

	tests := []struct {
		name                string
		body                string
		expectedDescription string
		descriptionIsNull   bool
	}{
		{
			name:                "Normal description",
			body:                `{"title":"Test task","description":"Test description","status":"todo"}`,
			expectedDescription: "Test description",
		},
		{
			name:                "Empty description",
			body:                `{"title":"Test task","description":"","status":"todo"}`,
			expectedDescription: "",
		},
		{
			name:              "Null description",
			body:              `{"title":"Test task","description":null,"status":"todo"}`,
			descriptionIsNull: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/tasks",
				strings.NewReader(tc.body),
			)
			rec := httptest.NewRecorder()

			cfg.handlerCreateTask(rec, req)

			result := rec.Result()
			defer result.Body.Close()

			if result.StatusCode != http.StatusCreated {
				t.Fatalf("Expected 201, got %d: %s",
					result.StatusCode, rec.Body.String())
			}

			var task Task
			if err := json.NewDecoder(result.Body).Decode(&task); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if task.ID.String() == "00000000-0000-0000-0000-000000000000" {
				t.Error("Expected non-zero task ID")
			}

			if task.Title != "Test task" {
				t.Errorf("Expected title Test task, got %q", task.Title)
			}

			if task.Status != "todo" {
				t.Errorf("Expected status todo, got %q", task.Status)
			}

			if task.Description != tc.expectedDescription {
				t.Errorf("Expected description %q, got %q",
					tc.expectedDescription, task.Description)
			}

			if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
				t.Error("Expected non-zero timestamps")
			}

			var storedDescription sql.NullString

			err := db.QueryRow(
				"SELECT description FROM tasks WHERE id = $1",
				task.ID,
			).Scan(&storedDescription)
			if err != nil {
				t.Fatalf("Failed to query created task: %v", err)
			}

			if tc.descriptionIsNull && storedDescription.Valid {
				t.Error("Expected SQL NULL description")
			}

			if !tc.descriptionIsNull && !storedDescription.Valid {
				t.Error("Expected non-NULL description")
			}

			if storedDescription.Valid &&
				storedDescription.String != tc.expectedDescription {
				t.Errorf("Unexpected stored description: %q",
					storedDescription.String)
			}

			_, err = db.Exec("DELETE FROM tasks WHERE id = $1", task.ID)
			if err != nil {
				t.Fatalf("Failed to clean up test task: %v", err)
			}
		})
	}
}

func TestCreateTaskDatabaseError(t *testing.T) {
	db, cfg := setupTestDB(t)

	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/tasks",
		strings.NewReader(`{
            "title": "Database error test",
            "description": "Testing HTTP 500",
            "status": "todo"
        }`),
	)

	rec := httptest.NewRecorder()
	cfg.handlerCreateTask(rec, req)

	result := rec.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf(
			"Expected %d, got %d",
			http.StatusInternalServerError,
			result.StatusCode,
		)
	}

	var response struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(result.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if response.Error != "Internal server error" {
		t.Errorf(
			"Expected generic error message, got %q",
			response.Error,
		)
	}
}
