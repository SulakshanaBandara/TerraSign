package remote

import (
	"fmt"
	"net/http"
	"os"
)

// handleLockdown toggles lockdown mode
func (s *SigningService) handleLockdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mode := r.URL.Query().Get("mode")
	if mode != "on" && mode != "off" {
		http.Error(w, "Invalid mode (use 'on' or 'off')", http.StatusBadRequest)
		return
	}

	lockdownFile := "LOCKDOWN_MODE"
	if mode == "on" {
		// Create lockdown file
		if err := os.WriteFile(lockdownFile, []byte("EMERGENCY LOCKDOWN"), 0644); err != nil {
			http.Error(w, "Failed to enable lockdown", http.StatusInternalServerError)
			return
		}
		fmt.Println("[EMERGENCY LOCKDOWN ENABLED]")
	} else {
		// Remove lockdown file
		if err := os.Remove(lockdownFile); err != nil && !os.IsNotExist(err) {
			http.Error(w, "Failed to disable lockdown", http.StatusInternalServerError)
			return
		}
		fmt.Println("[LOCKDOWN DISABLED]")
	}

	w.WriteHeader(http.StatusOK)
}

// isLockdown checks if lockdown is active
func (s *SigningService) isLockdown() bool {
	_, err := os.Stat("LOCKDOWN_MODE")
	return err == nil
}

// checkLockdown middleware to reject requests during lockdown
func (s *SigningService) checkLockdown(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Allow checking/disabling lockdown
		if r.URL.Path == "/lockdown" {
			next(w, r)
			return
		}

		if s.isLockdown() {
			http.Error(w, "EMERGENCY LOCKDOWN ACTIVE - REJECTING ALL REQUESTS", http.StatusServiceUnavailable)
			return
		}

		next(w, r)
	}
}
