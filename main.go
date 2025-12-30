package main

import (
	"embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
)

//go:embed web/index.html web/favicon.svg
var content embed.FS

// ConvertRequest represents the API request body
type ConvertRequest struct {
	SQL    string `json:"sql"`
	Config Config `json:"config"`
}

// ConvertResponse represents the API response body
type ConvertResponse struct {
	Code  string `json:"code,omitempty"`
	Error string `json:"error,omitempty"`
}

func main() {
	// Serve embedded HTML at root
	http.HandleFunc("/", serveIndex)

	// Serve favicon
	http.HandleFunc("/favicon.svg", serveFavicon)

	// API endpoint for conversion
	http.HandleFunc("/api/convert", handleConvert)

	port := ":7860"
	log.Printf("🚀 SQL to Go Converter server starting on http://localhost%s", port)
	log.Fatal(http.ListenAndServe(port, nil))
}

// serveIndex serves the embedded index.html
func serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data, err := content.ReadFile("web/index.html")
	if err != nil {
		http.Error(w, "Failed to load page", http.StatusInternalServerError)
		log.Printf("Error reading index.html: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

// serveFavicon serves the embedded favicon.svg
func serveFavicon(w http.ResponseWriter, r *http.Request) {
	data, err := content.ReadFile("web/favicon.svg")
	if err != nil {
		http.NotFound(w, r)
		log.Printf("Error reading favicon.svg: %v", err)
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year
	w.Write(data)
}

// handleConvert handles POST /api/convert
func handleConvert(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for local development
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only accept POST
	if r.Method != "POST" {
		sendError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read and decode request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req ConvertRequest
	if err := json.Unmarshal(body, &req); err != nil {
		sendError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate SQL is not empty
	if strings.TrimSpace(req.SQL) == "" {
		sendError(w, "SQL cannot be empty", http.StatusBadRequest)
		return
	}

	// Parse SQL
	structs, err := ParseSQL(req.SQL)
	if err != nil {
		sendError(w, "SQL parsing error: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Generate Go code
	code := GenerateGoCode(structs, req.Config)

	// Send success response
	response := ConvertResponse{
		Code: code,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// sendError sends an error response
func sendError(w http.ResponseWriter, message string, statusCode int) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ConvertResponse{
		Error: message,
	})
}
