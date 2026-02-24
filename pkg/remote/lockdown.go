package remote

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const lockdownFile = "LOCKDOWN_MODE"
const lockdownAuditLog = "terrasign-lockdown-audit.log"

// LockdownState holds info about the current lockdown
type LockdownState struct {
	Active    bool   `json:"active"`
	Identity  string `json:"identity"`
	Timestamp string `json:"timestamp"`
	Reason    string `json:"reason"`
	Hostname  string `json:"hostname"`
}

// readLockdownState loads lockdown info from the state file
func readLockdownState() *LockdownState {
	data, err := os.ReadFile(lockdownFile)
	if err != nil {
		return &LockdownState{Active: false}
	}
	var state LockdownState
	if err := json.Unmarshal(data, &state); err != nil {
		// Legacy plain-text lockdown file
		return &LockdownState{Active: true, Identity: "unknown (legacy)", Timestamp: "unknown"}
	}
	state.Active = true
	return &state
}

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

	// Read caller identity from header (set by CLI after GPG verification)
	identity := r.Header.Get("X-Terrasign-Identity")
	reason := r.Header.Get("X-Terrasign-Reason")
	hostname := r.Header.Get("X-Terrasign-Hostname")
	if identity == "" {
		identity = "unknown"
	}
	if reason == "" {
		reason = "not specified"
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)

	if mode == "on" {
		state := LockdownState{
			Active:    true,
			Identity:  identity,
			Timestamp: timestamp,
			Reason:    reason,
			Hostname:  hostname,
		}
		data, _ := json.MarshalIndent(state, "", "  ")
		if err := os.WriteFile(lockdownFile, data, 0644); err != nil {
			http.Error(w, "Failed to enable lockdown", http.StatusInternalServerError)
			return
		}
		writeServerAuditLog("LOCKDOWN_ACTIVATED", identity, hostname, reason, timestamp)

		banner := fmt.Sprintf(
			"[EMERGENCY LOCKDOWN ACTIVATED]\nTime:     %s\nBy:       %s\nHost:     %s\nReason:   %s",
			timestamp, identity, hostname, reason,
		)
		fmt.Println(strings.Repeat("=", 60))
		fmt.Println(banner)
		fmt.Println(strings.Repeat("=", 60))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    "LOCKDOWN_ACTIVE",
			"identity":  identity,
			"timestamp": timestamp,
			"reason":    reason,
		})
	} else {
		if err := os.Remove(lockdownFile); err != nil && !os.IsNotExist(err) {
			http.Error(w, "Failed to disable lockdown", http.StatusInternalServerError)
			return
		}
		writeServerAuditLog("LOCKDOWN_DEACTIVATED", identity, hostname, reason, timestamp)

		fmt.Printf("[LOCKDOWN LIFTED] By: %s at %s\n", identity, timestamp)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    "LOCKDOWN_LIFTED",
			"identity":  identity,
			"timestamp": timestamp,
		})
	}
}

// isLockdown checks if lockdown is active
func (s *SigningService) isLockdown() bool {
	_, err := os.Stat(lockdownFile)
	return err == nil
}

// checkLockdown middleware to reject requests during lockdown
func (s *SigningService) checkLockdown(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Always allow the lockdown endpoint itself
		if r.URL.Path == "/lockdown" {
			next(w, r)
			return
		}

		if s.isLockdown() {
			state := readLockdownState()
			msg := fmt.Sprintf(
				"EMERGENCY LOCKDOWN ACTIVE - ALL REQUESTS REJECTED\nActivated by: %s\nTime: %s\nReason: %s",
				state.Identity, state.Timestamp, state.Reason,
			)
			http.Error(w, msg, http.StatusServiceUnavailable)
			return
		}

		next(w, r)
	}
}

// writeServerAuditLog appends an entry to the server-side audit log
func writeServerAuditLog(action, identity, hostname, reason, timestamp string) {
	f, err := os.OpenFile(lockdownAuditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[WARN] Could not write server audit log: %v\n", err)
		return
	}
	defer f.Close()

	entry := fmt.Sprintf(
		"[%s] ACTION=%s IDENTITY=%q HOSTNAME=%s REASON=%q\n",
		timestamp, action, identity, hostname, reason,
	)
	_, _ = f.WriteString(entry)
}
