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
	
	viosName := flag.String("vios-name", "", "Name of the new Virtual I/O Server (Required)")
	dedicatedProc := flag.Bool("dedicated-proc", false, "Use dedicated processors instead of shared")
	
	// Typical VIOS Sizing Defaults
	desMem := flag.Int("mem", 8192, "Desired Memory in MB (Default: 8192)")
	desCpus := flag.Float64("cpus", 1.0, "Desired Processing Units (Default: 1.0)")
	desVcpus := flag.Int("vcpus", 2, "Desired Virtual Processors (Default: 2)")

	maxVirtualSlots := flag.Int("max-slots", 500, "Maximum Virtual I/O Slots")
	sharingMode := flag.String("sharing-mode", "uncapped", "Sharing Mode: capped, uncapped, share idle procs")
	uncappedWeight := flag.Int("weight", 128, "Uncapped Weight (0-255)")

	verbose := flag.Bool("verbose", false, "Enable verbose XML and HTTP output")
	flag.Parse()

	if *verbose {
		cliLogger.EnableDebug()
	} else {
		cliLogger.SetLevel(0) // InfoLevel
	}

	if *password == "" || *sysName == "" || *viosName == "" {
		fmt.Println("Usage: createvios -system-name <sys> -vios-name <name>")
		cliLogger.Fatal("Missing required arguments.")
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
		restClient.Logoff(ctx)
	}()

	cliLogger.Debug("Resolving System", "system", *sysName)
	_, sysUUID, err := restClient.GetManagedSystemByNameQuick(ctx, *sysName, *verbose)
	if err != nil || sysUUID == "" {
		cliLogger.Fatal("Failed to resolve Managed System", "system", *sysName, "error", err)
	}

	// --- PRE-FLIGHT EXISTENCE CHECK ---
	cliLogger.Info("Verifying if VIOS already exists...")
	viosList, err := restClient.GetVirtualIOServersQuick(ctx, sysUUID, *verbose)
	if err == nil {
		for _, vios := range viosList {
			if strings.EqualFold(vios.PartitionName, *viosName) {
				fmt.Println("\n=========================================================================")
				cliLogger.Info("✅ Virtual I/O Server already exists. Skipping creation.", "vios_name", *viosName, "uuid", vios.UUID)
				fmt.Println("=========================================================================")
				os.Exit(0) // Idempotent Exit
			}
		}
	}

	// Check Context immediately before executing heavy operations
	if ctx.Err() != nil {
		cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
	}

	// =========================================================================
	// 4. BUILD REQUEST & EXECUTE
	// =========================================================================
	req := hmc.CreateViosRequest{
		Name:             *viosName,
		MinMem:           4096,          // 4GB Minimum Base
		DesiredMem:       *desMem,
		MaxMem:           *desMem * 4,   // 4x Scale
		MinProcUnits:     0.1,
		DesiredProcUnits: *desCpus,
		MaxProcUnits:     *desCpus * 4,
		MinVcpus:         1,
		DesiredVcpus:     *desVcpus,
		MaxVcpus:         *desVcpus * 4,
		SharingMode:      *sharingMode,
		UncappedWeight:   *uncappedWeight,
		MaxVirtualSlots:  *maxVirtualSlots,
		DedicatedProc:    *dedicatedProc,
	}

	cliLogger.Info("Provisioning Virtual I/O Server...", "vios", *viosName, "mem_mb", req.DesiredMem, "cpus", req.DesiredProcUnits)

	newUUID, err := restClient.CreateVirtualIOServer(ctx, sysUUID, req, *verbose)
	if err != nil {
		if ctx.Err() != nil {
			cliLogger.Fatal("Operation aborted by user (Ctrl+C)")
		}
		cliLogger.Fatal("Failed to provision VIOS", "error", err)
	}

	fmt.Println("\n=========================================================================")
	cliLogger.Info("SUCCESS: Virtual I/O Server Created!", "vios_name", *viosName, "uuid", newUUID)
	fmt.Println("=========================================================================")
}

// Made with Bob
