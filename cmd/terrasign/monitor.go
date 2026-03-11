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

	// Interactive mode
	scanner := bufio.NewScanner(os.Stdin)

	for {
		// Clear screen and show pending plans
		fmt.Print("\033[H\033[2J")

		fmt.Println("=================================================================================")
		fmt.Println("                   TERRASIGN INTERACTIVE DASHBOARD                              ")
		fmt.Println("=================================================================================")
		fmt.Printf("Service: %s   |   Time: %s\n", serviceURL, time.Now().Format("15:04:05"))
		if policy, err2 := client.GetPolicy(); err2 == nil {
			setBy := policy.SetBy
			if setBy == "" {
				setBy = "default"
			}
			fmt.Printf("Policy: Requires %d approval(s)   |   Set by: %s\n", policy.ApprovalThreshold, setBy)
		}
		fmt.Println("---------------------------------------------------------------------------------")

		pending, err := client.ListPending()
		if err != nil {
			fmt.Printf("Error fetching data: %v\n", err)
		} else {
			if len(pending) == 0 {
				fmt.Println("\n  No pending plans. System secure.")
			} else {
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
				fmt.Fprintln(w, "\nID\tSUBMITTER\tCREATED AT\tSTATUS")
				fmt.Fprintln(w, "--\t---------\t----------\t------")

				for _, p := range pending {
					status := p.Status
					if p.Status == "pending" && p.ApprovalThreshold > 1 {
						status = fmt.Sprintf("pending (%s approvals)", p.ApprovalCount())
					} else if p.Status == "approved" {
						status = fmt.Sprintf("[approved] (%s)", p.ApprovalCount())
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						p.ID,
						p.Submitter,
						p.CreatedAt.Format("15:04:05"),
						status)
				}
				w.Flush()
			}
		}

		fmt.Println("\n---------------------------------------------------------------------------------")
		fmt.Println("Actions: [i]nspect | [s]ign | [p]olicy | [r]efresh | [q]uit")
		fmt.Print("Enter action: ")

		if !scanner.Scan() {
			break
		}

		action := strings.TrimSpace(strings.ToLower(scanner.Text()))

		switch action {
		case "i", "inspect":
			fmt.Print("Enter submission ID: ")
			if !scanner.Scan() {
				continue
			}
			id := strings.TrimSpace(scanner.Text())
			if id != "" {
				fmt.Println("\n--- Plan Changes ---")
				if err := admin.Inspect(id); err != nil {
					fmt.Printf("Error: %v\n", err)
				}
				fmt.Print("\nPress Enter to continue...")
				scanner.Scan()
			}

		case "s", "sign":
			fmt.Print("Enter submission ID: ")
			if !scanner.Scan() {
				continue
			}
			id := strings.TrimSpace(scanner.Text())

			// Find project root to construct absolute path
			cwd, _ := os.Getwd()
			projectRoot := cwd
			for {
				if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
					break
				}
				parent := filepath.Dir(projectRoot)
				if parent == projectRoot {
					projectRoot = cwd
					break
				}
				projectRoot = parent
			}

			defaultKeyPath := filepath.Join(projectRoot, "examples", "simple-app", "admin.key")

			fmt.Printf("\nDefault key path: %s\n", defaultKeyPath)
			fmt.Println("Options:")
			fmt.Println("  [1] Use default path")
			fmt.Println("  [2] Enter custom path")
			fmt.Print("Choose option (1 or 2): ")

			if !scanner.Scan() {
				continue
			}
			choice := strings.TrimSpace(scanner.Text())

			var keyPath string
			if choice == "2" {
				fmt.Print("Enter custom key path: ")
				if !scanner.Scan() {
					continue
				}
				keyPath = strings.TrimSpace(scanner.Text())
				if keyPath == "" {
					fmt.Println("Error: Key path cannot be empty")
					fmt.Print("\nPress Enter to continue...")
					scanner.Scan()
					continue
				}
			} else {
				keyPath = defaultKeyPath
			}

			if id != "" {
				// Ask for reviewer name to track who signed
				fmt.Print("Enter your name/ID for audit (e.g. alice): ")
				var reviewer string
				if scanner.Scan() {
					reviewer = strings.TrimSpace(scanner.Text())
				}
				if reviewer == "" {
					reviewer = "admin"
				}

				// We MUST change directory to the simple-app directory so that the downloaded
				// .sig and .bundle files land where ts-verify expects them.
				targetDir := filepath.Join(projectRoot, "examples", "simple-app")
				os.Chdir(targetDir)

				if err := admin.Sign(id, keyPath, reviewer); err != nil {
					fmt.Printf("Error: %v\n", err)
				} else {
					fmt.Println("[OK] Plan signed successfully")
				}

				// Change back to original directory just in case
				os.Chdir(cwd)

				fmt.Print("\nPress Enter to continue...")
				scanner.Scan()
			}

		case "p", "policy":
			// Show current policy
			if policy, err := client.GetPolicy(); err == nil {
				fmt.Printf("\nCurrent global policy: %d approval(s) required\n", policy.ApprovalThreshold)
				if policy.SetBy != "" {
					fmt.Printf("Set by: %s at %s\n", policy.SetBy, policy.SetAt)
				}
				if policy.Reason != "" {
					fmt.Printf("Reason: %s\n", policy.Reason)
				}
				if len(policy.AuthorizedKeys) > 0 {
					fmt.Printf("Authorized signing keys:\n")
					for i, ak := range policy.AuthorizedKeys {
						fmt.Printf("  [%d] %s  (fingerprint: %s...)\n", i+1, ak.Name, ak.Fingerprint[:16])
					}
				} else {
					fmt.Printf("Authorized keys: any key accepted\n")
				}
			}

			fmt.Print("\nEnter new threshold (1-9, or Enter to keep current): ")
			if !scanner.Scan() {
				continue
			}
			newThreshStr := strings.TrimSpace(scanner.Text())
			if newThreshStr == "" {
				continue
			}
			var newThresh int
			if _, err := fmt.Sscanf(newThreshStr, "%d", &newThresh); err != nil || newThresh < 1 {
				fmt.Println("Invalid threshold. Must be a number >= 1.")
				fmt.Print("Press Enter to continue...")
				scanner.Scan()
				continue
			}

			fmt.Print("Enter your name (for audit log): ")
			var policyAdmin string
			if scanner.Scan() {
				policyAdmin = strings.TrimSpace(scanner.Text())
			}
			if policyAdmin == "" {
				policyAdmin = "admin"
			}

			// Collect authorized public key paths — one per approver slot
			fmt.Printf("\nEnter the public key file path for each authorized approver.\n")
			fmt.Printf("Each approver must use their own distinct key. Press Enter with no input to finish.\n")
			var authorizedKeyPaths []string
			for i := 1; i <= newThresh; i++ {
				fmt.Printf("  Authorized key [%d/%d] (path to .pub file): ", i, newThresh)
				if !scanner.Scan() {
					break
				}
				kp := strings.TrimSpace(scanner.Text())
				if kp == "" {
					fmt.Printf("  (skipped — any key will be accepted for slot %d)\n", i)
					continue
				}
				// Verify file exists
				if _, err := os.Stat(kp); err != nil {
					fmt.Printf("  [WARN] File not found: %s — skipping.\n", kp)
					continue
				}
				authorizedKeyPaths = append(authorizedKeyPaths, kp)
				fmt.Printf("  [OK] Added: %s\n", kp)
			}

			fmt.Print("Reason for policy change: ")
			var reason string
			if scanner.Scan() {
				reason = strings.TrimSpace(scanner.Text())
			}

			if _, err := client.SetPolicy(newThresh, policyAdmin, reason, authorizedKeyPaths...); err != nil {
				fmt.Printf("Error setting policy: %v\n", err)
			} else {
				fmt.Printf("\n[OK] Policy updated: all new plans now require %d approval(s).\n", newThresh)
				if len(authorizedKeyPaths) > 0 {
					fmt.Printf("[OK] Only %d registered key(s) may approve plans.\n", len(authorizedKeyPaths))
				} else {
					fmt.Printf("[INFO] No specific keys registered — any key will be accepted.\n")
				}
			}
			fmt.Print("Press Enter to continue...")
			scanner.Scan()

		case "r", "refresh":
			// Just loop again
			continue

		case "q", "quit":
			fmt.Println("Exiting dashboard...")
			return

		default:
			fmt.Println("Invalid action. Press Enter to continue...")
			scanner.Scan()
		}
	}
}
