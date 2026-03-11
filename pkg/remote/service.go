package remote

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
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
	http.HandleFunc("/approvals/", s.checkLockdown(s.handleGetApprovals))
	http.HandleFunc("/upload-signature/", s.checkLockdown(s.handleUploadSignature))
	http.HandleFunc("/upload-bundle/", s.checkLockdown(s.handleUploadBundle))
	http.HandleFunc("/reject/", s.checkLockdown(s.handleReject))
	http.HandleFunc("/policy", s.handlePolicy)     // Admin-only: get/set global approval policy
	http.HandleFunc("/lockdown", s.handleLockdown) // No middleware for lockdown handler

	addr := fmt.Sprintf(":%d", s.config.Port)
	fmt.Printf("Starting signing service on %s\n", addr)
	fmt.Printf("Storage directory: %s\n", s.config.StorageDir)

	// Print current policy at startup
	if policy, err := s.storage.GetGlobalPolicy(); err == nil {
		fmt.Printf("Global approval threshold: %d\n", policy.ApprovalThreshold)
	}

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

	// Always use global server policy as the approval threshold.
	// Clients cannot lower the requirement; they can only request a higher one.
	policy, err := s.storage.GetGlobalPolicy()
	if err != nil {
		policy = &GlobalPolicy{ApprovalThreshold: 1}
	}
	threshold := policy.ApprovalThreshold

	// Allow caller to request a higher threshold only (cannot downgrade policy)
	if t := r.URL.Query().Get("threshold"); t != "" {
		if n, err2 := strconv.Atoi(t); err2 == nil && n > threshold {
			threshold = n
		}
	}

	// Store the plan
	submission, err2 := s.storage.StorePlan(r.Body, submitter, threshold)
	if err2 != nil {
		http.Error(w, fmt.Sprintf("Failed to store plan: %v", err2), http.StatusInternalServerError)
		return
	}

	fmt.Printf("[SUBMIT] Plan from %s — requires %d approval(s) (global policy)\n", submitter, threshold)

	// Return submission ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                 submission.ID,
		"status":             submission.Status,
		"approval_threshold": submission.ApprovalThreshold,
	})
}

// handlePolicy handles GET (read) and POST (set) for the global approval policy
func (s *SigningService) handlePolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		policy, err := s.storage.GetGlobalPolicy()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read policy: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(policy)

	case http.MethodPost:
		var req struct {
			Threshold int    `json:"threshold"`
			SetBy     string `json:"set_by"`
			Reason    string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}
		if req.Threshold < 1 {
			http.Error(w, "threshold must be >= 1", http.StatusBadRequest)
			return
		}
		if req.SetBy == "" {
			req.SetBy = "admin"
		}
		newPolicy := &GlobalPolicy{
			ApprovalThreshold: req.Threshold,
			SetBy:             req.SetBy,
			SetAt:             time.Now().Format(time.RFC3339),
			Reason:            req.Reason,
		}
		if err := s.storage.SetGlobalPolicy(newPolicy); err != nil {
			http.Error(w, fmt.Sprintf("Failed to set policy: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(newPolicy)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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
	case "bundle":
		filePath = s.storage.GetBundlePath(id)
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

// handleGetApprovals returns approval status for a specific submission
func (s *SigningService) handleGetApprovals(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/approvals/"):]
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                 submission.ID,
		"status":             submission.Status,
		"approval_threshold": submission.ApprovalThreshold,
		"approvals":          submission.Approvals,
		"approval_count":     len(submission.Approvals),
		"is_fully_approved":  submission.IsFullyApproved(),
	})
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

// handleReject handles admin rejection of a plan submission
func (s *SigningService) handleReject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /reject/{id}
	id := r.URL.Path[len("/reject/"):]
	if id == "" {
		http.Error(w, "Missing submission ID", http.StatusBadRequest)
		return
	}

	reason := r.URL.Query().Get("reason")
	reviewer := r.URL.Query().Get("reviewer")
	if reviewer == "" {
		reviewer = "admin"
	}
	if reason == "" {
		reason = "rejected by admin"
	}

	submission, err := s.storage.GetSubmission(id)
	if err != nil {
		http.Error(w, "Submission not found", http.StatusNotFound)
		return
	}

	if submission.Status != "pending" {
		http.Error(w, fmt.Sprintf("Submission is already %s", submission.Status), http.StatusConflict)
		return
	}

	now := time.Now()
	submission.Status = "rejected"
	submission.ReviewedBy = reviewer
	submission.ReviewedAt = &now
	submission.RejectionReason = reason

	if err := s.storage.UpdateSubmission(submission); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update submission: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Printf("[REJECTED] Plan %s rejected by %s. Reason: %s\n", id, reviewer, reason)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":     id,
		"status": "rejected",
		"reason": reason,
	})
}
