package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func printUsage() {
	fmt.Println("Usage: virtualswitch <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  list    List all Virtual Switches on the Managed System")
	fmt.Println("  get     Get comprehensive details of a specific Virtual Switch")
	fmt.Println("  create  Provision a new Virtual Switch")
	fmt.Println("  update  Modify an existing Virtual Switch (Rename or change mode)")
	fmt.Println("  delete  Permanently remove a Virtual Switch")
	fmt.Println("\nUse 'virtualswitch <command> -h' for more information about a specific command.")
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
	getCmd := flag.NewFlagSet("get", flag.ExitOnError)
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	updateCmd := flag.NewFlagSet("update", flag.ExitOnError)
	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)

	// Shared Variables
	var hmcIP, username, password, sysName, switchName string
	var verbose bool

	// Action-Specific Variables
	var switchMode, newSwitchName, newSwitchMode string

	// ✨ HELPER: Global flags used by ALL commands
	bindCommonFlags := func(fs *flag.FlagSet) {
		fs.StringVar(&hmcIP, "hmc-ip", "", "HMC IP address")
		fs.StringVar(&username, "hmc-user", "", "HMC username")
		fs.StringVar(&password, "hmc-pass", "", "HMC password")
		fs.StringVar(&sysName, "system-name", "", "Managed System Name (Required)")
		fs.BoolVar(&verbose, "verbose", false, "Enable verbose XML and HTTP output")
	}

	// --- Bind Flags to Subcommands ---
	bindCommonFlags(listCmd)

	bindCommonFlags(getCmd)
	getCmd.StringVar(&switchName, "switch-name", "", "Name of the Virtual Switch to retrieve (Required)")

	bindCommonFlags(createCmd)
	createCmd.StringVar(&switchName, "switch-name", "", "Name of the new Virtual Switch (Required)")
	createCmd.StringVar(&switchMode, "switch-mode", "Veb", "Switch Mode: 'Veb' or 'Vepa' (Default: Veb)")

	bindCommonFlags(updateCmd)
	updateCmd.StringVar(&switchName, "switch-name", "", "Current name of the Virtual Switch (Required)")
	updateCmd.StringVar(&newSwitchName, "new-name", "", "New name to assign to the Virtual Switch (Optional)")
	updateCmd.StringVar(&newSwitchMode, "new-mode", "", "New mode: 'Veb' or 'Vepa' (Optional)")

	bindCommonFlags(deleteCmd)
	deleteCmd.StringVar(&switchName, "switch-name", "", "Name of the Virtual Switch to delete (Required)")

	// Route the Subcommand
	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "list":
		listCmd.Parse(os.Args[2:])
	case "get":
		getCmd.Parse(os.Args[2:])
	case "create":
		createCmd.Parse(os.Args[2:])
	case "update":
		updateCmd.Parse(os.Args[2:])
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

	if cmd == "get" || cmd == "delete" {
		if switchName == "" {
			cliLogger.Fatal(fmt.Sprintf("Missing required argument for %s", cmd), "required", "switch-name")
		}
	}
	if cmd == "create" {
		if switchName == "" {
			cliLogger.Fatal("Missing required argument for create", "required", "switch-name")
		}
	}
	if cmd == "update" {
		if switchName == "" || (newSwitchName == "" && newSwitchMode == "") {
			cliLogger.Fatal("Missing required arguments for update", "required", "switch-name AND (new-name OR new-mode)")
		}
	}

	// =========================================================================
	// 3. AUTHENTICATION & SYSTEM RESOLUTION
	// =========================================================================
	cliLogger.Info("Logging into HMC", "ip", hmcIP)
	restClient := hmc.NewRestClient(hmcIP)

	if verbose {
		restClient.EnableVerboseLogging()
	}

	if err := restClient.Login(ctx, username, password, verbose); err != nil {
		cliLogger.Fatal("HMC Logon failed", "error", err)
	}
	defer func() {
		cliLogger.Info("Closing HMC Session...")
		restClient.Logoff(context.Background())
	}()

	cliLogger.Debug("Resolving System", "system", sysName)
	// Fallback to searching quick all if GetManagedSystemByNameQuick doesn't exist yet
	systems, err := restClient.GetManagedSystemQuickAll(ctx, verbose)
	if err != nil {
		cliLogger.Fatal("Failed to get managed systems", "error", err)
	}

	var sysUUID string
	for _, sys := range systems {
		if strings.EqualFold(sys.SystemName, sysName) {
			sysUUID = sys.UUID
			break
		}
	}

	if sysUUID == "" {
		cliLogger.Fatal("Managed System not found on HMC", "system", sysName)
	}

	// Helper to resolve Switch Name -> UUID for Get/Update/Delete operations
	resolveSwitchUUID := func(name string) string {
		switches, err := restClient.GetVirtualSwitchQuickAll(ctx, sysUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to resolve Virtual Switches", "error", err)
		}
		for _, sw := range switches {
			if strings.EqualFold(sw.SwitchName, name) {
				return sw.UUID
			}
		}
		return ""
	}

	// Check Context before heavy operations
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
		cliLogger.Info("Fetching Virtual Switch Inventory", "system", sysName)

		switches, err := restClient.GetVirtualSwitchQuickAll(ctx, sysUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to fetch Virtual Switches", "error", err)
		}

		if len(switches) == 0 {
			cliLogger.Warn("No Virtual Switches found on this Managed System.")
			os.Exit(0)
		}

		fmt.Println("=====================================================================================================")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "SWITCH NAME\tMODE\tUUID")
		fmt.Fprintln(w, "-----------\t----\t----")

		for _, sw := range switches {
			fmt.Fprintf(w, "%s\t%s\t%s\n", sw.SwitchName, sw.SwitchMode, sw.UUID)
		}

		w.Flush()
		fmt.Println("=====================================================================================================")
		cliLogger.Info("Scan Complete", "total_switches", len(switches))

	// -------------------------------------------------------------------------
	// GET MODE (Singular Detail)
	// -------------------------------------------------------------------------
	case "get":
		cliLogger.Info("Looking up Virtual Switch to retrieve details", "switch_name", switchName)
		targetUUID := resolveSwitchUUID(switchName)
		if targetUUID == "" {
			cliLogger.Fatal("Virtual Switch not found", "switch_name", switchName)
		}

		cliLogger.Info("Fetching detailed Virtual Switch config", "uuid", targetUUID)
		detailedSwitch, err := restClient.GetVirtualSwitch(ctx, sysUUID, targetUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to get Virtual Switch details", "error", err)
		}

		fmt.Println("\n=========================================================================")
		fmt.Printf(" 🔍 DETAILS FOR VIRTUAL SWITCH: %s\n", detailedSwitch.SwitchName)
		fmt.Println("=========================================================================")
		
		prettyJSON, _ := json.MarshalIndent(detailedSwitch, "", "    ")
		fmt.Println(string(prettyJSON))
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// CREATE MODE
	// -------------------------------------------------------------------------
	case "create":
		req := hmc.CreateVirtualSwitchRequest{
			SwitchName: switchName,
			SwitchMode: switchMode,
		}

		cliLogger.Info("Provisioning Virtual Switch", "name", switchName, "mode", switchMode)

		vSwitch, err := restClient.CreateVirtualSwitch(ctx, sysUUID, req, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to create Virtual Switch", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("✨ SUCCESS: Virtual Switch Provisioned!")
		fmt.Printf("   Switch Name:    %s\n", vSwitch.SwitchName)
		fmt.Printf("   Switch Mode:    %s\n", vSwitch.SwitchMode)
		fmt.Printf("   UUID:           %s\n", vSwitch.UUID)
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// UPDATE MODE
	// -------------------------------------------------------------------------
	case "update":
		cliLogger.Info("Looking up Virtual Switch to update", "switch_name", switchName)
		targetUUID := resolveSwitchUUID(switchName)
		if targetUUID == "" {
			cliLogger.Fatal("Virtual Switch not found", "switch_name", switchName)
		}

		cliLogger.Info("Updating Virtual Switch", "uuid", targetUUID)
		err := restClient.UpdateVirtualSwitch(ctx, sysUUID, targetUUID, newSwitchName, newSwitchMode, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to update Virtual Switch", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("✏️  SUCCESS: Virtual Switch Updated!")
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// DELETE MODE
	// -------------------------------------------------------------------------
	case "delete":
		cliLogger.Warn("Looking up Virtual Switch for permanent deletion", "switch_name", switchName)
		targetUUID := resolveSwitchUUID(switchName)
		if targetUUID == "" {
			cliLogger.Info("Virtual Switch not found. No action needed.", "switch_name", switchName)
			os.Exit(0) // Idempotent
		}

		cliLogger.Warn("Attempting to delete Virtual Switch", "switch_name", switchName, "uuid", targetUUID)
		err := restClient.DeleteVirtualSwitch(ctx, sysUUID, targetUUID, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to delete Virtual Switch", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("🗑️  SUCCESS: Virtual Switch Deleted!", "switch_name", switchName)
		fmt.Println("=========================================================================")
	}
}