package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	hmc "github.com/sudeeshjohn/powerhmc-go"
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
	// 2. FLAGS & VALIDATION
	// =========================================================================
	hmcIP := flag.String("hmc-ip", "192.0.2.1", "HMC IP address")
	username := flag.String("hmc-user", "REDACTED_HMC_USER<==", "HMC username")
	password := flag.String("hmc-pass", "REDACTED_HMC_PASS<==", "HMC password")
	sysName := flag.String("system-name", "LTC09U31-ZZ", "Managed System Name (Required)")

	viosName := flag.String("vios-name", "", "Name of the Virtual I/O Server to delete (Required)")
	force := flag.Bool("force", false, "Acknowledge this is a destructive action (Required for safety)")
	verbose := flag.Bool("verbose", false, "Enable verbose XML and HTTP output")

	flag.Parse()

	if *verbose {
		cliLogger.EnableDebug()
	} else {
		cliLogger.SetLevel(0) // InfoLevel
	}

	if *password == "" || *sysName == "" || *viosName == "" {
		fmt.Println("Usage: deletevios -system-name <sys> -vios-name <name> -force")
		cliLogger.Fatal("Missing required arguments.")
	}

	if !*force {
		cliLogger.Fatal("Safety Lock: You must provide the -force flag to confirm VIOS deletion.")
	}

	// =========================================================================
	// 3. AUTHENTICATION & SYSTEM RESOLUTION
	// =========================================================================
	cliLogger.Info("Logging into HMC", "ip", *hmcIP)
	restClient := hmc.NewHmcRestClient(*hmcIP)

	if *verbose {
		restClient.EnableVerboseLogging()
	}

	if err := restClient.Login(ctx, *username, *password, *verbose); err != nil {
		cliLogger.Fatal("HMC Logon failed", "error", err)
	}
	defer func() {
		cliLogger.Info("Closing HMC Session...")
		restClient.Logoff(context.Background())
	}()

	cliLogger.Debug("Resolving System", "system", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(ctx, *sysName, *verbose)
	if err != nil || sysUUID == "" {
		cliLogger.Fatal("Failed to resolve Managed System", "system", *sysName, "error", err)
	}

	// --- PRE-FLIGHT RESOLUTION & IDEMPOTENCY CHECK ---
	cliLogger.Info("Verifying if VIOS exists and checking power state...")
	viosList, err := restClient.GetVirtualIOServersQuick(ctx, sysUUID, *verbose)
	if err != nil {
		cliLogger.Fatal("Failed to fetch VIOS inventory", "error", err)
	}

	var targetViosUUID string
	var targetViosState string

	for _, vios := range viosList {
		if strings.EqualFold(vios.PartitionName, *viosName) {
			targetViosUUID = vios.UUID
			targetViosState = vios.PartitionState
			break
		}
	}

	// Idempotent Exit
	if targetViosUUID == "" {
		fmt.Println("\n=========================================================================")
		cliLogger.Info("✅ Virtual I/O Server not found. No action needed.", "vios_name", *viosName)
		fmt.Println("=========================================================================")
		os.Exit(0)
	}

	// Power State Validation
	if !strings.EqualFold(targetViosState, "Not Activated") && !strings.EqualFold(targetViosState, "not activated") {
		cliLogger.Fatal("VIOS is currently running. You must power it off before deletion.", "vios", *viosName, "state", targetViosState)
	}

	// Check Context immediately before executing heavy operations
	if ctx.Err() != nil {
		cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
	}

	// =========================================================================
	// 4. EXECUTE DELETION
	// =========================================================================
	cliLogger.Warn("Executing permanent Virtual I/O Server deletion...", "vios", *viosName, "uuid", targetViosUUID)

	err = restClient.DeleteVirtualIOServer(ctx, sysUUID, targetViosUUID, *verbose)
	if err != nil {
		if ctx.Err() != nil {
			cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
		}
		cliLogger.Fatal("Failed to delete VIOS", "error", err)
	}

	fmt.Println("\n=========================================================================")
	cliLogger.Info("🗑️  SUCCESS: Virtual I/O Server Deleted!", "vios_name", *viosName)
	fmt.Println("=========================================================================")
}

// Made with Bob
