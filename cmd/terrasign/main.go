package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sulakshanakarunarathne/terrasign/pkg/remote"
	"github.com/sulakshanakarunarathne/terrasign/pkg/signer"
	"github.com/sulakshanakarunarathne/terrasign/pkg/terraform"
	"github.com/sulakshanakarunarathne/terrasign/pkg/verifier"
)

const defaultServiceURL = "http://localhost:8080"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "sign":
		handleSign()
	case "verify":
		handleVerify()
	case "wrap":
		handleWrap()
	case "submit-for-review":
		handleSubmitForReview()
	case "admin":
		handleAdmin()
	case "server":
		handleServer()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("TerraSign - Terraform Plan Signing and Verification")
	fmt.Println("\nUsage: terrasign <command> [args]")
	fmt.Println("\nCommands:")
	fmt.Println("  sign                  Sign a Terraform plan (local)")
	fmt.Println("  verify                Verify a signed plan")
	fmt.Println("  wrap                  Wrap terraform apply with verification")
	fmt.Println("  submit-for-review     Submit plan to signing service (CI workflow)")
	fmt.Println("  admin                 Admin commands (list, download, sign)")
	fmt.Println("  server                Start the signing service")
	fmt.Println("\nUse 'terrasign <command> --help' for more information")
}

func handleSign() {
	signCmd := flag.NewFlagSet("sign", flag.ExitOnError)
	keyPath := signCmd.String("key", "", "Path to private key (for key-based signing)")
	
	signCmd.Parse(os.Args[2:])
	
	if signCmd.NArg() < 1 {
		fmt.Println("Usage: terrasign sign [flags] <plan-file>")
		signCmd.PrintDefaults()
		os.Exit(1)
	}
	
	err := signer.Sign(signCmd.Arg(0), *keyPath)
	if err != nil {
		fmt.Printf("Error signing plan: %v\n", err)
		os.Exit(1)
	}
}

func handleVerify() {
	verifyCmd := flag.NewFlagSet("verify", flag.ExitOnError)
	identity := verifyCmd.String("identity", "", "The expected identity (email) in the certificate")
	issuer := verifyCmd.String("issuer", "https://github.com/login/oauth", "The expected OIDC issuer (default: GitHub)")
	keyPath := verifyCmd.String("key", "", "Path to public key (for key-based verification)")
	
	verifyCmd.Parse(os.Args[2:])
	
	if verifyCmd.NArg() < 1 {
		fmt.Println("Usage: terrasign verify [flags] <plan-file>")
		verifyCmd.PrintDefaults()
		os.Exit(1)
	}
	
	if *keyPath == "" && *identity == "" {
		fmt.Println("Error: either --key or --identity flag is required for verification")
		verifyCmd.PrintDefaults()
		os.Exit(1)
	}

	err := verifier.Verify(verifyCmd.Arg(0), *keyPath, *identity, *issuer)
	if err != nil {
		fmt.Printf("Error verifying plan: %v\n", err)
		os.Exit(1)
	}
}

func handleWrap() {
	wrapCmd := flag.NewFlagSet("wrap", flag.ExitOnError)
	identity := wrapCmd.String("identity", "", "Identity to verify against")
	issuer := wrapCmd.String("issuer", "https://github.com/login/oauth", "OIDC Issuer")
	keyPath := wrapCmd.String("key", "", "Path to public key (for key-based verification)")

	wrapCmd.Parse(os.Args[2:])

	terraformArgs := wrapCmd.Args()
	if len(terraformArgs) == 0 {
		fmt.Println("Usage: terrasign wrap [flags] -- <terraform args>")
		os.Exit(1)
	}
	
	err := terraform.Execute(terraformArgs, *keyPath, *identity, *issuer)
	if err != nil {
		fmt.Printf("Error executing terraform: %v\n", err)
		os.Exit(1)
	}
}

