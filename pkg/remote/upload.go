package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// handleUploadSignature handles signature upload from admin
func (s *SigningService) handleUploadSignature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /upload-signature/{id}
	id := r.URL.Path[len("/upload-signature/"):]
	if id == "" {
		http.Error(w, "Missing submission ID", http.StatusBadRequest)
		return
	}

	// Get submission to verify it exists
	_, err := s.storage.GetSubmission(id)
	if err != nil {
		http.Error(w, "Submission not found", http.StatusNotFound)
		return
	}

	// Save signature file
	sigPath := s.storage.GetSignaturePath(id)
	sigFile, err := os.Create(sigPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create signature file: %v", err), http.StatusInternalServerError)
		return
	}
	defer sigFile.Close()

	if _, err := io.Copy(sigFile, r.Body); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write signature: %v", err), http.StatusInternalServerError)
		return
	}

	// NOTE: We no longer call MarkSigned here. The bundle upload is what triggers AddApproval.
	// This endpoint now just stores the raw .sig file for backward compatibility.
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Signature uploaded successfully for submission %s\n", id)
}

// handleUploadBundle handles bundle upload from admin.
// It records a formal Approval entry for the approver (via ?approver=<name> query param)
// and auto-promotes the plan to "approved" when the approval threshold is reached.
func (s *SigningService) handleUploadBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path: /upload-bundle/{id}
	id := r.URL.Path[len("/upload-bundle/"):]
	if id == "" {
		http.Error(w, "Missing submission ID", http.StatusBadRequest)
		return
	}

	// Get approver name from query param
	approver := r.URL.Query().Get("approver")
	if approver == "" {
		approver = "admin"
	}

	keyHint := r.URL.Query().Get("key_hint")

	// Get submission to verify it exists
	_, err := s.storage.GetSubmission(id)
	if err != nil {
		http.Error(w, "Submission not found", http.StatusNotFound)
		return
	}

	// Save the bundle per-approver so each admin's signature is stored separately
	bundlePath := s.storage.GetBundlePathForApprover(id, approver)
	bundleData, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read bundle: %v", err), http.StatusInternalServerError)
		return
	}

	if err := os.WriteFile(bundlePath, bundleData, 0644); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write bundle: %v", err), http.StatusInternalServerError)
		return
	}

	// Also save as the primary bundle (the last approver's — used by the verifier)
	primaryPath := s.storage.GetBundlePath(id)
	if err := os.WriteFile(primaryPath, bundleData, 0644); err != nil {
		// Not fatal — log and continue
		fmt.Printf("[WARN] Could not write primary bundle: %v\n", err)
	}

	// Compute a SHA-256 fingerprint of the approver's public key.
	// The client sends the raw public key bytes in the X-Public-Key-Content header.
	// This fingerprint is used server-side to detect the same physical key signing twice.
	var keyFingerprint string
	if pubKeyContent := r.Header.Get("X-Public-Key-Content"); pubKeyContent != "" {
		hash := sha256.Sum256([]byte(pubKeyContent))
		keyFingerprint = hex.EncodeToString(hash[:])
	}

	// Record the approval
	approval := Approval{
		Reviewer:       approver,
		ApprovedAt:     time.Now(),
		KeyHint:        keyHint,
		KeyFingerprint: keyFingerprint,
		BundleFile:     filepath.Base(bundlePath),
	}

	submission, err := s.storage.AddApproval(id, approval)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to record approval: %v", err), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"id":%q,"approvals":%d,"threshold":%d,"status":%q}`,
		id, len(submission.Approvals), submission.ApprovalThreshold, submission.Status)
}
