package main

import (
	"log"
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func printUsage() {
	fmt.Println("Usage: phyioadapters <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  list    List all physical I/O devices on the system (optionally filtered by -lpar-name)")
	fmt.Println("  attach  Map Physical Location Codes or DRC Indexes to an LPAR")
	fmt.Println("  detach  Unmap Physical Location Codes or DRC Indexes from an LPAR")
	fmt.Println("\nUse 'phyioadapters <command> -h' for more information about a command.")
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
	attachCmd := flag.NewFlagSet("attach", flag.ExitOnError)
	detachCmd := flag.NewFlagSet("detach", flag.ExitOnError)

	// Variables
	var hmcIP, username, password, sysName, lparName, targets, lparProfile string
	var verbose, forceSave bool

	// ✨ HELPER 1: Global flags used by ALL commands
	bindCommonFlags := func(fs *flag.FlagSet) {
		fs.StringVar(&hmcIP, "hmc-ip", "", "HMC IP address")
		fs.StringVar(&username, "hmc-user", "", "HMC username")
		fs.StringVar(&password, "hmc-pass", "", "HMC password")
		fs.StringVar(&sysName, "system-name", "", "Managed System Name")
		fs.BoolVar(&verbose, "verbose", false, "Enable verbose HTTP output")
	}

	// ✨ HELPER 2: Mutation flags used by attach/detach
	bindActionFlags := func(fs *flag.FlagSet, action string) {
		bindCommonFlags(fs)
		fs.StringVar(&lparName, "lpar-name", "", "Target LPAR Name (Required)")
		fs.StringVar(&targets, "targets", "", fmt.Sprintf("Comma-separated list of adapters to %s (Required)", action))
		fs.StringVar(&lparProfile, "lpar-profile", "default_profile", "Name of the LPAR profile to overwrite")
		fs.BoolVar(&forceSave, "force-save", true, "Force overwrite of the target profile")
	}

	// --- Bind Flags to Subcommands ---
	bindCommonFlags(listCmd)
	listCmd.StringVar(&lparName, "lpar-name", "", "Target LPAR Name (Optional filter)")

	bindActionFlags(attachCmd, "ATTACH")
	bindActionFlags(detachCmd, "DETACH")

	// Route the Subcommand
	cmd := os.Args[1]
	switch cmd {
	case "list":
		listCmd.Parse(os.Args[2:])
	case "attach":
		attachCmd.Parse(os.Args[2:])
	case "detach":
		detachCmd.Parse(os.Args[2:])
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

	if cmd == "attach" || cmd == "detach" {
		if lparName == "" {
			log.Fatal("Missing required argument")
		}
		if targets == "" {
			log.Fatal("Missing required argument")
		}
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
		log.Fatal("System not found")
	}

	log.Println("Fetching detailed hardware inventory")
	detailedSystem, err := restClient.GetManagedSystem(context.Background(), sysUUID, verbose)
	if err != nil {
		log.Fatal("Failed to fetch detailed system info")
	}

	var lparObj *hmc.LogicalPartitionQuick
	var lparUUID string
	var lparState string
	
	if lparName != "" {
		log.Printf("Resolving LPAR: lpar=%v", lparName)
		var resolvedLparUUID string
		lparObj, resolvedLparUUID, err = restClient.GetLogicalPartitionByName(context.Background(), sysUUID, lparName, verbose)
		if err != nil || resolvedLparUUID == "" {
			log.Fatal("LPAR not found")
		}
		lparUUID = resolvedLparUUID
		lparState = lparObj.PartitionState
	}

	// =========================================================================
	// 4. READ-ONLY: TABULAR LIST MODE
	// =========================================================================
	if cmd == "list" {
		log.Printf("Physical I/O Slot Inventory: system=%v lpar_filter=%v", sysName, lparName)
		fmt.Println("======================================================================================================================")

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "DEVICE NAME (LOC CODE)\tDRC INDEX\tASSIGNABLE\tATTACHED PARTITION\tSTATUS / DESCRIPTION")
		fmt.Fprintln(w, "----------------------\t---------\t----------\t------------------\t--------------------")

		totalDevices, matchedDevices := 0, 0
		for _, bus := range detailedSystem.IOConfig.IOBuses {
			for _, slot := range bus.IOSlots {
				totalDevices++
				adapter := slot.RelatedIOAdapter

				if lparName != "" && !strings.EqualFold(slot.PartitionName, lparName) {
					continue
				}
				matchedDevices++

				assignable := "No"
				if adapter.LogicalPartitionAssignmentCapable {
					assignable = "Yes"
				}

				desc := adapter.Description
				if desc == "" || desc == "Empty slot" {
					desc = slot.Description
				}

				locCode := adapter.DeviceName
				if locCode == "" {
					locCode = slot.PhysicalLocationCode
				}

				drcIndex := slot.ConnectorIndex
				if drcIndex == "" {
					drcIndex = "-"
				}

				attached := slot.PartitionName
				if attached == "" {
					if desc == "Empty slot" {
						attached = "-"
					} else {
						attached = "Unassigned / Hypervisor"
					}
				} else {
					attached = fmt.Sprintf("%s (%d)", slot.PartitionName, slot.PartitionID)
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", locCode, drcIndex, assignable, attached, desc)
			}
		}

		w.Flush()
		fmt.Println("======================================================================================================================")
		
		if lparName != "" {
			log.Printf("Scan Complete: matched=%v total_scanned=%v", matchedDevices, totalDevices)
		} else {
			log.Printf("Scan Complete: total_scanned=%v", totalDevices)
		}
		return // Exit early
	}

	// =========================================================================
	// 5. PRE-FLIGHT VALIDATION & PREPARATION (ATTACH / DETACH)
	// =========================================================================
	log.Println("Pre-Flight Validation & Preparation...")

	// ✨ TRACK ORIGINAL POWER STATE
	wasRunning := false

	if !strings.EqualFold(lparState, "Not Activated") {
		wasRunning = true
		log.Printf("LPAR is currently running. Powering off before continuing: lpar=%v state=%v", lparName, lparState)

		// --- INTERACTIVE COUNTDOWN PROMPT ---
		inputCh := make(chan string)
		go func() {
			reader := bufio.NewReader(os.Stdin)
			text, _ := reader.ReadString('\n')
			inputCh <- strings.TrimSpace(strings.ToLower(text))
		}()

		timeout := 10
		confirmed := false
	promptLoop:
		for i := timeout; i > 0; i-- {
			fmt.Printf("\r⚠️  LPAR '%s' is running. OK to power off? [y/N] (Timeout in %ds): ", lparName, i)
			select {
			case resp := <-inputCh:
				if resp == "y" || resp == "yes" {
					confirmed = true
					fmt.Println() // Add a clean newline after input
					break promptLoop
				}
				fmt.Println()
				log.Fatal("Operation aborted by user.")
			case <-time.After(1 * time.Second):
				// Just tick down and re-draw the line
			}
		}

		if !confirmed {
			fmt.Println()
			log.Fatal("Operation timed out. Aborting.")
		}
		// -------------------------------------

		status, err := restClient.PowerOffPartition(ctx, lparUUID, "Immediate", false, verbose)
		if err != nil {
			log.Fatal("Failed to power off LPAR")
		}
		log.Printf("LPAR powered off successfully: job_status=%v", status)

		time.Sleep(3 * time.Second)
	} else {
		log.Printf("LPAR is confirmed powered off: lpar=%v", lparName)
	}

	// Clean target strings
	var cleanTargets []string
	for _, t := range strings.Split(targets, ",") {
		if strings.TrimSpace(t) != "" {
			cleanTargets = append(cleanTargets, strings.TrimSpace(t))
		}
	}

	var processed []string
	var skipped []string

	// =========================================================================
	// 6. DIRECTORY PREPARATION & "BEFORE" XML DUMP
	// =========================================================================
	log.Println("Preparing LPAR XML Diff Dumps...")
	outDir := "outs"
	os.MkdirAll(outDir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	beforeFile := fmt.Sprintf("%s/lpar_before_phyio_%s.xml", outDir, timestamp)
	afterFile := fmt.Sprintf("%s/lpar_after_phyio_%s.xml", outDir, timestamp)

	if beforeXML, err := restClient.GetRawLparXML(sysUUID, lparUUID, verbose); err == nil {
		os.WriteFile(beforeFile, []byte(beforeXML), 0644)
		log.Printf("Saved BEFORE state: file=%v", beforeFile)
	}

	// =========================================================================
	// 7. EXECUTE THE OPERATION (Context Aware)
	// =========================================================================
	log.Printf("Executing Operation: command=%s targets=%d lpar=%s", cmd, len(cleanTargets), lparName)

	if cmd == "attach" {
		processed, skipped, err = restClient.MapPhysicalIOAdapters(ctx, sysUUID, lparUUID, lparName, cleanTargets, detailedSystem, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Aborted by user (Ctrl+C)")
			}
			log.Fatal("Attachment Failed")
		}
	} else if cmd == "detach" {
		processed, skipped, err = restClient.UnmapPhysicalIOAdapters(ctx, sysUUID, lparUUID, lparName, cleanTargets, detailedSystem, verbose)
		if err != nil {
			if ctx.Err() != nil {
				log.Fatal("Aborted by user (Ctrl+C)")
			}
			log.Fatal("Detachment Failed")
		}
	}

	// =========================================================================
	// 8. SAVE LPAR PROFILE (PERSISTENCE)
	// =========================================================================
	if len(processed) > 0 {
		log.Printf("Saving active configuration to LPAR profile: profile=%v", lparProfile)
		saveErr := restClient.SaveCurrentLparConfig(context.Background(), lparUUID, lparProfile, forceSave, verbose)
		if saveErr != nil {
			log.Printf("Physical adapters modified dynamically, but failed to save LPAR profile: error=%v", saveErr)
		} else {
			log.Println("LPAR profile saved successfully. Changes will persist across reboots.")
		}
	} else {
		log.Println("No architectural changes were made to the LPAR. Profile save skipped.")
	}

	// =========================================================================
	// 9. LOG RESULTS & FINAL XML DUMP
	// =========================================================================
	fmt.Println("\n=========================================================================")
	log.Println("OPERATION RESULTS")

	for _, s := range skipped {
		reason := "Already attached to LPAR"
		if cmd == "detach" {
			reason = "Not attached to LPAR"
		}
		log.Printf("Skipped adapter: adapter=%v reason=%v", s, reason)
	}
	
	for _, p := range processed {
		log.Printf("Successfully processed adapter: adapter=%v action=%v", p, cmd)
	}

	if len(processed) > 0 {
		log.Println("Fetching AFTER state for Diff Tool...")
		if afterXML, err := restClient.GetRawLparXML(sysUUID, lparUUID, verbose); err == nil {
			os.WriteFile(afterFile, []byte(afterXML), 0644)
			diffCmd := fmt.Sprintf("code --diff %s %s", beforeFile, afterFile)
			log.Printf("Diff ready: command=%v", diffCmd)
		}
	}
	fmt.Println("=========================================================================")

	// =========================================================================
	// 10. RESTORE POWER STATE (If needed)
	// =========================================================================
	if wasRunning {
		fmt.Println()
		log.Printf("Restoring original LPAR power state...: lpar=%v", lparName)

		// Get LPAR detailed info to accurately extract the saved profile UUID
		lparDetailed, err := restClient.GetLogicalPartitionDetailed(context.Background(), lparUUID, verbose)
		if err != nil {
			log.Fatal("Failed to retrieve LPAR details for Power On")
		}

		profileHref := lparDetailed.AssociatedPartitionProfile.Href
		var profileUUID string
		if len(profileHref) >= 36 {
			profileUUID = profileHref[len(profileHref)-36:]
		}

		// Configure PowerOn options (inheriting OS Type from initial resolution)
		options := &hmc.PowerOnOptions{
			ProfileUUID: profileUUID,
			Keylock:     "normal",
			OSType:      lparObj.OperatingSystemType, 
		}

		status, err := restClient.PowerOnPartition(ctx, lparUUID, options, verbose)
		if err != nil {
			log.Fatal("Failed to power on LPAR")
		}
		
		log.Printf("LPAR powered on successfully: job_status=%v", status)
		fmt.Println("=========================================================================")
	}
}