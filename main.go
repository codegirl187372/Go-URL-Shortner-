package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

type URLRecord struct {
	ShortCode   string    `json:"short_code"`
	OriginalURL string    `json:"original_url"`
	ShortURL    string    `json:"short_url"`
	Clicks      int64     `json:"clicks"`
	CreatedAt   time.Time `json:"created_at"`
	LastAccess  time.Time `json:"last_access,omitempty"`
}

type ShortenRequest struct {
	URL         string `json:"url"`
	CustomAlias string `json:"custom_alias,omitempty"`
}

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type URLStore struct {
	mu      sync.RWMutex
	records map[string]*URLRecord
	baseURL string
}

func NewURLStore(baseURL string) *URLStore {
	return &URLStore{
		records: make(map[string]*URLRecord),
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (s *URLStore) Create(originalURL, customAlias string) (*URLRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	code := customAlias
	if code != "" {
		if _, exists := s.records[code]; exists {
			return nil, errors.New("custom alias is already taken")
		}
	} else {
		var err error
		code, err = generateRandomCode(6)
		if err != nil {
			return nil, errors.New("failed to generate unique code")
		}
	}

	record := &URLRecord{
		ShortCode:   code,
		OriginalURL: originalURL,
		ShortURL:    fmt.Sprintf("%s/%s", s.baseURL, code),
		Clicks:      0,
		CreatedAt:   time.Now().UTC(),
	}

	s.records[code] = record
	return record, nil
}

func (s *URLStore) GetAndIncrement(code string) (*URLRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.records[code]
	if !exists {
		return nil, false
	}

	record.Clicks++
	record.LastAccess = time.Now().UTC()
	return record, true
}

func (s *URLStore) GetStats(code string) (*URLRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[code]
	if !exists {
		return nil, false
	}
	return record, true
}

func (s *URLStore) ListAll() []*URLRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*URLRecord, 0, len(s.records))
	for _, rec := range s.records {
		list = append(list, rec)
	}
	return list
}

func (s *URLStore) Delete(code string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.records[code]; !exists {
		return false
	}
	delete(s.records, code)
	return true
}

func generateRandomCode(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func isValidURL(rawURL string) bool {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func writeJSON(w http.ResponseWriter, status int, payload APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

type responseObserver struct {
	http.ResponseWriter
	statusCode int
}

func (ro *responseObserver) WriteHeader(code int) {
	ro.statusCode = code
	ro.ResponseWriter.WriteHeader(code)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		observer := &responseObserver{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(observer, r)

		duration := time.Since(start)
		log.Printf("[%s] %s %s -> %d (%v)", r.Method, r.URL.Path, r.RemoteAddr, observer.statusCode, duration)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC RECOVERED: %v", err)
				writeJSON(w, http.StatusInternalServerError, APIResponse{
					Success: false,
					Error:   "Internal server error occurred",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type ServerHandler struct {
	store *URLStore
}

func (h *ServerHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "URL Shortener API is healthy and running",
		Data: map[string]interface{}{
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
		},
	})
}

func (h *ServerHandler) ShortenURL(w http.ResponseWriter, r *http.Request) {
	var req ShortenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid JSON request body",
		})
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	req.CustomAlias = strings.TrimSpace(req.CustomAlias)

	if !isValidURL(req.URL) {
		writeJSON(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid URL format. Must start with http:// or https://",
		})
		return
	}

    record, err := h.store.Create(req.URL, req.CustomAlias)
	if err != nil {
		writeJSON(w, http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Message: "Short URL generated successfully",
		Data:    record,
	})
}

func (h *ServerHandler) RedirectURL(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" || code == "favicon.ico" {
		http.NotFound(w, r)
		return
	}

	record, exists := h.store.GetAndIncrement(code)
	if !exists {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Short URL not found",
		})
		return
	}

	http.Redirect(w, r, record.OriginalURL, http.StatusFound)
}

func (h *ServerHandler) GetURLStats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	record, exists := h.store.GetStats(code)
	if !exists {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Analytics not found for the given short code",
		})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    record,
	})
}

func (h *ServerHandler) ListAllURLs(w http.ResponseWriter, r *http.Request) {
	records := h.store.ListAll()
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("Retrieved %d shortened URLs", len(records)),
		Data:    records,
	})
}

func (h *ServerHandler) DeleteURL(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if deleted := h.store.Delete(code); !deleted {
		writeJSON(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "Short URL not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Short URL deleted successfully",
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	baseURL := fmt.Sprintf("http://localhost:%s", port)

	store := NewURLStore(baseURL)
	handler := &ServerHandler{store: store}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handler.HealthCheck)
	mux.HandleFunc("POST /api/shorten", handler.ShortenURL)
	mux.HandleFunc("GET /api/urls", handler.ListAllURLs)
	mux.HandleFunc("GET /api/stats/{code}", handler.GetURLStats)
	mux.HandleFunc("DELETE /api/urls/{code}", handler.DeleteURL)
	mux.HandleFunc("GET /{code}", handler.RedirectURL)

	wrappedRouter := recoveryMiddleware(loggingMiddleware(mux))

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      wrappedRouter,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("🚀 Server starting on %s", baseURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⚠️ Shutting down server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited cleanly")
}
