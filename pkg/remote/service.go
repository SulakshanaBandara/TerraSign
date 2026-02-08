package remote

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// SigningService is the HTTP service for remote plan signing
type SigningService struct {
	storage *Storage
	config  SigningServiceConfig
}

// NewSigningService creates a new signing service
func NewSigningService(config SigningServiceConfig) (*SigningService, error) {
	storage, err := NewStorage(config.StorageDir)
	if err != nil {
		return nil, err
	}

	return &SigningService{
		storage: storage,
		config:  config,
	}, nil
}

// Start starts the HTTP server
func (s *SigningService) Start() error {
	http.HandleFunc("/submit", s.checkLockdown(s.handleSubmit))
	http.HandleFunc("/status/", s.checkLockdown(s.handleStatus))
	http.HandleFunc("/download/", s.checkLockdown(s.handleDownload))
	http.HandleFunc("/list-pending", s.checkLockdown(s.handleListPending))
	http.HandleFunc("/upload-signature/", s.checkLockdown(s.handleUploadSignature))
	http.HandleFunc("/lockdown", s.handleLockdown) // No middleware for lockdown handler

	addr := fmt.Sprintf(":%d", s.config.Port)
	fmt.Printf("Starting signing service on %s\n", addr)
	fmt.Printf("Storage directory: %s\n", s.config.StorageDir)
	return http.ListenAndServe(addr, nil)
}

// handleSubmit handles plan submission from CI
func (s *SigningService) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get submitter from header or query param
	submitter := r.Header.Get("X-Submitter")
	if submitter == "" {
		submitter = r.URL.Query().Get("submitter")
	}
	if submitter == "" {
		submitter = "unknown"
	}

	// Store the plan
	submission, err := s.storage.StorePlan(r.Body, submitter)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to store plan: %v", err), http.StatusInternalServerError)
		return
	}

	// Return submission ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":     submission.ID,
		"status": submission.Status,
	})
}

// handleStatus returns the status of a submission
func (s *SigningService) handleStatus(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/status/"):]
	if id == "" {
		http.Error(w, "Missing submission ID", http.StatusBadRequest)
		return
	}

	submission, err := s.storage.GetSubmission(id)
	if err != nil {
		http.Error(w, "Submission not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(submission)
}

// handleDownload allows admins to download plans and signatures
func (s *SigningService) handleDownload(w http.ResponseWriter, r *http.Request) {
	// Format: /download/{id}/{file}
	// Format: /download/{id}/{file}
	path := r.URL.Path[len("/download/"):]
	
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	
	id := parts[0]
	fileType := parts[1]

	if id == "" || fileType == "" {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	var filePath string
	switch fileType {
	case "plan":
		filePath = s.storage.GetPlanPath(id)
	case "signature":
		filePath = s.storage.GetSignaturePath(id)
	default:
		http.Error(w, "Invalid file type", http.StatusBadRequest)
		return
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, filePath)
}

// handleListPending returns all pending submissions
func (s *SigningService) handleListPending(w http.ResponseWriter, r *http.Request) {
	pending, err := s.storage.ListPending()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list pending: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pending)
}

// MarkSigned marks a submission as signed (called after admin signs)
func (s *SigningService) MarkSigned(id, reviewer string) error {
	submission, err := s.storage.GetSubmission(id)
	if err != nil {
		return err
	}

	now := time.Now()
	submission.Status = "approved"
	submission.ReviewedBy = reviewer
	submission.ReviewedAt = &now
	submission.SignedAt = &now

	return s.storage.UpdateSubmission(submission)
}
