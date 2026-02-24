package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sulakshanakarunarathne/terrasign/pkg/remote"
)

const lockdownAuditLog = "terrasign-lockdown-audit.log"

// getLockdownIdentity returns the GPG identity for the given key ID.
// It parses the GPG colon-format output where uid field is at index 9.
func getLockdownIdentity(gpgKeyID string) (string, error) {
	args := []string{"--keyid-format", "long", "--with-colons", "--list-secret-keys"}
	if gpgKeyID != "" {
		args = append(args, gpgKeyID)
	}
	cmd := exec.Command("gpg", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("key '%s' not found in GPG keyring", gpgKeyID)
	}

	// GPG colon format: uid lines have this structure:
	// uid:validity:creation:expiration:hash:owner_trust:name_email::::
	// The name+email is at field index 9 (0-based)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "uid:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 10 && parts[9] != "" {
				return parts[9], nil
			}
		}
	}
	return "", fmt.Errorf("no identity (uid) found for key '%s'", gpgKeyID)
}

// verifyGPGSignedRequest confirms the key exists in the keyring and returns its identity
func verifyGPGSignedRequest(gpgKeyID string) (string, error) {
	// Check key exists first with a simple list command
	checkCmd := exec.Command("gpg", "--list-secret-keys", gpgKeyID)
	if err := checkCmd.Run(); err != nil {
		return "", fmt.Errorf("GPG key '%s' not found in keyring — run: gpg --list-secret-keys --keyid-format LONG", gpgKeyID)
	}

	// Now get the identity
	identity, err := getLockdownIdentity(gpgKeyID)
	if err != nil {
		return "", err
	}
	return identity, nil
}

// writeLockdownAuditLog appends a structured entry to the audit log
func writeLockdownAuditLog(action, identity, reason string, success bool) {
	f, err := os.OpenFile(lockdownAuditLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[WARN] Could not write audit log: %v\n", err)
		return
	}
	defer f.Close()

	timestamp := time.Now().UTC().Format(time.RFC3339)
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}

	hostname, _ := os.Hostname()

	entry := fmt.Sprintf(
		"[%s] ACTION=%s STATUS=%s IDENTITY=%q HOSTNAME=%s REASON=%q\n",
		timestamp, action, status, identity, hostname, reason,
	)
	_, _ = f.WriteString(entry)
}

