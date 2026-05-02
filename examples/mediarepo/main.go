package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
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
	cliLogger := hmc.NewDefaultLogger()
	cliLogger.SetPrefix("[CLI]")

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
		fs.StringVar(&hmcIP, "hmc-ip", "192.0.2.1", "HMC IP address")
		fs.StringVar(&username, "hmc-user", "REDACTED_HMC_USER<==", "HMC username")
		fs.StringVar(&password, "hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
		fs.StringVar(&sysName, "system-name", "LTC09U31-ZZ", "Managed System Name (Required)")
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
		cliLogger.Error("Unknown command", "command", cmd)
		printUsage()
		os.Exit(1)
	}

	// Apply Verbosity to Logger
	if verbose {
		cliLogger.EnableDebug()
	} else {
		cliLogger.SetLevel(0) // InfoLevel
	}

	// --- Shared Validation ---
	if password == "" || sysName == "" {
		cliLogger.Fatal("Missing required arguments", "required", "hmc-pass, system-name")
	}

	if cmd != "list" && viosName == "" {
		cliLogger.Fatal("Missing required argument", "required", "vios-name")
	}

	if cmd == "create" {
		if vgName == "" {
			cliLogger.Fatal("Missing required argument", "required", "vg-name")
		}
		if repSize <= 0 {
			cliLogger.Fatal("Invalid argument", "error", "rep-size must be greater than 0")
		}
	}
	if cmd == "extend" && addSize <= 0 {
		cliLogger.Fatal("Invalid argument", "error", "add-size must be greater than 0")
	}

	// =========================================================================
	// 3. AUTHENTICATION & RESOLUTION
	// =========================================================================
	cliLogger.Info("Logging into HMC", "ip", hmcIP)
	restClient := hmc.NewHmcRestClient(hmcIP)

	if verbose {
		restClient.EnableVerboseLogging()
	}

	if err := restClient.Login(context.Background(), username, password, verbose); err != nil {
		cliLogger.Fatal("HMC Logon failed", "error", err)
	}
	defer func() {
		cliLogger.Info("Closing HMC Session...")
		restClient.Logoff(context.Background())
	}()

	cliLogger.Debug("Resolving System", "system", sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), sysName, verbose)
	if err != nil || sysUUID == "" {
		cliLogger.Fatal("Failed to resolve Managed System", "system", sysName, "error", err)
	}

	var targetViosUUID string
	if viosName != "" {
		cliLogger.Debug("Resolving VIOS", "vios", viosName)
		targetViosUUID, err = hmc.GetViosID(context.Background(), restClient, sysUUID, viosName, verbose)
		if err != nil || targetViosUUID == "" {
			cliLogger.Fatal("VIOS not found on system", "vios", viosName, "system", sysName)
		}
	}

	// Check Context immediately before executing heavy operations
	if ctx.Err() != nil {
		cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
	}

	// =========================================================================
	// 4. EXECUTE COMMAND
	// =========================================================================

	switch cmd {

	// -------------------------------------------------------------------------
	// LIST MODE
	// -------------------------------------------------------------------------
	case "list":
		cliLogger.Info("Fetching Virtual Media Repository Inventory", "system", sysName)

		viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID, verbose)
		if err != nil || len(viosList) == 0 {
			cliLogger.Fatal("Failed to fetch VIOS instances for system", "system", sysName)
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
					cliLogger.Info("Media Repository Found",
						"vios", vios.PartitionName,
						"volume_group", vg.GroupName,
						"repo_name", vg.MediaRepositoryName,
						"capacity_gb", vg.MediaRepositorySize,
					)
				}
			}
		}

		if reposFound == 0 {
			cliLogger.Warn("No Virtual Media Repositories found matching your criteria.")
		} else {
			cliLogger.Info("Scan Complete", "total_repositories", reposFound)
		}

	// -------------------------------------------------------------------------
	// CREATE MODE
	// -------------------------------------------------------------------------
	case "create":
		cliLogger.Info("Initiating Media Repository Creation", "vios", viosName, "vg", vgName, "size_mb", repSize)

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		cliLogger.Debug("Verifying if a Media Repository already exists on this VIOS...")
		vgList, err := restClient.GetVolumeGroups(context.Background(), targetViosUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to fetch Volume Groups to verify repository existence", "error", err)
		}

		for _, vg := range vgList {
			if vg.MediaRepositoryName != "" {
				cliLogger.Info("Media Repository already exists. Skipping creation.", "vios", viosName, "vg", vg.GroupName, "repo_name", vg.MediaRepositoryName)
				os.Exit(0) // Idempotent Exit
			}
		}

		cliLogger.Info("Executing Media Repository creation via VIOS...", "size_mb", repSize)

		err = restClient.CreateMediaRepository(context.Background(), sysName, targetViosUUID, viosName, vgName, repSize, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to create Media Repository", "error", err)
		}

		cliLogger.Info("SUCCESS: Virtual Media Repository Created!", "vios", viosName, "vg", vgName)

	// -------------------------------------------------------------------------
	// EXTEND MODE
	// -------------------------------------------------------------------------
	case "extend":
		cliLogger.Info("Initiating Media Repository Extension", "vios", viosName, "add_size_mb", addSize)

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		cliLogger.Debug("Verifying if Media Repository exists...")
		vgList, err := restClient.GetVolumeGroups(context.Background(), targetViosUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to fetch Volume Groups to verify repository existence", "error", err)
		}

		repoExists := false
		for _, vg := range vgList {
			if vg.MediaRepositoryName != "" {
				repoExists = true
				break
			}
		}

		if !repoExists {
			cliLogger.Fatal("Virtual Media Repository does not exist on VIOS. Cannot extend.", "vios", viosName)
		}

		cliLogger.Info("Extending Media Repository", "vios", viosName, "add_mb", addSize)

		err = restClient.ChangeMediaRepository(context.Background(), sysName, targetViosUUID, viosName, addSize, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to extend Media Repository", "error", err)
		}

		cliLogger.Info("SUCCESS: Virtual Media Repository Extended!", "vios", viosName, "added_mb", addSize)

	// -------------------------------------------------------------------------
	// DELETE MODE
	// -------------------------------------------------------------------------
	case "delete":
		cliLogger.Warn("Initiating permanent Media Repository Deletion", "vios", viosName, "repo_name", repoName)

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		cliLogger.Debug("Verifying if Media Repository exists...")
		vgList, err := restClient.GetVolumeGroups(context.Background(), targetViosUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to fetch Volume Groups to verify repository existence", "error", err)
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
			cliLogger.Info("Virtual Media Repository not found on VIOS. No action needed.", "vios", viosName, "repo_name", repoName)
			os.Exit(0) // Idempotent Exit
		}

		cliLogger.Warn("Attempting to permanently delete Virtual Media Repository", "repo_name", repoName, "vios", viosName, "force", force)

		err = restClient.DeleteMediaRepository(context.Background(), sysName, targetViosUUID, viosName, repoName, force, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to delete Media Repository", "error", err)
		}

		cliLogger.Info("SUCCESS: Virtual Media Repository Deleted!", "repo_name", repoName, "vios", viosName)
	}
}