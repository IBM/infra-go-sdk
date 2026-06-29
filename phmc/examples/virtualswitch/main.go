package main

import (
	"log"
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

	if cmd == "get" || cmd == "delete" {
		if switchName == "" {
			log.Fatalf("Missing required argument for %s", cmd)
		}
	}
	if cmd == "create" {
		if switchName == "" {
			log.Fatal("Missing required argument for create")
		}
	}
	if cmd == "update" {
		if switchName == "" || (newSwitchName == "" && newSwitchMode == "") {
			log.Fatal("Missing required arguments for update")
		}
	}

	// =========================================================================
	// 3. AUTHENTICATION & SYSTEM RESOLUTION
	// =========================================================================
	log.Printf("Logging into HMC: ip=%v", hmcIP)
	restClient := hmc.NewRestClient(hmcIP)

	if err := restClient.Login(ctx, username, password, verbose); err != nil {
		log.Fatal("HMC Logon failed")
	}
	defer func() {
		log.Println("Closing HMC Session...")
		restClient.Logoff(context.Background())
	}()

	log.Printf("Resolving System: system=%v", sysName)
	// Fallback to searching quick all if GetManagedSystemByNameQuick doesn't exist yet
	systems, err := restClient.GetManagedSystemQuickAll(ctx, verbose)
	if err != nil {
		log.Fatal("Failed to get managed systems")
	}

	var sysUUID string
	for _, sys := range systems {
		if strings.EqualFold(sys.SystemName, sysName) {
			sysUUID = sys.UUID
			break
		}
	}

	if sysUUID == "" {
		log.Fatal("Managed System not found on HMC")
	}

	// Helper to resolve Switch Name -> UUID for Get/Update/Delete operations
	resolveSwitchUUID := func(name string) string {
		switches, err := restClient.GetVirtualSwitchQuickAll(ctx, sysUUID, verbose)
		if err != nil {
			log.Fatal("Failed to resolve Virtual Switches")
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
		log.Printf("Fetching Virtual Switch Inventory: system=%v", sysName)

		switches, err := restClient.GetVirtualSwitchQuickAll(ctx, sysUUID, verbose)
		if err != nil {
			log.Fatal("Failed to fetch Virtual Switches")
		}

		if len(switches) == 0 {
			log.Println("[WARN] No Virtual Switches found on this Managed System.")
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
		log.Printf("Scan Complete: total_switches=%v", len(switches))

	// -------------------------------------------------------------------------
	// GET MODE (Singular Detail)
	// -------------------------------------------------------------------------
	case "get":
		log.Printf("Looking up Virtual Switch to retrieve details: switch_name=%v", switchName)
		targetUUID := resolveSwitchUUID(switchName)
		if targetUUID == "" {
			log.Fatal("Virtual Switch not found")
		}

		log.Printf("Fetching detailed Virtual Switch config: uuid=%v", targetUUID)
		detailedSwitch, err := restClient.GetVirtualSwitch(ctx, sysUUID, targetUUID, verbose)
		if err != nil {
			log.Fatal("Failed to get Virtual Switch details")
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

		log.Printf("Provisioning Virtual Switch: name=%v mode=%v", switchName, switchMode)

		vSwitch, err := restClient.CreateVirtualSwitch(ctx, sysUUID, req, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to create Virtual Switch")
		}

		fmt.Println("\n=========================================================================")
		log.Println("✨ SUCCESS: Virtual Switch Provisioned!")
		fmt.Printf("   Switch Name:    %s\n", vSwitch.SwitchName)
		fmt.Printf("   Switch Mode:    %s\n", vSwitch.SwitchMode)
		fmt.Printf("   UUID:           %s\n", vSwitch.UUID)
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// UPDATE MODE
	// -------------------------------------------------------------------------
	case "update":
		log.Printf("Looking up Virtual Switch to update: switch_name=%v", switchName)
		targetUUID := resolveSwitchUUID(switchName)
		if targetUUID == "" {
			log.Fatal("Virtual Switch not found")
		}

		log.Printf("Updating Virtual Switch: uuid=%v", targetUUID)
		err := restClient.UpdateVirtualSwitch(ctx, sysUUID, targetUUID, newSwitchName, newSwitchMode, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to update Virtual Switch")
		}

		fmt.Println("\n=========================================================================")
		log.Println("✏️  SUCCESS: Virtual Switch Updated!")
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// DELETE MODE
	// -------------------------------------------------------------------------
	case "delete":
		log.Printf("Looking up Virtual Switch for permanent deletion: switch_name=%v", switchName)
		targetUUID := resolveSwitchUUID(switchName)
		if targetUUID == "" {
			log.Printf("Virtual Switch not found. No action needed.: switch_name=%v", switchName)
			os.Exit(0) // Idempotent
		}

		log.Printf("Attempting to delete Virtual Switch: switch_name=%v uuid=%v", switchName, targetUUID)
		err := restClient.DeleteVirtualSwitch(ctx, sysUUID, targetUUID, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to delete Virtual Switch")
		}

		fmt.Println("\n=========================================================================")
		log.Printf("🗑️  SUCCESS: Virtual Switch Deleted!: switch_name=%v", switchName)
		fmt.Println("=========================================================================")
	}
}