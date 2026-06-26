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

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Adjust to your actual package path
)

func printUsage() {
	fmt.Println("Usage: virtualnetwork <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  list    List all Virtual Networks (VLANs) on the Managed System")
	fmt.Println("  get     Get comprehensive details of a specific Virtual Network")
	fmt.Println("  create  Provision a new Virtual Network and bind it to a Virtual Switch")
	fmt.Println("  update  Rename an existing Virtual Network")
	fmt.Println("  delete  Permanently remove a Virtual Network")
	fmt.Println("\nUse 'virtualnetwork <command> -h' for more information about a specific command.")
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
	var hmcIP, username, password, sysName, netName string
	var verbose bool

	// Action-Specific Variables
	var vlanID int
	var vswitchName, newNetName string
	var taggedNetwork bool

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
	getCmd.StringVar(&netName, "net-name", "", "Name of the Virtual Network to retrieve (Required)")

	bindCommonFlags(createCmd)
	createCmd.StringVar(&netName, "net-name", "", "Name of the new Virtual Network (Required)")
	createCmd.IntVar(&vlanID, "vlan-id", 0, "VLAN ID for the network (Required)")
	createCmd.StringVar(&vswitchName, "vswitch", "ETHERNET0(Default)", "Target Virtual Switch name (Required)")
	createCmd.BoolVar(&taggedNetwork, "tagged", false, "Set to true if this is a tagged network (802.1Q)")

	bindCommonFlags(updateCmd)
	updateCmd.StringVar(&netName, "net-name", "", "Current name of the Virtual Network (Required)")
	updateCmd.StringVar(&newNetName, "new-name", "", "New name to assign to the Virtual Network (Required)")

	bindCommonFlags(deleteCmd)
	deleteCmd.StringVar(&netName, "net-name", "", "Name of the Virtual Network to delete (Required)")

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

	if cmd == "get" {
		if netName == "" {
			cliLogger.Fatal("Missing required argument for get", "required", "net-name")
		}
	}
	if cmd == "create" {
		if netName == "" || vlanID <= 0 {
			cliLogger.Fatal("Missing required argument for create", "required", "net-name, vlan-id")
		}
	}
	if cmd == "update" {
		if netName == "" || newNetName == "" {
			cliLogger.Fatal("Missing required argument for update", "required", "net-name, new-name")
		}
	}
	if cmd == "delete" {
		if netName == "" {
			cliLogger.Fatal("Missing required argument for delete", "required", "net-name")
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
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(ctx, sysName, verbose)
	if err != nil || sysUUID == "" {
		cliLogger.Fatal("Failed to resolve Managed System", "system", sysName, "error", err)
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
		cliLogger.Info("Fetching Virtual Network Inventory", "system", sysName)

		networks, err := restClient.GetVirtualNetworks(ctx, sysUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to fetch Virtual Networks", "error", err)
		}

		if len(networks) == 0 {
			cliLogger.Warn("No Virtual Networks found on this Managed System.")
			os.Exit(0)
		}

		fmt.Println("=====================================================================================================")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NETWORK NAME\tVLAN ID\tTAGGED\tUUID")
		fmt.Fprintln(w, "------------\t-------\t------\t----")

		for _, vnet := range networks {
			fmt.Fprintf(w, "%s\t%d\t%t\t%s\n", vnet.NetworkName, vnet.NetworkVLANID, vnet.TaggedNetwork, vnet.UUID)
		}

		w.Flush()
		fmt.Println("=====================================================================================================")
		cliLogger.Info("Scan Complete", "total_networks", len(networks))

	// -------------------------------------------------------------------------
	// GET MODE (Singular Detail)
	// -------------------------------------------------------------------------
	case "get":
		cliLogger.Info("Looking up Virtual Network to retrieve details", "net_name", netName)

		// 1. Fetch all networks to map the Name to the UUID
		networks, err := restClient.GetVirtualNetworks(ctx, sysUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to fetch Virtual Networks for resolution", "error", err)
		}

		var targetUUID string
		for _, vnet := range networks {
			if strings.EqualFold(vnet.NetworkName, netName) {
				targetUUID = vnet.UUID
				break
			}
		}

		if targetUUID == "" {
			cliLogger.Fatal("Virtual Network not found", "net_name", netName)
		}

		cliLogger.Info("Fetching detailed Virtual Network config", "uuid", targetUUID)
		
		// 2. Query the exact UUID
		detailedVnet, err := restClient.GetVirtualNetwork(ctx, sysUUID, targetUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to get Virtual Network details", "error", err)
		}

		fmt.Println("\n=========================================================================")
		fmt.Printf(" 🔍 DETAILS FOR VIRTUAL NETWORK: %s\n", detailedVnet.NetworkName)
		fmt.Println("=========================================================================")
		
		// 3. Marshal the native Go struct beautifully into JSON
		prettyJSON, _ := json.MarshalIndent(detailedVnet, "", "    ")
		fmt.Println(string(prettyJSON))
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// CREATE MODE
	// -------------------------------------------------------------------------
	case "create":
		cliLogger.Info("Resolving Target Virtual Switch", "vswitch", vswitchName)
		vswitches, err := restClient.GetVirtualSwitchQuickAll(ctx, sysUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to get Virtual Switches", "error", err)
		}

		var vswitchUUID string
		for _, vswitch := range vswitches {
			if strings.EqualFold(vswitch.SwitchName, vswitchName) {
				vswitchUUID = vswitch.UUID
				break
			}
		}

		if vswitchUUID == "" {
			cliLogger.Fatal("Virtual Switch not found on Managed System", "vswitch", vswitchName)
		}

		req := hmc.CreateVirtualNetworkRequest{
			NetworkName:   netName,
			NetworkVLANID: vlanID,
			TaggedNetwork: taggedNetwork,
			VSwitchUUID:   vswitchUUID,
		}

		cliLogger.Info("Provisioning Virtual Network", "name", netName, "vlan", vlanID)

		vnet, err := restClient.CreateVirtualNetwork(ctx, sysUUID, req, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to create Virtual Network", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("✨ SUCCESS: Virtual Network Provisioned!")
		fmt.Printf("   Network Name:   %s\n", vnet.NetworkName)
		fmt.Printf("   VLAN ID:        %d\n", vnet.NetworkVLANID)
		fmt.Printf("   Tagged:         %t\n", vnet.TaggedNetwork)
		fmt.Printf("   UUID:           %s\n", vnet.UUID)
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// UPDATE MODE
	// -------------------------------------------------------------------------
	case "update":
		cliLogger.Info("Looking up Virtual Network to update", "net_name", netName)

		networks, err := restClient.GetVirtualNetworks(ctx, sysUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to fetch Virtual Networks for resolution", "error", err)
		}

		var targetUUID string
		for _, vnet := range networks {
			if strings.EqualFold(vnet.NetworkName, netName) {
				targetUUID = vnet.UUID
				break
			}
		}

		if targetUUID == "" {
			cliLogger.Fatal("Virtual Network not found", "net_name", netName)
		}

		cliLogger.Info("Renaming Virtual Network", "old_name", netName, "new_name", newNetName)

		err = restClient.UpdateVirtualNetwork(ctx, sysUUID, targetUUID, newNetName, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to update Virtual Network", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("✏️  SUCCESS: Virtual Network Updated!", "new_name", newNetName)
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// DELETE MODE
	// -------------------------------------------------------------------------
	case "delete":
		cliLogger.Warn("Looking up Virtual Network for permanent deletion", "net_name", netName)

		networks, err := restClient.GetVirtualNetworks(ctx, sysUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to fetch Virtual Networks for resolution", "error", err)
		}

		var targetUUID string
		for _, vnet := range networks {
			if strings.EqualFold(vnet.NetworkName, netName) {
				targetUUID = vnet.UUID
				break
			}
		}

		if targetUUID == "" {
			cliLogger.Info("Virtual Network not found. No action needed.", "net_name", netName)
			os.Exit(0) // Idempotent
		}

		cliLogger.Warn("Attempting to delete Virtual Network", "net_name", netName, "uuid", targetUUID)

		err = restClient.DeleteVirtualNetwork(ctx, sysUUID, targetUUID, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to delete Virtual Network", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("🗑️  SUCCESS: Virtual Network Deleted!", "net_name", netName)
		fmt.Println("=========================================================================")
	}
}