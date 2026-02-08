package main

import (
	"fmt"
	"os"
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

	// Clear screen
	fmt.Print("\033[H\033[2J")

	for {
		// Move cursor to top-left
		fmt.Print("\033[H")
		
		fmt.Println("=================================================================================")
		fmt.Println("                       TERRASIGN SECURITY MONITOR                                ")
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
		fmt.Println("Press Ctrl+C to exit")
		
		time.Sleep(2 * time.Second)
	}
}
