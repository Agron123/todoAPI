package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

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

func TestGetAllTasks(t *testing.T) {
	db, cfg := setupTestDB(t)
	type testTask struct {
		ID     uuid.UUID
		Status string
	}

	var created []testTask

	for _, status := range []string{"todo", "in_progress", "done"} {
		id := uuid.New()

		_, err := db.ExecContext(
			context.Background(),
			`INSERT INTO tasks (id, title, description, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())`,
			id,
			"GET integration test "+id.String(),
			"Test description",
			status,
		)
		if err != nil {
			t.Fatalf("Failed to insert test task: %v", err)
		}

		created = append(created, testTask{ID: id, Status: status})

		t.Cleanup(func() {
			_, _ = db.ExecContext(
				context.Background(),
				"DELETE FROM tasks WHERE id = $1",
				id,
			)
		})
	}

	tests := []struct {
		name           string
		url            string
		expectedStatus int
		expectedIDs    []uuid.UUID
	}{
		{
			name:           "Get all tasks",
			url:            "/api/tasks",
			expectedStatus: http.StatusOK,
			expectedIDs: []uuid.UUID{
				created[0].ID,
				created[1].ID,
				created[2].ID,
			},
		},
		{
			name:           "Filter todo",
			url:            "/api/tasks?status=todo",
			expectedStatus: http.StatusOK,
			expectedIDs:    []uuid.UUID{created[0].ID},
		},
		{
			name:           "Filter in_progress",
			url:            "/api/tasks?status=in_progress",
			expectedStatus: http.StatusOK,
			expectedIDs:    []uuid.UUID{created[1].ID},
		},
		{
			name:           "Filter done",
			url:            "/api/tasks?status=done",
			expectedStatus: http.StatusOK,
			expectedIDs:    []uuid.UUID{created[2].ID},
		},
		{
			name:           "Empty status returns all",
			url:            "/api/tasks?status=",
			expectedStatus: http.StatusOK,
			expectedIDs: []uuid.UUID{
				created[0].ID,
				created[1].ID,
				created[2].ID,
			},
		},
		{
			name:           "Invalid status",
			url:            "/api/tasks?status=invalid",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			rec := httptest.NewRecorder()

			cfg.handlerGetAlltasks(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Fatalf(
					"Expected HTTP %d, got %d. Body: %s",
					tt.expectedStatus,
					rec.Code,
					rec.Body.String(),
				)
			}

			if tt.expectedStatus != http.StatusOK {
				return
			}

			var tasks []Task
			if err := json.Unmarshal(rec.Body.Bytes(), &tasks); err != nil {
				t.Fatalf("Invalid JSON response: %v", err)
			}

			if tasks == nil {
				t.Fatal("Expected JSON array, got null")
			}

			found := make(map[uuid.UUID]Task)
			for _, task := range tasks {
				found[task.ID] = task
			}

			for _, id := range tt.expectedIDs {
				if _, ok := found[id]; !ok {
					t.Errorf("Expected task %s was not returned", id)
				}
			}

			status := req.URL.Query().Get("status")
			if status != "" {
				for _, task := range tasks {
					if task.Status != status {
						t.Errorf(
							"Expected status %q, got %q",
							status,
							task.Status,
						)
					}
				}
			}
		})
	}
}

func TestGetTaskByID(t *testing.T) {
	db, cfg := setupTestDB(t)

	id := uuid.New()
	title := "Get task by ID integration test"

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO tasks (id, title, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`,
		id,
		title,
		"Test description",
		"todo",
	)
	if err != nil {
		t.Fatalf("Failed to insert test task: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			"DELETE FROM tasks WHERE id = $1",
			id,
		)
	})

	tests := []struct {
		name           string
		taskID         string
		expectedStatus int
	}{
		{
			name:           "Existing task",
			taskID:         id.String(),
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid UUID",
			taskID:         "abc",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Nonexistent task",
			taskID:         "00000000-0000-4000-8000-000000000000",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodGet,
				"/api/tasks/"+tt.taskID,
				nil,
			)

			req.SetPathValue("taskID", tt.taskID)

			rec := httptest.NewRecorder()
			cfg.handlerGetTaskByID(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Fatalf(
					"Expected HTTP %d, got %d. Body: %s",
					tt.expectedStatus,
					rec.Code,
					rec.Body.String(),
				)
			}

			if tt.expectedStatus != http.StatusOK {
				return
			}

			var task Task
			if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
				t.Fatalf("Invalid JSON response: %v", err)
			}

			if task.ID != id {
				t.Errorf("Expected ID %s, got %s", id, task.ID)
			}
			if task.Title != title {
				t.Errorf("Expected title %q, got %q", title, task.Title)
			}
			if task.Status != "todo" {
				t.Errorf("Expected status todo, got %q", task.Status)
			}
			if task.Description != "Test description" {
				t.Errorf("Unexpected description: %q", task.Description)
			}
			if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
				t.Error("Expected nonzero timestamps")
			}
		})
	}
}

func TestGetTaskByIDDatabaseError(t *testing.T) {
	db, cfg := setupTestDB(t)

	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close DB: %v", err)
	}

	id := uuid.New().String()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/tasks/"+id,
		nil,
	)
	req.SetPathValue("taskID", id)

	rec := httptest.NewRecorder()
	cfg.handlerGetTaskByID(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf(
			"Expected 500, got %d. Body: %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	if strings.Contains(strings.ToLower(rec.Body.String()), "database is closed") {
		t.Error("Response leaks internal database error")
	}
}

func TestGetAllTasksDatabaseError(t *testing.T) {
	db, cfg := setupTestDB(t)

	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close DB: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()

	cfg.handlerGetAlltasks(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf(
			"Expected 500, got %d. Body: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

var _ = sql.ErrNoRows
