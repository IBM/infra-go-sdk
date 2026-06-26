package main

import (
	"context"
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
	fmt.Println("Usage: clientnetadapter <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  list    List all Virtual Ethernet Adapters on the specified LPAR")
	fmt.Println("  create  Provision a new Virtual Ethernet Adapter to the LPAR")
	fmt.Println("  delete  Remove a Virtual Ethernet Adapter from the LPAR")
	fmt.Println("\nUse 'clientnetadapter <command> -h' for more information about a specific command.")
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
	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)

	// Shared Variables
	var hmcIP, username, password, sysName, lparName string
	var verbose bool

	// Action-Specific Variables
	var vswitchName, adapterUUID string
	var vlanID int

	// ✨ HELPER: Global flags used by ALL commands
	bindCommonFlags := func(fs *flag.FlagSet) {
		fs.StringVar(&hmcIP, "hmc-ip", "", "HMC IP address")
		fs.StringVar(&username, "hmc-user", "", "HMC username")
		fs.StringVar(&password, "hmc-pass", "", "HMC password")
		fs.StringVar(&sysName, "system-name", "", "Managed System Name (Required)")
		fs.StringVar(&lparName, "lpar-name", "ocp-sno-lpar", "Target LPAR Name (Required)")
		fs.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	}

	// --- Bind Flags to Subcommands ---
	bindCommonFlags(listCmd)

	bindCommonFlags(createCmd)
	createCmd.StringVar(&vswitchName, "vswitch", "ETHERNET0(Default)", "Target Virtual Switch name (Required for create)")
	createCmd.IntVar(&vlanID, "vlan", 1, "VLAN ID for the new adapter (Required for create)")

	bindCommonFlags(deleteCmd)
	deleteCmd.StringVar(&adapterUUID, "adapter-uuid", "", "UUID of the Client Network Adapter to delete (Required for delete)")

	// Route the Subcommand
	cmd := strings.ToLower(os.Args[1])
	switch cmd {
	case "list":
		listCmd.Parse(os.Args[2:])
	case "create":
		createCmd.Parse(os.Args[2:])
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
		cliLogger.SetLevel(0) // 0 equates to InfoLevel
	}

	// --- Shared Validation ---
	if password == "" || sysName == "" || lparName == "" {
		cliLogger.Fatal("Missing required arguments", "required", "hmc-pass, system-name, lpar-name")
	}

	if cmd == "create" && vswitchName == "" {
		cliLogger.Fatal("Missing required argument", "required", "vswitch")
	}

	if cmd == "delete" && adapterUUID == "" {
		cliLogger.Fatal("Missing required argument", "required", "adapter-uuid")
	}

	// =========================================================================
	// 3. AUTHENTICATION & SYSTEM/LPAR RESOLUTION
	// =========================================================================
	cliLogger.Info("Logging into HMC", "ip", hmcIP)
	restClient := hmc.NewRestClient(hmcIP)

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

	cliLogger.Debug("Resolving LPAR", "lpar", lparName)
	lparDetails, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, lparName, verbose)
	if err != nil || lparUUID == "" {
		cliLogger.Fatal("Failed to resolve LPAR Name", "lpar", lparName, "error", err)
	}
	cliLogger.Info("Target LPAR resolved", "uuid", lparUUID, "state", lparDetails.PartitionState)

	// Check Context immediately before executing heavy operations
	if ctx.Err() != nil {
		cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
	}

	// =========================================================================
	// 4. EXECUTE COMMAND (LIST, CREATE, OR DELETE)
	// =========================================================================

	switch cmd {

	// -------------------------------------------------------------------------
	// LIST MODE
	// -------------------------------------------------------------------------
	case "list":
		cliLogger.Info("Fetching Virtual Ethernet Adapters", "lpar", lparName)

		adapters, err := restClient.GetClientNetworkAdapters(context.Background(), sysUUID, lparUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to retrieve client network adapters", "error", err)
		}

		if len(adapters) == 0 {
			cliLogger.Warn("No Client Network Adapters found for this LPAR.")
			os.Exit(0)
		}

		fmt.Printf("\n📋 Client Network Adapters for '%s':\n", lparName)
		fmt.Println("=====================================================================================================")

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ADAPTER UUID\tMAC ADDRESS\tVLAN ID\tVIRTUAL SWITCH\tSLOT")
		fmt.Fprintln(w, "------------\t-----------\t-------\t--------------\t----")

		for _, a := range adapters {
			mac := hmc.FormatMACAddress(a.MACAddress)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", a.UUID, mac, a.PortVLANID, a.VirtualSwitchName, a.VirtualSlotNumber)
		}
		w.Flush()
		fmt.Println("=====================================================================================================")
		cliLogger.Info("Scan Complete", "total_found", len(adapters))

	// -------------------------------------------------------------------------
	// CREATE MODE
	// -------------------------------------------------------------------------
	case "create":
		cliLogger.Info("Resolving Target Virtual Switch", "vswitch", vswitchName)
		vswitches, err := restClient.GetVirtualSwitchQuickAll(context.Background(), sysUUID, verbose)
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

		cliLogger.Info("Provisioning new Virtual Ethernet Adapter", "vlan", vlanID, "vswitch", vswitchName)

		adapter, err := restClient.CreateClientNetworkAdapter(context.Background(), sysUUID, lparUUID, vswitchUUID, vlanID, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to provision Virtual Ethernet Adapter", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("✨ SUCCESS: Virtual Ethernet Adapter Provisioned!")
		fmt.Printf("   Adapter UUID:   %s\n", adapter.UUID)
		fmt.Printf("   MAC Address:    %s\n", hmc.FormatMACAddress(adapter.MACAddress))
		fmt.Printf("   VLAN ID:        %s\n", adapter.PortVLANID)
		fmt.Printf("   Virtual Slot:   %s\n", adapter.VirtualSlotNumber)
		fmt.Printf("   Location Code:  %s\n", adapter.LocationCode)
		fmt.Printf("   Virtual Switch: %s\n", adapter.VirtualSwitchName)
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// DELETE MODE
	// -------------------------------------------------------------------------
	case "delete":
		cliLogger.Info("Deleting Virtual Ethernet Adapter", "adapter_uuid", adapterUUID)

		err := restClient.DeleteClientNetworkAdapter(context.Background(), lparUUID, adapterUUID, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to delete Virtual Ethernet Adapter", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("🗑️ SUCCESS: Virtual Ethernet Adapter Deleted!", "adapter_uuid", adapterUUID)
		fmt.Println("=========================================================================")
	}
}