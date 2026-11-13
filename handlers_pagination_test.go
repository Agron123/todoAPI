package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGetAllTasksPaginationValidation(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{"default parameters", "", http.StatusOK},
		{"valid limit and offset", "?limit=10&offset=0", http.StatusOK},
		{"minimum limit", "?limit=1", http.StatusOK},
		{"maximum limit", "?limit=100", http.StatusOK},
		{"zero limit", "?limit=0", http.StatusBadRequest},
		{"negative limit", "?limit=-1", http.StatusBadRequest},
		{"limit above maximum", "?limit=101", http.StatusBadRequest},
		{"invalid limit", "?limit=abc", http.StatusBadRequest},
		{"negative offset", "?offset=-1", http.StatusBadRequest},
		{"invalid offset", "?offset=abc", http.StatusBadRequest},
		{"invalid status", "?status=invalid", http.StatusBadRequest},
		{"invalid limit with status", "?status=done&limit=abc", http.StatusBadRequest},
		{"invalid offset with status", "?status=done&offset=-1", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, cfg := setupTestDB(t)
			req := httptest.NewRequest(http.MethodGet, "/api/tasks"+tt.query, nil)
			rec := httptest.NewRecorder()

			cfg.handlerGetAlltasks(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s",
					rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestGetAllTasksPaginationResults(t *testing.T) {
	_, cfg := setupTestDB(t)

	// Use a unique title prefix so existing database rows do not
	// interfere with the expected ordering.
	prefix := uuid.NewString()
	for i := 0; i < 5; i++ {
		title := fmt.Sprintf("%s-task-%d", prefix, i)
		body := fmt.Sprintf(`{"title":%q,"status":"done"}`, title)

		req := httptest.NewRequest(http.MethodPost, "/api/tasks",
			strings.NewReader(body))
		rec := httptest.NewRecorder()
		cfg.handlerCreateTask(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("create task %d: status = %d, body = %s",
				i, rec.Code, rec.Body.String())
		}
		time.Sleep(time.Millisecond)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/api/tasks?status=done&limit=2&offset=1", nil)
	rec := httptest.NewRecorder()
	cfg.handlerGetAlltasks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s",
			rec.Code, rec.Body.String())
	}

	var tasks []Task
	if err := json.Unmarshal(rec.Body.Bytes(), &tasks); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}
	if tasks[0].Title != prefix+"-task-3" ||
		tasks[1].Title != prefix+"-task-2" {
		t.Fatalf("unexpected pagination order: %q, %q",
			tasks[0].Title, tasks[1].Title)
	}
}
