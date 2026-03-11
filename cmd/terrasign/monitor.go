package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/sulakshanakarunarathne/terrasign/pkg/remote"
	"github.com/sulakshanakarunarathne/terrasign/pkg/signer"
)

func handleMonitor() {
	serviceURL := defaultServiceURL
	for i, arg := range os.Args {
		if arg == "--service" && i+1 < len(os.Args) {
			serviceURL = os.Args[i+1]
		}
	}

	client := remote.NewClient(serviceURL)
	admin := NewAdminCommands(serviceURL)

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\033[H\033[2J") // clear screen

		fmt.Println("=================================================================================")
		fmt.Println("                   TERRASIGN APPROVAL DASHBOARD                                  ")
		fmt.Println("=================================================================================")
		fmt.Printf("Service: %s   |   Time: %s\n", serviceURL, time.Now().Format("15:04:05"))

		if policy, err := client.GetPolicy(); err == nil {
			setBy := policy.SetBy
			if setBy == "" {
				setBy = "default"
			}
			fmt.Printf("Policy : Requires %d approval(s)   |   Set by: %s\n", policy.ApprovalThreshold, setBy)
			if len(policy.AuthorizedKeys) > 0 {
				names := make([]string, 0, len(policy.AuthorizedKeys))
				for _, ak := range policy.AuthorizedKeys {
					names = append(names, ak.Name)
				}
				fmt.Printf("         Authorized keys: %s\n", strings.Join(names, ", "))
			}
		}
		fmt.Println("---------------------------------------------------------------------------------")

		pending, err := client.ListPending()
		if err != nil {
			fmt.Printf("Error fetching plans: %v\n", err)
		} else if len(pending) == 0 {
			fmt.Println("\n  No pending plans. System secure.")
		} else {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "\nID\tSUBMITTER\tCREATED AT\tSTATUS")
			fmt.Fprintln(w, "--\t---------\t----------\t------")
			for _, p := range pending {
				status := p.Status
				switch {
				case p.Status == "pending" && p.ApprovalThreshold > 1:
					status = fmt.Sprintf("pending (%s approvals)", p.ApprovalCount())
				case p.Status == "pending":
					status = "pending (0/1 approvals)"
				case p.Status == "approved":
					status = fmt.Sprintf("[approved] (%s)", p.ApprovalCount())
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					p.ID, p.Submitter, p.CreatedAt.Format("15:04:05"), status)
			}
			w.Flush()
		}

		fmt.Println("\n---------------------------------------------------------------------------------")
		fmt.Println("Actions: [s]ign  |  [i]nspect  |  [p]olicy  |  [r]efresh  |  [q]uit")
		fmt.Print("Action: ")

		if !scanner.Scan() {
			break
		}
		action := strings.TrimSpace(strings.ToLower(scanner.Text()))

		switch action {

		// ── SIGN ─────────────────────────────────────────────────────────────────
		case "s", "sign":
			fmt.Print("Submission ID: ")
			if !scanner.Scan() {
				continue
			}
			id := strings.TrimSpace(scanner.Text())
			if id == "" {
				continue
			}

			fmt.Print("Your name / reviewer ID (for audit): ")
			if !scanner.Scan() {
				continue
			}
			reviewer := strings.TrimSpace(scanner.Text())
			if reviewer == "" {
				reviewer = "admin"
			}

			fmt.Print("Path to your signing key (.key file): ")
			if !scanner.Scan() {
				continue
			}
			keyPath := strings.TrimSpace(scanner.Text())
			if keyPath == "" {
				fmt.Println("[ERROR] Key path cannot be empty.")
				fmt.Print("Press Enter to continue...")
				scanner.Scan()
				continue
			}
			if _, err := os.Stat(keyPath); err != nil {
				fmt.Printf("[ERROR] Key file not found: %s\n", keyPath)
				fmt.Print("Press Enter to continue...")
				scanner.Scan()
				continue
			}

			// Sign runs in a temp dir — no need to chdir
			if err := admin.Sign(id, keyPath, reviewer); err != nil {
				fmt.Printf("[ERROR] %v\n", err)
			} else {
				fmt.Println("[OK] Plan signed successfully.")
			}
			fmt.Print("Press Enter to continue...")
			scanner.Scan()

		// ── INSPECT ───────────────────────────────────────────────────────────────
		case "i", "inspect":
			fmt.Print("Submission ID: ")
			if !scanner.Scan() {
				continue
			}
			id := strings.TrimSpace(scanner.Text())
			if id == "" {
				continue
			}
			fmt.Println("\n--- Plan Changes ---")
			if err := admin.Inspect(id); err != nil {
				fmt.Printf("[ERROR] %v\n", err)
			}
			fmt.Print("\nPress Enter to continue...")
			scanner.Scan()

		// ── POLICY ────────────────────────────────────────────────────────────────
		case "p", "policy":
			// Show current policy
			if policy, err := client.GetPolicy(); err == nil {
				fmt.Printf("\nCurrent policy: %d approval(s) required\n", policy.ApprovalThreshold)
				if policy.SetBy != "" {
					fmt.Printf("  Set by : %s at %s\n", policy.SetBy, policy.SetAt)
				}
				if policy.Reason != "" {
					fmt.Printf("  Reason : %s\n", policy.Reason)
				}
				if len(policy.AuthorizedKeys) > 0 {
					fmt.Println("  Authorized signers:")
					for i, ak := range policy.AuthorizedKeys {
						fmt.Printf("    [%d] %s\n", i+1, ak.Name)
					}
				} else {
					fmt.Println("  Authorized signers: any key")
				}
			}

			fmt.Print("\nNew threshold (number of required approvals): ")
			if !scanner.Scan() {
				continue
			}
			newThreshStr := strings.TrimSpace(scanner.Text())
			if newThreshStr == "" {
				continue
			}
			var newThresh int
			if _, err := fmt.Sscanf(newThreshStr, "%d", &newThresh); err != nil || newThresh < 1 {
				fmt.Println("[ERROR] Threshold must be a number >= 1.")
				fmt.Print("Press Enter...")
				scanner.Scan()
				continue
			}

			// ── identify the policy admin ─────────────────────────────────────
			fmt.Print("Your name (for audit log): ")
			var policyAdmin string
			if scanner.Scan() {
				policyAdmin = strings.TrimSpace(scanner.Text())
			}
			if policyAdmin == "" {
				policyAdmin = "admin"
			}

			// Ask the policy-setter to sign with their own key — proves identity
			fmt.Print("Path to YOUR signing key (.key file, to authenticate this policy change): ")
			if !scanner.Scan() {
				continue
			}
			adminKeyPath := strings.TrimSpace(scanner.Text())
			if adminKeyPath == "" || func() bool { _, e := os.Stat(adminKeyPath); return e != nil }() {
				fmt.Println("[ERROR] Signing key not found. Policy change cancelled.")
				fmt.Print("Press Enter...")
				scanner.Scan()
				continue
			}

			// Sign the policy data with the admin's key to prove authorization
			policyPayload := fmt.Sprintf("threshold=%d|set_by=%s|time=%s",
				newThresh, policyAdmin, time.Now().Format(time.RFC3339))
			policyPayloadFile := filepath.Join(os.TempDir(), "ts-policy-payload.txt")
			if err := os.WriteFile(policyPayloadFile, []byte(policyPayload), 0600); err == nil {
				defer os.Remove(policyPayloadFile)
				if err := signer.SignWithOptions(policyPayloadFile, adminKeyPath, true); err != nil {
					fmt.Printf("[ERROR] Failed to authenticate with your key: %v\n", err)
					fmt.Println("Policy change cancelled.")
					fmt.Print("Press Enter...")
					scanner.Scan()
					continue
				}
				// Clean up sig/bundle from temp
				os.Remove(policyPayloadFile + ".sig")
				os.Remove(policyPayloadFile + ".bundle")
				fmt.Printf("[OK] Identity verified: policy change authenticated by %s.\n", policyAdmin)
			}

			// ── collect authorized approver public keys ────────────────────────
			fmt.Printf("\nRegister the public key (.pub) for each of the %d required approver(s).\n", newThresh)
			fmt.Println("Press Enter to skip (any key will be accepted for that slot).")
			var authorizedKeyPaths []string
			for i := 1; i <= newThresh; i++ {
				fmt.Printf("  Approver [%d/%d] public key (.pub file): ", i, newThresh)
				if !scanner.Scan() {
					break
				}
				kp := strings.TrimSpace(scanner.Text())
				if kp == "" {
					fmt.Printf("  (skipped — slot %d accepts any key)\n", i)
					continue
				}
				if _, err := os.Stat(kp); err != nil {
					fmt.Printf("  [WARN] File not found: %s — skipped.\n", kp)
					continue
				}
				authorizedKeyPaths = append(authorizedKeyPaths, kp)
				fmt.Printf("  [OK] %s registered.\n", filepath.Base(kp))
			}

			fmt.Print("Reason for this policy change: ")
			var reason string
			if scanner.Scan() {
				reason = strings.TrimSpace(scanner.Text())
			}

			if _, err := client.SetPolicy(newThresh, policyAdmin, reason, authorizedKeyPaths...); err != nil {
				fmt.Printf("[ERROR] %v\n", err)
			} else {
				fmt.Printf("\n[OK] Policy updated: %d approval(s) required for all future plans.\n", newThresh)
				if len(authorizedKeyPaths) > 0 {
					fmt.Printf("[OK] %d key(s) registered as authorized approvers.\n", len(authorizedKeyPaths))
				} else {
					fmt.Println("[INFO] No specific keys registered: any key is accepted.")
				}
			}
			fmt.Print("Press Enter to continue...")
			scanner.Scan()

		case "r", "refresh":
			continue

		case "q", "quit":
			fmt.Println("Exiting dashboard.")
			return

		default:
			fmt.Print("Unknown action. Press Enter...")
			scanner.Scan()
		}
	}
}
