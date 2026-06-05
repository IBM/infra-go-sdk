package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	hmc "github.ibm.com/sudeeshjohn/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// 1. SET UP CONTEXT & GRACEFUL CANCELLATION (Ctrl+C)
	// =========================================================================
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cliLogger := hmc.NewDefaultLogger()
	cliLogger.SetPrefix("[CLI]")

	// =========================================================================
	// 2. CONFIGURATION & FLAGS
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "", "HMC IP address")
	username := flag.String("hmc-user", "", "HMC username")
	password := flag.String("hmc-pass", "", "HMC password")
	sysName := flag.String("system-name", "", "Managed System Name")
	viosName := flag.String("vios-name", "", "Target VIOS Name")
	
	// Update specific flags
	resourceType := flag.String("type", "NFS", "Update source: NFS, SFTP, USB, IBMWebsite, or HMC")
	updateName := flag.String("update-name", "VIOS_UPDATE", "A friendly name for this update operation")
	serverIP := flag.String("server-ip", "", "IP of the NFS or SFTP server")
	remoteDir := flag.String("remote-dir", ".", "Directory path relative to the mount location")
	mountLoc := flag.String("mount-loc", "", "NFS Export path on the remote server (e.g., /exports/aix/vios_update)")
	mountOpts := flag.String("mount-opts", "vers=4", "Mount options (NFS only)")
	
	// Maintenance and Restart Safety Flags
	forcePrepare := flag.Bool("force-prepare", false, "Proceed with update even if redundancy validation fails")
	restart := flag.Bool("restart", false, "Automatically restart the VIOS after a successful update")
	verbose := flag.Bool("verbose", false, "Enable verbose output")
	flag.Parse()

	if *password == "" || *sysName == "" || *viosName == "" {
		cliLogger.Fatal("Error: hmc-pass, system-name, and vios-name are required.")
	}

	if *verbose {
		cliLogger.EnableDebug()
	} else {
		cliLogger.SetLevel(0) // InfoLevel
	}

	fmt.Println("=========================================================================")
	fmt.Printf(" 🚀 Initiating VIOS Maintenance & Update (%s)\n", *resourceType)
	fmt.Printf("    - Target VIOS : %s\n", *viosName)
	fmt.Printf("    - Remote Host : %s\n", *serverIP)
	fmt.Printf("    - Auto-Restart: %t\n", *restart)
	fmt.Println("=========================================================================")

	// =========================================================================
	// 3. AUTHENTICATION & RESOLUTION
	// =========================================================================
	cliLogger.Info("Logging into HMC", "ip", *hmcIP)
	restClient := hmc.NewRestClient(*hmcIP)
	
	if *verbose {
		restClient.EnableVerboseLogging()
	}

	if err := restClient.Login(ctx, *username, *password, *verbose); err != nil {
		cliLogger.Fatal("HMC Logon failed", "error", err)
	}
	defer restClient.Logoff(context.Background())

	cliLogger.Debug("Resolving System", "system", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(ctx, *sysName, *verbose)
	if err != nil || sysUUID == "" {
		cliLogger.Fatal("Failed to resolve Managed System", "system", *sysName, "error", err)
	}

	cliLogger.Debug("Resolving VIOS", "vios", *viosName)
	viosUUID, err := hmc.GetViosID(ctx, restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		cliLogger.Fatal("VIOS not found on system", "vios", *viosName, "system", *sysName)
	}

	// =========================================================================
	// 4. PRE-FLIGHT HMC REPOSITORY CLEANUP
	// =========================================================================
	cliLogger.Info("Verifying HMC internal repository capacity...")
	
	cachedUpdates, err := restClient.ListVIOSHMCUpdates(ctx, *verbose)
	if err != nil {
		cliLogger.Fatal("Failed to list cached VIOS updates on HMC", "error", err)
	}

	if len(cachedUpdates) >= 3 {
		oldestImage := cachedUpdates[0] // Safely grab the first one in the list
		cliLogger.Warn(fmt.Sprintf("HMC repository is full (%d/3 limit). Evicting oldest image: '%s'", len(cachedUpdates), oldestImage))
		
		err = restClient.DeleteVIOSHMCUpdate(ctx, oldestImage, *verbose)
		if err != nil {
			cliLogger.Fatal("Failed to delete stale update image", "error", err)
		}
		cliLogger.Info("✅ Oldest image evicted successfully. Space secured for new update.")
	} else {
		cliLogger.Info(fmt.Sprintf("✅ HMC repository has available space (%d/3 used).", len(cachedUpdates)))
	}

	// =========================================================================
	// 5. PREPARE FOR MAINTENANCE (FAILOVER)
	// =========================================================================
	cliLogger.Info("Validating redundancy and preparing VIOS for maintenance...")
	
	report, err := restClient.PrepareVIOSMaintenance(ctx, viosUUID, *forcePrepare, *verbose)
	if err != nil {
		if ctx.Err() != nil {
			cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
		}
		cliLogger.Fatal("Failed to prepare VIOS for maintenance", "error", err)
	}

	// Evaluate the redundancy report
	hasFailures := false
	if len(report.VirtualSCSIValidationResults.Failure) > 0 {
		cliLogger.Warn("VSCSI Redundancy Failures Detected", "messages", report.VirtualSCSIValidationResults.Failure)
		hasFailures = true
	}
	if len(report.VirtualFCValidationResults.Failure) > 0 {
		cliLogger.Warn("NPIV (vFC) Redundancy Failures Detected", "messages", report.VirtualFCValidationResults.Failure)
		hasFailures = true
	}
	if len(report.VirtualLANValidationResults.Failure) > 0 {
		cliLogger.Warn("Virtual LAN Redundancy Failures Detected", "messages", report.VirtualLANValidationResults.Failure)
		hasFailures = true
	}
	if len(report.VirtualNICValidationResults.Failure) > 0 {
		cliLogger.Warn("vNIC Redundancy Failures Detected", "messages", report.VirtualNICValidationResults.Failure)
		hasFailures = true
	}

	// Halt if it's unsafe, unless the user explicitly bypassed safety checks
	if hasFailures && !*forcePrepare {
		cliLogger.Fatal("ABORTING UPDATE: Redundancy validation failed. Client LPARs would lose I/O paths! Use -force-prepare=true to override.")
	} else if hasFailures && *forcePrepare {
		cliLogger.Warn("Redundancy validation failed, but -force-prepare is true. PROCEEDING WITH UPDATE!")
	} else {
		cliLogger.Info("✅ Redundancy validation passed. VIOS I/O has been safely unconfigured/failed-over.")
	}

	// =========================================================================
	// 6. EXECUTE VIOS UPDATE
	// =========================================================================
	cliLogger.Info("Configuring Update Options...")

	opts := hmc.UpdateVIOSOptions{
		ResourceType:    *resourceType,
		Name:            *updateName,
		ServerHostOrIP:  *serverIP,
		RemoteDirectory: *remoteDir,
		MountLocation:   *mountLoc,
		MountOptions:    *mountOpts,
		SaveFile:        true,
		RestartVIOS:     *restart,
	}

	cliLogger.Warn("Triggering update. This will take a significant amount of time (10-45 minutes)...")
	
	output, err := restClient.UpdateVIOS(ctx, viosUUID, opts, *verbose)
	if err != nil {
		if ctx.Err() != nil {
			cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
		}
		cliLogger.Fatal("VIOS Update Failed", "error", err)
	}

	// =========================================================================
	// 7. DISPLAY RESULTS
	// =========================================================================
	fmt.Println("\n=========================================================================")
	cliLogger.Info("✅ VIOS Update Process Completed Successfully!")
	fmt.Println("=========================================================================")
	fmt.Println("\n--- VIOS install.log Output ---")
	fmt.Println(output)
	fmt.Println("-------------------------------")

	if *restart {
		cliLogger.Info("The VIOS is now restarting to apply the updates.")
	} else {
		cliLogger.Warn("RestartVIOS was set to false. You must manually reboot the VIOS to apply the fixpack!")
	}
}