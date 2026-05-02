package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"

	hmc "github.com/sudeeshjohn/powerhmc-go" // Adjust to your actual package path
)

func printUsage() {
	fmt.Println("Usage: virtualdisk <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  list    List Virtual Disks (LVs) on the VIOS (supports filtering by VIOS/VG)")
	fmt.Println("  create  Provision a new Virtual Disk on a VIOS (supports auto-discovery)")
	fmt.Println("  extend  Increase the capacity of an existing Virtual Disk")
	fmt.Println("  delete  Permanently remove a Virtual Disk from a VIOS")
	fmt.Println("\nUse 'virtualdisk <command> -h' for more information about a specific command.")
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
	var hmcIP, username, password, sysName, viosName, diskName string
	var verbose bool

	// Action-Specific Variables
	var vgName string
	var diskSize, addSize int

	// ✨ HELPER 1: Global flags used by ALL commands
	bindCommonFlags := func(fs *flag.FlagSet) {
		fs.StringVar(&hmcIP, "hmc-ip", "192.0.2.1", "HMC IP address")
		fs.StringVar(&username, "hmc-user", "REDACTED_HMC_USER<==", "HMC username")
		fs.StringVar(&password, "hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
		fs.StringVar(&sysName, "system-name", "LTC09U31-ZZ", "Managed System Name (Required)")
		fs.StringVar(&viosName, "vios-name", "", "Target VIOS Name (Optional for list/create, Required for extend/delete)")
		fs.BoolVar(&verbose, "verbose", false, "Enable verbose output")
	}

	// ✨ HELPER 2: Disk mutation flags (create, extend, delete)
	bindDiskFlags := func(fs *flag.FlagSet, action string) {
		bindCommonFlags(fs)
		fs.StringVar(&diskName, "disk-name", "auto_lv01", fmt.Sprintf("Name of the Virtual Disk to %s (Required)", action))
	}

	// --- Bind Flags to Subcommands ---
	bindCommonFlags(listCmd)
	listCmd.StringVar(&vgName, "vg-name", "", "Filter by Target Volume Group (Optional)")

	bindDiskFlags(createCmd, "create")
	createCmd.StringVar(&vgName, "vg-name", "", "Target Volume Group (If empty, safely auto-selects the best VG)")
	createCmd.IntVar(&diskSize, "disk-size", 10240, "Size of the Virtual Disk in Megabytes (Required)")

	bindDiskFlags(extendCmd, "extend")
	extendCmd.IntVar(&addSize, "add-size", 10240, "Amount of additional space to add in Megabytes (Required)")

	bindDiskFlags(deleteCmd, "delete")

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

	if cmd == "create" {
		if diskName == "" { cliLogger.Fatal("Missing required argument", "required", "disk-name") }
		if diskSize <= 0 { cliLogger.Fatal("Invalid argument", "error", "disk-size must be greater than 0") }
	}
	if cmd == "extend" {
		if viosName == "" { cliLogger.Fatal("Missing required argument", "required", "vios-name") }
		if diskName == "" { cliLogger.Fatal("Missing required argument", "required", "disk-name") }
		if addSize <= 0 { cliLogger.Fatal("Invalid argument", "error", "add-size must be greater than 0") }
	}
	if cmd == "delete" {
		if viosName == "" { cliLogger.Fatal("Missing required argument", "required", "vios-name") }
		if diskName == "" { cliLogger.Fatal("Missing required argument", "required", "disk-name") }
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
		cliLogger.Info("Fetching Virtual Disk Inventory", "system", sysName)

		viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID, verbose)
		if err != nil || len(viosList) == 0 {
			cliLogger.Fatal("Failed to fetch VIOS instances for system", "system", sysName)
		}

		fmt.Println("=====================================================================================================")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "VIOS NAME\tVOLUME GROUP\tDISK NAME\tCAPACITY (GB)\tUNIQUE DEVICE ID (UDID)")
		fmt.Fprintln(w, "---------\t------------\t---------\t-------------\t-----------------------")

		totalDisks := 0

		for _, vios := range viosList {
			if viosName != "" && !strings.EqualFold(vios.PartitionName, viosName) {
				continue
			}

			vgList, err := restClient.GetVolumeGroups(context.Background(), vios.UUID, verbose)
			if err != nil {
				continue
			}

			for _, vg := range vgList {
				if vgName != "" && !strings.EqualFold(vg.GroupName, vgName) {
					continue
				}

				for _, vd := range vg.VirtualDisks {
					totalDisks++
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", vios.PartitionName, vg.GroupName, vd.DiskName, vd.DiskCapacity, vd.UniqueDeviceID)
				}
			}
		}

		w.Flush()
		fmt.Println("=====================================================================================================")
		
		if totalDisks == 0 {
			cliLogger.Warn("No Virtual Disks found matching your criteria.")
		} else {
			cliLogger.Info("Scan Complete", "total_disks_found", totalDisks)
		}

	// -------------------------------------------------------------------------
	// CREATE MODE
	// -------------------------------------------------------------------------
	case "create":
		cliLogger.Info("Initiating Virtual Disk Creation", "disk_name", diskName, "disk_size_mb", diskSize)
		requiredGB := float64(diskSize) / 1024.0

		cliLogger.Debug("Fetching VIOS inventory for Smart Capacity Discovery...")
		viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID, verbose)
		if err != nil || len(viosList) == 0 {
			cliLogger.Fatal("Failed to fetch VIOS instances for system", "system", sysName)
		}

		var targetViosUUID, targetViosName, targetVgName string
		var usingRootVgFallback bool

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		cliLogger.Info("Verifying if Virtual Disk already exists...")
		diskExists := false
		existingVios := ""
		existingVg := ""

		for _, vios := range viosList {
			if viosName != "" && !strings.EqualFold(vios.PartitionName, viosName) {
				continue
			}
			vgList, err := restClient.GetVolumeGroups(context.Background(), vios.UUID, verbose)
			if err != nil {
				continue
			}
			for _, vg := range vgList {
				for _, vd := range vg.VirtualDisks {
					if strings.EqualFold(vd.DiskName, diskName) {
						diskExists = true
						existingVios = vios.PartitionName
						existingVg = vg.GroupName
						break
					}
				}
				if diskExists { break }
			}
			if diskExists { break }
		}

		// Idempotent Exit
		if diskExists {
			fmt.Println("\n=========================================================================")
			cliLogger.Info("Virtual Disk already exists. Skipping creation.", "disk_name", diskName, "vios", existingVios, "vg", existingVg)
			fmt.Println("=========================================================================")
			os.Exit(0)
		}

		// --- SMART CAPACITY SCAN ---
		cliLogger.Info("Scanning for optimal Volume Group", "required_gb", requiredGB)
		for _, vios := range viosList {
			if viosName != "" && !strings.EqualFold(vios.PartitionName, viosName) { continue }

			vgList, err := restClient.GetVolumeGroups(context.Background(), vios.UUID, verbose)
			if err != nil { continue }

			for _, vg := range vgList {
				if vgName != "" && !strings.EqualFold(vg.GroupName, vgName) { continue }

				freeSpaceGB, parseErr := strconv.ParseFloat(vg.FreeSpace, 64)
				if parseErr != nil { continue }

				cliLogger.Debug("Checked Volume Group capacity", "vios", vios.PartitionName, "vg", vg.GroupName, "free_gb", freeSpaceGB)

				if freeSpaceGB >= requiredGB {
					if vgName == "" {
						// Smart selection: Avoid rootvg if possible
						if strings.ToLower(vg.GroupName) == "rootvg" {
							if targetVgName == "" {
								targetViosUUID = vios.UUID
								targetViosName = vios.PartitionName
								targetVgName = vg.GroupName
								usingRootVgFallback = true
								cliLogger.Warn("rootvg has space. Keeping as fallback, but searching for data VG...")
							}
						} else {
							targetViosUUID = vios.UUID
							targetViosName = vios.PartitionName
							targetVgName = vg.GroupName
							usingRootVgFallback = false
							cliLogger.Info("PERFECT MATCH! Selected data VG", "vg", targetVgName, "vios", targetViosName)
							break
						}
					} else {
						// Explicit match requested
						targetViosUUID = vios.UUID
						targetViosName = vios.PartitionName
						targetVgName = vg.GroupName
						cliLogger.Info("MATCH FOUND! Selected requested VG", "vg", targetVgName, "vios", targetViosName)
						break
					}
				}
			}
			if targetVgName != "" && !usingRootVgFallback {
				break
			}
		}

		if targetVgName == "" {
			if vgName != "" {
				cliLogger.Fatal("Volume Group either does not exist or has insufficient free space", "vg", vgName, "required_gb", requiredGB)
			} else {
				cliLogger.Fatal("System Exhaustion: Could not find any Volume Group with sufficient free space", "required_gb", requiredGB)
			}
		} else if usingRootVgFallback {
			cliLogger.Warn("No data Volume Groups had enough space. Falling back to rootvg", "vg", targetVgName, "vios", targetViosName)
		}

		cliLogger.Info("Executing Virtual Disk creation via VIOS...", "disk", diskName, "size_mb", diskSize, "vg", targetVgName)

		err = restClient.CreateVirtualDisk(context.Background(), sysName, targetViosUUID, targetViosName, targetVgName, diskName, diskSize, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to create Virtual Disk", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("SUCCESS: Virtual Disk Created!", "disk_name", diskName, "vios", targetViosName, "vg", targetVgName)
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// EXTEND MODE
	// -------------------------------------------------------------------------
	case "extend":
		cliLogger.Info("Initiating Virtual Disk Extension", "disk_name", diskName, "add_size_mb", addSize, "vios", viosName)

		cliLogger.Debug("Resolving VIOS UUID", "vios", viosName)
		viosUUID, err := hmc.GetViosID(context.Background(), restClient, sysUUID, viosName, verbose)
		if err != nil || viosUUID == "" {
			cliLogger.Fatal("VIOS not found on system", "vios", viosName, "system", sysName)
		}

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		cliLogger.Info("Verifying if Virtual Disk exists...")
		vgList, err := restClient.GetVolumeGroups(context.Background(), viosUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to fetch Volume Groups to verify disk existence", "error", err)
		}

		diskExists := false
		for _, vg := range vgList {
			for _, vd := range vg.VirtualDisks {
				if strings.EqualFold(vd.DiskName, diskName) {
					diskExists = true
					break
				}
			}
			if diskExists { break }
		}

		if !diskExists {
			cliLogger.Fatal("Virtual Disk does not exist on VIOS. Cannot extend.", "disk_name", diskName, "vios", viosName)
		}

		cliLogger.Info("Extending Virtual Disk", "disk", diskName, "vios", viosName, "add_mb", addSize)

		err = restClient.ExtendVirtualDisk(context.Background(), sysName, viosUUID, viosName, diskName, addSize, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to extend Virtual Disk", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("SUCCESS: Virtual Disk Extended!", "disk_name", diskName, "added_mb", addSize)
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// DELETE MODE
	// -------------------------------------------------------------------------
	case "delete":
		cliLogger.Warn("Initiating Virtual Disk Deletion", "disk_name", diskName, "vios", viosName)

		cliLogger.Debug("Resolving VIOS UUID", "vios", viosName)
		viosUUID, err := hmc.GetViosID(context.Background(), restClient, sysUUID, viosName, verbose)
		if err != nil || viosUUID == "" {
			cliLogger.Fatal("VIOS not found on system", "vios", viosName, "system", sysName)
		}

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		cliLogger.Info("Verifying if Virtual Disk exists...")
		vgList, err := restClient.GetVolumeGroups(context.Background(), viosUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to fetch Volume Groups to verify disk existence", "error", err)
		}

		diskExists := false
		for _, vg := range vgList {
			for _, vd := range vg.VirtualDisks {
				if strings.EqualFold(vd.DiskName, diskName) {
					diskExists = true
					break
				}
			}
			if diskExists { break }
		}

		// Idempotent Exit
		if !diskExists {
			fmt.Println("\n=========================================================================")
			cliLogger.Info("Virtual Disk not found on VIOS. No action needed.", "disk_name", diskName, "vios", viosName)
			fmt.Println("=========================================================================")
			os.Exit(0)
		}

		cliLogger.Warn("Attempting to permanently delete Virtual Disk", "disk", diskName, "vios", viosName)

		err = restClient.DeleteVirtualDisk(context.Background(), sysName, viosName, diskName, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Failed to delete Virtual Disk", "error", err)
		}

		fmt.Println("\n=========================================================================")
		cliLogger.Info("SUCCESS: Virtual Disk Deleted!", "disk_name", diskName)
		fmt.Println("=========================================================================")
	}
}