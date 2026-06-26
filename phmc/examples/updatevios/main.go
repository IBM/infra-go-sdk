package main

import (
	"log"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	hmc "github.com/IBM/infra-go-sdk/phmc" // Adjust to your actual package path
)

func main() {
	// =========================================================================
	// 1. SET UP CONTEXT & GRACEFUL CANCELLATION (Ctrl+C)
	// =========================================================================
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()


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
	_ = verbose

	if *password == "" || *sysName == "" || *viosName == "" {
		log.Fatal("Error: hmc-pass, system-name, and vios-name are required.")
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
	log.Printf("Logging into HMC: ip=%v", *hmcIP)
	restClient := hmc.NewRestClient(*hmcIP)
	

	if err := restClient.Login(ctx, *username, *password, *verbose); err != nil {
		log.Fatal("HMC Logon failed")
	}
	defer restClient.Logoff(context.Background())

	log.Printf("Resolving System: system=%v", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(ctx, *sysName, *verbose)
	if err != nil || sysUUID == "" {
		log.Fatal("Failed to resolve Managed System")
	}

	log.Printf("Resolving VIOS: vios=%v", *viosName)
	viosUUID, err := hmc.GetViosID(ctx, restClient, sysUUID, *viosName, *verbose)
	if err != nil || viosUUID == "" {
		log.Fatal("VIOS not found on system")
	}

	// =========================================================================
	// 4. PRE-FLIGHT HMC REPOSITORY CLEANUP
	// =========================================================================
	log.Println("Verifying HMC internal repository capacity...")
	
	cachedUpdates, err := restClient.ListVIOSHMCUpdates(ctx, *verbose)
	if err != nil {
		log.Fatal("Failed to list cached VIOS updates on HMC")
	}

	if len(cachedUpdates) >= 3 {
		oldestImage := cachedUpdates[0] // Safely grab the first one in the list
		log.Printf("HMC repository is full (%d/3 limit). Evicting oldest: %s", len(cachedUpdates), oldestImage)
		
		err = restClient.DeleteVIOSHMCUpdate(ctx, oldestImage, *verbose)
		if err != nil {
			log.Fatal("Failed to delete stale update image")
		}
		log.Println("✅ Oldest image evicted successfully. Space secured for new update.")
	} else {
		log.Printf("HMC repository has available space (%d/3 used).", len(cachedUpdates))
	}

	// =========================================================================
	// 5. PREPARE FOR MAINTENANCE (FAILOVER)
	// =========================================================================
	log.Println("Validating redundancy and preparing VIOS for maintenance...")
	
	report, err := restClient.PrepareVIOSMaintenance(ctx, viosUUID, *forcePrepare, *verbose)
	if err != nil {
		if ctx.Err() != nil {
			log.Fatal("Operation aborted by user (Ctrl+C)")
		}
		log.Fatal("Failed to prepare VIOS for maintenance")
	}

	// Evaluate the redundancy report
	hasFailures := false
	if len(report.VirtualSCSIValidationResults.Failure) > 0 {
		log.Printf("VSCSI Redundancy Failures Detected: messages=%v", report.VirtualSCSIValidationResults.Failure)
		hasFailures = true
	}
	if len(report.VirtualFCValidationResults.Failure) > 0 {
		log.Println("NPIV (vFC")
		hasFailures = true
	}
	if len(report.VirtualLANValidationResults.Failure) > 0 {
		log.Printf("Virtual LAN Redundancy Failures Detected: messages=%v", report.VirtualLANValidationResults.Failure)
		hasFailures = true
	}
	if len(report.VirtualNICValidationResults.Failure) > 0 {
		log.Printf("vNIC Redundancy Failures Detected: messages=%v", report.VirtualNICValidationResults.Failure)
		hasFailures = true
	}

	// Halt if it's unsafe, unless the user explicitly bypassed safety checks
	if hasFailures && !*forcePrepare {
		log.Fatal("ABORTING UPDATE: Redundancy validation failed. Client LPARs would lose I/O paths! Use -force-prepare=true to override.")
	} else if hasFailures && *forcePrepare {
		log.Println("[WARN] Redundancy validation failed, but -force-prepare is true. PROCEEDING WITH UPDATE!")
	} else {
		log.Println("✅ Redundancy validation passed. VIOS I/O has been safely unconfigured/failed-over.")
	}

	// =========================================================================
	// 6. EXECUTE VIOS UPDATE
	// =========================================================================
	log.Println("Configuring Update Options...")

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

	log.Println("Triggering update. This will take a significant amount of time (10-45 minutes")
	
	output, err := restClient.UpdateVIOS(ctx, viosUUID, opts, *verbose)
	if err != nil {
		if ctx.Err() != nil {
			log.Fatal("Operation aborted by user (Ctrl+C)")
		}
		log.Fatal("VIOS Update Failed")
	}

	// =========================================================================
	// 7. DISPLAY RESULTS
	// =========================================================================
	fmt.Println("\n=========================================================================")
	log.Println("✅ VIOS Update Process Completed Successfully!")
	fmt.Println("=========================================================================")
	fmt.Println("\n--- VIOS install.log Output ---")
	fmt.Println(output)
	fmt.Println("-------------------------------")

	if *restart {
		log.Println("The VIOS is now restarting to apply the updates.")
	} else {
		log.Println("[WARN] RestartVIOS was set to false. You must manually reboot the VIOS to apply the fixpack!")
	}
}