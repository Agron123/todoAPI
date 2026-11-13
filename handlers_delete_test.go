package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestDeleteTaskInvalidUUID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"Invalid UUID", "abc"},
		{"Empty UUID", ""},
		{"Malformed UUID", "123-456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodDelete,
				"/api/tasks/"+tt.id,
				nil,
			)
			req.SetPathValue("taskID", tt.id)

			rec := httptest.NewRecorder()
			cfg := &apiConfig{}

			cfg.handlerDeleteTaskByID(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf(
					"Expected 400, got %d. Body: %s",
					rec.Code,
					rec.Body.String(),
				)
			}
		})
	}
}

func TestDeleteTaskSuccess(t *testing.T) {
	db, cfg := setupTestDB(t)

	taskID := uuid.New()

	_, err := db.ExecContext(
		context.Background(),
		`INSERT INTO tasks
			(id, title, description, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, NOW(), NOW())`,
		taskID,
		"Task to delete",
		"DELETE integration test",
		"todo",
	)
	if err != nil {
		t.Fatalf("Failed to insert test task: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(
			context.Background(),
			"DELETE FROM tasks WHERE id = $1",
			taskID,
		)
	})

	deleteTask := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(
			http.MethodDelete,
			"/api/tasks/"+taskID.String(),
			nil,
		)
		req.SetPathValue("taskID", taskID.String())

		rec := httptest.NewRecorder()
		cfg.handlerDeleteTaskByID(rec, req)
		return rec
	}

	rec := deleteTask()

	if rec.Code != http.StatusNoContent {
		t.Fatalf(
			"Expected 204, got %d. Body: %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	if rec.Body.Len() != 0 {
		t.Errorf(
			"Expected empty response body, got %q",
			rec.Body.String(),
		)
	}

	var count int
	err = db.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM tasks WHERE id = $1",
		taskID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check database: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 matching tasks, got %d", count)
	}

	rec = deleteTask()

	if rec.Code != http.StatusNotFound {
		t.Errorf(
			"Expected 404 on second DELETE, got %d. Body: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	_, cfg := setupTestDB(t)

	taskID := uuid.NewString()

	req := httptest.NewRequest(
		http.MethodDelete,
		"/api/tasks/"+taskID,
		nil,
	)
	req.SetPathValue("taskID", taskID)

	rec := httptest.NewRecorder()
	cfg.handlerDeleteTaskByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf(
			"Expected 404, got %d. Body: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

func TestDeleteTaskDatabaseError(t *testing.T) {
	db, cfg := setupTestDB(t)

	if err := db.Close(); err != nil {
		t.Fatalf("Failed to close database: %v", err)
	}

	taskID := uuid.NewString()

	req := httptest.NewRequest(
		http.MethodDelete,
		"/api/tasks/"+taskID,
		nil,
	)
	req.SetPathValue("taskID", taskID)

	rec := httptest.NewRecorder()
	cfg.handlerDeleteTaskByID(rec, req)

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
		t.Error("Response exposes internal database error")
	}
}

func TestDeleteTaskNotFoundJSON(t *testing.T) {
	_, cfg := setupTestDB(t)

	taskID := uuid.NewString()

	req := httptest.NewRequest(
		http.MethodDelete,
		"/api/tasks/"+taskID,
		nil,
	)
	req.SetPathValue("taskID", taskID)

	rec := httptest.NewRecorder()
	cfg.handlerDeleteTaskByID(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d", rec.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Expected valid JSON error response: %v", err)
	}
}
