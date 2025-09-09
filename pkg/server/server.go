package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"eth-blockchain-parser/pkg/database"
)

// Server represents the HTTP server with database access
type Server struct {
	dm       *database.DatabaseManager
	txRepo   *database.TransactionRepository
	addrRepo *database.AddressRepository
	logger   *log.Logger
	config   *ServerConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port     string
	Username string
	Password string
	Host     string
}

// DefaultServerConfig returns default server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Port:     "8015",
		Username: "admin",
		Password: "password123", // Change this in production!
		Host:     "localhost",
	}
}

// APIResponse represents a standard API response format
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Count   int         `json:"count,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

// PaginationMeta holds pagination information
type PaginationMeta struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	Total   int  `json:"total"`
	HasNext bool `json:"has_next"`
	HasPrev bool `json:"has_prev"`
}

// NewServer creates a new HTTP server instance
func NewServer(dm *database.DatabaseManager, config *ServerConfig, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}

	if config == nil {
		config = DefaultServerConfig()
	}

	return &Server{
		dm:       dm,
		txRepo:   database.NewTransactionRepository(dm, logger),
		addrRepo: database.NewAddressRepository(dm, logger),
		logger:   logger,
		config:   config,
	}
}

// basicAuth middleware for HTTP Basic Authentication
func (s *Server) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			s.unauthorized(w, "Missing Authorization header")
			return
		}

		// Use constant time comparison to prevent timing attacks
		usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(s.config.Username)) == 1
		passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(s.config.Password)) == 1

		if !usernameMatch || !passwordMatch {
			s.unauthorized(w, "Invalid credentials")
			return
		}

		next(w, r)
	}
}

// unauthorized sends a 401 Unauthorized response
func (s *Server) unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("WWW-Authenticate", `Basic realm="SQLite API"`)
	s.sendError(w, http.StatusUnauthorized, message)
}

// sendJSON sends a JSON response
func (s *Server) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := APIResponse{
		Success: status < 400,
		Data:    data,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Printf("Failed to encode JSON response: %v", err)
	}
}

// sendError sends an error response
func (s *Server) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := APIResponse{
		Success: false,
		Error:   message,
	}

	json.NewEncoder(w).Encode(response)
}

// getAllTransactions handles GET /api/transactions
func (s *Server) getAllTransactions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Parse pagination parameters
	page := s.getIntParam(r, "page", 1)
	limit := s.getIntParam(r, "limit", 100)
	if limit > 1000 {
		limit = 1000 // Maximum limit
	}
	offset := (page - 1) * limit

	// Get transactions with pagination
	db, err := s.dm.DB()
	if err != nil {
		s.sendError(w, http.StatusInternalServerError, "Database connection failed")
		return
	}

	query := `
		SELECT * FROM transactions 
		ORDER BY block_number DESC, transaction_index DESC 
		LIMIT ? OFFSET ?`

	var transactions []*database.Transaction
	err = db.SelectContext(ctx, &transactions, query, limit, offset)
	if err != nil {
		s.logger.Printf("Failed to fetch transactions: %v", err)
		s.sendError(w, http.StatusInternalServerError, "Failed to fetch transactions")
		return
	}

	// Get total count for pagination
	var total int
	err = db.GetContext(ctx, &total, "SELECT COUNT(*) FROM transactions")
	if err != nil {
		s.logger.Printf("Failed to get transaction count: %v", err)
		total = len(transactions) // Fallback
	}

	// Prepare pagination meta
	meta := PaginationMeta{
		Page:    page,
		Limit:   limit,
		Total:   total,
		HasNext: offset+limit < total,
		HasPrev: page > 1,
	}

	// Send response with pagination
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := APIResponse{
		Success: true,
		Data:    transactions,
		Count:   len(transactions),
		Meta:    meta,
	}

	json.NewEncoder(w).Encode(response)
}

// getTransactionByHash handles GET /api/transactions/{hash}
func (s *Server) getTransactionByHash(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Extract hash from URL path
	hash := r.URL.Path[len("/api/transactions/"):]
	if hash == "" {
		s.sendError(w, http.StatusBadRequest, "Transaction hash required")
		return
	}

	transaction, err := s.txRepo.GetByHash(ctx, hash)
	if err != nil {
		s.logger.Printf("Failed to fetch transaction %s: %v", hash, err)
		s.sendError(w, http.StatusInternalServerError, "Failed to fetch transaction")
		return
	}

	if transaction == nil {
		s.sendError(w, http.StatusNotFound, "Transaction not found")
		return
	}

	s.sendJSON(w, http.StatusOK, transaction)
}

// getTransactionsByAddress handles GET /api/addresses/{address}/transactions
func (s *Server) getTransactionsByAddress(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Extract address from URL path
	address := r.URL.Path[len("/api/addresses/"):]
	if idx := len(address) - len("/transactions"); idx > 0 && address[idx:] == "/transactions" {
		address = address[:idx]
	}

	if address == "" {
		s.sendError(w, http.StatusBadRequest, "Address required")
		return
	}

	// Parse pagination
	page := s.getIntParam(r, "page", 1)
	limit := s.getIntParam(r, "limit", 100)
	if limit > 1000 {
		limit = 1000
	}
	offset := (page - 1) * limit

	transactions, err := s.txRepo.GetByAddress(ctx, address, limit, offset)
	if err != nil {
		s.logger.Printf("Failed to fetch transactions for address %s: %v", address, err)
		s.sendError(w, http.StatusInternalServerError, "Failed to fetch transactions")
		return
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"address":      address,
		"transactions": transactions,
		"count":        len(transactions),
		"pagination": map[string]interface{}{
			"page":  page,
			"limit": limit,
		},
	})
}

// healthCheck handles GET /health
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	// Check database connection
	if err := s.dm.Ping(); err != nil {
		s.sendError(w, http.StatusServiceUnavailable, "Database unavailable")
		return
	}

	s.sendJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"version":   "1.0.0",
	})
}

// getIntParam extracts integer parameter from query string with default value
func (s *Server) getIntParam(r *http.Request, param string, defaultValue int) int {
	str := r.URL.Query().Get(param)
	if str == "" {
		return defaultValue
	}

	if val, err := strconv.Atoi(str); err == nil && val > 0 {
		return val
	}

	return defaultValue
}

// setupRoutes configures HTTP routes
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Public health check (no auth required)
	mux.HandleFunc("/health", s.healthCheck)

	// Protected API endpoints (require authentication)
	mux.HandleFunc("/api/transactions", s.basicAuth(s.getAllTransactions))
	mux.HandleFunc("/api/transactions/", s.basicAuth(s.getTransactionByHash))
	mux.HandleFunc("/api/addresses/", s.basicAuth(s.getTransactionsByAddress))

	// API documentation endpoint
	mux.HandleFunc("/api", s.basicAuth(s.apiDocs))

	return mux
}

// apiDocs provides API documentation
func (s *Server) apiDocs(w http.ResponseWriter, r *http.Request) {
	docs := map[string]interface{}{
		"title":   "SQLite Blockchain API",
		"version": "1.0.0",
		"endpoints": map[string]interface{}{
			"GET /health":                               "Health check (no auth required)",
			"GET /api/transactions":                     "Get all transactions with pagination (?page=1&limit=100)",
			"GET /api/transactions/{hash}":              "Get transaction by hash",
			"GET /api/addresses/{address}/transactions": "Get transactions for specific address",
		},
		"authentication": "Basic HTTP Authentication required for /api/* endpoints",
		"pagination":     "Use ?page=X&limit=Y query parameters",
		"limits": map[string]interface{}{
			"transactions_max_limit": 1000,
		},
	}

	s.sendJSON(w, http.StatusOK, docs)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := s.setupRoutes()

	// Add request logging middleware
	handler := s.loggingMiddleware(mux)

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", s.config.Host, s.config.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Printf("Starting HTTP server on http://%s:%s", s.config.Host, s.config.Port)
	s.logger.Printf("API endpoints available at /api (Basic Auth required)")
	s.logger.Printf("Health check available at /health (no auth required)")
	s.logger.Printf("Username: %s, Password: %s", s.config.Username, s.config.Password)

	return server.ListenAndServe()
}

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a wrapper to capture the status code
		wrapper := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)
		s.logger.Printf("%s %s %d %v %s", r.Method, r.URL.Path, wrapper.statusCode, duration, r.RemoteAddr)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
