package main

import (
	"log"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
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
		fs.StringVar(&hmcIP, "hmc-ip", "", "HMC IP address")
		fs.StringVar(&username, "hmc-user", "", "HMC username")
		fs.StringVar(&password, "hmc-pass", "", "HMC password")
		fs.StringVar(&sysName, "system-name", "", "Managed System Name (Required)")
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

	if cmd == "create" {
		if diskName == "" { log.Fatal("Missing required argument") }
		if diskSize <= 0 { log.Fatal("Invalid argument") }
	}
	if cmd == "extend" {
		if viosName == "" { log.Fatal("Missing required argument") }
		if diskName == "" { log.Fatal("Missing required argument") }
		if addSize <= 0 { log.Fatal("Invalid argument") }
	}
	if cmd == "delete" {
		if viosName == "" { log.Fatal("Missing required argument") }
		if diskName == "" { log.Fatal("Missing required argument") }
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
		log.Printf("Fetching Virtual Disk Inventory: system=%v", sysName)

		viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID, verbose)
		if err != nil || len(viosList) == 0 {
			log.Fatal("Failed to fetch VIOS instances for system")
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
			log.Println("[WARN] No Virtual Disks found matching your criteria.")
		} else {
			log.Printf("Scan Complete: total_disks_found=%v", totalDisks)
		}

	// -------------------------------------------------------------------------
	// CREATE MODE
	// -------------------------------------------------------------------------
	case "create":
		log.Printf("Initiating Virtual Disk Creation: disk_name=%v disk_size_mb=%v", diskName, diskSize)
		requiredGB := float64(diskSize) / 1024.0

		log.Println("Fetching VIOS inventory for Smart Capacity Discovery...")
		viosList, err := restClient.GetVirtualIOServersQuick(context.Background(), sysUUID, verbose)
		if err != nil || len(viosList) == 0 {
			log.Fatal("Failed to fetch VIOS instances for system")
		}

		var targetViosUUID, targetViosName, targetVgName string
		var usingRootVgFallback bool

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		log.Println("Verifying if Virtual Disk already exists...")
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
			log.Printf("Virtual Disk already exists. Skipping creation.: disk_name=%v vios=%v vg=%v", diskName, existingVios, existingVg)
			fmt.Println("=========================================================================")
			os.Exit(0)
		}

		// --- SMART CAPACITY SCAN ---
		log.Printf("Scanning for optimal Volume Group: required_gb=%v", requiredGB)
		for _, vios := range viosList {
			if viosName != "" && !strings.EqualFold(vios.PartitionName, viosName) { continue }

			vgList, err := restClient.GetVolumeGroups(context.Background(), vios.UUID, verbose)
			if err != nil { continue }

			for _, vg := range vgList {
				if vgName != "" && !strings.EqualFold(vg.GroupName, vgName) { continue }

				freeSpaceGB, parseErr := strconv.ParseFloat(vg.FreeSpace, 64)
				if parseErr != nil { continue }

				log.Printf("Checked Volume Group capacity: vios=%v vg=%v free_gb=%v", vios.PartitionName, vg.GroupName, freeSpaceGB)

				if freeSpaceGB >= requiredGB {
					if vgName == "" {
						// Smart selection: Avoid rootvg if possible
						if strings.ToLower(vg.GroupName) == "rootvg" {
							if targetVgName == "" {
								targetViosUUID = vios.UUID
								targetViosName = vios.PartitionName
								targetVgName = vg.GroupName
								usingRootVgFallback = true
								log.Println("[WARN] rootvg has space. Keeping as fallback, but searching for data VG...")
							}
						} else {
							targetViosUUID = vios.UUID
							targetViosName = vios.PartitionName
							targetVgName = vg.GroupName
							usingRootVgFallback = false
							log.Printf("PERFECT MATCH! Selected data VG: vg=%v vios=%v", targetVgName, targetViosName)
							break
						}
					} else {
						// Explicit match requested
						targetViosUUID = vios.UUID
						targetViosName = vios.PartitionName
						targetVgName = vg.GroupName
						log.Printf("MATCH FOUND! Selected requested VG: vg=%v vios=%v", targetVgName, targetViosName)
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
				log.Fatal("Volume Group either does not exist or has insufficient free space")
			} else {
				log.Fatal("System Exhaustion: Could not find any Volume Group with sufficient free space")
			}
		} else if usingRootVgFallback {
			log.Printf("No data Volume Groups had enough space. Falling back to rootvg: vg=%v vios=%v", targetVgName, targetViosName)
		}

		log.Printf("Executing Virtual Disk creation via VIOS...: disk=%v size_mb=%v vg=%v", diskName, diskSize, targetVgName)

		err = restClient.CreateVirtualDisk(context.Background(), sysName, targetViosUUID, targetViosName, targetVgName, diskName, diskSize, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to create Virtual Disk")
		}

		fmt.Println("\n=========================================================================")
		log.Printf("SUCCESS: Virtual Disk Created!: disk_name=%v vios=%v vg=%v", diskName, targetViosName, targetVgName)
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// EXTEND MODE
	// -------------------------------------------------------------------------
	case "extend":
		log.Printf("Initiating Virtual Disk Extension: disk_name=%v add_size_mb=%v vios=%v", diskName, addSize, viosName)

		log.Printf("Resolving VIOS UUID: vios=%v", viosName)
		viosUUID, err := hmc.GetViosID(context.Background(), restClient, sysUUID, viosName, verbose)
		if err != nil || viosUUID == "" {
			log.Fatal("VIOS not found on system")
		}

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		log.Println("Verifying if Virtual Disk exists...")
		vgList, err := restClient.GetVolumeGroups(context.Background(), viosUUID, verbose)
		if err != nil {
			log.Fatal("Failed to fetch Volume Groups to verify disk existence")
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
			log.Fatal("Virtual Disk does not exist on VIOS. Cannot extend.")
		}

		log.Printf("Extending Virtual Disk: disk=%v vios=%v add_mb=%v", diskName, viosName, addSize)

		err = restClient.ExtendVirtualDisk(context.Background(), sysName, viosUUID, viosName, diskName, addSize, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to extend Virtual Disk")
		}

		fmt.Println("\n=========================================================================")
		log.Printf("SUCCESS: Virtual Disk Extended!: disk_name=%v added_mb=%v", diskName, addSize)
		fmt.Println("=========================================================================")

	// -------------------------------------------------------------------------
	// DELETE MODE
	// -------------------------------------------------------------------------
	case "delete":
		log.Printf("Initiating Virtual Disk Deletion: disk_name=%v vios=%v", diskName, viosName)

		log.Printf("Resolving VIOS UUID: vios=%v", viosName)
		viosUUID, err := hmc.GetViosID(context.Background(), restClient, sysUUID, viosName, verbose)
		if err != nil || viosUUID == "" {
			log.Fatal("VIOS not found on system")
		}

		// --- PRE-FLIGHT EXISTENCE CHECK ---
		log.Println("Verifying if Virtual Disk exists...")
		vgList, err := restClient.GetVolumeGroups(context.Background(), viosUUID, verbose)
		if err != nil {
			log.Fatal("Failed to fetch Volume Groups to verify disk existence")
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
			log.Printf("Virtual Disk not found on VIOS. No action needed.: disk_name=%v vios=%v", diskName, viosName)
			fmt.Println("=========================================================================")
			os.Exit(0)
		}

		log.Printf("Attempting to permanently delete Virtual Disk: disk=%v vios=%v", diskName, viosName)

		err = restClient.DeleteVirtualDisk(context.Background(), sysName, viosName, diskName, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Operation aborted by user (Ctrl+C)")
			}
			log.Fatal("Failed to delete Virtual Disk")
		}

		fmt.Println("\n=========================================================================")
		log.Printf("SUCCESS: Virtual Disk Deleted!: disk_name=%v", diskName)
		fmt.Println("=========================================================================")
	}
}