func handleSubmitForReview() {
	submitCmd := flag.NewFlagSet("submit-for-review", flag.ExitOnError)
	serviceURL := submitCmd.String("service", defaultServiceURL, "Signing service URL")
	submitter := submitCmd.String("submitter", "ci-pipeline", "Submitter identifier")
	wait := submitCmd.Bool("wait", false, "Wait for signature before returning")
	timeout := submitCmd.Duration("timeout", 30*time.Minute, "Timeout for waiting")

	submitCmd.Parse(os.Args[2:])

	if submitCmd.NArg() < 1 {
		fmt.Println("Usage: terrasign submit-for-review [flags] <plan-file>")
		submitCmd.PrintDefaults()
		os.Exit(1)
	}

	planPath := submitCmd.Arg(0)
	client := remote.NewClient(*serviceURL)

	fmt.Printf("Submitting plan for review...\n")
	id, err := client.SubmitPlan(planPath, *submitter)
	if err != nil {
		fmt.Printf("Error submitting plan: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Plan submitted successfully!\n")
	fmt.Printf("Submission ID: %s\n", id)
	fmt.Printf("\nAdmin can review and sign with:\n")
	fmt.Printf("  terrasign admin download %s\n", id)
	fmt.Printf("  terrasign admin sign %s --key <admin-key>\n", id)

	if *wait {
		fmt.Printf("\nWaiting for signature (timeout: %s)...\n", timeout)
		if err := client.WaitForSignature(id, *timeout); err != nil {
			fmt.Printf("Error waiting for signature: %v\n", err)
			os.Exit(1)
		}

		// Download signature
		sigPath := planPath + ".sig"
		if err := client.DownloadSignature(id, sigPath); err != nil {
			fmt.Printf("Error downloading signature: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Signature downloaded to: %s\n", sigPath)
	}
}

func handleAdmin() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: terrasign admin <subcommand> [args]")
		fmt.Println("\nSubcommands:")
		fmt.Println("  list-pending          List all pending submissions")
		fmt.Println("  download <id>         Download a plan for review")
		fmt.Println("  sign <id>             Sign an approved plan")
		fmt.Println("  reject <id>           Reject a plan submission")
		fmt.Println("\nFlags:")
		fmt.Println("  --service <url>       Signing service URL (default: http://localhost:8080)")
		os.Exit(1)
	}

	// We need to parse --service flag which might appear before the subcommand
	// But `flag` package expects flags after the command.
	// Since os.Args[2] is either the subcommand OR a flag, we need to handle this manually or structure differently.
	// Let's use a simple approach: if os.Args[2] starts with "-", parses flags first.
	
	serviceURL := defaultServiceURL
	args := os.Args[2:]

	if args[0] == "list-pending" {
		fs := flag.NewFlagSet("list-pending", flag.ExitOnError)
		srv := fs.String("service", defaultServiceURL, "Service URL")
		fs.Parse(args[1:])
		serviceURL = *srv
		
		admin := NewAdminCommands(serviceURL)
		if err := admin.ListPending(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Check download
	if args[0] == "download" {
		fs := flag.NewFlagSet("download", flag.ExitOnError)
		_ = fs.String("service", defaultServiceURL, "Service URL")
		
		// Let's manually scan for --service in args
		for i, arg := range args {
			if arg == "--service" && i+1 < len(args) {
				serviceURL = args[i+1]
			}
		}

		// Now find ID and OutputDir (ignoring --service and its value)
		var cmdArgs []string
		skipNext := false
		for _, arg := range args[1:] {
			if skipNext {
				skipNext = false
				continue
			}
			if arg == "--service" {
				skipNext = true
				continue
			}
			cmdArgs = append(cmdArgs, arg)
		}

		if len(cmdArgs) < 1 {
			fmt.Println("Usage: terrasign admin download <submission-id> [output-dir] [--service <url>]")
			os.Exit(1)
		}
		
		id := cmdArgs[0]
		outputDir := "."
		if len(cmdArgs) > 1 {
			outputDir = cmdArgs[1]
		}

		admin := NewAdminCommands(serviceURL)
		if err := admin.Download(id, outputDir); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Check sign
	if args[0] == "sign" {
		fs := flag.NewFlagSet("sign", flag.ExitOnError)
		_ = fs.String("service", defaultServiceURL, "Service URL")
		keyPath := fs.String("key", "", "Path to admin private key (required)")
		reviewer := fs.String("reviewer", "admin", "Reviewer name")
		
		// terrasign admin sign <id> --key ... --service ...
		// flags must be parsed. Since <id> is a positional arg, `flag` stops there.
		// We have to parse flags from args ignoring the ID, or require ID last?
		// Standard Go flag usage puts flags before positional args.
		// terrasign admin sign --key ... --service ... <id>
		
		// Let's modify usage to be standard: flags then args
		// But for backward compat/ease, let's manually parse --service again.
		
		for i, arg := range args {
			if arg == "--service" && i+1 < len(args) {
				serviceURL = args[i+1]
			}
		}
		
		// Now use flagset for the rest, but we have to filter out positional ID to let Parse work?
		// Actually, let's just use manual parsing for everything here to be consistent with weird CLI structure
		var id string
		var key string
		var rev = "admin"
		
		skipNext := false
		for i, arg := range args[1:] {
			if skipNext {
				skipNext = false
				continue
			}
			if arg == "--service" {
				skipNext = true
				continue
			}
			if arg == "--key" && i+2 < len(args) { // i is index in slice starting from 1
				key = args[1:][i+1]
				skipNext = true
				continue
			}
			if arg == "--reviewer" && i+2 < len(args) {
				rev = args[1:][i+1]
				skipNext = true
				continue
			}
			// If it looks like a flag but we didn't handle it
			if strings.HasPrefix(arg, "-") {
				continue 
			}
			if id == "" {
				id = arg
			}
		}

		// Fallback to flagset if manual parsing didn't find key (maybe passed as -key)
		if key == "" {
			// Try to parse using flagset, hoping flags are before ID
			fs.Parse(args[1:])
			// We can't access srv since we ignored it earlier, but we can check if fs parsed anything
			
			// Actually, let's just rely on manual parsing being sufficient. 
			// If key is still empty, check if it was parsed by fs into *keyPath
			if *keyPath != "" {
				key = *keyPath
			}
			if *reviewer != "admin" {
				rev = *reviewer
			}
			if fs.NArg() > 0 && id == "" {
				id = fs.Arg(0)
			}
		}

		if id == "" || key == "" {
			fmt.Println("Usage: terrasign admin sign [flags] <submission-id>\nFlags:\n  --key <path> (required)\n  --service <url>\n  --reviewer <name>")
			os.Exit(1)
		}

		admin := NewAdminCommands(serviceURL)
		if err := admin.Sign(id, key, rev); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Printf("Unknown admin subcommand: %s\n", args[0])
	os.Exit(1)
}

func handleServer() {
	serverCmd := flag.NewFlagSet("server", flag.ExitOnError)
	port := serverCmd.Int("port", 8080, "Port to listen on")
	storageDir := serverCmd.String("storage", "./terrasign-storage", "Storage directory for plans")

	serverCmd.Parse(os.Args[2:])

	config := remote.SigningServiceConfig{
		StorageDir: *storageDir,
		Port:       *port,
	}

	service, err := remote.NewSigningService(config)
	if err != nil {
		fmt.Printf("Error creating service: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting TerraSign signing service...\n")
	if err := service.Start(); err != nil {
		fmt.Printf("Error starting service: %v\n", err)
		os.Exit(1)
	}
}
