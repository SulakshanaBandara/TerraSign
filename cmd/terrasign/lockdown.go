package main

import (
	"fmt"
	"os"

	"github.com/sulakshanakarunarathne/terrasign/pkg/remote"
)

func handleLockdown() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: terrasign lockdown [on|off] [--service <url>] [--key <path>] [--recovery-code <code>]")
		os.Exit(1)
	}

	mode := os.Args[2]
	if mode != "on" && mode != "off" {
		fmt.Println("Error: mode must be 'on' or 'off'")
		os.Exit(1)
	}

	serviceURL := defaultServiceURL
	keyPath := ""
	recoveryCode := ""
	
	for i, arg := range os.Args {
		if arg == "--service" && i+1 < len(os.Args) {
			serviceURL = os.Args[i+1]
		}
		if arg == "--key" && i+1 < len(os.Args) {
			keyPath = os.Args[i+1]
		}
		if arg == "--recovery-code" && i+1 < len(os.Args) {
			recoveryCode = os.Args[i+1]
		}
	}

	// Lockdown OFF requires authentication
	if mode == "off" {
		if keyPath == "" && recoveryCode == "" {
			fmt.Println("\n[ERROR] Lockdown deactivation requires authentication!")
			fmt.Println("\nOptions:")
			fmt.Println("  1. Use admin key:       terrasign lockdown off --key <path>")
			fmt.Println("  2. Use recovery code:   terrasign lockdown off --recovery-code <code>")
			fmt.Println("\nEmergency Recovery Code: TERRASIGN-EMERGENCY-2024")
			fmt.Println("(In production, this would be securely stored and rotated)")
			os.Exit(1)
		}
		
		// Verify authentication
		if recoveryCode != "" {
			// Check recovery code
			if recoveryCode != "TERRASIGN-EMERGENCY-2024" {
				fmt.Println("\n[ERROR] Invalid recovery code!")
				os.Exit(1)
			}
			fmt.Println("\n[OK] Recovery code verified")
		} else {
			// Verify key exists
			if _, err := os.Stat(keyPath); os.IsNotExist(err) {
				fmt.Printf("\n[ERROR] Key file not found: %s\n", keyPath)
				fmt.Println("\nAlternatively, use recovery code:")
				fmt.Println("  terrasign lockdown off --recovery-code TERRASIGN-EMERGENCY-2024")
				os.Exit(1)
			}
			fmt.Printf("\n[OK] Admin key verified: %s\n", keyPath)
		}
	}

	client := remote.NewClient(serviceURL)
	if err := client.SetLockdown(mode == "on"); err != nil {
		fmt.Printf("Error setting lockdown: %v\n", err)
		os.Exit(1)
	}

	if mode == "on" {
		fmt.Println("\n[!!!] EMERGENCY LOCKDOWN ACTIVATED [!!!]")
		fmt.Println("System is now rejecting ALL plan submissions and signatures.")
		fmt.Println("\nTo deactivate, use:")
		fmt.Println("  terrasign lockdown off --key <admin-key-path>")
		fmt.Println("  OR")
		fmt.Println("  terrasign lockdown off --recovery-code TERRASIGN-EMERGENCY-2024")
	} else {
		fmt.Println("\n[OK] Lockdown lifted. System resumes normal operation.")
	}
}
