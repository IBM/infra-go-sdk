package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc"
)

func printUsage() {
	fmt.Println("Usage: networkbridge <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  list    List all Network Bridges on the Managed System")
	fmt.Println("  get     Get comprehensive details of a specific Network Bridge")
	fmt.Println("  create  Provision a new Network Bridge")
	fmt.Println("  update  Modify an existing Network Bridge configuration")
	fmt.Println("  delete  Permanently remove a Network Bridge configuration")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cliLogger := hmc.NewDefaultLogger()
	cliLogger.SetPrefix("[CLI]")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	getCmd := flag.NewFlagSet("get", flag.ExitOnError)
	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	updateCmd := flag.NewFlagSet("update", flag.ExitOnError)
	deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)

	var hmcIP, username, password, sysName, loadGroupVlanStr string
	var portVlan, controlChannel int
	var verbose, failover, loadBalancing, largeSend, jumboFrames bool
	var primaryVios, primaryDev, secondaryVios, secondaryDev, dummyVswitch string

	bindCommonFlags := func(fs *flag.FlagSet) {
		fs.StringVar(&hmcIP, "hmc-ip", "", "HMC IP address")
		fs.StringVar(&username, "hmc-user", "", "HMC username")
		fs.StringVar(&password, "hmc-pass", "", "HMC password")
		fs.StringVar(&sysName, "system-name", "", "Managed System Name")
		fs.BoolVar(&verbose, "verbose", false, "Enable verbose XML and HTTP traffic logs")
	}

	bindCommonFlags(listCmd)
	bindCommonFlags(getCmd)
	getCmd.IntVar(&portVlan, "port-vlan", 0, "Port VLAN ID (Required)")

	bindCommonFlags(createCmd)
	createCmd.IntVar(&portVlan, "port-vlan", 0, "The untagged Port VLAN ID (Required)")
	createCmd.StringVar(&dummyVswitch, "vswitch", "", "Target Virtual Switch name (Automatically derived during network lookup tracking)")
	createCmd.BoolVar(&failover, "failover", false, "Enable SEA Failover architecture loops")
	createCmd.BoolVar(&loadBalancing, "load-balancing", false, "Enable Load Balancing splits")
	createCmd.BoolVar(&largeSend, "large-send", false, "Enable Large Send packet optimizations")
	createCmd.BoolVar(&jumboFrames, "jumbo-frames", false, "Enable Jumbo Frames (MTU 9000)")
	createCmd.IntVar(&controlChannel, "ctrl-chan", 0, "Control Channel VLAN ID")
	createCmd.StringVar(&primaryVios, "primary-vios", "", "Name of primary target VIOS partition")
	createCmd.StringVar(&primaryDev, "primary-dev", "", "Primary backing physical network dev asset")
	createCmd.StringVar(&secondaryVios, "secondary-vios", "", "Name of secondary target VIOS partition")
	createCmd.StringVar(&secondaryDev, "secondary-dev", "", "Secondary backing physical network dev asset")
	createCmd.StringVar(&loadGroupVlanStr, "load-group-vlans", "", "Comma-separated tracking data VLANs (Required for active/active setups, e.g., 1127,1128)")

	bindCommonFlags(updateCmd)
	updateCmd.IntVar(&portVlan, "port-vlan", 0, "Port VLAN ID (Required)")
	updateCmd.BoolVar(&failover, "failover", false, "Toggle SEA Failover tracking loop state")
	updateCmd.BoolVar(&loadBalancing, "load-balancing", false, "Toggle Load Balancing active paths state")
	updateCmd.BoolVar(&largeSend, "large-send", false, "Toggle Large Send options")
	updateCmd.BoolVar(&jumboFrames, "jumbo-frames", false, "Toggle Jumbo Frames parameters")

	bindCommonFlags(deleteCmd)
	deleteCmd.IntVar(&portVlan, "port-vlan", 0, "Port VLAN ID (Required)")

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
	default:
		printUsage()
		os.Exit(1)
	}

	if verbose {
		cliLogger.EnableDebug()
	}

	cliLogger.Info("Logging into HMC", "ip", hmcIP)
	restClient := hmc.NewRestClient(hmcIP)
	if verbose {
		restClient.EnableVerboseLogging()
	}
	if err := restClient.Login(ctx, username, password, verbose); err != nil {
		cliLogger.Fatal("HMC Logon failed", "error", err)
	}
	defer restClient.Logoff(context.Background())

	systems, err := restClient.GetManagedSystemQuickAll(ctx, verbose)
	if err != nil {
		cliLogger.Fatal("Failed to fetch tracking configuration matrices", "error", err)
	}
	var sysUUID string
	for _, sys := range systems {
		if strings.EqualFold(sys.SystemName, sysName) {
			sysUUID = sys.UUID
			break
		}
	}
	if sysUUID == "" {
		cliLogger.Fatal("Managed System target identifier could not be verified", "name", sysName)
	}

	resolveBridgeUUID := func(vlan int) string {
		bridges, _ := restClient.GetNetworkBridges(ctx, sysUUID, verbose)
		for _, b := range bridges {
			if b.PortVLANID == vlan {
				return b.UUID
			}
		}
		return ""
	}

	switch cmd {
	case "list":
		bridges, err := restClient.GetNetworkBridges(ctx, sysUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to list active bridges", "error", err)
		}
		fmt.Println("=====================================================================================================")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "PORT VLAN ID\tFAILOVER\tLOAD BALANCING\tSEAs\tUUID")
		fmt.Fprintln(w, "------------\t--------\t--------------\t----\t----")
		for _, b := range bridges {
			fmt.Fprintf(w, "%d\t%t\t%t\t%d\t%s\n", b.PortVLANID, b.FailoverEnabled, b.LoadBalancingEnabled, len(b.SharedEthernetAdapters), b.UUID)
		}
		w.Flush()
		fmt.Println("=====================================================================================================")

	case "get":
		uuid := resolveBridgeUUID(portVlan)
		if uuid == "" {
			cliLogger.Fatal("Network Bridge target context could not be resolved for VLAN tag", "vlan", portVlan)
		}
		bridge, err := restClient.GetNetworkBridge(ctx, sysUUID, uuid, verbose)
		if err != nil {
			cliLogger.Fatal("Get metadata processing operational execution phase failed", "error", err)
		}
		pretty, _ := json.MarshalIndent(bridge, "", "  ")
		fmt.Println(string(pretty))

	case "create":
		primaryViosUUID, _ := hmc.GetViosID(ctx, restClient, sysUUID, primaryVios, verbose)
		secondaryViosUUID := ""
		if failover {
			secondaryViosUUID, _ = hmc.GetViosID(ctx, restClient, sysUUID, secondaryVios, verbose)
		}

		var loadBalancedVLANs []int
		if loadGroupVlanStr != "" {
			parts := strings.Split(loadGroupVlanStr, ",")
			for _, p := range parts {
				v, err := strconv.Atoi(strings.TrimSpace(p))
				if err != nil {
					cliLogger.Fatal("Malformed load-group-vlans parameters detected; integer casting aborted", "token", p)
				}
				loadBalancedVLANs = append(loadBalancedVLANs, v)
			}
		}

		req := hmc.CreateNetworkBridgeRequest{
			PortVLANID:             portVlan,
			FailoverEnabled:        failover,
			LoadBalancingEnabled:   loadBalancing,
			LargeSend:              largeSend,
			JumboFramesEnabled:     jumboFrames,
			ControlChannelID:       controlChannel,
			PrimaryViosUUID:        primaryViosUUID,
			PrimaryBackingDevice:   primaryDev,
			SecondaryViosUUID:      secondaryViosUUID,
			SecondaryBackingDevice: secondaryDev,
			LoadGroupVLANs:         loadBalancedVLANs,
		}

		bridge, err := restClient.CreateNetworkBridge(ctx, sysUUID, req, verbose)
		if err != nil {
			cliLogger.Fatal("Bridge deployment orchestration routine mapping failure", "error", err)
		}
		cliLogger.Info("✨ SUCCESS: Network Bridge Created!", "uuid", bridge.UUID)

	case "update":
		uuid := resolveBridgeUUID(portVlan)
		if uuid == "" {
			cliLogger.Fatal("Network Bridge target asset matching parameters not found", "vlan", portVlan)
		}
		err := restClient.UpdateNetworkBridge(ctx, sysUUID, uuid, failover, loadBalancing, largeSend, jumboFrames, verbose)
		if err != nil {
			cliLogger.Fatal("Target asset modification transaction rejected", "error", err)
		}
		cliLogger.Info("✏️ SUCCESS: Network Bridge Configuration Updated!")

	case "delete":
		uuid := resolveBridgeUUID(portVlan)
		if uuid == "" {
			cliLogger.Info("Network profile configuration space is already clean. Skipping execution.")
			return
		}
		if err := restClient.DeleteNetworkBridge(ctx, sysUUID, uuid, verbose); err != nil {
			cliLogger.Fatal("Target asset destructive decommissioning transaction rejected", "error", err)
		}
		cliLogger.Info("🗑️ SUCCESS: Network Bridge Configurations Swept!")
	}
}