func handleLockdown() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: terrasign lockdown [on|off] [-k <key-id>] [-r <reason>] [--service <url>] [--recovery-code <code>]")
		os.Exit(1)
	}

	mode := os.Args[2]
	if mode != "on" && mode != "off" {
		fmt.Println("Error: mode must be 'on' or 'off'")
		os.Exit(1)
	}

	serviceURL := defaultServiceURL
	gpgKeyID := ""
	recoveryCode := ""
	reason := "Not specified"
	verifiedIdentity := "unknown"

	for i, arg := range os.Args {
		if arg == "--service" && i+1 < len(os.Args) {
			serviceURL = os.Args[i+1]
		}
		// --key and -k both accepted
		if (arg == "--key" || arg == "-k") && i+1 < len(os.Args) {
			gpgKeyID = os.Args[i+1]
		}
		if arg == "--recovery-code" && i+1 < len(os.Args) {
			recoveryCode = os.Args[i+1]
		}
		// --reason and -r both accepted
		if (arg == "--reason" || arg == "-r") && i+1 < len(os.Args) {
			reason = os.Args[i+1]
		}
	}

	// -------------------------------------------------------
	// LOCKDOWN ON: Require GPG authentication
	// -------------------------------------------------------
	if mode == "on" {
		if gpgKeyID == "" {
			fmt.Println("\n[ERROR] Lockdown activation requires your key!")
			fmt.Println("\nUsage:")
			fmt.Println("  terrasign lockdown on -k <gpg-key-id> -r \"<reason>\"")
			fmt.Println("\nExample:")
			fmt.Println("  terrasign lockdown on -k A1B2C3D4E5F6 -r \"Suspected compromised credentials\"")
			fmt.Println("\nTo find your key ID: gpg --list-secret-keys --keyid-format LONG")
			writeLockdownAuditLog("LOCKDOWN_ON_ATTEMPT", "UNKNOWN", reason, false)
			os.Exit(1)
		}

		fmt.Printf("\nVerifying GPG identity for key: %s ...\n", gpgKeyID)
		identity, err := verifyGPGSignedRequest(gpgKeyID)
		if err != nil {
			fmt.Printf("\n[ERROR] GPG verification failed: %v\n", err)
			fmt.Println("Lockdown NOT activated.")
			writeLockdownAuditLog("LOCKDOWN_ON_ATTEMPT", gpgKeyID, reason, false)
			os.Exit(1)
		}

		fmt.Printf("[OK] GPG identity confirmed: %s\n", identity)
		fmt.Printf("     Reason recorded: %s\n\n", reason)

		// Write audit log BEFORE activating
		writeLockdownAuditLog("LOCKDOWN_ACTIVATED", identity, reason, true)
		verifiedIdentity = identity
	}

	// -------------------------------------------------------
	// LOCKDOWN OFF: Require GPG key OR recovery code
	// -------------------------------------------------------
	if mode == "off" {
		if gpgKeyID == "" && recoveryCode == "" {
			fmt.Println("\n[ERROR] Lockdown deactivation requires authentication!")
			fmt.Println("\nOptions:")
			fmt.Println("  1. Use admin key file:  terrasign lockdown off -k <path/to/admin.key>")
			fmt.Println("  2. Use recovery code:   terrasign lockdown off --recovery-code TERRASIGN-EMERGENCY")
			writeLockdownAuditLog("LOCKDOWN_OFF_ATTEMPT", "UNKNOWN", reason, false)
			os.Exit(1)
		}

		if recoveryCode != "" {
			if recoveryCode != "TERRASIGN-EMERGENCY" {
				fmt.Println("\n[ERROR] Invalid recovery code!")
				writeLockdownAuditLog("LOCKDOWN_OFF_ATTEMPT", "RECOVERY_CODE", "Invalid recovery code used", false)
				os.Exit(1)
			}
			fmt.Println("\n[OK] Emergency recovery code accepted.")
			writeLockdownAuditLog("LOCKDOWN_DEACTIVATED", "EMERGENCY_RECOVERY_CODE", reason, true)
		} else {
			fmt.Printf("\nVerifying GPG identity for key: %s ...\n", gpgKeyID)
			identity, err := verifyGPGSignedRequest(gpgKeyID)
			if err != nil {
				fmt.Printf("\n[ERROR] GPG verification failed: %v\n", err)
				fmt.Println("Lockdown NOT lifted.")
				writeLockdownAuditLog("LOCKDOWN_OFF_ATTEMPT", gpgKeyID, reason, false)
				os.Exit(1)
			}
			fmt.Printf("[OK] GPG identity confirmed: %s\n", identity)
			writeLockdownAuditLog("LOCKDOWN_DEACTIVATED", identity, reason, true)
			verifiedIdentity = identity
		}
	}

	// Send to server with verified identity in headers
	hostname, _ := os.Hostname()
	client := remote.NewClient(serviceURL)
	if err := client.SetLockdown(mode == "on", verifiedIdentity, hostname, reason); err != nil {
		fmt.Printf("Error communicating with server: %v\n", err)
		os.Exit(1)
	}

	if mode == "on" {
		hostname, _ := os.Hostname()
		fmt.Println("\n============================================================")
		fmt.Println("  [EMERGENCY LOCKDOWN ACTIVATED]")
		fmt.Println("============================================================")
		fmt.Printf("  Time      : %s\n", time.Now().UTC().Format(time.RFC3339))
		fmt.Printf("  Initiated : %s (GPG verified)\n", gpgKeyID)
		fmt.Printf("  Host      : %s\n", hostname)
		fmt.Printf("  Reason    : %s\n", reason)
		fmt.Println("------------------------------------------------------------")
		fmt.Println("  ALL plan submissions and signatures are now REJECTED.")
		fmt.Printf("  Audit log : %s\n", lockdownAuditLog)
		fmt.Println("------------------------------------------------------------")
		fmt.Println("  To deactivate:")
		fmt.Println("    terrasign lockdown off --gpg-key <key-id>")
		fmt.Println("    OR: terrasign lockdown off --recovery-code TERRASIGN-EMERGENCY")
		fmt.Println("============================================================")
	} else {
		fmt.Println("\n[OK] Lockdown lifted. System resumes normal operation.")
		fmt.Printf("     Audit log updated: %s\n", lockdownAuditLog)
	}
}
