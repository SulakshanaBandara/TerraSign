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
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", 
						p.ID, 
						p.Submitter, 
						p.CreatedAt.Format("15:04:05"), 
						p.Status)
				}
				w.Flush()
			}
		}
		
		fmt.Println("\n---------------------------------------------------------------------------------")
		fmt.Println("Actions: [i]nspect | [s]ign | [r]efresh | [q]uit")
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
				if err := admin.Sign(id, keyPath, "admin"); err != nil {
					fmt.Printf("Error: %v\n", err)
				} else {
					fmt.Println("✓ Plan signed successfully!")
				}
				fmt.Print("\nPress Enter to continue...")
				scanner.Scan()
			}
			
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
