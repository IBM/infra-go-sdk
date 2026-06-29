package main

import (
	"log"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func printUsage() {
	fmt.Println("Usage: mediarepo <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  list    List Virtual Media Repositories on the system")
	fmt.Println("  create  Provision a new Virtual Media Repository on a VIOS")
	fmt.Println("  extend  Increase the capacity of an existing Virtual Media Repository")
	fmt.Println("  delete  Permanently remove a Virtual Media Repository from a VIOS")
	fmt.Println("\nUse 'mediarepo <command> -h' for more information about a specific command.")
}

func main() {
	// =========================================================================
	// 1. SET UP CONTEXT & GRACEFUL CANCELLATION (Ctrl+C)
	// =========================================================================
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize CLI Logger

	// =========================================================================
	// 2. SUBCOMMAND ROUTER & CONFIGURATION
	// =========================================================================
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	extendCmd := flag.NewFlagSet("extend", flag.ExitOnError)
	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)

	// Shared Variables
	var hmcIP, username, password, sysName, viosName string
	var verbose bool

	// Action-Specific Variables
	var vgName, repoName string
	var repSize, addSize int
	var force bool

	// ✨ HELPER: Global flags used by ALL commands
	bindCommonFlags := func(fs *flag.FlagSet) {
		fs.StringVar(&hmcIP, "hmc-ip", "", "HMC IP address")
		fs.StringVar(&username, "hmc-user", "", "HMC username")
		fs.StringVar(&password, "hmc-pass", "", "HMC password")
		fs.StringVar(&sysName, "system-name", "", "Managed System Name (Required)")
		fs.StringVar(&viosName, "vios-name", "", "Target VIOS Name (Optional for list, Required for others)")
		fs.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	}

	// --- Bind Flags to Subcommands ---
	bindCommonFlags(listCmd)

	bindCommonFlags(createCmd)
	createCmd.StringVar(&vgName, "vg-name", "rootvg", "Storage Pool / Volume Group to build the repository in (Required)")
	createCmd.IntVar(&repSize, "rep-size", 20480, "Size of the Virtual Media Repository in Megabytes (Required)")

	bindCommonFlags(extendCmd)
	extendCmd.IntVar(&addSize, "add-size", 10240, "Amount of additional space to add in Megabytes (Required)")

	bindCommonFlags(deleteCmd)
	deleteCmd.StringVar(&repoName, "repo-name", "VMLibrary", "Name of the repository to delete")
	deleteCmd.BoolVar(&force, "force", false, "Force deletion even if ISO media exists inside")

	// Route the Subcommand
	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "list":
		listCmd.Parse(os.Args[2:])
	case "create":
		createCmd.Parse(os.Args[2:])
	case "extend":
		extendCmd.Parse(os.Args[2:])
	case "delete":
		deleteCmd.Parse(os.Args[2:])
	case "help", "-h", "-help", "--help":
		printUsage()
		os.Exit(0)
	default:
		log.Printf("Unknown command: command=%v", cmd)
		printUsage()
		os.Exit(1)
	}

	// Apply Verbosity to Logger
	if verbose {
	} else {
		log.Printf(": %v", 0)
	}

	// --- Shared Validation ---
	if password == "" || sysName == "" {
		log.Fatal("Missing required arguments")
	}

	if cmd != "list" && viosName == "" {
		log.Fatal("Missing required argument")
	}

	if cmd == "create" {
		if vgName == "" {
			log.Fatal("Missing required argument")
		}
		if repSize <= 0 {
			log.Fatal("Invalid argument")
		}
	}
	if cmd == "extend" && addSize <= 0 {
		log.Fatal("Invalid argument")
	}

	// =========================================================================
	// 3. AUTHENTICATION & RESOLUTION
	// =========================================================================
	log.Printf("Logging into HMC: ip=%v", hmcIP)
	restClient := hmc.NewRestClient(hmcIP)

	if err := restClient.Login(context.Background(), username, password, verbose); err != nil {
		log.Fatal("HMC Logon failed")
	}
	defer func() {
		log.Println("Closing HMC Session...")
		restClient.Logoff(context.Background())
	}()

	log.Printf("Resolving System: system=%v", sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), sysName, verbose)
	if err != nil || sysUUID == "" {
		log.Fatal("Failed to resolve Managed System")
	}

	var targetViosUUID string
	if viosName != "" {
		log.Printf("Resolving VIOS: vios=%v", viosName)
		targetViosUUID, err = hmc.GetViosID(context.Background(), restClient, sysUUID, viosName, verbose)
		if err != nil || targetViosUUID == "" {
			log.Fatal("VIOS not found on system")
		}
	}

	// Check Context immediately before executing heavy operations
	if ctx.Err() != nil {
		log.Fatal("Operation aborted by user (Ctrl+C)")
	}

	// =========================================================================
	// 4. EXECUTE COMMAND
	// =========================================================================

	switch cmd {

	// -------------------------------------------------------------------------
	// LIST MODE
	// -------------------------------------------------------------------------
	case "list":
		log.Printf("Fetching Virtual Media Repository Inventory: system=%v", sysName)

		viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID, verbose)
		if err != nil || len(viosList) == 0 {
			log.Fatal("Failed to fetch VIOS instances for system")
		}

		reposFound := 0

		for _, vios := range viosList {
			if viosName != "" && !strings.EqualFold(vios.PartitionName, viosName) {
				continue
			}

			vgList, err := restClient.GetVolumeGroups(context.Background(), vios.UUID, verbose)
			if err != nil {
				continue
			}

			for _, vg := range vgList {
				if vg.MediaRepositoryName != "" {
					reposFound++
					log.Printf("[INFO] Media Repository Found %v", "vios", vios.PartitionName,
						"volume_group", vg.GroupName,
						"repo_name", vg.MediaRepositoryName,
						"capacity_gb", vg.MediaRepositorySize,)
				}
			}
		}

		if reposFound == 0 {
			log.Println("[WARN] No Virtual Media Repositories found matching your criteria.")
		} else {
			log.Printf("Scan Complete: total_repositories=%v", reposFound)
		}

	// -------------------------------------------------------------------------
	// CREATE MODE
	// -------------------------------------------------------------------------
	case "create":
		log.Printf("Initiating Media Repository Creation: vios=%v vg=%v size_mb=%v", viosName, vgName, repSize)

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		log.Println("Verifying if a Media Repository already exists on this VIOS...")
		vgList, err := restClient.GetVolumeGroups(context.Background(), targetViosUUID, verbose)
		if err != nil {
			log.Fatal("Failed to fetch Volume Groups to verify repository existence")
		}

		for _, vg := range vgList {
			if vg.MediaRepositoryName != "" {
				log.Printf("Media Repository already exists. Skipping creation.: vios=%v vg=%v repo_name=%v", viosName, vg.GroupName, vg.MediaRepositoryName)
				os.Exit(0) // Idempotent Exit
			}
		}

		log.Printf("Executing Media Repository creation via VIOS...: size_mb=%v", repSize)

		err = restClient.CreateMediaRepository(context.Background(), sysName, targetViosUUID, viosName, vgName, repSize, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to create Media Repository")
		}

		log.Printf("SUCCESS: Virtual Media Repository Created!: vios=%v vg=%v", viosName, vgName)

	// -------------------------------------------------------------------------
	// EXTEND MODE
	// -------------------------------------------------------------------------
	case "extend":
		log.Printf("Initiating Media Repository Extension: vios=%v add_size_mb=%v", viosName, addSize)

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		log.Println("Verifying if Media Repository exists...")
		vgList, err := restClient.GetVolumeGroups(context.Background(), targetViosUUID, verbose)
		if err != nil {
			log.Fatal("Failed to fetch Volume Groups to verify repository existence")
		}

		repoExists := false
		for _, vg := range vgList {
			if vg.MediaRepositoryName != "" {
				repoExists = true
				break
			}
		}

		if !repoExists {
			log.Fatal("Virtual Media Repository does not exist on VIOS. Cannot extend.")
		}

		log.Printf("Extending Media Repository: vios=%v add_mb=%v", viosName, addSize)

		err = restClient.ChangeMediaRepository(context.Background(), sysName, targetViosUUID, viosName, addSize, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to extend Media Repository")
		}

		log.Printf("SUCCESS: Virtual Media Repository Extended!: vios=%v added_mb=%v", viosName, addSize)

	// -------------------------------------------------------------------------
	// DELETE MODE
	// -------------------------------------------------------------------------
	case "delete":
		log.Printf("Initiating permanent Media Repository Deletion: vios=%v repo_name=%v", viosName, repoName)

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		log.Println("Verifying if Media Repository exists...")
		vgList, err := restClient.GetVolumeGroups(context.Background(), targetViosUUID, verbose)
		if err != nil {
			log.Fatal("Failed to fetch Volume Groups to verify repository existence")
		}

		repoExists := false
		for _, vg := range vgList {
			// Ensure we are deleting the exact one we specify (usually VMLibrary)
			if strings.EqualFold(vg.MediaRepositoryName, repoName) {
				repoExists = true
				break
			}
		}

		if !repoExists {
			log.Printf("Virtual Media Repository not found on VIOS. No action needed.: vios=%v repo_name=%v", viosName, repoName)
			os.Exit(0) // Idempotent Exit
		}

		log.Printf("Attempting to permanently delete Virtual Media Repository: repo_name=%v vios=%v force=%v", repoName, viosName, force)

		err = restClient.DeleteMediaRepository(context.Background(), sysName, targetViosUUID, viosName, repoName, force, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to delete Media Repository")
		}

		log.Printf("SUCCESS: Virtual Media Repository Deleted!: repo_name=%v vios=%v", repoName, viosName)
	}
}