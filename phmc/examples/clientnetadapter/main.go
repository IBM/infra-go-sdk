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
	"text/tabwriter"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
	exutil "github.com/IBM/infra-go-sdk/phmc/examples/exutil"
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
	var debug, debugFull bool
	var verbose bool

	// Action-Specific Variables
	var vswitchName, adapterUUID string
	var vlanID int

	// ✨ HELPER: Global flags used by ALL commands
	bindCommonFlags := func(fs *flag.FlagSet) {
		fs.StringVar(&hmcIP, "hmc-ip", "", "HMC IP address")
		fs.StringVar(&username, "hmc-user", "", "HMC username")
		fs.StringVar(&password, "hmc-pass", "", "HMC password")
		fs.BoolVar(&debug,     "debug",      false, "Log each HTTP request/response (bodies truncated at 2048 bytes)")
		fs.BoolVar(&debugFull, "debug-full",  false, "Log each HTTP request/response with full body (no truncation)")
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
	if password == "" || sysName == "" || lparName == "" {
		log.Fatal("Missing required arguments")
	}

	if cmd == "create" && vswitchName == "" {
		log.Fatal("Missing required argument")
	}

	if cmd == "delete" && adapterUUID == "" {
		log.Fatal("Missing required argument")
	}

	// =========================================================================
	// 3. AUTHENTICATION & SYSTEM/LPAR RESOLUTION
	// =========================================================================
	log.Printf("Logging into HMC: ip=%v", hmcIP)
	restClient := exutil.NewClient(hmcIP, debug, debugFull)

	if err := restClient.Login(context.Background(), username, password); err != nil {
		log.Fatal("HMC Logon failed")
	}
	defer func() {
		log.Println("Closing HMC Session...")
		restClient.Logoff(context.Background())
	}()

	log.Printf("Resolving System: system=%v", sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(context.Background(), sysName)
	if err != nil || sysUUID == "" {
		log.Fatal("Failed to resolve Managed System")
	}

	log.Printf("Resolving LPAR: lpar=%v", lparName)
	lparDetails, lparUUID, err := restClient.GetLogicalPartitionByName(context.Background(), sysUUID, lparName)
	if err != nil || lparUUID == "" {
		log.Fatal("Failed to resolve LPAR Name")
	}
	log.Printf("Target LPAR resolved: uuid=%v state=%v", lparUUID, lparDetails.PartitionState)

	// Check Context immediately before executing heavy operations
	if ctx.Err() != nil {
		log.Fatal("Operation aborted by user (Ctrl+C)")
	}

	// =========================================================================
	// 4. EXECUTE COMMAND (LIST, CREATE, OR DELETE)
	// =========================================================================

	switch cmd {

	// -------------------------------------------------------------------------
	// LIST MODE
	// -------------------------------------------------------------------------
	case "list":
		log.Printf("Fetching Virtual Ethernet Adapters: lpar=%v", lparName)

		adapters, err := restClient.GetClientNetworkAdapters(context.Background(), sysUUID, lparUUID)
		if err != nil {
			log.Fatal("Failed to retrieve client network adapters")
		}

		if len(adapters) == 0 {
			log.Println("[WARN] No Client Network Adapters found for this LPAR.")
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
		log.Printf("Scan Complete: total_found=%v", len(adapters))

	// -------------------------------------------------------------------------
	// CREATE MODE
	// -------------------------------------------------------------------------
	case "create":
		log.Printf("Resolving Target Virtual Switch: vswitch=%v", vswitchName)
		vswitches, err := restClient.GetVirtualSwitchQuickAll(context.Background(), sysUUID)
		if err != nil {
			log.Fatal("Failed to get Virtual Switches")
		}

		var vswitchUUID string
		for _, vswitch := range vswitches {
			if strings.EqualFold(vswitch.SwitchName, vswitchName) {
				vswitchUUID = vswitch.UUID
				break
			}
		}

		if vswitchUUID == "" {
			log.Fatal("Virtual Switch not found on Managed System")
		}

		log.Printf("Provisioning new Virtual Ethernet Adapter: vlan=%v vswitch=%v", vlanID, vswitchName)

		adapter, err := restClient.CreateClientNetworkAdapter(context.Background(), sysUUID, lparUUID, vswitchUUID, vlanID)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to provision Virtual Ethernet Adapter")
		}

		fmt.Println("\n=========================================================================")
		log.Println("✨ SUCCESS: Virtual Ethernet Adapter Provisioned!")
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
		log.Printf("Deleting Virtual Ethernet Adapter: adapter_uuid=%v", adapterUUID)

		err := restClient.DeleteClientNetworkAdapter(context.Background(), lparUUID, adapterUUID)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to delete Virtual Ethernet Adapter")
		}

		fmt.Println("\n=========================================================================")
		log.Printf("🗑️ SUCCESS: Virtual Ethernet Adapter Deleted!: adapter_uuid=%v", adapterUUID)
		fmt.Println("=========================================================================")
	}
}
