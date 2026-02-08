package main

import (
	"fmt"
	"os"

	"github.com/sulakshanakarunarathne/terrasign/pkg/remote"
)

func handleLockdown() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: terrasign lockdown [on|off] [--service <url>]")
		os.Exit(1)
	}

	mode := os.Args[2]
	if mode != "on" && mode != "off" {
		fmt.Println("Error: mode must be 'on' or 'off'")
		os.Exit(1)
	}

	serviceURL := defaultServiceURL
	for i, arg := range os.Args {
		if arg == "--service" && i+1 < len(os.Args) {
			serviceURL = os.Args[i+1]
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
	} else {
		fmt.Println("\n[OK] Lockdown lifted. System resumes normal operation.")
	}
}
