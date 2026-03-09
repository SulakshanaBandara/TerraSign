package remote

import (
	"fmt"
	"io"
	"net/http"
	"os"
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

	// Mark as signed
	if err := s.MarkSigned(id, "admin"); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update status: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Signature uploaded successfully for submission %s\n", id)
}

// handleUploadBundle handles bundle upload from admin
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

	// Get submission to verify it exists
	_, err := s.storage.GetSubmission(id)
	if err != nil {
		http.Error(w, "Submission not found", http.StatusNotFound)
		return
	}

	// Save bundle file
	bundlePath := s.storage.GetBundlePath(id)
	bundleFile, err := os.Create(bundlePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create bundle file: %v", err), http.StatusInternalServerError)
		return
	}
	defer bundleFile.Close()

	if _, err := io.Copy(bundleFile, r.Body); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write bundle: %v", err), http.StatusInternalServerError)
		return
	}

	// We do NOT mark signed here, because handleUploadSignature already does that,
	// or we can allow either one to mark it signed. We will call MarkSigned anyway just in case.
	if err := s.MarkSigned(id, "admin"); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update status: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Bundle uploaded successfully for submission %s\n", id)
}
