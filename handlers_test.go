package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMultipleCases(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		expectedStatus int
	}{
		{
			name:           "Empty title",
			body:           `{"title":"","status":"todo"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "NUL in title",
			body:           `{"title":"darbas\u0000","status":"todo"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "NUL in description",
			body:           `{"title":"darbas","description":"opa\u0000","status":"todo"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid status",
			body:           `{"title":"darbas","description":"opa","status":"invalid"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Whitespace title",
			body:           `{"title":"    ","description":"opa","status":"todo"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Malformed JSON",
			body:           `{"title":`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty body",
			body:           ``,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Multiple JSON objects",
			body:           `{"title":"darbas","status":"todo"} {"title":"kitas","status":"done"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid JSON type",
			body:           `{"title":123,"status":"todo"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing status",
			body:           `{"title":"darbas"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing title",
			body:           `{"status":"todo"}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &apiConfig{}
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/tasks",
				strings.NewReader(tc.body),
			)
			rec := httptest.NewRecorder()
			cfg.handlerCreateTask(rec, req)
			result := rec.Result()
			defer result.Body.Close()

			if result.StatusCode != tc.expectedStatus {
				t.Errorf("Expected %v, got %d", tc.expectedStatus, result.StatusCode)
			}
		})
	}
}

func TestCreateTaskBodyTooLarge(t *testing.T) {
	cfg := &apiConfig{}

	body := `{"title":"` + strings.Repeat("a", 1<<20) + `","status":"todo"}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/tasks",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()
	cfg.handlerCreateTask(rec, req)

	result := rec.Result()
	defer result.Body.Close()

	if result.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf(
			"Expected %d, got %d",
			http.StatusRequestEntityTooLarge,
			result.StatusCode,
		)
	}
}
