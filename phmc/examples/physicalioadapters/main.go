package main

import (
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

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Adjust to your actual package path
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
	if password == "" || sysName == "" {
		cliLogger.Fatal("Missing required arguments", "required", "hmc-pass, system-name")
	}

	if cmd == "attach" || cmd == "detach" {
		if lparName == "" {
			cliLogger.Fatal("Missing required argument", "required", "lpar-name")
		}
		if targets == "" {
			cliLogger.Fatal("Missing required argument", "required", "targets (comma-separated list)")
		}
	}

	// =========================================================================
	// 3. AUTHENTICATION & RESOLUTION
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
		cliLogger.Fatal("System not found", "system", sysName, "error", err)
	}

	cliLogger.Debug("Fetching detailed hardware inventory")
	detailedSystem, err := restClient.GetManagedSystem(context.Background(), sysUUID, verbose)
	if err != nil {
		cliLogger.Fatal("Failed to fetch detailed system info", "error", err)
	}

	var lparObj *hmc.LogicalPartitionQuick
	var lparUUID string
	var lparState string
	
	if lparName != "" {
		cliLogger.Debug("Resolving LPAR", "lpar", lparName)
		var resolvedLparUUID string
		lparObj, resolvedLparUUID, err = restClient.GetLogicalPartitionByName(context.Background(), sysUUID, lparName, verbose)
		if err != nil || resolvedLparUUID == "" {
			cliLogger.Fatal("LPAR not found", "lpar", lparName)
		}
		lparUUID = resolvedLparUUID
		lparState = lparObj.PartitionState
	}

	// =========================================================================
	// 4. READ-ONLY: TABULAR LIST MODE
	// =========================================================================
	if cmd == "list" {
		cliLogger.Info("Physical I/O Slot Inventory", "system", sysName, "lpar_filter", lparName)
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
			cliLogger.Info("Scan Complete", "matched", matchedDevices, "total_scanned", totalDevices)
		} else {
			cliLogger.Info("Scan Complete", "total_scanned", totalDevices)
		}
		return // Exit early
	}

	// =========================================================================
	// 5. PRE-FLIGHT VALIDATION & PREPARATION (ATTACH / DETACH)
	// =========================================================================
	cliLogger.Info("Pre-Flight Validation & Preparation...")

	// ✨ TRACK ORIGINAL POWER STATE
	wasRunning := false

	if !strings.EqualFold(lparState, "Not Activated") {
		wasRunning = true
		cliLogger.Warn("LPAR is currently running. Powering off before continuing", "lpar", lparName, "state", lparState)

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
				cliLogger.Fatal("Operation aborted by user.")
			case <-time.After(1 * time.Second):
				// Just tick down and re-draw the line
			}
		}

		if !confirmed {
			fmt.Println()
			cliLogger.Fatal("Operation timed out. Aborting.")
		}
		// -------------------------------------

		status, err := restClient.PowerOffPartition(ctx, lparUUID, "Immediate", false, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to power off LPAR", "error", err)
		}
		cliLogger.Info("LPAR powered off successfully", "job_status", status)

		time.Sleep(3 * time.Second)
	} else {
		cliLogger.Info("LPAR is confirmed powered off", "lpar", lparName)
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
	cliLogger.Info("Preparing LPAR XML Diff Dumps...")
	outDir := "outs"
	os.MkdirAll(outDir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	beforeFile := fmt.Sprintf("%s/lpar_before_phyio_%s.xml", outDir, timestamp)
	afterFile := fmt.Sprintf("%s/lpar_after_phyio_%s.xml", outDir, timestamp)

	if beforeXML, err := restClient.GetRawLparXML(sysUUID, lparUUID, verbose); err == nil {
		os.WriteFile(beforeFile, []byte(beforeXML), 0644)
		cliLogger.Debug("Saved BEFORE state", "file", beforeFile)
	}

	// =========================================================================
	// 7. EXECUTE THE OPERATION (Context Aware)
	// =========================================================================
	cliLogger.Info("Executing Operation", "command", cmd, "targets", len(cleanTargets), "lpar", lparName)

	if cmd == "attach" {
		processed, skipped, err = restClient.MapPhysicalIOAdapters(ctx, sysUUID, lparUUID, lparName, cleanTargets, detailedSystem, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Attachment Failed", "error", err)
		}
	} else if cmd == "detach" {
		processed, skipped, err = restClient.UnmapPhysicalIOAdapters(ctx, sysUUID, lparUUID, lparName, cleanTargets, detailedSystem, verbose)
		if err != nil {
			if ctx.Err() != nil {
				cliLogger.Fatal("Aborted by user (Ctrl+C)")
			}
			cliLogger.Fatal("Detachment Failed", "error", err)
		}
	}

	// =========================================================================
	// 8. SAVE LPAR PROFILE (PERSISTENCE)
	// =========================================================================
	if len(processed) > 0 {
		cliLogger.Info("Saving active configuration to LPAR profile", "profile", lparProfile)
		saveErr := restClient.SaveCurrentLparConfig(context.Background(), lparUUID, lparProfile, forceSave, verbose)
		if saveErr != nil {
			cliLogger.Warn("Physical adapters modified dynamically, but failed to save LPAR profile", "error", saveErr)
		} else {
			cliLogger.Info("LPAR profile saved successfully. Changes will persist across reboots.")
		}
	} else {
		cliLogger.Info("No architectural changes were made to the LPAR. Profile save skipped.")
	}

	// =========================================================================
	// 9. LOG RESULTS & FINAL XML DUMP
	// =========================================================================
	fmt.Println("\n=========================================================================")
	cliLogger.Info("OPERATION RESULTS")

	for _, s := range skipped {
		reason := "Already attached to LPAR"
		if cmd == "detach" {
			reason = "Not attached to LPAR"
		}
		cliLogger.Warn("Skipped adapter", "adapter", s, "reason", reason)
	}
	
	for _, p := range processed {
		cliLogger.Info("Successfully processed adapter", "adapter", p, "action", cmd)
	}

	if len(processed) > 0 {
		cliLogger.Info("Fetching AFTER state for Diff Tool...")
		if afterXML, err := restClient.GetRawLparXML(sysUUID, lparUUID, verbose); err == nil {
			os.WriteFile(afterFile, []byte(afterXML), 0644)
			diffCmd := fmt.Sprintf("code --diff %s %s", beforeFile, afterFile)
			cliLogger.Info("Diff ready", "command", diffCmd)
		}
	}
	fmt.Println("=========================================================================")

	// =========================================================================
	// 10. RESTORE POWER STATE (If needed)
	// =========================================================================
	if wasRunning {
		fmt.Println()
		cliLogger.Info("Restoring original LPAR power state...", "lpar", lparName)

		// Get LPAR detailed info to accurately extract the saved profile UUID
		lparDetailed, err := restClient.GetLogicalPartitionDetailed(context.Background(), lparUUID, verbose)
		if err != nil {
			cliLogger.Fatal("Failed to retrieve LPAR details for Power On", "error", err)
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
			cliLogger.Fatal("Failed to power on LPAR", "error", err)
		}
		
		cliLogger.Info("LPAR powered on successfully", "job_status", status)
		fmt.Println("=========================================================================")
	}
}