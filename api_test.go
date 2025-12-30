package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIConvert_Success(t *testing.T) {
	req := ConvertRequest{
		SQL: "CREATE TABLE users (id INT NOT NULL, name VARCHAR(255) NOT NULL)",
		Config: Config{
			AddJSONTag: true,
		},
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleConvert(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp ConvertResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Error != "" {
		t.Errorf("Expected no error, got: %s", resp.Error)
	}

	if resp.Code == "" {
		t.Error("Expected generated code, got empty string")
	}

	if !contains(resp.Code, "type Users struct") {
		t.Error("Expected struct definition in generated code")
	}
}

func TestAPIConvert_InvalidSQL(t *testing.T) {
	req := ConvertRequest{
		SQL: "Halo ini bukan SQL",
		Config: Config{
			AddJSONTag: true,
		},
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleConvert(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp ConvertResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Error == "" {
		t.Error("Expected error message, got empty string")
	}

	if !contains(resp.Error, "SQL parsing error") {
		t.Errorf("Expected 'SQL parsing error' in error message, got: %s", resp.Error)
	}

	fmt.Printf("✅ Error message returned: %s\n", resp.Error)
}

func TestAPIConvert_EmptySQL(t *testing.T) {
	req := ConvertRequest{
		SQL: "",
		Config: Config{
			AddJSONTag: true,
		},
	}

	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleConvert(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp ConvertResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Error == "" {
		t.Error("Expected error message for empty SQL")
	}
}

func TestAPIConvert_InvalidJSON(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/convert", bytes.NewReader([]byte("invalid json")))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handleConvert(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp ConvertResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Error == "" {
		t.Error("Expected error message for invalid JSON")
	}
}

func TestAPIConvert_WrongMethod(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/convert", nil)
	w := httptest.NewRecorder()

	handleConvert(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && stringContains(s, substr)
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
