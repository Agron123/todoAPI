package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestUpdateTaskValidation(t *testing.T) {
	tests := []struct {
		name       string
		taskID     string
		body       string
		wantStatus int
	}{
		{
			name:       "Invalid UUID",
			taskID:     "invalid-id",
			body:       `{"title":"Test","status":"todo"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Empty title",
			body:       `{"title":"","status":"todo"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Whitespace title",
			body:       `{"title":"   ","status":"todo"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Invalid status",
			body:       `{"title":"Test","status":"invalid"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Missing status",
			body:       `{"title":"Test"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Missing title",
			body:       `{"status":"todo"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Malformed JSON",
			body:       `{"title":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Multiple JSON objects",
			body:       `{"title":"Test","status":"todo"} {"extra":true}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Invalid JSON field type",
			body:       `{"title":123,"status":"todo"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "NUL in title",
			body:       `{"title":"Bad\u0000title","status":"todo"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "NUL in description",
			body:       `{"title":"Test","status":"todo","description":"Bad\u0000description"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskID := tt.taskID
			if taskID == "" {
				taskID = uuid.NewString()
			}

			req := httptest.NewRequest(
				http.MethodPut,
				"/api/tasks/"+taskID,
				strings.NewReader(tt.body),
			)
			req.SetPathValue("taskID", taskID)

			rec := httptest.NewRecorder()

			cfg := &apiConfig{}
			cfg.handlerUpdateTaskByID(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf(
					"Expected HTTP %d, got %d. Body: %s",
					tt.wantStatus,
					rec.Code,
					rec.Body.String(),
				)
			}
		})
	}
}

func TestUpdateTaskBodyTooLarge(t *testing.T) {
	taskID := uuid.NewString()

	body := `{"title":"` +
		strings.Repeat("A", 1<<20) +
		`","status":"todo"}`

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/tasks/"+taskID,
		strings.NewReader(body),
	)
	req.SetPathValue("taskID", taskID)

	rec := httptest.NewRecorder()
	cfg := &apiConfig{}

	cfg.handlerUpdateTaskByID(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf(
			"Expected HTTP 413, got %d. Body: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

func TestUpdateTaskSuccess(t *testing.T) {
	db, cfg := setupTestDB(t)

	taskID := uuid.New()
	originalTitle := "Original task"
	originalDescription := "Original description"

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO tasks (id, title, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`,
		taskID,
		originalTitle,
		originalDescription,
		"todo",
	)
	if err != nil {
		t.Fatalf("Failed to insert task: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			"DELETE FROM tasks WHERE id = $1",
			taskID,
		)
	})

	var originalCreatedAt time.Time
	err = db.QueryRowContext(
		context.Background(),
		"SELECT created_at FROM tasks WHERE id = $1",
		taskID,
	).Scan(&originalCreatedAt)
	if err != nil {
		t.Fatalf("Failed to read original timestamp: %v", err)
	}

	tests := []struct {
		name              string
		body              string
		wantTitle         string
		wantStatus        string
		wantDescription   string
		wantDescriptionDB sql.NullString
	}{
		{
			name: "Update all fields",
			body: `{
				"title":"Updated task",
				"description":"Updated description",
				"status":"in_progress"
			}`,
			wantTitle:       "Updated task",
			wantStatus:      "in_progress",
			wantDescription: "Updated description",
			wantDescriptionDB: sql.NullString{
				String: "Updated description",
				Valid:  true,
			},
		},
		{
			name: "Omitted description becomes NULL",
			body: `{
				"title":"Task without description",
				"status":"done"
			}`,
			wantTitle:         "Task without description",
			wantStatus:        "done",
			wantDescription:   "",
			wantDescriptionDB: sql.NullString{Valid: false},
		},
		{
			name: "Explicit null description",
			body: `{
				"title":"Task with null description",
				"description":null,
				"status":"todo"
			}`,
			wantTitle:         "Task with null description",
			wantStatus:        "todo",
			wantDescription:   "",
			wantDescriptionDB: sql.NullString{Valid: false},
		},
		{
			name: "Empty description remains empty string",
			body: `{
				"title":"Task with empty description",
				"description":"",
				"status":"in_progress"
			}`,
			wantTitle:       "Task with empty description",
			wantStatus:      "in_progress",
			wantDescription: "",
			wantDescriptionDB: sql.NullString{
				String: "",
				Valid:  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPut,
				"/api/tasks/"+taskID.String(),
				strings.NewReader(tt.body),
			)
			req.SetPathValue("taskID", taskID.String())

			rec := httptest.NewRecorder()
			cfg.handlerUpdateTaskByID(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf(
					"Expected 200, got %d. Body: %s",
					rec.Code,
					rec.Body.String(),
				)
			}

			var task Task
			if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
				t.Fatalf("Invalid JSON response: %v", err)
			}

			if task.ID != taskID {
				t.Errorf("Expected ID %s, got %s", taskID, task.ID)
			}
			if task.Title != tt.wantTitle {
				t.Errorf("Expected title %q, got %q", tt.wantTitle, task.Title)
			}
			if task.Status != tt.wantStatus {
				t.Errorf("Expected status %q, got %q", tt.wantStatus, task.Status)
			}
			if task.Description != tt.wantDescription {
				t.Errorf(
					"Expected description %q, got %q",
					tt.wantDescription,
					task.Description,
				)
			}
			if !task.CreatedAt.Equal(originalCreatedAt) {
				t.Error("created_at changed during update")
			}
			if task.UpdatedAt.IsZero() {
				t.Error("updated_at is zero")
			}

			var dbTitle, dbStatus string
			var dbDescription sql.NullString
			var dbCreatedAt, dbUpdatedAt time.Time

			err := db.QueryRowContext(
				context.Background(),
				`SELECT title, status, description, created_at, updated_at
				 FROM tasks WHERE id = $1`,
				taskID,
			).Scan(
				&dbTitle,
				&dbStatus,
				&dbDescription,
				&dbCreatedAt,
				&dbUpdatedAt,
			)
			if err != nil {
				t.Fatalf("Failed to verify DB record: %v", err)
			}

			if dbTitle != tt.wantTitle || dbStatus != tt.wantStatus {
				t.Error("Database contains unexpected title or status")
			}
			if dbDescription.Valid != tt.wantDescriptionDB.Valid ||
				(dbDescription.Valid &&
					dbDescription.String != tt.wantDescriptionDB.String) {
				t.Errorf(
					"Unexpected DB description: %+v",
					dbDescription,
				)
			}
			if !dbCreatedAt.Equal(originalCreatedAt) {
				t.Error("DB created_at changed")
			}
			if !task.UpdatedAt.Equal(dbUpdatedAt) {
				t.Error("Response updated_at differs from DB")
			}
		})
	}
}

func TestUpdateTaskNotFound(t *testing.T) {
	_, cfg := setupTestDB(t)

	taskID := uuid.NewString()
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/tasks/"+taskID,
		strings.NewReader(`{"title":"Test","status":"todo"}`),
	)
	req.SetPathValue("taskID", taskID)

	rec := httptest.NewRecorder()
	cfg.handlerUpdateTaskByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf(
			"Expected 404, got %d. Body: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

func TestUpdateTaskDatabaseError(t *testing.T) {
	db, cfg := setupTestDB(t)

	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close DB: %v", err)
	}

	taskID := uuid.NewString()
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/tasks/"+taskID,
		strings.NewReader(`{"title":"Test","status":"todo"}`),
	)
	req.SetPathValue("taskID", taskID)

	rec := httptest.NewRecorder()
	cfg.handlerUpdateTaskByID(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf(
			"Expected 500, got %d. Body: %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	if strings.Contains(
		strings.ToLower(rec.Body.String()),
		"database is closed",
	) {
		t.Error("Response leaks internal DB error")
	}
}
